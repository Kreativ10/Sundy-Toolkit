package report

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/SundySystems/sundy-toolkit/internal/audit"
	"github.com/SundySystems/sundy-toolkit/internal/network"
	"github.com/SundySystems/sundy-toolkit/internal/snapshot"
	"github.com/SundySystems/sundy-toolkit/internal/systeminfo"
	"github.com/SundySystems/sundy-toolkit/internal/util"
)

func Create(dst string, anonymous bool) (string, error) {
	if dst == "" {
		dst = fmt.Sprintf("sundy-report-%s.tar.gz", time.Now().Format("20060102-150405"))
	}
	abs, err := filepath.Abs(dst)
	if err != nil {
		return "", err
	}
	tmp, err := os.MkdirTemp("", "sundy-report-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	ov := systeminfo.Collect()
	if anonymous {
		ov.Hostname = "redacted"
		ov.IPv4 = "redacted"
		ov.IPv6 = "redacted"
	}
	if err := writeSafe(tmp, "system.json", ov.JSON()+"\n", anonymous); err != nil {
		return "", err
	}
	ar := audit.Run(true)
	if err := writeSafe(tmp, "audit.json", audit.JSON(ar)+"\n", anonymous); err != nil {
		return "", err
	}
	ns := network.Inspect()
	if err := writeSafe(tmp, "network-interfaces.txt", ns.Interfaces, anonymous); err != nil {
		return "", err
	}
	if err := writeSafe(tmp, "routes.txt", ns.Routes, anonymous); err != nil {
		return "", err
	}
	if err := writeSafe(tmp, "rules.txt", ns.Rules, anonymous); err != nil {
		return "", err
	}
	if err := writeSafe(tmp, "listening.txt", ns.Listening, anonymous); err != nil {
		return "", err
	}
	cmds := map[string][]string{"processes.txt": {"ps", "auxww"}, "mounts.txt": {"mount"}, "df.txt": {"df", "-hT"}, "lsblk.txt": {"lsblk", "-o", "NAME,TYPE,SIZE,FSTYPE,MOUNTPOINTS"}, "kernel.txt": {"dmesg", "--level=err,warn"}}
	for file, c := range cmds {
		if util.Exists(c[0]) {
			r := util.Run(10*time.Second, c[0], c[1:]...)
			if err := writeSafe(tmp, file, r.Stdout+"\n"+r.Stderr, anonymous); err != nil {
				return "", err
			}
		}
	}
	if util.Exists("systemctl") {
		r := util.Run(8*time.Second, "systemctl", "--failed", "--no-pager")
		if err := writeSafe(tmp, "failed-services.txt", r.Stdout+"\n"+r.Stderr, anonymous); err != nil {
			return "", err
		}
	}
	if err := snapshot.ArchivePaths(abs, []string{tmp}); err != nil {
		return "", err
	}
	return abs, nil
}
func writeSafe(dir, name, content string, anonymous bool) error {
	content = redactSecrets(content)
	if anonymous {
		content = redactIPs(content)
		if host, err := os.Hostname(); err == nil && host != "" {
			content = strings.ReplaceAll(content, host, "redacted")
		}
	}
	return os.WriteFile(filepath.Join(dir, name), []byte(content), 0600)
}

var secretRx = regexp.MustCompile(`(?i)((?:password|passwd|token|secret|api[_-]?key|authorization)["']?\s*[:=]\s*)(?:"[^"\n]*"|'[^'\n]*'|[^\s,;]+)`)
var bearerRx = regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._~+/-]+=*`)
var cliSecretRx = regexp.MustCompile(`(?i)(--(?:password|passwd|token|secret|api[_-]?key)\s+)(?:"[^"\n]*"|'[^'\n]*'|[^\s]+)`)

func redactSecrets(s string) string {
	s = bearerRx.ReplaceAllString(s, "Bearer [REDACTED]")
	s = secretRx.ReplaceAllStringFunc(s, func(match string) string {
		prefix := secretRx.FindStringSubmatch(match)[1]
		value := strings.TrimPrefix(match, prefix)
		if strings.HasPrefix(value, "\"") {
			return prefix + "\"[REDACTED]\""
		}
		if strings.HasPrefix(value, "'") {
			return prefix + "'[REDACTED]'"
		}
		return prefix + "[REDACTED]"
	})
	s = cliSecretRx.ReplaceAllString(s, "${1}[REDACTED]")
	return bearerRx.ReplaceAllString(s, "Bearer [REDACTED]")
}

var ipv4Rx = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
var ipv6Rx = regexp.MustCompile(`[0-9A-Fa-f:]*:[0-9A-Fa-f:]+(?:%[A-Za-z0-9_.-]+)?`)

func redactIPs(s string) string {
	s = ipv4Rx.ReplaceAllStringFunc(s, func(v string) string {
		ip := net.ParseIP(v)
		if ip == nil || ip.IsLoopback() {
			return v
		}
		return "x.x.x.x"
	})
	return ipv6Rx.ReplaceAllStringFunc(s, func(v string) string {
		address, _, _ := strings.Cut(v, "%")
		ip := net.ParseIP(address)
		if ip == nil || ip.IsLoopback() {
			return v
		}
		return "redacted-ipv6"
	})
}
