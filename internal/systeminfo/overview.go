package systeminfo

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/SundySystems/sundy-toolkit/internal/platform"
	"github.com/SundySystems/sundy-toolkit/internal/util"
)

type Overview struct {
	Hostname  string        `json:"hostname"`
	Platform  platform.Info `json:"platform"`
	Uptime    string        `json:"uptime"`
	CPU       string        `json:"cpu"`
	Cores     int           `json:"cores"`
	Memory    string        `json:"memory"`
	Swap      string        `json:"swap"`
	RootDisk  string        `json:"root_disk"`
	Load      string        `json:"load"`
	IPv4      string        `json:"ipv4"`
	IPv6      string        `json:"ipv6"`
	Listening int           `json:"listening_ports"`
}

func Collect() Overview {
	host, _ := os.Hostname()
	p := platform.Detect()
	return Overview{Hostname: host, Platform: p, Uptime: uptime(), CPU: cpuModel(), Cores: cpuCores(), Memory: memoryLine(), Swap: swapLine(), RootDisk: rootDisk(), Load: load(), IPv4: firstIP("-4"), IPv6: firstIP("-6"), Listening: listeningCount()}
}
func (o Overview) JSON() string { b, _ := json.MarshalIndent(o, "", "  "); return string(b) }

func uptime() string {
	b, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return "unknown"
	}
	f, _ := strconv.ParseFloat(strings.Fields(string(b))[0], 64)
	d := time.Duration(f) * time.Second
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	return fmt.Sprintf("%dd %02dh %02dm", days, hours, mins)
}
func cpuModel() string {
	f, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return "unknown"
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := s.Text()
		if strings.HasPrefix(line, "model name") || strings.HasPrefix(line, "Hardware") {
			if i := strings.Index(line, ":"); i >= 0 {
				return strings.TrimSpace(line[i+1:])
			}
		}
	}
	return "unknown"
}
func cpuCores() int {
	f, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return 0
	}
	defer f.Close()
	n := 0
	s := bufio.NewScanner(f)
	for s.Scan() {
		if strings.HasPrefix(s.Text(), "processor") {
			n++
		}
	}
	return n
}
func memInfo() map[string]uint64 {
	m := map[string]uint64{}
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return m
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		parts := strings.Fields(s.Text())
		if len(parts) >= 2 {
			v, _ := strconv.ParseUint(parts[1], 10, 64)
			m[strings.TrimSuffix(parts[0], ":")] = v * 1024
		}
	}
	return m
}
func human(v uint64) string {
	const g = 1024 * 1024 * 1024
	const m = 1024 * 1024
	if v >= g {
		return fmt.Sprintf("%.1f GB", float64(v)/g)
	}
	return fmt.Sprintf("%.1f MB", float64(v)/m)
}
func memoryLine() string {
	m := memInfo()
	total := m["MemTotal"]
	avail := m["MemAvailable"]
	used := total - avail
	return fmt.Sprintf("%s / %s", human(used), human(total))
}
func swapLine() string {
	m := memInfo()
	total := m["SwapTotal"]
	free := m["SwapFree"]
	return fmt.Sprintf("%s / %s", human(total-free), human(total))
}
func rootDisk() string {
	var s syscall.Statfs_t
	if syscall.Statfs("/", &s) != nil {
		return "unknown"
	}
	total := s.Blocks * uint64(s.Bsize)
	free := s.Bavail * uint64(s.Bsize)
	return fmt.Sprintf("%s / %s", human(total-free), human(total))
}
func load() string {
	b, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return "unknown"
	}
	p := strings.Fields(string(b))
	if len(p) < 3 {
		return "unknown"
	}
	return strings.Join(p[:3], " ")
}
func firstIP(fam string) string {
	if !util.Exists("ip") {
		return "unknown"
	}
	r := util.Run(3*time.Second, "ip", fam, "-o", "addr", "show", "scope", "global")
	if r.Code != 0 {
		return "none"
	}
	for _, line := range strings.Split(r.Stdout, "\n") {
		p := strings.Fields(line)
		for i, x := range p {
			if x == "inet" || x == "inet6" {
				if i+1 < len(p) {
					return strings.Split(p[i+1], "/")[0]
				}
			}
		}
	}
	return "none"
}
func listeningCount() int {
	if !util.Exists("ss") {
		return 0
	}
	r := util.Run(3*time.Second, "ss", "-H", "-lntup")
	if r.Code != 0 || r.Stdout == "" {
		return 0
	}
	return len(strings.Split(strings.TrimSpace(r.Stdout), "\n"))
}
