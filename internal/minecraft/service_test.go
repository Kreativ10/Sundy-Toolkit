package minecraft

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SundySystems/sundy-toolkit/internal/apps"
)

func TestServiceEscaping(t *testing.T) {
	app := apps.App{Name: "survival", Directory: `/srv/mine craft/percent%/dollar$`, Service: "sundy-minecraft-survival"}
	unit := systemdUnit(app, `/opt/sundy tools/$binary%`, "/var/lib/sundy custom", "/run/sundy", true)
	for _, want := range []string{`WorkingDirectory=/srv/mine craft/percent%%/dollar$`, `ExecStart="/opt/sundy tools/$$binary%%" runtime minecraft "survival"`, "KillMode=mixed", "TimeoutStopSec=120", "Restart=on-failure", `Environment="SUNDY_STATE_DIR=/var/lib/sundy custom"`} {
		if !strings.Contains(unit, want) {
			t.Errorf("missing %q in %s", want, unit)
		}
	}
	script := openrcScript(app, "/opt/sundy's tools/sundy", "/var/lib/state's", "/run/minecraft", true)
	cmd := exec.Command("sh", "-n")
	cmd.Stdin = strings.NewReader(script)
	if data, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("OpenRC shell syntax: %s %v", data, err)
	}
	if !strings.Contains(script, "supervisor=supervise-daemon") {
		t.Fatal("OpenRC lacks crash supervision")
	}
	if !strings.Contains(openrcScript(app, "sundy", "/state", "/run", false), "command_background=true") {
		t.Fatal("foreground OpenRC daemon would block startup")
	}
}

func TestSystemdUnitVerification(t *testing.T) {
	if _, err := exec.LookPath("systemd-analyze"); err != nil {
		t.Skip("systemd-analyze not installed")
	}
	app := apps.App{Name: "survival", Directory: filepath.Join(t.TempDir(), "with spaces")}
	file := filepath.Join(t.TempDir(), "sundy-minecraft-survival.service")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(systemdUnit(app, exe, t.TempDir(), t.TempDir(), true)), 0644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("systemd-analyze", "verify", file).CombinedOutput(); err != nil {
		t.Fatalf("invalid unit: %v\n%s", err, out)
	}
}
