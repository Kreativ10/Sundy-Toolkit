package snapshot

import (
	"errors"
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
	var saveErrors []error
	for _, i := range selected {
		if i < 0 || i >= len(SystemComponents) {
			continue
		}
		c := SystemComponents[i]
		if err := saveComponent(d, c.Key); err != nil {
			m.Notes += fmt.Sprintf("%s: %v; ", c.Key, err)
			saveErrors = append(saveErrors, fmt.Errorf("%s: %w", c.Key, err))
		}
	}
	if err := s.Update(m); err != nil {
		return m, err
	}
	return m, errors.Join(saveErrors...)
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
		for _, file := range []string{"sshd_config", "ssh_config"} {
			if err := copyOptionalFile(filepath.Join("/etc/ssh", file), filepath.Join(tmp, file)); err != nil {
				return err
			}
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
		list, err := apps.List()
		if err != nil {
			return err
		}
		tmp, err := os.MkdirTemp("", "sundy-minecraft-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmp)
		if err := copyOptionalFile(filepath.Join(util.StateDir(), "apps.json"), filepath.Join(tmp, "apps.json")); err != nil {
			return err
		}
		for _, a := range list {
			if a.Kind != "minecraft" {
				continue
			}
			inst := filepath.Join(tmp, "instance-"+filepath.Base(a.ID))
			if err := os.MkdirAll(inst, 0700); err != nil {
				return err
			}
			for _, f := range []string{"server.properties", "eula.txt", "bukkit.yml", "spigot.yml", "paper-global.yml", "paper-world-defaults.yml"} {
				src := filepath.Join(a.Directory, f)
				if err := copyOptionalFile(src, filepath.Join(inst, f)); err != nil {
					return err
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
	// Validate every component before taking a backup or changing files.
	var selected []int
	for _, key := range keys {
		index := -1
		for i, c := range SystemComponents {
			if c.Key == key {
				index = i
				break
			}
		}
		if index < 0 {
			return fmt.Errorf("unknown component %q", key)
		}
		if key == "ssh" || key == "minecraft" || key == "packages" {
			return fmt.Errorf("component %s is review-only; restore its configuration manually from the snapshot", key)
		}
		if _, err := os.Stat(filepath.Join(s.SnapDir(m.ID), "component-"+key+".tar.gz")); err != nil {
			return fmt.Errorf("component %s is not present in snapshot", key)
		}
		selected = append(selected, index)
	}
	rollback, err := s.SaveSystem("auto-before-restore-"+m.Name, selected, true)
	if err != nil {
		return fmt.Errorf("could not create emergency rollback snapshot: %w", err)
	}
	for _, key := range keys {
		archive := filepath.Join(s.SnapDir(m.ID), "component-"+key+".tar.gz")
		if _, err := os.Stat(archive); err != nil {
			return fmt.Errorf("component %s is not present in snapshot", key)
		}
		if err := ExtractArchive(archive); err != nil {
			rollbackErr := ExtractArchive(filepath.Join(s.SnapDir(rollback.ID), "component-"+key+".tar.gz"))
			return errors.Join(fmt.Errorf("restore %s failed; rollback attempted: %w", key, err), rollbackErr)
		}
		if err := verifyRestored(key); err != nil {
			rollbackErr := ExtractArchive(filepath.Join(s.SnapDir(rollback.ID), "component-"+key+".tar.gz"))
			return errors.Join(fmt.Errorf("%s validation failed; rollback attempted: %w", key, err), rollbackErr)
		}
	}
	return nil
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
		if err := copyOptionalFile(filepath.Join(src, name), filepath.Join(dst, name)); err != nil {
			return err
		}
	}
	return nil
}

func copyOptionalFile(src, dst string) error {
	if _, err := os.Stat(src); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return util.CopyFile(src, dst, 0600)
}
