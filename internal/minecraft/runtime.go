package minecraft

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"github.com/SundySystems/sundy-toolkit/internal/apps"
	"github.com/SundySystems/sundy-toolkit/internal/ui"
	"github.com/SundySystems/sundy-toolkit/internal/util"
)

func runtimeDir() string {
	if dir := os.Getenv("SUNDY_RUNTIME_DIR"); dir != "" {
		return dir
	}
	return "/run/sundy/minecraft"
}

func minecraftApp(name string) (apps.App, error) {
	a, err := apps.Get(name)
	if err != nil {
		return a, err
	}
	if a.Kind != "minecraft" || !nameRx.MatchString(a.Name) {
		return a, fmt.Errorf("%q is not a valid Minecraft instance", name)
	}
	return a, nil
}

func RunSupervisor(name string) error {
	a, err := minecraftApp(name)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return runSupervisor(ctx, a, runtimeDir(), filepath.Join(util.StateDir(), "minecraft", a.Name), 90*time.Second)
}

// consoleHub gives each client a bounded output queue. A stalled terminal must
// never block Java's stdout or another operator's console.
type consoleHub struct {
	mu      sync.Mutex
	log     io.Writer
	clients map[net.Conn]chan []byte
	wg      sync.WaitGroup
	closed  bool
}

func (h *consoleHub) Write(p []byte) (int, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	n, err := h.log.Write(p)
	if err != nil {
		return n, err
	}
	for conn, queue := range h.clients {
		select {
		case queue <- append([]byte(nil), p...):
		default:
			conn.Close()
			close(queue)
			delete(h.clients, conn)
		}
	}
	return len(p), nil
}

func (h *consoleHub) add(conn net.Conn) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		conn.Close()
		return false
	}
	queue := make(chan []byte, 32)
	h.clients[conn] = queue
	h.wg.Add(1)
	go func() {
		defer h.wg.Done()
		defer h.remove(conn)
		for data := range queue {
			conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
			if _, err := conn.Write(data); err != nil {
				return
			}
		}
	}()
	return true
}

func (h *consoleHub) remove(conn net.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if queue, ok := h.clients[conn]; ok {
		close(queue)
		delete(h.clients, conn)
	}
	conn.Close()
}

func (h *consoleHub) close() {
	h.mu.Lock()
	h.closed = true
	for conn, queue := range h.clients {
		conn.Close()
		close(queue)
		delete(h.clients, conn)
	}
	h.mu.Unlock()
	h.wg.Wait()
}

func runSupervisor(ctx context.Context, a apps.App, runDir, logDir string, stopTimeout time.Duration) error {
	if !nameRx.MatchString(a.Name) {
		return fmt.Errorf("invalid instance name")
	}
	if err := os.MkdirAll(runDir, 0750); err != nil {
		return err
	}
	// Lock before removing a stale socket. A second supervisor must not steal a live console.
	lock, err := os.OpenFile(filepath.Join(runDir, a.Name+".lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return fmt.Errorf("instance %s is already supervised: %w", a.Name, err)
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	sock := filepath.Join(runDir, a.Name+".sock")
	if err := os.Remove(sock); err != nil && !os.IsNotExist(err) {
		return err
	}
	ln, err := net.Listen("unix", sock)
	if err != nil {
		return err
	}
	defer ln.Close()
	if err := os.Chmod(sock, 0600); err != nil {
		return err
	}
	if err := os.MkdirAll(logDir, 0750); err != nil {
		return err
	}
	log, err := os.OpenFile(filepath.Join(logDir, "console.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0640)
	if err != nil {
		return err
	}
	defer log.Close()
	hub := &consoleHub{log: log, clients: make(map[net.Conn]chan []byte)}
	defer hub.close()
	mem := a.Metadata["memory"]
	if mem == "" {
		mem = "2G"
	}
	if !memoryRx.MatchString(mem) {
		return fmt.Errorf("invalid heap size %q", mem)
	}
	java := a.Metadata["java"]
	if java == "" {
		java = "java"
	}
	cmd := exec.Command(java, "-Xms"+initialMemory(mem), "-Xmx"+mem, "-jar", a.Metadata["jar"], "nogui")
	cmd.Dir = a.Directory
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	// exec owns the copy goroutine and waits for it, so final log output is not lost.
	cmd.Stdout, cmd.Stderr = hub, hub
	cmd.WaitDelay = 5 * time.Second
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	defer stdin.Close()
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start Java: %w", err)
	}
	var inputMu sync.Mutex
	writeCommand := func(line string) error {
		inputMu.Lock()
		defer inputMu.Unlock()
		_, err := io.WriteString(stdin, line+"\n")
		return err
	}
	var clients sync.WaitGroup
	accepted := make(chan struct{})
	go func() {
		defer close(accepted)
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			if !hub.add(conn) {
				return
			}
			clients.Add(1)
			go func() {
				defer clients.Done()
				defer hub.remove(conn)
				scanner := bufio.NewScanner(conn)
				for scanner.Scan() {
					line := scanner.Text()
					if line == ":detach" {
						return
					}
					if err := writeCommand(line); err != nil {
						return
					}
				}
			}()
		}
	}()
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	select {
	case err = <-exited:
	case <-ctx.Done():
		go func() { _ = writeCommand("stop") }()
		timer := time.NewTimer(stopTimeout)
		select {
		case err = <-exited:
			timer.Stop()
		case <-timer.C:
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			<-exited
			err = fmt.Errorf("server did not stop within %s and was killed", stopTimeout)
		}
	}
	ln.Close()
	<-accepted
	hub.close()
	clients.Wait()
	if err != nil {
		return fmt.Errorf("Minecraft exited: %w; see %s", err, filepath.Join(logDir, "console.log"))
	}
	return nil
}

// Keep the initial heap modest; Xmx is a limit, not the machine's available RAM.
func initialMemory(maximum string) string {
	if maximum[len(maximum)-1] == 'M' || maximum[len(maximum)-1] == 'm' {
		var amount int
		fmt.Sscanf(maximum, "%d", &amount)
		if amount < 512 {
			return maximum
		}
	}
	return "512M"
}

func Console(name string) error {
	a, err := minecraftApp(name)
	if err != nil {
		return err
	}
	conn, err := net.Dial("unix", filepath.Join(runtimeDir(), a.Name+".sock"))
	if err != nil {
		return fmt.Errorf("server console is unavailable: %w; run sudo sundy minecraft status %s", err, a.Name)
	}
	defer conn.Close()
	done := make(chan error, 1)
	go func() { _, err := io.Copy(os.Stdout, conn); done <- err }()
	fmt.Println("Connected. Type :detach to leave without stopping the server.")
	input := ui.Input()
	var line []byte
	for {
		select {
		case err := <-done:
			return err
		default:
		}
		if input.Buffered() == 0 {
			fds := []unix.PollFd{{Fd: int32(os.Stdin.Fd()), Events: unix.POLLIN}}
			n, err := unix.Poll(fds, 100)
			if errors.Is(err, syscall.EINTR) {
				continue
			}
			if err != nil {
				return err
			}
			if n == 0 {
				continue
			}
		}
		b, err := input.ReadByte()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if b == '\n' {
			command := string(line)
			line = line[:0]
			if command == ":detach" || command == ":detach\r" {
				return nil
			}
			conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if _, err := fmt.Fprintln(conn, command); err != nil {
				return err
			}
		} else {
			line = append(line, b)
			if len(line) > 64*1024 {
				return fmt.Errorf("console command is too long")
			}
		}
	}
}
