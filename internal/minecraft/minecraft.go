package minecraft

import (
	"bufio"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/SundySystems/sundy-toolkit/internal/apps"
	"github.com/SundySystems/sundy-toolkit/internal/platform"
	"github.com/SundySystems/sundy-toolkit/internal/util"
)

type Config struct {
	Name, Directory, Jar, Memory string
	Port                         int
	AcceptEULA                   bool
	AutoStart                    bool
	Restart                      bool
}

func InstallExisting(c Config) (apps.App, error) {
	if err := util.RequireRoot(); err != nil {
		return apps.App{}, err
	}
	if c.Name == "" || c.Directory == "" {
		return apps.App{}, fmt.Errorf("name and directory are required")
	}
	if c.Port < 1 || c.Port > 65535 {
		return apps.App{}, fmt.Errorf("invalid port")
	}
	abs, err := filepath.Abs(c.Directory)
	if err != nil {
		return apps.App{}, err
	}
	c.Directory = abs
	st, err := os.Stat(abs)
	if err != nil || !st.IsDir() {
		return apps.App{}, fmt.Errorf("server directory does not exist: %s", abs)
	}
	if c.Jar == "" {
		c.Jar = findJar(abs)
	}
	if c.Jar == "" {
		return apps.App{}, fmt.Errorf("no server jar found; specify --jar")
	}
	if !filepath.IsAbs(c.Jar) {
		c.Jar = filepath.Join(abs, c.Jar)
	}
	if _, err := os.Stat(c.Jar); err != nil {
		return apps.App{}, fmt.Errorf("server jar not found: %s", c.Jar)
	}
	if !util.Exists("java") {
		if err := installJava(); err != nil {
			return apps.App{}, err
		}
	}
	if err := setProperty(filepath.Join(abs, "server.properties"), "server-port", strconv.Itoa(c.Port)); err != nil {
		return apps.App{}, err
	}
	if c.AcceptEULA {
		if err := setProperty(filepath.Join(abs, "eula.txt"), "eula", "true"); err != nil {
			return apps.App{}, err
		}
	}
	if c.Memory == "" {
		c.Memory = "2G"
	}
	a := apps.App{ID: "minecraft-" + safe(c.Name), Kind: "minecraft", Name: c.Name, Directory: abs, Port: c.Port, Service: serviceName(c.Name), InstalledAt: time.Now(), Metadata: map[string]string{"jar": c.Jar, "memory": c.Memory, "restart": strconv.FormatBool(c.Restart)}}
	if err := apps.Save(a); err != nil {
		return a, err
	}
	if err := installService(a, c.AutoStart, c.Restart); err != nil {
		return a, err
	}
	return a, nil
}

func CreateVanilla(c Config, version string) (apps.App, error) {
	if err := util.RequireRoot(); err != nil {
		return apps.App{}, err
	}
	if err := os.MkdirAll(c.Directory, 0755); err != nil {
		return apps.App{}, err
	}
	jar := filepath.Join(c.Directory, "server.jar")
	resolved, err := DownloadVanilla(version, jar)
	if err != nil {
		return apps.App{}, err
	}
	c.Jar = jar
	a, err := InstallExisting(c)
	if err == nil {
		a.Version = resolved
		_ = apps.Save(a)
	}
	return a, err
}

func DownloadVanilla(version, dst string) (string, error) {
	client := &http.Client{Timeout: 45 * time.Second}
	var manifest struct {
		Latest struct {
			Release string `json:"release"`
		} `json:"latest"`
		Versions []struct{ ID, URL string }
	}
	if err := getJSON(client, "https://piston-meta.mojang.com/mc/game/version_manifest_v2.json", &manifest); err != nil {
		return "", err
	}
	if version == "" || version == "latest" {
		version = manifest.Latest.Release
	}
	var vurl string
	for _, v := range manifest.Versions {
		if v.ID == version {
			vurl = v.URL
			break
		}
	}
	if vurl == "" {
		return "", fmt.Errorf("Minecraft version %s not found", version)
	}
	var meta struct {
		Downloads struct {
			Server struct {
				URL, SHA1 string
				Size      int64
			} `json:"server"`
		} `json:"downloads"`
	}
	if err := getJSON(client, vurl, &meta); err != nil {
		return "", err
	}
	if meta.Downloads.Server.URL == "" {
		return "", fmt.Errorf("server download is unavailable for %s", version)
	}
	resp, err := client.Get(meta.Downloads.Server.URL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("download returned %s", resp.Status)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return "", err
	}
	tmp := dst + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return "", err
	}
	h := sha1.New()
	_, cp := io.Copy(io.MultiWriter(f, h), resp.Body)
	ce := f.Close()
	if cp != nil {
		return "", cp
	}
	if ce != nil {
		return "", ce
	}
	if want := strings.ToLower(meta.Downloads.Server.SHA1); want != "" && hex.EncodeToString(h.Sum(nil)) != want {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("download checksum mismatch")
	}
	if err := os.Rename(tmp, dst); err != nil {
		return "", err
	}
	return version, nil
}
func getJSON(c *http.Client, url string, v any) error {
	r, e := c.Get(url)
	if e != nil {
		return e
	}
	defer r.Body.Close()
	if r.StatusCode/100 != 2 {
		return fmt.Errorf("%s returned %s", url, r.Status)
	}
	return json.NewDecoder(r.Body).Decode(v)
}

func RunSupervisor(name string) error {
	a, err := apps.Get(name)
	if err != nil {
		return err
	}
	if a.Kind != "minecraft" {
		return fmt.Errorf("%s is not a Minecraft app", name)
	}
	jar := a.Metadata["jar"]
	mem := a.Metadata["memory"]
	if mem == "" {
		mem = "2G"
	}
	runDir := "/run/sundy/minecraft"
	if err := os.MkdirAll(runDir, 0755); err != nil {
		return err
	}
	sock := filepath.Join(runDir, safe(a.Name)+".sock")
	_ = os.Remove(sock)
	ln, err := net.Listen("unix", sock)
	if err != nil {
		return err
	}
	defer func() { ln.Close(); os.Remove(sock) }()
	_ = os.Chmod(sock, 0660)
	logDir := filepath.Join(util.StateDir(), "minecraft", safe(a.Name))
	_ = os.MkdirAll(logDir, 0750)
	log, err := os.OpenFile(filepath.Join(logDir, "console.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0640)
	if err != nil {
		return err
	}
	defer log.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := exec.CommandContext(ctx, "java", "-Xms"+mem, "-Xmx"+mem, "-jar", jar, "nogui")
	cmd.Dir = a.Directory
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	clients := map[net.Conn]bool{}
	var mu sync.Mutex
	broadcast := func(line string) {
		log.WriteString(line)
		log.Sync()
		mu.Lock()
		defer mu.Unlock()
		for c := range clients {
			if _, e := io.WriteString(c, line); e != nil {
				c.Close()
				delete(clients, c)
			}
		}
	}
	pump := func(r io.Reader) {
		s := bufio.NewScanner(r)
		buf := make([]byte, 64*1024)
		s.Buffer(buf, 1024*1024)
		for s.Scan() {
			broadcast(s.Text() + "\n")
		}
	}
	go pump(stdout)
	go pump(stderr)
	go func() {
		for {
			c, e := ln.Accept()
			if e != nil {
				return
			}
			mu.Lock()
			clients[c] = true
			mu.Unlock()
			go func(conn net.Conn) {
				defer func() { mu.Lock(); delete(clients, conn); mu.Unlock(); conn.Close() }()
				sc := bufio.NewScanner(conn)
				for sc.Scan() {
					line := sc.Text()
					if line == ":detach" {
						return
					}
					io.WriteString(stdin, line+"\n")
				}
			}(c)
		}
	}()
	err = cmd.Wait()
	ln.Close()
	return err
}

func Console(name string) error {
	a, err := apps.Get(name)
	if err != nil {
		return err
	}
	sock := filepath.Join("/run/sundy/minecraft", safe(a.Name)+".sock")
	c, err := net.Dial("unix", sock)
	if err != nil {
		return fmt.Errorf("server console is unavailable (%v); is %s running?", err, a.Service)
	}
	defer c.Close()
	done := make(chan struct{})
	go func() { io.Copy(os.Stdout, c); close(done) }()
	fmt.Println("Connected. Type :detach to leave without stopping the server.")
	sc := bufio.NewScanner(os.Stdin)
	for sc.Scan() {
		line := sc.Text()
		fmt.Fprintln(c, line)
		if line == ":detach" {
			return nil
		}
	}
	return nil
}

func installService(a apps.App, autostart, restart bool) error {
	p := platform.Detect()
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if real, e := filepath.EvalSymlinks(exe); e == nil {
		exe = real
	}
	switch p.Init {
	case "systemd":
		policy := "no"
		if restart {
			policy = "on-failure"
		}
		unit := fmt.Sprintf(`[Unit]
Description=Sundy Minecraft - %s
After=network.target

[Service]
Type=simple
WorkingDirectory=%s
ExecStart=%s runtime minecraft %s
Restart=%s
RestartSec=5
LimitNOFILE=1048576

[Install]
WantedBy=multi-user.target
`, a.Name, a.Directory, exe, a.Name, policy)
		path := filepath.Join("/etc/systemd/system", a.Service+".service")
		if err := os.WriteFile(path, []byte(unit), 0644); err != nil {
			return err
		}
		if r := util.Run(10*time.Second, "systemctl", "daemon-reload"); r.Code != 0 {
			return fmt.Errorf("systemctl daemon-reload: %s", r.Stderr)
		}
		if autostart {
			if r := util.Run(10*time.Second, "systemctl", "enable", "--now", a.Service); r.Code != 0 {
				return fmt.Errorf("enable service: %s", r.Stderr)
			}
		}
	case "openrc":
		script := fmt.Sprintf("#!/sbin/openrc-run\nname=\"Sundy Minecraft %s\"\ncommand=\"%s\"\ncommand_args=\"runtime minecraft %s\"\ncommand_background=false\ndirectory=\"%s\"\ndepend() { need net; }\n", a.Name, exe, a.Name, a.Directory)
		path := filepath.Join("/etc/init.d", a.Service)
		if err := os.WriteFile(path, []byte(script), 0755); err != nil {
			return err
		}
		if autostart {
			_ = util.RunStreaming("rc-update", "add", a.Service, "default")
			_ = util.RunStreaming("rc-service", a.Service, "start")
		}
	default:
		return fmt.Errorf("automatic service installation currently requires systemd or OpenRC; app definition was saved")
	}
	return nil
}
func installJava() error {
	p := platform.Detect()
	pkg := "openjdk-21-jre-headless"
	switch p.Package {
	case "dnf", "yum":
		pkg = "java-21-openjdk-headless"
	case "pacman":
		pkg = "jre21-openjdk-headless"
	case "apk":
		pkg = "openjdk21-jre-headless"
	case "zypper":
		pkg = "java-21-openjdk-headless"
	}
	return platform.Packages().Install(pkg)
}
func findJar(dir string) string {
	for _, n := range []string{"server.jar", "paper.jar", "purpur.jar", "fabric-server-launch.jar", "forge.jar"} {
		if _, e := os.Stat(filepath.Join(dir, n)); e == nil {
			return filepath.Join(dir, n)
		}
	}
	ents, _ := os.ReadDir(dir)
	for _, e := range ents {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".jar") {
			return filepath.Join(dir, e.Name())
		}
	}
	return ""
}
func setProperty(path, key, val string) error {
	var lines []string
	if b, e := os.ReadFile(path); e == nil {
		lines = strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n")
	}
	found := false
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "#") || !strings.Contains(t, "=") {
			continue
		}
		p := strings.SplitN(t, "=", 2)
		if strings.TrimSpace(p[0]) == key {
			lines[i] = key + "=" + val
			found = true
		}
	}
	if !found {
		lines = append(lines, key+"="+val)
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644)
}
func safe(s string) string {
	r := strings.NewReplacer("/", "-", " ", "-", "..", "-")
	return r.Replace(s)
}
func serviceName(name string) string { return "sundy-minecraft-" + safe(name) }
