package snapshot

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SundySystems/sundy-toolkit/internal/apps"
	"github.com/SundySystems/sundy-toolkit/internal/util"
)

type Component struct{ Key, Label, Description string }

var SystemComponents = []Component{
	{"network", "Network", "Persistent network, DNS and WireGuard configuration"},
	{"firewall", "Firewall", "nftables, firewalld and UFW configuration"},
	{"ssh", "SSH", "OpenSSH daemon/client configuration (host keys are excluded)"},
	{"services", "Services", "Local systemd/OpenRC service definitions"},
	{"system", "System settings", "sysctl, limits, modules and selected host settings"},
	{"docker", "Docker", "Docker daemon configuration"},
	{"minecraft", "Minecraft", "Sundy-managed instance metadata and server.properties/EULA"},
	{"packages", "Package state", "Installed package inventory for reference"},
}

func (s Store) SaveSystem(name string, selected []int, automatic bool) (Meta, error) {
	if err := util.RequireRoot(); err != nil {
		return Meta{}, err
	}
	if name == "" {
		name = "system-" + time.Now().Format("20060102-150405")
	}
	labels := []string{}
	for _, i := range selected {
		if i >= 0 && i < len(SystemComponents) {
			labels = append(labels, SystemComponents[i].Label)
		}
	}
	m, err := s.Create(name, "system", labels, automatic)
	if err != nil {
		return m, err
	}
	d := s.SnapDir(m.ID)
	for _, i := range selected {
		if i < 0 || i >= len(SystemComponents) {
			continue
		}
		c := SystemComponents[i]
		if err := saveComponent(d, c.Key); err != nil {
			m.Notes += fmt.Sprintf("%s: %v; ", c.Key, err)
		}
	}
	if err := s.Update(m); err != nil {
		return m, err
	}
	return m, nil
}

func saveComponent(dir, key string) error {
	switch key {
	case "network":
		return ArchivePaths(filepath.Join(dir, "component-network.tar.gz"), []string{"/etc/NetworkManager/system-connections", "/etc/systemd/network", "/etc/netplan", "/etc/network/interfaces", "/etc/network/interfaces.d", "/etc/resolv.conf", "/etc/systemd/resolved.conf", "/etc/wireguard"})
	case "firewall":
		if util.Exists("nft") {
			r := util.Run(8*time.Second, "nft", "list", "ruleset")
			_ = os.WriteFile(filepath.Join(dir, "nftables-runtime.conf"), []byte(r.Stdout+"\n"), 0600)
		}
		if util.Exists("iptables-save") {
			r := util.Run(8*time.Second, "iptables-save")
			_ = os.WriteFile(filepath.Join(dir, "iptables-runtime.rules"), []byte(r.Stdout+"\n"), 0600)
		}
		return ArchivePaths(filepath.Join(dir, "component-firewall.tar.gz"), []string{"/etc/nftables.conf", "/etc/firewalld", "/etc/ufw"})
	case "ssh":
		// Host private keys are intentionally not included in Sundy configuration snapshots.
		tmp, err := os.MkdirTemp("", "sundy-ssh-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmp)
		if b, e := os.ReadFile("/etc/ssh/sshd_config"); e == nil {
			_ = os.WriteFile(filepath.Join(tmp, "sshd_config"), b, 0600)
		}
		if b, e := os.ReadFile("/etc/ssh/ssh_config"); e == nil {
			_ = os.WriteFile(filepath.Join(tmp, "ssh_config"), b, 0600)
		}
		if e := copyDirFiltered("/etc/ssh/sshd_config.d", filepath.Join(tmp, "sshd_config.d")); e != nil && !os.IsNotExist(e) {
			return e
		}
		if e := copyDirFiltered("/etc/ssh/ssh_config.d", filepath.Join(tmp, "ssh_config.d")); e != nil && !os.IsNotExist(e) {
			return e
		}
		return ArchivePaths(filepath.Join(dir, "component-ssh.tar.gz"), []string{tmp})
	case "services":
		return ArchivePaths(filepath.Join(dir, "component-services.tar.gz"), []string{"/etc/systemd/system", "/etc/init.d"})
	case "system":
		return ArchivePaths(filepath.Join(dir, "component-system.tar.gz"), []string{"/etc/sysctl.conf", "/etc/sysctl.d", "/etc/security/limits.conf", "/etc/security/limits.d", "/etc/modules-load.d", "/etc/modprobe.d"})
	case "docker":
		return ArchivePaths(filepath.Join(dir, "component-docker.tar.gz"), []string{"/etc/docker"})
	case "minecraft":
		list, _ := apps.List()
		tmp, err := os.MkdirTemp("", "sundy-minecraft-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmp)
		if b, e := os.ReadFile(filepath.Join(util.StateDir(), "apps.json")); e == nil {
			_ = os.WriteFile(filepath.Join(tmp, "apps.json"), b, 0600)
		}
		for _, a := range list {
			if a.Kind != "minecraft" {
				continue
			}
			inst := filepath.Join(tmp, a.Name)
			_ = os.MkdirAll(inst, 0700)
			for _, f := range []string{"server.properties", "eula.txt", "bukkit.yml", "spigot.yml", "paper-global.yml", "paper-world-defaults.yml"} {
				src := filepath.Join(a.Directory, f)
				if b, e := os.ReadFile(src); e == nil {
					_ = os.WriteFile(filepath.Join(inst, f), b, 0600)
				}
			}
		}
		return ArchivePaths(filepath.Join(dir, "component-minecraft.tar.gz"), []string{tmp})
	case "packages":
		p := packageInventory()
		return os.WriteFile(filepath.Join(dir, "packages.txt"), []byte(p), 0600)
	}
	return fmt.Errorf("unknown component %s", key)
}

func (s Store) RestoreSystem(idOrName string, keys []string, apply bool) error {
	if err := util.RequireRoot(); err != nil {
		return err
	}
	if !apply {
		return fmt.Errorf("restore requires --apply")
	}
	m, err := s.Get(idOrName)
	if err != nil {
		return err
	}
	if m.Type != "system" {
		return fmt.Errorf("snapshot is %s, not system", m.Type)
	}
	if len(keys) == 0 {
		return fmt.Errorf("select at least one component to restore")
	}
	// Take a broad emergency snapshot before any restoration.
	all := make([]int, len(SystemComponents))
	for i := range all {
		all[i] = i
	}
	_, _ = s.SaveSystem("auto-before-restore-"+m.Name, all, true)
	for _, key := range keys {
		archive := filepath.Join(s.SnapDir(m.ID), "component-"+key+".tar.gz")
		if _, err := os.Stat(archive); err != nil {
			return fmt.Errorf("component %s is not present in snapshot", key)
		}
		if key == "ssh" {
			if err := restoreSSHArchive(archive); err != nil {
				return err
			}
			continue
		}
		if key == "minecraft" {
			return fmt.Errorf("Minecraft component is intentionally review-only in v0.1; restore server configuration manually from the snapshot bundle")
		}
		if err := ExtractArchive(archive); err != nil {
			return fmt.Errorf("restore %s: %w", key, err)
		}
		if err := verifyRestored(key); err != nil {
			return fmt.Errorf("%s restored but validation failed: %w", key, err)
		}
	}
	return nil
}

func restoreSSHArchive(archive string) error {
	tmp, err := os.MkdirTemp("", "sundy-ssh-restore-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	// Generic ExtractArchive restores absolute paths, while SSH snapshots intentionally use a temp root.
	// For v0.1, keep SSH recovery safe by not automatically overwriting daemon config.
	_ = archive
	return fmt.Errorf("SSH automatic restore is disabled to avoid locking out remote sessions; inspect the snapshot and restore explicitly")
}

func verifyRestored(key string) error {
	switch key {
	case "firewall":
		if util.Exists("nft") {
			r := util.Run(5*time.Second, "nft", "-c", "-f", "/etc/nftables.conf")
			if util.FileExists("/etc/nftables.conf") && r.Code != 0 {
				return fmt.Errorf("nftables validation: %s", r.Stderr)
			}
		}
	case "docker":
		if util.Exists("dockerd") && util.FileExists("/etc/docker/daemon.json") {
			r := util.Run(8*time.Second, "dockerd", "--validate", "--config-file=/etc/docker/daemon.json")
			if r.Code != 0 {
				return fmt.Errorf("Docker config validation: %s", r.Stderr)
			}
		}
	}
	return nil
}

func packageInventory() string {
	for _, c := range [][]string{{"dpkg-query", "-W", "-f=${binary:Package}\t${Version}\n"}, {"rpm", "-qa"}, {"pacman", "-Q"}, {"apk", "info", "-vv"}, {"xbps-query", "-l"}} {
		if util.Exists(c[0]) {
			r := util.Run(30*time.Second, c[0], c[1:]...)
			if r.Code == 0 {
				return r.Stdout
			}
		}
	}
	return "package inventory unavailable\n"
}
func copyDirFiltered(src, dst string) error {
	ents, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, 0700); err != nil {
		return err
	}
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.Contains(name, "ssh_host_") && strings.HasSuffix(name, "_key") {
			continue
		}
		b, er := os.ReadFile(filepath.Join(src, name))
		if er == nil {
			_ = os.WriteFile(filepath.Join(dst, name), b, 0600)
		}
	}
	return nil
}
