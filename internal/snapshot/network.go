package snapshot

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SundySystems/sundy-toolkit/internal/platform"
	"github.com/SundySystems/sundy-toolkit/internal/util"
)

var NetworkComponents = []string{"Interfaces & addresses", "Routes", "Policy routing rules", "DNS", "Persistent network config", "Firewall", "Network sysctl", "WireGuard"}

func (s Store) SaveNetwork(name string, selected []int, automatic bool) (Meta, error) {
	if err := util.RequireRoot(); err != nil {
		return Meta{}, err
	}
	if name == "" {
		name = "network-" + time.Now().Format("20060102-150405")
	}
	comps := make([]string, 0, len(selected))
	for _, i := range selected {
		if i >= 0 && i < len(NetworkComponents) {
			comps = append(comps, NetworkComponents[i])
		}
	}
	m, err := s.Create(name, "network", comps, automatic)
	if err != nil {
		return m, err
	}
	d := s.SnapDir(m.ID)
	selectedMap := map[int]bool{}
	for _, i := range selected {
		selectedMap[i] = true
	}
	var captureErrors []error
	capture := func(file, cmd string, args ...string) {
		r := util.Run(8*time.Second, cmd, args...)
		if r.Code != 0 {
			m.Notes += fmt.Sprintf("%s unavailable: %s; ", file, r.Stderr)
			return
		}
		if err := os.WriteFile(filepath.Join(d, file), []byte(r.Stdout+"\n"), 0600); err != nil {
			captureErrors = append(captureErrors, err)
		}
	}
	if selectedMap[0] {
		capture("ip-address.txt", "ip", "-details", "addr", "show")
		capture("ip-link.txt", "ip", "-details", "link", "show")
	}
	if selectedMap[1] {
		capture("routes.txt", "ip", "route", "show", "table", "all")
	}
	if selectedMap[2] {
		capture("rules.txt", "ip", "rule", "show")
	}
	var configPaths []string
	if selectedMap[3] {
		configPaths = append(configPaths, "/etc/resolv.conf", "/etc/systemd/resolved.conf")
	}
	if selectedMap[4] {
		configPaths = append(configPaths, "/etc/NetworkManager/system-connections", "/etc/systemd/network", "/etc/netplan", "/etc/network/interfaces", "/etc/network/interfaces.d")
	}
	if selectedMap[5] {
		if util.Exists("nft") {
			capture("nftables.conf", "nft", "list", "ruleset")
		}
		if util.Exists("iptables-save") {
			capture("iptables.rules", "iptables-save")
		}
		configPaths = append(configPaths, "/etc/nftables.conf", "/etc/firewalld")
	}
	if selectedMap[6] {
		capture("sysctl-network.txt", "sysctl", "-a")
		configPaths = append(configPaths, "/etc/sysctl.conf", "/etc/sysctl.d")
	}
	if selectedMap[7] {
		configPaths = append(configPaths, "/etc/wireguard")
		if util.Exists("wg") {
			capture("wireguard.txt", "wg", "show", "all")
		}
	}
	if len(configPaths) > 0 {
		if err := ArchivePaths(filepath.Join(d, "config.tar.gz"), configPaths); err != nil {
			captureErrors = append(captureErrors, err)
		}
	}
	m.Notes += "Transient interface/route state is captured for diagnosis; persistent configuration files are restorable."
	if err := s.Update(m); err != nil {
		captureErrors = append(captureErrors, err)
	}
	return m, errors.Join(captureErrors...)
}

func (s Store) RestoreNetwork(idOrName string, apply bool) error {
	if err := util.RequireRoot(); err != nil {
		return err
	}
	m, err := s.Get(idOrName)
	if err != nil {
		return err
	}
	if m.Type != "network" {
		return fmt.Errorf("snapshot is %s, not network", m.Type)
	}
	archive := filepath.Join(s.SnapDir(m.ID), "config.tar.gz")
	if _, err := os.Stat(archive); err != nil {
		return fmt.Errorf("snapshot has no restorable persistent configuration")
	}
	if !apply {
		return fmt.Errorf("restore requires explicit --apply; this protects remote SSH sessions")
	}
	all := make([]int, len(NetworkComponents))
	for i := range all {
		all[i] = i
	}
	rollback, err := s.SaveNetwork("auto-before-restore-"+m.Name, all, true)
	if err != nil {
		return fmt.Errorf("could not create emergency rollback snapshot: %w", err)
	}
	if err := ExtractArchive(archive); err != nil {
		rollbackErr := ExtractArchive(filepath.Join(s.SnapDir(rollback.ID), "config.tar.gz"))
		return errors.Join(fmt.Errorf("restore failed; rollback attempted: %w", err), rollbackErr)
	}
	reloadErr := reloadNetwork()
	if reloadErr != nil {
		rb := filepath.Join(s.SnapDir(rollback.ID), "config.tar.gz")
		return errors.Join(fmt.Errorf("network reload failed; rollback attempted: %w", reloadErr), ExtractArchive(rb), reloadNetwork())
	}
	if !connectivityOK() {
		rb := filepath.Join(s.SnapDir(rollback.ID), "config.tar.gz")
		return errors.Join(fmt.Errorf("connectivity verification failed; automatic rollback attempted"), ExtractArchive(rb), reloadNetwork())
	}
	return nil
}

func reloadNetwork() error {
	if util.Exists("nmcli") && util.Run(5*time.Second, "nmcli", "-t", "-f", "RUNNING", "general").Stdout == "running" {
		r := util.Run(20*time.Second, "nmcli", "connection", "reload")
		if r.Code != 0 {
			return fmt.Errorf("nmcli reload: %s", r.Stderr)
		}
		devices := util.Run(8*time.Second, "nmcli", "-t", "-f", "DEVICE,STATE", "device", "status")
		for _, line := range strings.Split(devices.Stdout, "\n") {
			p := strings.SplitN(line, ":", 2)
			if len(p) != 2 || p[0] == "" || p[0] == "lo" || p[1] != "connected" {
				continue
			}
			reapply := util.Run(20*time.Second, "nmcli", "device", "reapply", p[0])
			if reapply.Code != 0 {
				return fmt.Errorf("nmcli reapply %s: %s", p[0], reapply.Stderr)
			}
		}
		return nil
	}
	if util.Exists("netplan") {
		r := util.Run(30*time.Second, "netplan", "apply")
		if r.Code != 0 {
			return fmt.Errorf("netplan apply: %s", r.Stderr)
		}
		return nil
	}
	if util.Exists("networkctl") && platform.Services().State("systemd-networkd") == "active" {
		r := util.Run(20*time.Second, "networkctl", "reload")
		if r.Code != 0 {
			return fmt.Errorf("networkctl reload: %s", r.Stderr)
		}
		return nil
	}
	return fmt.Errorf("no supported active network backend detected")
}
func connectivityOK() bool {
	if !util.Exists("ip") {
		return false
	}
	r := util.Run(4*time.Second, "ip", "route", "show", "default")
	if strings.TrimSpace(r.Stdout) == "" {
		return false
	}
	if util.Exists("ping") {
		p := util.Run(6*time.Second, "ping", "-c", "1", "-W", "3", "1.1.1.1")
		return p.Code == 0
	}
	return true
}
