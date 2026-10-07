package install

import (
	"fmt"
	"strings"

	"github.com/SundySystems/sundy-toolkit/internal/platform"
	"github.com/SundySystems/sundy-toolkit/internal/util"
)

type Preset struct {
	ID, Name, Category, Description string
	Packages                        map[string][]string
	Services                        []string
	Notes                           []string
}

var Catalog = []Preset{
	{ID: "essentials", Name: "Administrator essentials", Category: "Core", Description: "Everyday CLI tools for server administration.", Packages: map[string][]string{"apt-get": {"curl", "wget", "git", "jq", "rsync", "tmux", "htop", "lsof", "strace", "tree", "unzip", "tar"}, "dnf": {"curl", "wget", "git", "jq", "rsync", "tmux", "htop", "lsof", "strace", "tree", "unzip", "tar"}, "yum": {"curl", "wget", "git", "jq", "rsync", "tmux", "htop", "lsof", "strace", "tree", "unzip", "tar"}, "pacman": {"curl", "wget", "git", "jq", "rsync", "tmux", "htop", "lsof", "strace", "tree", "unzip", "tar"}, "apk": {"curl", "wget", "git", "jq", "rsync", "tmux", "htop", "lsof", "strace", "tree", "unzip", "tar"}, "zypper": {"curl", "wget", "git", "jq", "rsync", "tmux", "htop", "lsof", "strace", "tree", "unzip", "tar"}}},
	{ID: "network-tools", Name: "Network toolkit", Category: "Network", Description: "Diagnostics, packet capture, DNS, throughput and socket tools.", Packages: map[string][]string{"apt-get": {"iproute2", "iputils-ping", "dnsutils", "nmap", "mtr-tiny", "iperf3", "tcpdump", "ethtool", "whois", "socat"}, "dnf": {"iproute", "iputils", "bind-utils", "nmap", "mtr", "iperf3", "tcpdump", "ethtool", "whois", "socat"}, "yum": {"iproute", "iputils", "bind-utils", "nmap", "mtr", "iperf3", "tcpdump", "ethtool", "whois", "socat"}, "pacman": {"iproute2", "iputils", "bind", "nmap", "mtr", "iperf3", "tcpdump", "ethtool", "whois", "socat"}, "apk": {"iproute2", "iputils", "bind-tools", "nmap", "mtr", "iperf3", "tcpdump", "ethtool", "whois", "socat"}, "zypper": {"iproute2", "iputils", "bind-utils", "nmap", "mtr", "iperf", "tcpdump", "ethtool", "whois", "socat"}}},
	{ID: "storage-tools", Name: "Storage toolkit", Category: "Storage", Description: "SMART/NVMe/RAID/I/O diagnostics.", Packages: map[string][]string{"apt-get": {"smartmontools", "nvme-cli", "mdadm", "iotop", "fio"}, "dnf": {"smartmontools", "nvme-cli", "mdadm", "iotop", "fio"}, "yum": {"smartmontools", "nvme-cli", "mdadm", "iotop", "fio"}, "pacman": {"smartmontools", "nvme-cli", "mdadm", "iotop", "fio"}, "apk": {"smartmontools", "nvme-cli", "mdadm", "iotop", "fio"}, "zypper": {"smartmontools", "nvme-cli", "mdadm", "iotop", "fio"}}},
	{ID: "docker", Name: "Docker Engine", Category: "Containers", Description: "Docker engine and Compose plugin where packaged by the distribution.", Packages: map[string][]string{"apt-get": {"docker.io", "docker-compose-v2"}, "dnf": {"docker", "docker-compose-plugin"}, "yum": {"docker", "docker-compose-plugin"}, "pacman": {"docker", "docker-compose"}, "apk": {"docker", "docker-cli-compose"}, "zypper": {"docker", "docker-compose"}}, Services: []string{"docker"}, Notes: []string{"For production hosts that require Docker Inc. packages, use the official Docker repository instead of the distro package."}},
	{ID: "nginx", Name: "NGINX", Category: "Web", Description: "High performance HTTP/reverse proxy server.", Packages: all("nginx"), Services: []string{"nginx"}},
	{ID: "caddy", Name: "Caddy", Category: "Web", Description: "Automatic-HTTPS web server (package availability varies by distro).", Packages: all("caddy"), Services: []string{"caddy"}},
	{ID: "postgresql", Name: "PostgreSQL", Category: "Database", Description: "PostgreSQL server from distribution repositories.", Packages: all("postgresql"), Services: []string{"postgresql"}},
	{ID: "mariadb", Name: "MariaDB", Category: "Database", Description: "MariaDB server.", Packages: map[string][]string{"apt-get": {"mariadb-server"}, "dnf": {"mariadb-server"}, "yum": {"mariadb-server"}, "pacman": {"mariadb"}, "apk": {"mariadb", "mariadb-client"}, "zypper": {"mariadb"}}, Services: []string{"mariadb"}},
	{ID: "redis", Name: "Redis", Category: "Database", Description: "Redis/Valkey-compatible in-memory datastore.", Packages: map[string][]string{"apt-get": {"redis-server"}, "dnf": {"redis"}, "yum": {"redis"}, "pacman": {"redis"}, "apk": {"redis"}, "zypper": {"redis"}}, Services: []string{"redis", "redis-server"}},
	{ID: "wireguard", Name: "WireGuard", Category: "Network", Description: "WireGuard VPN userspace tooling.", Packages: map[string][]string{"apt-get": {"wireguard-tools"}, "dnf": {"wireguard-tools"}, "yum": {"wireguard-tools"}, "pacman": {"wireguard-tools"}, "apk": {"wireguard-tools"}, "zypper": {"wireguard-tools"}}},
	{ID: "fail2ban", Name: "Fail2ban", Category: "Security", Description: "Log-driven banning framework.", Packages: all("fail2ban"), Services: []string{"fail2ban"}},
	{ID: "kvm", Name: "KVM / libvirt", Category: "Virtualization", Description: "Virtualization host packages.", Packages: map[string][]string{"apt-get": {"qemu-kvm", "libvirt-daemon-system", "libvirt-clients", "bridge-utils"}, "dnf": {"qemu-kvm", "libvirt", "virt-install"}, "yum": {"qemu-kvm", "libvirt", "virt-install"}, "pacman": {"qemu-full", "libvirt", "virt-install", "dnsmasq"}, "zypper": {"qemu-kvm", "libvirt", "virt-install"}}, Services: []string{"libvirtd"}},
	{ID: "dev", Name: "Development toolchain", Category: "Development", Description: "Git, compilers, Python and common build tools.", Packages: map[string][]string{"apt-get": {"git", "build-essential", "python3", "python3-pip"}, "dnf": {"git", "gcc", "gcc-c++", "make", "python3", "python3-pip"}, "yum": {"git", "gcc", "gcc-c++", "make", "python3", "python3-pip"}, "pacman": {"git", "base-devel", "python", "python-pip"}, "apk": {"git", "build-base", "python3", "py3-pip"}, "zypper": {"git", "gcc", "gcc-c++", "make", "python3", "python3-pip"}}},
}

func all(name string) map[string][]string {
	return map[string][]string{"apt-get": {name}, "dnf": {name}, "yum": {name}, "pacman": {name}, "apk": {name}, "zypper": {name}, "xbps-install": {name}, "emerge": {name}}
}
func Find(id string) (Preset, bool) {
	for _, p := range Catalog {
		if p.ID == id {
			return p, true
		}
	}
	return Preset{}, false
}
func InstallPreset(p Preset) error {
	if err := util.RequireRoot(); err != nil {
		return err
	}
	pm := platform.Packages()
	pkgs := p.Packages[pm.Name]
	if len(pkgs) == 0 {
		return fmt.Errorf("preset %s does not yet map packages for %s", p.ID, pm.Name)
	}
	if err := pm.Install(pkgs...); err != nil {
		return err
	}
	sm := platform.Services()
	for _, svc := range p.Services {
		if sm.State(svc) != "unknown" {
			_ = sm.Action("enable", svc)
			_ = sm.Action("start", svc)
			if sm.State(svc) == "active" {
				continue
			}
		}
	}
	return nil
}
func Plan(p Preset) string {
	pm := platform.Packages()
	return fmt.Sprintf("Preset: %s\nCategory: %s\nPackages (%s): %s\nServices: %s", p.Name, p.Category, pm.Name, strings.Join(p.Packages[pm.Name], ", "), strings.Join(p.Services, ", "))
}
