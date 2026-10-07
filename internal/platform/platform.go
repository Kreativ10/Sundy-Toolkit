package platform

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/SundySystems/sundy-toolkit/internal/util"
)

type Info struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Version   string `json:"version"`
	Like      string `json:"id_like"`
	Arch      string `json:"arch"`
	Kernel    string `json:"kernel"`
	Package   string `json:"package_manager"`
	Init      string `json:"init_system"`
	Container string `json:"virtualization"`
}

func Detect() Info {
	vals := map[string]string{}
	if f, err := os.Open("/etc/os-release"); err == nil {
		defer f.Close()
		s := bufio.NewScanner(f)
		for s.Scan() {
			line := s.Text()
			if i := strings.IndexByte(line, '='); i > 0 {
				k := line[:i]
				v := strings.Trim(strings.TrimSpace(line[i+1:]), "\"")
				vals[k] = v
			}
		}
	}
	kernel := util.Run(2*time.Second, "uname", "-r").Stdout
	virt := "unknown"
	if util.Exists("systemd-detect-virt") {
		r := util.Run(2*time.Second, "systemd-detect-virt")
		if r.Code == 0 && r.Stdout != "" {
			virt = r.Stdout
		} else {
			virt = "none"
		}
	}
	info := Info{ID: vals["ID"], Name: vals["PRETTY_NAME"], Version: vals["VERSION_ID"], Like: vals["ID_LIKE"], Arch: runtime.GOARCH, Kernel: kernel, Package: detectPackage(), Init: detectInit(), Container: virt}
	if info.Name == "" {
		info.Name = info.ID
	}
	if info.ID == "" {
		info.ID = "linux"
	}
	return info
}

func detectPackage() string {
	for _, p := range []string{"apt-get", "dnf", "yum", "pacman", "zypper", "apk", "xbps-install", "emerge", "nix-env"} {
		if util.Exists(p) {
			return p
		}
	}
	return "unknown"
}
func detectInit() string {
	if _, err := os.Stat("/run/systemd/system"); err == nil && util.Exists("systemctl") {
		return "systemd"
	}
	if util.Exists("rc-service") {
		return "openrc"
	}
	if util.Exists("sv") {
		return "runit"
	}
	return "unknown"
}

type PackageManager struct{ Name string }

func Packages() PackageManager { return PackageManager{Name: detectPackage()} }

func (p PackageManager) Install(names ...string) error {
	if len(names) == 0 {
		return nil
	}
	var cmd string
	var args []string
	switch p.Name {
	case "apt-get":
		cmd = "apt-get"
		args = append([]string{"install", "-y"}, names...)
	case "dnf":
		cmd = "dnf"
		args = append([]string{"install", "-y"}, names...)
	case "yum":
		cmd = "yum"
		args = append([]string{"install", "-y"}, names...)
	case "pacman":
		cmd = "pacman"
		args = append([]string{"-S", "--needed", "--noconfirm"}, names...)
	case "zypper":
		cmd = "zypper"
		args = append([]string{"--non-interactive", "install"}, names...)
	case "apk":
		cmd = "apk"
		args = append([]string{"add"}, names...)
	case "xbps-install":
		cmd = "xbps-install"
		args = append([]string{"-Sy"}, names...)
	case "emerge":
		cmd = "emerge"
		args = names
	case "nix-env":
		cmd = "nix-env"
		args = append([]string{"-iA"}, names...)
	default:
		return fmt.Errorf("no supported package manager detected")
	}
	if err := util.RunStreaming(cmd, args...); err != nil {
		return fmt.Errorf("package install failed: %w", err)
	}
	return nil
}

func (p PackageManager) Remove(names ...string) error {
	if len(names) == 0 {
		return nil
	}
	var cmd string
	var args []string
	switch p.Name {
	case "apt-get":
		cmd = "apt-get"
		args = append([]string{"remove", "-y"}, names...)
	case "dnf":
		cmd = "dnf"
		args = append([]string{"remove", "-y"}, names...)
	case "yum":
		cmd = "yum"
		args = append([]string{"remove", "-y"}, names...)
	case "pacman":
		cmd = "pacman"
		args = append([]string{"-Rns", "--noconfirm"}, names...)
	case "zypper":
		cmd = "zypper"
		args = append([]string{"--non-interactive", "remove"}, names...)
	case "apk":
		cmd = "apk"
		args = append([]string{"del"}, names...)
	default:
		return fmt.Errorf("package removal is not implemented for %s", p.Name)
	}
	return util.RunStreaming(cmd, args...)
}

type ServiceManager struct{ Init string }

func Services() ServiceManager { return ServiceManager{Init: detectInit()} }
func (s ServiceManager) Action(action, name string) error {
	switch s.Init {
	case "systemd":
		return util.RunStreaming("systemctl", action, name)
	case "openrc":
		switch action {
		case "start", "stop", "restart", "status":
			return util.RunStreaming("rc-service", name, action)
		case "enable":
			return util.RunStreaming("rc-update", "add", name, "default")
		case "disable":
			return util.RunStreaming("rc-update", "del", name, "default")
		}
	case "runit":
		if action == "start" {
			action = "up"
		}
		if action == "stop" {
			action = "down"
		}
		return util.RunStreaming("sv", action, name)
	}
	return fmt.Errorf("unsupported init system: %s", s.Init)
}
func (s ServiceManager) State(name string) string {
	switch s.Init {
	case "systemd":
		r := util.Run(3*time.Second, "systemctl", "is-active", name)
		if r.Code == 0 {
			return strings.TrimSpace(r.Stdout)
		}
		if r.Stdout != "" {
			return r.Stdout
		}
		return "inactive"
	case "openrc":
		r := util.Run(3*time.Second, "rc-service", name, "status")
		if r.Code == 0 {
			return "active"
		}
		return "inactive"
	case "runit":
		r := util.Run(3*time.Second, "sv", "status", name)
		if r.Code == 0 {
			return "active"
		}
		return "inactive"
	}
	return "unknown"
}
