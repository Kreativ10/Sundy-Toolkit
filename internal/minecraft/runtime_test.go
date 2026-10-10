package minecraft

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/SundySystems/sundy-toolkit/internal/apps"
)

func testApp(t *testing.T, script string) apps.App {
	t.Helper()
	dir := t.TempDir()
	java := filepath.Join(dir, "java")
	if err := os.WriteFile(java, []byte("#!/bin/sh\n"+script), 0755); err != nil {
		t.Fatal(err)
	}
	return apps.App{Name: "test", Kind: "minecraft", Directory: dir, Metadata: map[string]string{"java": java, "jar": "server.jar", "memory": "256M"}}
}

func connectConsole(t *testing.T, socket string) net.Conn {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.Dial("unix", socket)
		if err == nil {
			conn.SetDeadline(time.Now().Add(5 * time.Second))
			return conn
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("console did not become available")
	return nil
}

func waitSupervisor(t *testing.T, done <-chan error, success bool) {
	t.Helper()
	select {
	case err := <-done:
		if (err == nil) != success {
			t.Fatalf("supervisor returned %v, success wanted %v", err, success)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("supervisor did not exit")
	}
}

func TestSupervisorConsoleDetachStopAndLock(t *testing.T) {
	a := testApp(t, `while IFS= read -r line; do
printf 'reply:%s\n' "$line"
[ "$line" = stop ] && { printf 'world saved\n'; exit 0; }
done
`)
	run, logs := t.TempDir(), t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- runSupervisor(ctx, a, run, logs, time.Second) }()
	conn := connectConsole(t, filepath.Join(run, "test.sock"))
	defer conn.Close()
	fmt.Fprintln(conn, "list")
	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil || line != "reply:list\n" {
		t.Fatalf("console: %q %v", line, err)
	}
	if err := runSupervisor(ctx, a, run, logs, time.Second); err == nil {
		t.Fatal("duplicate supervisor was allowed")
	}
	fmt.Fprintln(conn, ":detach")
	if _, err := reader.ReadByte(); err == nil {
		t.Fatal("detach did not close console")
	}
	select {
	case err := <-done:
		t.Fatalf("detach stopped server: %v", err)
	default:
	}
	conn2 := connectConsole(t, filepath.Join(run, "test.sock"))
	defer conn2.Close()
	cancel()
	waitSupervisor(t, done, true)
	data, err := os.ReadFile(filepath.Join(logs, "console.log"))
	if err != nil || !strings.Contains(string(data), "world saved\n") {
		t.Fatalf("missing final log: %s %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(run, "test.sock")); !os.IsNotExist(err) {
		t.Fatalf("stale socket: %v", err)
	}
}

func TestMultipleInstancesKeepIndependentConsoles(t *testing.T) {
	state, run := t.TempDir(), t.TempDir()
	t.Setenv("SUNDY_STATE_DIR", state)
	t.Setenv("SUNDY_RUNTIME_DIR", run)
	type instance struct {
		app  apps.App
		done chan error
	}
	var instances []instance
	for i, name := range []string{"first", "second"} {
		a := testApp(t, "while IFS= read -r line; do printf '"+name+":%s\\n' \"$line\"; [ \"$line\" = stop ] && exit 0; done\n")
		a.ID, a.Name, a.Port = "minecraft-"+name, name, 25565+i
		if err := apps.Save(a); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan error, 1)
		go func() {
			registered, err := minecraftApp(a.Name)
			if err != nil {
				done <- err
				return
			}
			done <- runSupervisor(ctx, registered, runtimeDir(), filepath.Join(state, "minecraft", a.Name), time.Second)
		}()
		instances = append(instances, instance{a, done})
	}
	list, err := apps.List()
	if err != nil || len(list) != 2 {
		t.Fatalf("second registration lost an instance: %v %v", list, err)
	}
	connections := make([]net.Conn, len(instances))
	for i, in := range instances {
		conn := connectConsole(t, filepath.Join(run, in.app.Name+".sock"))
		connections[i] = conn
		defer conn.Close()
	}
	// Both consoles are connected before either server stops. The second must
	// still answer commands after stopping the first.
	for i, in := range instances {
		conn := connections[i]
		fmt.Fprintln(conn, "list")
		line, err := bufio.NewReader(conn).ReadString('\n')
		if err != nil || line != in.app.Name+":list\n" {
			t.Fatalf("%s console: %q %v", in.app.Name, line, err)
		}
		fmt.Fprintln(conn, "stop")
		waitSupervisor(t, in.done, true)
	}
	for _, in := range instances {
		data, err := os.ReadFile(filepath.Join(state, "minecraft", in.app.Name, "console.log"))
		if err != nil || string(data) != in.app.Name+":list\n"+in.app.Name+":stop\n" {
			t.Fatalf("%s log: %q %v", in.app.Name, data, err)
		}
	}
}

func TestSupervisorPreservesLargeAndFinalOutput(t *testing.T) {
	a := testApp(t, "head -c 1500000 /dev/zero | tr '\\000' x\nprintf '\\nfinal stdout\\n'\nprintf 'final stderr\\n' >&2\n")
	logs := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := runSupervisor(ctx, a, t.TempDir(), logs, time.Second); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(logs, "console.log"))
	if err != nil || len(data) != 1500027 || !bytes.Contains(data, []byte("final stdout\n")) || !bytes.Contains(data, []byte("final stderr\n")) {
		t.Fatalf("lost output: length=%d error=%v", len(data), err)
	}
}

func TestSupervisorCrashAndForcedShutdown(t *testing.T) {
	t.Run("crash", func(t *testing.T) {
		a := testApp(t, "printf 'bad config\\n' >&2\nexit 42\n")
		err := runSupervisor(context.Background(), a, t.TempDir(), t.TempDir(), time.Second)
		if err == nil || !strings.Contains(err.Error(), "exit status 42") {
			t.Fatalf("crash hidden: %v", err)
		}
	})
	t.Run("ignores stop", func(t *testing.T) {
		a := testApp(t, "while IFS= read -r line; do :; done\n")
		run := t.TempDir()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan error, 1)
		go func() { done <- runSupervisor(ctx, a, run, t.TempDir(), 50*time.Millisecond) }()
		conn := connectConsole(t, filepath.Join(run, "test.sock"))
		defer conn.Close()
		cancel()
		waitSupervisor(t, done, false)
	})
}

func TestSlowConsoleCannotBlockOutput(t *testing.T) {
	var log bytes.Buffer
	hub := &consoleHub{log: &log, clients: make(map[net.Conn]chan []byte)}
	server, client := net.Pipe()
	defer client.Close()
	defer hub.close()
	hub.add(server)
	data := bytes.Repeat([]byte("x"), 8192)
	done := make(chan struct{})
	go func() {
		for i := 0; i < 1000; i++ {
			hub.Write(data)
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("slow client blocked Java output")
	}
	if log.Len() != 1000*len(data) {
		t.Fatal("log output was lost")
	}
}

func TestSupervisorSIGTERM(t *testing.T) {
	if os.Getenv("SUNDY_SIGNAL_HELPER") == "1" {
		if err := RunSupervisor("test"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	a := testApp(t, "while IFS= read -r line; do [ \"$line\" = stop ] && { printf 'saved on SIGTERM\\n'; exit 0; }; done\n")
	a.ID = "minecraft-test"
	state, run := t.TempDir(), t.TempDir()
	t.Setenv("SUNDY_STATE_DIR", state)
	t.Setenv("SUNDY_RUNTIME_DIR", run)
	if err := apps.Save(a); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestSupervisorSIGTERM$")
	cmd.Env = append(os.Environ(), "SUNDY_SIGNAL_HELPER=1")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Process.Kill()
	conn := connectConsole(t, filepath.Join(run, "test.sock"))
	defer conn.Close()
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	waitSupervisor(t, done, true)
	data, err := os.ReadFile(filepath.Join(state, "minecraft", "test", "console.log"))
	if err != nil || !bytes.Contains(data, []byte("saved on SIGTERM")) {
		t.Fatalf("SIGTERM did not save world: %s %v", data, err)
	}
}
