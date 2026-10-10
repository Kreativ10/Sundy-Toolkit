package minecraft

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/SundySystems/sundy-toolkit/internal/apps"
	"github.com/SundySystems/sundy-toolkit/internal/platform"
	"github.com/SundySystems/sundy-toolkit/internal/util"
)

type Config struct {
	Name, Directory, Jar, Memory   string
	Java                           string
	Port                           int
	AcceptEULA, AutoStart, Restart bool
	javaMajor                      int
	version                        string
}

var nameRx = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,47}$`)
var memoryRx = regexp.MustCompile(`(?i)^[1-9][0-9]*[MG]$`)

func normalizeConfig(c Config) (Config, error) {
	if !nameRx.MatchString(c.Name) {
		return c, fmt.Errorf("instance name must contain 1-48 letters, digits, underscores or hyphens and start with a letter or digit")
	}
	if strings.TrimSpace(c.Directory) == "" {
		return c, fmt.Errorf("server directory is required")
	}
	for _, value := range []string{c.Directory, c.Jar, c.Java} {
		if strings.ContainsAny(value, "\x00\r\n") {
			return c, fmt.Errorf("paths must not contain control characters")
		}
	}
	if c.Port < 1 || c.Port > 65535 {
		return c, fmt.Errorf("port must be between 1 and 65535")
	}
	if c.Memory == "" {
		c.Memory = "2G"
	}
	c.Memory = strings.ToUpper(c.Memory)
	if !memoryRx.MatchString(c.Memory) {
		return c, fmt.Errorf("memory must be a positive integer followed by M or G, for example 2048M or 2G")
	}
	amount, err := strconv.ParseUint(c.Memory[:len(c.Memory)-1], 10, 32)
	if err != nil || (strings.HasSuffix(c.Memory, "M") && amount < 64) {
		return c, fmt.Errorf("invalid Java heap size %q (minimum 64M)", c.Memory)
	}
	c.Directory, err = filepath.Abs(c.Directory)
	if err != nil {
		return c, err
	}
	if c.Jar != "" && !filepath.IsAbs(c.Jar) {
		c.Jar = filepath.Join(c.Directory, c.Jar)
	}
	return c, nil
}

func checkAvailable(c Config) error {
	list, err := apps.List()
	if err != nil {
		return err
	}
	for _, a := range list {
		if a.ID == "minecraft-"+c.Name || a.Name == c.Name {
			return fmt.Errorf("instance %q already exists; use minecraft start/restart/status to manage it", c.Name)
		}
		if a.Kind == "minecraft" && (a.Directory == c.Directory || a.Port == c.Port) {
			return fmt.Errorf("directory or port is already registered to %s", a.Name)
		}
	}
	return nil
}

func InstallExisting(c Config) (apps.App, error) {
	var err error
	c, err = normalizeConfig(c)
	if err != nil {
		return apps.App{}, err
	}
	if err := util.RequireRoot(); err != nil {
		return apps.App{}, err
	}
	if err := checkAvailable(c); err != nil {
		return apps.App{}, err
	}
	init := platform.Detect().Init
	if init != "systemd" && init != "openrc" {
		return apps.App{}, fmt.Errorf("Minecraft service installation requires systemd or OpenRC")
	}
	st, err := os.Stat(c.Directory)
	if err != nil || !st.IsDir() {
		return apps.App{}, fmt.Errorf("server directory does not exist: %s", c.Directory)
	}
	if c.Jar == "" {
		c.Jar = findJar(c.Directory)
	}
	if c.Jar == "" {
		return apps.App{}, fmt.Errorf("no unambiguous server jar found; specify its filename")
	}
	st, err = os.Stat(c.Jar)
	if err != nil || !st.Mode().IsRegular() {
		return apps.App{}, fmt.Errorf("server jar is not a regular file: %s", c.Jar)
	}
	if c.javaMajor == 0 {
		c.javaMajor, err = jarJavaMajor(c.Jar)
		if err != nil {
			return apps.App{}, err
		}
	}
	java, err := ensureJava(c.Java, c.javaMajor)
	if err != nil {
		return apps.App{}, err
	}
	if !c.AcceptEULA {
		b, err := os.ReadFile(filepath.Join(c.Directory, "eula.txt"))
		if err != nil || !propertyEquals(string(b), "eula", "true") {
			return apps.App{}, fmt.Errorf("Minecraft EULA must be explicitly accepted before starting a server")
		}
	}
	if err := setProperty(filepath.Join(c.Directory, "server.properties"), "server-port", strconv.Itoa(c.Port)); err != nil {
		return apps.App{}, err
	}
	if c.AcceptEULA {
		if err := setProperty(filepath.Join(c.Directory, "eula.txt"), "eula", "true"); err != nil {
			return apps.App{}, err
		}
	}
	a := apps.App{ID: "minecraft-" + c.Name, Kind: "minecraft", Name: c.Name, Directory: c.Directory, Port: c.Port, Service: serviceName(c.Name), Version: c.version, InstalledAt: time.Now(), Metadata: map[string]string{"jar": c.Jar, "java": java, "java_major": strconv.Itoa(c.javaMajor), "memory": c.Memory, "restart": strconv.FormatBool(c.Restart)}}
	if err := apps.Save(a); err != nil {
		return a, err
	}
	if err := installService(a, c.AutoStart, c.Restart); err != nil {
		return a, fmt.Errorf("instance %s was saved, but service setup failed: %w; inspect with sudo sundy minecraft status %s", a.Name, err, a.Name)
	}
	return a, nil
}

func CreateVanilla(c Config, version string) (apps.App, error) {
	var err error
	c, err = normalizeConfig(c)
	if err != nil {
		return apps.App{}, err
	}
	if err := util.RequireRoot(); err != nil {
		return apps.App{}, err
	}
	if err := checkAvailable(c); err != nil {
		return apps.App{}, err
	}
	if init := platform.Detect().Init; init != "systemd" && init != "openrc" {
		return apps.App{}, fmt.Errorf("Minecraft service installation requires systemd or OpenRC")
	}
	if !c.AcceptEULA {
		return apps.App{}, fmt.Errorf("Minecraft EULA must be explicitly accepted")
	}
	entries, err := os.ReadDir(c.Directory)
	if err != nil && !os.IsNotExist(err) {
		return apps.App{}, err
	}
	if len(entries) > 0 {
		return apps.App{}, fmt.Errorf("new server directory must be empty; register an existing server instead")
	}
	meta, err := resolveVanilla(&httpClient, version)
	if err != nil {
		return apps.App{}, err
	}
	c.Java, err = ensureJava(c.Java, meta.JavaVersion.MajorVersion)
	if err != nil {
		return apps.App{}, err
	}
	if err := os.MkdirAll(c.Directory, 0755); err != nil {
		return apps.App{}, err
	}
	c.Jar = filepath.Join(c.Directory, "server.jar")
	if err := downloadServer(&httpClient, meta, c.Jar); err != nil {
		return apps.App{}, err
	}
	c.version, c.javaMajor = meta.ID, meta.JavaVersion.MajorVersion
	return InstallExisting(c)
}

func propertyEquals(text, key, value string) bool {
	for _, line := range strings.Split(text, "\n") {
		parts := strings.SplitN(strings.TrimSpace(line), "=", 2)
		if len(parts) == 2 && strings.TrimSpace(parts[0]) == key && strings.TrimSpace(parts[1]) == value {
			return true
		}
	}
	return false
}

func findJar(dir string) string {
	for _, n := range []string{"server.jar", "paper.jar", "purpur.jar", "fabric-server-launch.jar", "forge.jar"} {
		if st, e := os.Stat(filepath.Join(dir, n)); e == nil && st.Mode().IsRegular() {
			return filepath.Join(dir, n)
		}
	}
	ents, _ := os.ReadDir(dir)
	var candidates []string
	for _, e := range ents {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".jar") {
			candidates = append(candidates, filepath.Join(dir, e.Name()))
		}
	}
	if len(candidates) == 1 {
		return candidates[0]
	}
	return ""
}
func setProperty(path, key, val string) error {
	mode := os.FileMode(0644)
	if st, err := os.Stat(path); err == nil {
		mode = st.Mode().Perm()
	}
	var lines []string
	if b, e := os.ReadFile(path); e == nil {
		lines = strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n")
	} else if !os.IsNotExist(e) {
		return e
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
	return util.AtomicWrite(path, []byte(strings.TrimRight(strings.Join(lines, "\n"), "\n")+"\n"), mode)
}
func serviceName(name string) string { return "sundy-minecraft-" + name }
