package audit

import (
	"fmt"
	"strings"
	"time"

	"github.com/SundySystems/sundy-toolkit/internal/util"
)

type RepairResult struct {
	FindingID string
	Success   bool
	Message   string
}

func Repair(f Finding) RepairResult {
	if !f.Repairable {
		return RepairResult{f.ID, false, "finding does not have an automatic repair"}
	}
	if err := util.RequireRoot(); err != nil {
		return RepairResult{f.ID, false, err.Error()}
	}
	switch f.RepairAction {
	case "systemd-reset-restart":
		if f.Target == "" || strings.HasPrefix(f.Target, "-") || !strings.HasSuffix(f.Target, ".service") {
			return RepairResult{f.ID, false, "repair target must be a service unit"}
		}
		var check []string
		if strings.HasPrefix(f.Target, "nginx.") {
			check = []string{"nginx", "-t"}
		}
		if f.Target == "ssh.service" || f.Target == "sshd.service" {
			check = []string{"sshd", "-t"}
		}
		if len(check) > 0 && util.Exists(check[0]) {
			r := util.Run(8*time.Second, check[0], check[1:]...)
			if r.Code != 0 {
				return RepairResult{f.ID, false, "configuration validation failed: " + nonempty(r.Stderr, r.Stdout)}
			}
		}
		reset := util.Run(8*time.Second, "systemctl", "reset-failed", f.Target)
		if reset.Code != 0 {
			return RepairResult{f.ID, false, "reset-failed failed: " + reset.Stderr}
		}
		restart := util.Run(30*time.Second, "systemctl", "restart", f.Target)
		if restart.Code != 0 {
			return RepairResult{f.ID, false, "restart failed: " + nonempty(restart.Stderr, restart.Stdout)}
		}
		verify := util.Run(8*time.Second, "systemctl", "is-active", f.Target)
		if verify.Code == 0 && verify.Stdout == "active" {
			return RepairResult{f.ID, true, fmt.Sprintf("%s recovered and is active", f.Target)}
		}
		return RepairResult{f.ID, false, fmt.Sprintf("%s did not become active after the controlled restart", f.Target)}
	}
	return RepairResult{f.ID, false, "unknown repair action"}
}
func nonempty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
