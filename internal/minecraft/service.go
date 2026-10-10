package minecraft

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/SundySystems/sundy-toolkit/internal/apps"
	"github.com/SundySystems/sundy-toolkit/internal/platform"
	"github.com/SundySystems/sundy-toolkit/internal/util"
)

func unitQuote(value string) string  { return strconv.Quote(strings.ReplaceAll(value, "%", "%%")) }
func execQuote(value string) string  { return unitQuote(strings.ReplaceAll(value, "$", "$$")) }
func shellQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }

func systemdUnit(a apps.App, exe, state, run string, restart bool) string {
	policy := "no"
	if restart {
		policy = "on-failure"
	}
	return fmt.Sprintf(`[Unit]
Description=Sundy Minecraft - %s
After=network.target
StartLimitIntervalSec=120
StartLimitBurst=5

[Service]
Type=simple
WorkingDirectory=%s
Environment=%s
Environment=%s
ExecStart=%s runtime minecraft %s
Restart=%s
RestartSec=5
KillMode=mixed
TimeoutStopSec=120
LimitNOFILE=1048576
UMask=0027

[Install]
WantedBy=multi-user.target
`, a.Name, strings.ReplaceAll(a.Directory, "%", "%%"), unitQuote("SUNDY_STATE_DIR="+state), unitQuote("SUNDY_RUNTIME_DIR="+run), execQuote(exe), execQuote(a.Name), policy)
}

func openrcScript(a apps.App, exe, state, run string, restart bool) string {
	supervision := "command_background=true\npidfile=/run/" + a.Service + ".pid\n"
	if restart {
		supervision = "supervisor=supervise-daemon\nrespawn_delay=5\nrespawn_max=5\nrespawn_period=120\n"
	}
	return fmt.Sprintf("#!/sbin/openrc-run\nname=%s\ncommand=%s\ncommand_args=%s\ndirectory=%s\nexport SUNDY_STATE_DIR=%s\nexport SUNDY_RUNTIME_DIR=%s\n%sretry=TERM/110/KILL/5\numask=0027\ndepend() { need net; }\n",
		shellQuote("Sundy Minecraft "+a.Name), shellQuote(shellQuote(exe)), shellQuote("runtime minecraft "+shellQuote(a.Name)), shellQuote(shellQuote(a.Directory)), shellQuote(state), shellQuote(run), supervision)
}

func installService(a apps.App, autostart, restart bool) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}
	if strings.Contains(exe, "/go-build") {
		return fmt.Errorf("install a persistent sundy binary first; go run creates a temporary executable")
	}
	state, err := filepath.Abs(util.StateDir())
	if err != nil {
		return err
	}
	run, err := filepath.Abs(runtimeDir())
	if err != nil {
		return err
	}
	sm := platform.Services()
	switch sm.Init {
	case "systemd":
		if err := util.AtomicWrite(filepath.Join("/etc/systemd/system", a.Service+".service"), []byte(systemdUnit(a, exe, state, run, restart)), 0644); err != nil {
			return err
		}
		r := util.Run(10*time.Second, "systemctl", "daemon-reload")
		if r.Code != 0 {
			return fmt.Errorf("systemctl daemon-reload: %s", r.Stderr)
		}
	case "openrc":
		if err := util.AtomicWrite(filepath.Join("/etc/init.d", a.Service), []byte(openrcScript(a, exe, state, run, restart)), 0755); err != nil {
			return err
		}
	default:
		return fmt.Errorf("Minecraft service installation requires systemd or OpenRC")
	}
	if autostart {
		if err := sm.Action("enable", a.Service); err != nil {
			return fmt.Errorf("enable service: %w", err)
		}
		if err := sm.Action("start", a.Service); err != nil {
			return fmt.Errorf("start service: %w", err)
		}
		// Init systems report success before Java has even loaded the JAR.
		time.Sleep(2 * time.Second)
		if state := sm.State(a.Service); state != "active" {
			return fmt.Errorf("service became %s after starting; see %s", state, filepath.Join(util.StateDir(), "minecraft", a.Name, "console.log"))
		}
	}
	return nil
}
