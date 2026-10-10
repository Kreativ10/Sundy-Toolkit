//go:build integration

package minecraft

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SundySystems/sundy-toolkit/internal/apps"
)

// Opt-in: downloads an official server and creates a temporary local world.
func TestRealVanillaLifecycle(t *testing.T) {
	version := os.Getenv("SUNDY_MC_E2E_VERSION")
	if version == "" {
		t.Skip("set SUNDY_MC_E2E_VERSION and use -tags=integration to run a disposable real server")
	}
	meta, err := resolveVanilla(&httpClient, version)
	if err != nil {
		t.Fatal(err)
	}
	java, err := ensureJava("java", meta.JavaVersion.MajorVersion)
	if err != nil {
		t.Fatal(err)
	}
	dir, run, logs := t.TempDir(), t.TempDir(), t.TempDir()
	jar := filepath.Join(dir, "server.jar")
	if err := downloadServer(&httpClient, meta, jar); err != nil {
		t.Fatal(err)
	}
	portListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := portListener.Addr().(*net.TCPAddr).Port
	portListener.Close()
	// EULA is accepted only for this opt-in disposable test fixture.
	os.WriteFile(filepath.Join(dir, "eula.txt"), []byte("eula=true\n"), 0644)
	properties := fmt.Sprintf("server-ip=127.0.0.1\nserver-port=%d\nonline-mode=false\nview-distance=2\nsimulation-distance=2\nlevel-type=minecraft:flat\ngenerate-structures=false\n", port)
	os.WriteFile(filepath.Join(dir, "server.properties"), []byte(properties), 0644)
	app := apps.App{Name: "vanilla-e2e", Kind: "minecraft", Directory: dir, Metadata: map[string]string{"java": java, "jar": jar, "memory": "2G"}}
	for iteration := 0; iteration < 2; iteration++ {
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- runSupervisor(ctx, app, run, logs, 90*time.Second) }()
		stopped := false
		cleanup := func() {
			if !stopped {
				cancel()
				waitSupervisor(t, done, true)
				stopped = true
			}
		}
		t.Cleanup(cleanup)
		deadline := time.Now().Add(2 * time.Minute)
		ready := false
		for time.Now().Before(deadline) {
			select {
			case err := <-done:
				stopped = true
				data, _ := os.ReadFile(filepath.Join(logs, "console.log"))
				t.Fatalf("server exited before readiness: %v\n%s", err, data)
			default:
			}
			if err := queryServerStatus(port); err == nil {
				ready = true
				break
			}
			time.Sleep(250 * time.Millisecond)
		}
		if !ready {
			cleanup()
			data, _ := os.ReadFile(filepath.Join(logs, "console.log"))
			t.Fatalf("server did not answer Minecraft status: %s", data)
		}
		conn := connectConsole(t, filepath.Join(run, app.Name+".sock"))
		fmt.Fprintln(conn, "list")
		scanner := bufio.NewScanner(conn)
		answered := false
		for scanner.Scan() {
			if strings.Contains(scanner.Text(), "players online") {
				answered = true
				break
			}
		}
		if !answered {
			conn.Close()
			cleanup()
			t.Fatal("console list command did not receive a response")
		}
		fmt.Fprintln(conn, ":detach")
		conn.Close()
		if err := queryServerStatus(port); err != nil {
			cleanup()
			t.Fatalf("detach stopped server: %v", err)
		}
		cancel()
		select {
		case err := <-done:
			stopped = true
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(100 * time.Second):
			t.Fatal("real server did not stop")
		}
		if _, err := os.Stat(filepath.Join(dir, "world", "level.dat")); err != nil {
			t.Fatalf("world was not saved: %v", err)
		}
		t.Logf("Minecraft %s, Java %d+, lifecycle %d: TCP status, console command, detach, graceful stop and world save passed", meta.ID, meta.JavaVersion.MajorVersion, iteration+1)
	}
}

func appendVarInt(dst []byte, n int) []byte {
	for {
		value := byte(n & 127)
		n >>= 7
		if n != 0 {
			value |= 128
		}
		dst = append(dst, value)
		if n == 0 {
			return dst
		}
	}
}

func queryServerStatus(port int) error {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(time.Second))
	host := "127.0.0.1"
	handshake := []byte{0}
	handshake = appendVarInt(handshake, 0)
	handshake = appendVarInt(handshake, len(host))
	handshake = append(handshake, host...)
	handshake = binary.BigEndian.AppendUint16(handshake, uint16(port))
	handshake = append(handshake, 1)
	data := append(appendVarInt(nil, len(handshake)), handshake...)
	data = append(data, 1, 0)
	if _, err := conn.Write(data); err != nil {
		return err
	}
	r := bufio.NewReader(conn)
	readVar := func() (int, error) {
		v := 0
		for i := 0; i < 5; i++ {
			b, err := r.ReadByte()
			if err != nil {
				return 0, err
			}
			v |= int(b&127) << (7 * i)
			if b&128 == 0 {
				return v, nil
			}
		}
		return 0, fmt.Errorf("invalid varint")
	}
	length, err := readVar()
	if err != nil {
		return err
	}
	if length < 2 || length > 1<<20 {
		return fmt.Errorf("invalid packet length")
	}
	id, err := readVar()
	if err != nil || id != 0 {
		return fmt.Errorf("unexpected packet")
	}
	size, err := readVar()
	if err != nil || size > length || size < 0 {
		return fmt.Errorf("invalid status size")
	}
	payload := make([]byte, size)
	if _, err := io.ReadFull(r, payload); err != nil {
		return err
	}
	var status struct {
		Version struct{ Name string }
		Players struct{ Max int }
	}
	if err := json.NewDecoder(bytes.NewReader(payload)).Decode(&status); err != nil {
		return err
	}
	if status.Version.Name == "" || status.Players.Max == 0 {
		return fmt.Errorf("server not ready")
	}
	return nil
}
