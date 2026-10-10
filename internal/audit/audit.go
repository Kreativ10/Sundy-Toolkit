package audit

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/SundySystems/sundy-toolkit/internal/network"
	"github.com/SundySystems/sundy-toolkit/internal/platform"
	"github.com/SundySystems/sundy-toolkit/internal/util"
)

type Severity string

const (
	Info     Severity = "INFO"
	Low      Severity = "LOW"
	Medium   Severity = "MEDIUM"
	High     Severity = "HIGH"
	Critical Severity = "CRITICAL"
)

type Finding struct {
	ID             string   `json:"id"`
	Category       string   `json:"category"`
	Severity       Severity `json:"severity"`
	Title          string   `json:"title"`
	Explanation    string   `json:"explanation"`
	Evidence       string   `json:"evidence,omitempty"`
	Recommendation string   `json:"recommendation,omitempty"`
	Repairable     bool     `json:"repairable"`
	RepairAction   string   `json:"repair_action,omitempty"`
	Target         string   `json:"target,omitempty"`
}
type Result struct {
	Started  time.Time `json:"started"`
	Duration string    `json:"duration"`
	Score    int       `json:"score"`
	Findings []Finding `json:"findings"`
}

func Run(full bool) Result {
	start := time.Now()
	var f []Finding
	f = append(f, checkDisk()...)
	f = append(f, checkMemory()...)
	f = append(f, checkServices()...)
	f = append(f, checkSSH()...)
	f = append(f, checkFirewall()...)
	f = append(f, checkNetwork()...)
	if full {
		f = append(f, checkKernel()...)
		f = append(f, checkStorageHealth()...)
		f = append(f, checkTimeSync()...)
	}
	return Result{Started: start, Duration: time.Since(start).Round(time.Millisecond).String(), Score: score(f), Findings: f}
}
func JSON(v any) string { b, _ := json.MarshalIndent(v, "", "  "); return string(b) }
func score(f []Finding) int {
	s := 100
	for _, x := range f {
		switch x.Severity {
		case Critical:
			s -= 25
		case High:
			s -= 15
		case Medium:
			s -= 7
		case Low:
			s -= 2
		}
	}
	if s < 0 {
		s = 0
	}
	return s
}
func checkDisk() []Finding {
	var st syscall.Statfs_t
	if syscall.Statfs("/", &st) != nil {
		return nil
	}
	used := 100 - float64(st.Bavail)*100/float64(st.Blocks)
	if used >= 95 {
		return []Finding{{"disk-root-critical", "storage", Critical, "Root filesystem is almost full", fmt.Sprintf("The root filesystem is %.1f%% full. Services may fail to write logs, databases or temporary files.", used), "", "Free disk space before restarting write-heavy services.", false, "", ""}}
	}
	if used >= 85 {
		return []Finding{{"disk-root-high", "storage", Medium, "Root filesystem usage is high", fmt.Sprintf("The root filesystem is %.1f%% full.", used), "", "Review large files, logs, containers and package caches.", false, "", ""}}
	}
	return nil
}
func memvals() map[string]uint64 {
	m := map[string]uint64{}
	f, e := os.Open("/proc/meminfo")
	if e != nil {
		return m
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		p := strings.Fields(s.Text())
		if len(p) >= 2 {
			v, _ := strconv.ParseUint(p[1], 10, 64)
			m[strings.TrimSuffix(p[0], ":")] = v
		}
	}
	return m
}
func checkMemory() []Finding {
	m := memvals()
	st, sf := m["SwapTotal"], m["SwapFree"]
	if st > 0 {
		used := float64(st-sf) * 100 / float64(st)
		if used > 90 {
			return []Finding{{"swap-high", "performance", Medium, "Swap usage is very high", fmt.Sprintf("Swap is %.1f%% used. This may be normal after past memory pressure, but can also indicate active pressure.", used), "", "Check vmstat, top/btop and memory-heavy processes before clearing swap.", false, "", ""}}
		}
	}
	return nil
}
func checkServices() []Finding {
	if platform.Detect().Init != "systemd" || !util.Exists("systemctl") {
		return nil
	}
	r := util.Run(8*time.Second, "systemctl", "--failed", "--type=service", "--no-legend", "--plain")
	if r.Code != 0 && r.Stdout == "" {
		return nil
	}
	var out []Finding
	for _, line := range strings.Split(strings.TrimSpace(r.Stdout), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		unit := fields[0]
		status := util.Run(4*time.Second, "systemctl", "status", unit, "--no-pager", "-n", "8")
		ev := status.Stdout
		if ev == "" {
			ev = status.Stderr
		}
		out = append(out, Finding{ID: "service-" + sanitize(unit), Category: "services", Severity: High, Title: unit + " is in failed state", Explanation: serviceExplanation(unit), Evidence: tail(ev, 8), Recommendation: "Inspect the root cause. Sundy can reset the failed state, attempt one restart, and verify that the unit becomes active.", Repairable: true, RepairAction: "systemd-reset-restart", Target: unit})
	}
	return out
}
func serviceExplanation(unit string) string {
	base := "systemd reports this unit as failed. Blind restart loops are avoided: Sundy will only perform one controlled recovery attempt and verify the result."
	switch {
	case strings.HasPrefix(unit, "nginx") && util.Exists("nginx"):
		r := util.Run(5*time.Second, "nginx", "-t")
		if r.Code != 0 {
			return "NGINX configuration validation currently fails: " + strings.TrimSpace(r.Stderr)
		}
	case strings.HasPrefix(unit, "ssh") && util.Exists("sshd"):
		r := util.Run(5*time.Second, "sshd", "-t")
		if r.Code != 0 {
			return "OpenSSH configuration validation currently fails: " + strings.TrimSpace(r.Stderr)
		}
	}
	return base
}
func checkSSH() []Finding {
	path := "/etc/ssh/sshd_config"
	b, e := os.ReadFile(path)
	if e != nil {
		return nil
	}
	txt := strings.ToLower(string(b))
	var out []Finding
	if activeDirective(txt, "passwordauthentication", "yes") {
		out = append(out, Finding{"ssh-password-auth", "security", Medium, "SSH password authentication is enabled", "Password authentication increases exposure to credential guessing when SSH is reachable from untrusted networks.", path, "Prefer key authentication when operationally possible. Do not disable password auth until key access has been verified.", false, "", ""})
	}
	if activeDirective(txt, "permitrootlogin", "yes") {
		out = append(out, Finding{"ssh-root-login", "security", Medium, "Direct SSH root login is enabled", "Direct root login increases the impact of credential compromise.", path, "Prefer a named administrative account with sudo and tested key access.", false, "", ""})
	}
	return out
}
func activeDirective(txt, key, val string) bool {
	txt = strings.ToLower(txt)
	key = strings.ToLower(key)
	val = strings.ToLower(val)
	for _, l := range strings.Split(txt, "\n") {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		p := strings.Fields(l)
		if len(p) >= 2 && p[0] == key && p[1] == val {
			return true
		}
	}
	return false
}
func checkFirewall() []Finding {
	if util.Exists("nft") {
		r := util.Run(5*time.Second, "nft", "list", "ruleset")
		if r.Code == 0 && strings.TrimSpace(r.Stdout) == "" {
			return []Finding{{"firewall-empty", "security", Low, "nftables has no active rules", "No nftables rules were detected. This can be intentional on hosts protected elsewhere.", "", "Review exposure with `sundy network` before enabling a firewall.", false, "", ""}}
		}
		return nil
	}
	if util.Exists("ufw") {
		r := util.Run(4*time.Second, "ufw", "status")
		if strings.Contains(strings.ToLower(r.Stdout), "inactive") {
			return []Finding{{"ufw-inactive", "security", Low, "UFW is inactive", "UFW is installed but inactive.", "", "This is informational if another firewall or upstream ACL is used.", false, "", ""}}
		}
	}
	return nil
}
func checkNetwork() []Finding {
	var out []Finding
	for i, c := range network.Doctor() {
		if c.Status == "ok" {
			continue
		}
		sev := Medium
		if c.Status == "warn" {
			sev = Low
		}
		out = append(out, Finding{fmt.Sprintf("network-%d", i), "network", sev, c.Name, c.Detail, "", c.Suggestion, false, "", ""})
	}
	return out
}
func checkKernel() []Finding {
	if _, e := os.Stat("/var/run/reboot-required"); e == nil {
		return []Finding{{"reboot-required", "system", Low, "System reboot is required", "The distribution has marked the host as requiring a reboot, commonly after kernel or core library updates.", "/var/run/reboot-required", "Schedule a controlled reboot.", false, "", ""}}
	}
	return nil
}
func checkStorageHealth() []Finding {
	var out []Finding
	if util.Exists("nvme") {
		r := util.Run(8*time.Second, "nvme", "list")
		if r.Code != 0 {
			out = append(out, Finding{"nvme-query", "storage", Low, "NVMe health query failed", "nvme-cli could not enumerate devices.", r.Stderr, "Check permissions or device availability.", false, "", ""})
		}
	}
	return out
}
func checkTimeSync() []Finding {
	if util.Exists("timedatectl") {
		r := util.Run(4*time.Second, "timedatectl", "show", "-p", "NTPSynchronized", "--value")
		if strings.TrimSpace(r.Stdout) == "no" {
			return []Finding{{"time-not-synced", "system", Medium, "System clock is not synchronized", "timedatectl reports that NTP synchronization is not currently established.", "", "Check chronyd/systemd-timesyncd and outbound NTP connectivity.", false, "", ""}}
		}
	}
	return nil
}
func sanitize(s string) string { return strings.NewReplacer(".", "-", "@", "-", "/", "-").Replace(s) }
func tail(s string, n int) string {
	p := strings.Split(strings.TrimSpace(s), "\n")
	if len(p) > n {
		p = p[len(p)-n:]
	}
	return strings.Join(p, "\n")
}
