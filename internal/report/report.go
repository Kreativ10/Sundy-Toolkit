package report

import (
	"fmt"
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
	_ = os.WriteFile(filepath.Join(tmp, "system.json"), []byte(ov.JSON()+"\n"), 0600)
	ar := audit.Run(true)
	_ = os.WriteFile(filepath.Join(tmp, "audit.json"), []byte(audit.JSON(ar)+"\n"), 0600)
	ns := network.Inspect()
	writeSafe(tmp, "network-interfaces.txt", ns.Interfaces, anonymous)
	writeSafe(tmp, "routes.txt", ns.Routes, anonymous)
	writeSafe(tmp, "rules.txt", ns.Rules, anonymous)
	writeSafe(tmp, "listening.txt", ns.Listening, anonymous)
	cmds := map[string][]string{"processes.txt": {"ps", "auxww"}, "mounts.txt": {"mount"}, "df.txt": {"df", "-hT"}, "lsblk.txt": {"lsblk", "-o", "NAME,TYPE,SIZE,FSTYPE,MOUNTPOINTS"}, "kernel.txt": {"dmesg", "--level=err,warn"}}
	for file, c := range cmds {
		if util.Exists(c[0]) {
			r := util.Run(10*time.Second, c[0], c[1:]...)
			writeSafe(tmp, file, r.Stdout+"\n"+r.Stderr, anonymous)
		}
	}
	if util.Exists("systemctl") {
		r := util.Run(8*time.Second, "systemctl", "--failed", "--no-pager")
		writeSafe(tmp, "failed-services.txt", r.Stdout+"\n"+r.Stderr, anonymous)
	}
	if err := snapshot.ArchivePaths(abs, []string{tmp}); err != nil {
		return "", err
	}
	return abs, nil
}
func writeSafe(dir, name, content string, anonymous bool) {
	content = redactSecrets(content)
	if anonymous {
		content = redactIPs(content)
	}
	_ = os.WriteFile(filepath.Join(dir, name), []byte(content), 0600)
}

var secretRx = regexp.MustCompile(`(?i)(password|passwd|token|secret|api[_-]?key|authorization)(\s*[:=]\s*)([^\s]+)`)

func redactSecrets(s string) string { return secretRx.ReplaceAllString(s, "$1$2[REDACTED]") }

var ipv4Rx = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)

func redactIPs(s string) string {
	return ipv4Rx.ReplaceAllStringFunc(s, func(v string) string {
		if strings.HasPrefix(v, "127.") {
			return v
		}
		return "x.x.x.x"
	})
}
