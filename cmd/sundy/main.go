package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/SundySystems/sundy-toolkit/internal/apps"
	"github.com/SundySystems/sundy-toolkit/internal/audit"
	installpkg "github.com/SundySystems/sundy-toolkit/internal/install"
	"github.com/SundySystems/sundy-toolkit/internal/minecraft"
	"github.com/SundySystems/sundy-toolkit/internal/network"
	"github.com/SundySystems/sundy-toolkit/internal/platform"
	"github.com/SundySystems/sundy-toolkit/internal/report"
	"github.com/SundySystems/sundy-toolkit/internal/selfupdate"
	"github.com/SundySystems/sundy-toolkit/internal/snapshot"
	"github.com/SundySystems/sundy-toolkit/internal/systeminfo"
	"github.com/SundySystems/sundy-toolkit/internal/ui"
	"github.com/SundySystems/sundy-toolkit/internal/util"
)

var version = "dev"

func main() {
	if len(os.Args) == 1 {
		tui()
		return
	}
	cmd := os.Args[1]
	args := os.Args[2:]
	var err error
	switch cmd {
	case "tui":
		tui()
		return
	case "overview":
		err = cmdOverview(args)
	case "audit":
		err = cmdAudit(args)
	case "doctor":
		err = cmdDoctor(args)
	case "network":
		err = cmdNetwork(args)
	case "snapshots":
		err = cmdSnapshots(args)
	case "snapshot":
		err = cmdSnapshot(args)
	case "install":
		err = cmdInstall(args)
	case "apps":
		err = cmdApps(args)
	case "minecraft":
		err = cmdMinecraft(args)
	case "service", "services":
		err = cmdService(args)
	case "report":
		err = cmdReport(args)
	case "update":
		ui.Info("Downloading and verifying the latest release...")
		err = selfupdate.Run()
		if err == nil {
			ui.Success("Sundy Toolkit updated successfully")
		}
	case "runtime":
		err = cmdRuntime(args)
	case "version", "--version", "-v":
		fmt.Println("Sundy Toolkit", version)
		return
	case "help", "--help", "-h":
		usage()
		return
	default:
		err = fmt.Errorf("unknown command %q", cmd)
	}
	if err != nil {
		ui.Error(err.Error())
		os.Exit(1)
	}
}

func usage() {
	fmt.Print(ui.Logo())
	fmt.Printf(`
Sundy Toolkit %s — Linux administration, diagnostics, deployment and recovery.

Usage:
  sundy                       Interactive dashboard
  sundy overview [--json]     System overview
  sundy audit [--full]        System audit
  sundy audit --fix           Select and repair supported findings
  sundy doctor                Focused health diagnostics
  sundy network info          Network state
  sundy network doctor        Connectivity diagnostics
  sundy network save          Save selected network configuration
  sundy network restore NAME --apply
  sundy snapshots             List snapshots
  sundy snapshot show NAME    Inspect a snapshot
  sundy install               Installation center
  sundy install minecraft     Minecraft direct/console setup
  sundy install pterodactyl   Pterodactyl Panel/Wings setup
  sundy apps                  Managed applications
  sundy minecraft console NAME
  sundy service ACTION NAME   status/start/stop/restart/enable/disable
  sundy report [--anonymous]  Create support bundle
  sundy update                Update from signed release checksums

`, version)
}

func tui() {
	fmt.Print(ui.Logo())
	ov := systeminfo.Collect()
	ui.Box("Sundy Toolkit "+version, []string{ui.KeyValue("Host", ov.Hostname), ui.KeyValue("OS", ov.Platform.Name), ui.KeyValue("Kernel", ov.Platform.Kernel), ui.KeyValue("CPU", ov.CPU), ui.KeyValue("RAM", ov.Memory), ui.KeyValue("Disk /", ov.RootDisk), ui.KeyValue("Load", ov.Load), ui.KeyValue("IPv4", ov.IPv4)})
	for {
		n, err := ui.Menu("Control Center", []string{"System overview", "System audit & repair", "Network diagnostics", "Snapshots / recovery", "Install Center", "Managed applications", "Minecraft manager", "Services", "Create support report", "Exit"})
		if err != nil {
			ui.Error(err.Error())
			return
		}
		switch n {
		case 0:
			_ = cmdOverview(nil)
		case 1:
			_ = cmdAudit([]string{"--full", "--fix"})
		case 2:
			_ = networkMenu()
		case 3:
			_ = snapshotMenu()
		case 4:
			_ = cmdInstall(nil)
		case 5:
			_ = cmdApps(nil)
		case 6:
			_ = minecraftMenu()
		case 7:
			_ = servicesMenu()
		case 8:
			_ = cmdReport(nil)
		case 9:
			return
		}
		fmt.Println()
	}
}

func cmdOverview(args []string) error {
	fs := flag.NewFlagSet("overview", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	o := systeminfo.Collect()
	if *jsonOut {
		fmt.Println(o.JSON())
		return nil
	}
	ui.Header("System Overview")
	lines := []string{ui.KeyValue("Hostname", o.Hostname), ui.KeyValue("OS", o.Platform.Name), ui.KeyValue("Kernel", o.Platform.Kernel), ui.KeyValue("Architecture", o.Platform.Arch), ui.KeyValue("Virtualization", o.Platform.Container), ui.KeyValue("Package manager", o.Platform.Package), ui.KeyValue("Init", o.Platform.Init), ui.KeyValue("Uptime", o.Uptime), ui.KeyValue("CPU", o.CPU), ui.KeyValue("CPU cores", strconv.Itoa(o.Cores)), ui.KeyValue("Memory", o.Memory), ui.KeyValue("Swap", o.Swap), ui.KeyValue("Root disk", o.RootDisk), ui.KeyValue("Load", o.Load), ui.KeyValue("IPv4", o.IPv4), ui.KeyValue("IPv6", o.IPv6), ui.KeyValue("Listening sockets", strconv.Itoa(o.Listening))}
	ui.Box("Host", lines)
	return nil
}

func cmdAudit(args []string) error {
	fs := flag.NewFlagSet("audit", flag.ContinueOnError)
	full := fs.Bool("full", false, "")
	jsonOut := fs.Bool("json", false, "")
	fix := fs.Bool("fix", false, "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	ui.Info("Running audit...")
	r := audit.Run(*full)
	if *jsonOut {
		fmt.Println(audit.JSON(r))
		return nil
	}
	ui.Header(fmt.Sprintf("Audit — health score %d/100", r.Score))
	if len(r.Findings) == 0 {
		ui.Success("No actionable findings detected by the current checks.")
		return nil
	}
	for i, f := range r.Findings {
		sev := string(f.Severity)
		fmt.Printf("\n%s %s %s\n", ui.C(ui.Orange, fmt.Sprintf("[%02d]", i+1)), severityColor(f.Severity, sev), ui.C(ui.Bold, f.Title))
		fmt.Println("   " + f.Explanation)
		if f.Evidence != "" {
			fmt.Println(ui.C(ui.Gray, "   Evidence: ") + oneLine(f.Evidence, 220))
		}
		if f.Recommendation != "" {
			fmt.Println(ui.C(ui.Gray, "   Recommendation: ") + f.Recommendation)
		}
		if f.Repairable {
			fmt.Println(ui.C(ui.Green, "   Automatic repair available"))
		}
	}
	if *fix {
		return interactiveRepair(r.Findings)
	}
	return nil
}

func severityColor(s audit.Severity, v string) string {
	switch s {
	case audit.Critical, audit.High:
		return ui.C(ui.Red, v)
	case audit.Medium:
		return ui.C(ui.Yellow, v)
	default:
		return ui.C(ui.Gray, v)
	}
}
func interactiveRepair(findings []audit.Finding) error {
	var repairable []audit.Finding
	var labels []string
	for _, f := range findings {
		if f.Repairable {
			repairable = append(repairable, f)
			labels = append(labels, fmt.Sprintf("%s — %s", f.Severity, f.Title))
		}
	}
	if len(repairable) == 0 {
		ui.Warn("No findings in this audit have a safe automatic repair.")
		return nil
	}
	sel, err := ui.SelectMany("Fix Center — choose repairs", labels, false)
	if err != nil {
		return err
	}
	if len(sel) == 0 {
		return nil
	}
	ui.Header("Repair Plan")
	for _, i := range sel {
		fmt.Printf("  %s %s\n", ui.C(ui.Orange, "•"), repairable[i].Title)
	}
	if !ui.Confirm("Apply selected controlled repairs?", false) {
		return nil
	}
	for _, i := range sel {
		f := repairable[i]
		ui.Info("Repairing " + f.Title)
		rr := audit.Repair(f)
		if rr.Success {
			ui.Success(rr.Message)
		} else {
			ui.Error(rr.Message)
		}
	}
	return nil
}

func cmdDoctor(args []string) error {
	ui.Header("Sundy Doctor")
	checks := network.Doctor()
	for _, c := range checks {
		switch c.Status {
		case "ok":
			ui.Success(c.Name + ": " + c.Detail)
		case "warn":
			ui.Warn(c.Name + ": " + c.Detail)
		default:
			ui.Error(c.Name + ": " + c.Detail)
		}
		if c.Suggestion != "" && c.Status != "ok" {
			fmt.Println("  " + ui.C(ui.Gray, c.Suggestion))
		}
	}
	r := audit.Run(false)
	for _, f := range r.Findings {
		if f.Category == "services" || f.Severity == audit.Critical || f.Severity == audit.High {
			ui.Warn(f.Title + ": " + f.Explanation)
		}
	}
	return nil
}

func cmdNetwork(args []string) error {
	if len(args) == 0 {
		return networkMenu()
	}
	switch args[0] {
	case "info":
		s := network.Inspect()
		ui.Header("Interfaces")
		fmt.Println(s.Interfaces)
		ui.Header("Routes")
		fmt.Println(s.Routes)
		ui.Header("Policy rules")
		fmt.Println(s.Rules)
		ui.Header("DNS")
		fmt.Println(s.DNS)
		ui.Header("Listening sockets")
		fmt.Println(s.Listening)
		return nil
	case "doctor":
		return cmdDoctor(args[1:])
	case "save":
		return networkSave(args[1:])
	case "restore":
		if len(args) < 2 {
			return fmt.Errorf("usage: sundy network restore NAME --apply")
		}
		apply := contains(args[2:], "--apply")
		if !apply {
			return fmt.Errorf("restore is potentially disruptive; re-run with --apply after reviewing the snapshot")
		}
		return snapshot.NewStore().RestoreNetwork(args[1], true)
	default:
		return fmt.Errorf("unknown network command %q", args[0])
	}
}
func networkMenu() error {
	n, err := ui.Menu("Network", []string{"Inspect network", "Run network doctor", "Save network snapshot", "Restore network snapshot", "Back"})
	if err != nil {
		return err
	}
	switch n {
	case 0:
		return cmdNetwork([]string{"info"})
	case 1:
		return cmdNetwork([]string{"doctor"})
	case 2:
		return networkSave(nil)
	case 3:
		name, _ := ui.Prompt("Snapshot name/ID", "")
		if name == "" {
			return nil
		}
		if !ui.Confirm("Apply this snapshot? An emergency rollback snapshot will be created first.", false) {
			return nil
		}
		return snapshot.NewStore().RestoreNetwork(name, true)
	}
	return nil
}
func networkSave(args []string) error {
	name := ""
	if len(args) > 0 {
		name = args[0]
	}
	sel, err := ui.SelectMany("Network Snapshot", snapshot.NetworkComponents, true)
	if err != nil {
		return err
	}
	if name == "" {
		name, _ = ui.Prompt("Snapshot name", "network-working")
	}
	m, err := snapshot.NewStore().SaveNetwork(name, sel, false)
	if err != nil {
		return err
	}
	ui.Success("Saved snapshot " + m.Name + " (" + m.ID + ")")
	return nil
}

func cmdSnapshots(args []string) error {
	list, err := snapshot.NewStore().List()
	if err != nil {
		return err
	}
	ui.Header("Snapshots")
	if len(list) == 0 {
		ui.Muted("No snapshots yet.")
		return nil
	}
	for _, m := range list {
		auto := ""
		if m.Automatic {
			auto = " auto"
		}
		fmt.Printf("%-34s %-12s %-20s %s%s\n", m.Name, m.Type, m.Created.Format("2006-01-02 15:04"), m.ID, auto)
	}
	return nil
}
func cmdSnapshot(args []string) error {
	if len(args) == 0 {
		return snapshotMenu()
	}
	switch args[0] {
	case "show":
		if len(args) < 2 {
			return fmt.Errorf("usage: sundy snapshot show NAME")
		}
		m, err := snapshot.NewStore().Get(args[1])
		if err != nil {
			return err
		}
		ui.Box("Snapshot", []string{ui.KeyValue("Name", m.Name), ui.KeyValue("ID", m.ID), ui.KeyValue("Type", m.Type), ui.KeyValue("Created", m.Created.Format(time.RFC3339)), ui.KeyValue("Host", m.Hostname), ui.KeyValue("Components", strings.Join(m.Components, ", ")), ui.KeyValue("Notes", m.Notes)})
		return nil
	case "create":
		return systemSnapshotCreate()
	case "restore":
		if len(args) < 2 {
			return fmt.Errorf("usage: sundy snapshot restore NAME --components network,firewall --apply")
		}
		var keys []string
		apply := false
		for i := 2; i < len(args); i++ {
			if args[i] == "--apply" {
				apply = true
			}
			if args[i] == "--components" && i+1 < len(args) {
				keys = strings.Split(args[i+1], ",")
				i++
			}
		}
		return snapshot.NewStore().RestoreSystem(args[1], keys, apply)
	default:
		return fmt.Errorf("usage: sundy snapshot <show|create|restore>")
	}
}

func systemSnapshotCreate() error {
	labels := make([]string, len(snapshot.SystemComponents))
	for i, c := range snapshot.SystemComponents {
		labels[i] = c.Label + " — " + c.Description
	}
	sel, err := ui.SelectMany("System Snapshot", labels, true)
	if err != nil {
		return err
	}
	name, _ := ui.Prompt("Snapshot name", "system-working")
	m, err := snapshot.NewStore().SaveSystem(name, sel, false)
	if err != nil {
		return err
	}
	ui.Success("Saved system snapshot " + m.Name + " (" + m.ID + ")")
	return nil
}
func snapshotMenu() error {
	n, err := ui.Menu("Snapshots & Recovery", []string{"List snapshots", "Create full/selective system snapshot", "Save network state", "Restore network state", "Back"})
	if err != nil {
		return err
	}
	switch n {
	case 0:
		return cmdSnapshots(nil)
	case 1:
		return systemSnapshotCreate()
	case 2:
		return networkSave(nil)
	case 3:
		name, _ := ui.Prompt("Snapshot name/ID", "")
		if name != "" && ui.Confirm("Restore persistent network configuration?", false) {
			return snapshot.NewStore().RestoreNetwork(name, true)
		}
	}
	return nil
}

func cmdInstall(args []string) error {
	if len(args) == 0 {
		return installMenu()
	}
	id := args[0]
	switch id {
	case "list":
		return listPresets()
	case "minecraft":
		return installMinecraftInteractive()
	case "pterodactyl":
		return installPterodactylInteractive()
	}
	p, ok := installpkg.Find(id)
	if !ok {
		return fmt.Errorf("unknown preset %q", id)
	}
	ui.Header("Installation Plan")
	fmt.Println(installpkg.Plan(p))
	for _, n := range p.Notes {
		ui.Warn(n)
	}
	if !ui.Confirm("Install this preset?", false) {
		return nil
	}
	if err := installpkg.InstallPreset(p); err != nil {
		return err
	}
	ui.Success(p.Name + " installed")
	return nil
}
func listPresets() error {
	ui.Header("Install Presets")
	cats := map[string][]installpkg.Preset{}
	for _, p := range installpkg.Catalog {
		cats[p.Category] = append(cats[p.Category], p)
	}
	keys := make([]string, 0, len(cats))
	for k := range cats {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, c := range keys {
		fmt.Println("\n" + ui.C(ui.Orange2, c))
		for _, p := range cats[c] {
			fmt.Printf("  %-16s %-25s %s\n", p.ID, p.Name, p.Description)
		}
	}
	fmt.Printf("\n  %-16s %-25s %s\n", "minecraft", "Minecraft Server", "Direct console-managed instance or Pterodactyl path")
	fmt.Printf("  %-16s %-25s %s\n", "pterodactyl", "Pterodactyl", "Stable Panel and/or Wings installer")
	return nil
}
func installMenu() error {
	_ = listPresets()
	var ids []string
	var labels []string
	for _, p := range installpkg.Catalog {
		ids = append(ids, p.ID)
		labels = append(labels, p.Category+" / "+p.Name)
	}
	ids = append(ids, "minecraft", "pterodactyl", "back")
	labels = append(labels, "Game Hosting / Minecraft", "Game Hosting / Pterodactyl", "Back")
	n, err := ui.Menu("Install Center", labels)
	if err != nil {
		return err
	}
	if ids[n] == "back" {
		return nil
	}
	return cmdInstall([]string{ids[n]})
}

func installMinecraftInteractive() error {
	mode, err := ui.Menu("Minecraft Setup", []string{"Direct / console managed", "Pterodactyl Panel workflow", "Back"})
	if err != nil {
		return err
	}
	if mode == 1 {
		return installPterodactylInteractive()
	}
	if mode == 2 {
		return nil
	}
	src, err := ui.Menu("Server Source", []string{"Use existing server directory", "Create new Vanilla server"})
	if err != nil {
		return err
	}
	name, _ := ui.Prompt("Instance name", "survival")
	dir, _ := ui.Prompt("Server directory", "/srv/minecraft/"+name)
	portS, _ := ui.Prompt("Server port", "25565")
	port, _ := strconv.Atoi(portS)
	mem, _ := ui.Prompt("Java memory", "4G")
	auto := ui.Confirm("Start now and enable autostart?", true)
	restart := ui.Confirm("Restart automatically after crashes?", true)
	eula := ui.Confirm("Accept the Minecraft EULA for this server?", false)
	if !eula {
		return fmt.Errorf("Minecraft server installation stopped because the EULA was not accepted")
	}
	cfg := minecraft.Config{Name: name, Directory: dir, Port: port, Memory: mem, AcceptEULA: true, AutoStart: auto, Restart: restart}
	var a apps.App
	if src == 0 {
		jar, _ := ui.Prompt("Server JAR (blank = auto-detect)", "")
		cfg.Jar = jar
		a, err = minecraft.InstallExisting(cfg)
	} else {
		ver, _ := ui.Prompt("Minecraft version", "latest")
		a, err = minecraft.CreateVanilla(cfg, ver)
	}
	if err != nil {
		return err
	}
	ui.Success(fmt.Sprintf("Minecraft instance %s registered on port %d", a.Name, a.Port))
	fmt.Printf("Console: %s\n", ui.C(ui.Orange, "sundy minecraft console "+a.Name))
	return nil
}

func installPterodactylInteractive() error {
	mode, err := ui.Menu("Pterodactyl", []string{"Stable Panel (native, Ubuntu 24.04)", "Wings node", "Panel + Wings", "Back"})
	if err != nil {
		return err
	}
	if mode == 3 {
		return nil
	}
	if mode == 0 || mode == 2 {
		domain, _ := ui.Prompt("Panel domain", "panel.example.com")
		email, _ := ui.Prompt("Administrator/contact email", "admin@example.com")
		ui.Warn("The stable native Panel preset changes packages, MariaDB, Redis, NGINX, cron and systemd services.")
		if !ui.Confirm("Continue with Panel installation?", false) {
			return nil
		}
		pass, err := installpkg.InstallPanelStable(domain, email)
		if err != nil {
			return err
		}
		ui.Success("Pterodactyl Panel base installation completed")
		ui.Warn("Save this generated database password securely: " + pass)
		ui.Info("Run `cd /var/www/pterodactyl && php artisan p:user:make` to create the first Panel account, then configure TLS before exposing the Panel.")
	}
	if mode == 1 || mode == 2 {
		if !ui.Confirm("Install Wings and Docker dependencies?", false) {
			return nil
		}
		if err := installpkg.InstallWings(); err != nil {
			return err
		}
		ui.Success("Wings installed. Add a node in the Panel, then run the generated `wings configure` command on this host before starting wings.service.")
	}
	return nil
}

func cmdApps(args []string) error {
	list, err := apps.List()
	if err != nil {
		return err
	}
	ui.Header("Managed Applications")
	if len(list) == 0 {
		ui.Muted("No managed applications yet.")
		return nil
	}
	sm := platform.Services()
	for _, a := range list {
		status := a.Status
		if a.Service != "" {
			status = sm.State(a.Service)
		}
		fmt.Printf("%-22s %-12s %-10s %5d  %s\n", a.Name, a.Kind, status, a.Port, a.Directory)
	}
	return nil
}
func cmdMinecraft(args []string) error {
	if len(args) == 0 {
		return minecraftMenu()
	}
	action := args[0]
	if action == "install" {
		return installMinecraftInteractive()
	}
	if len(args) < 2 {
		return fmt.Errorf("usage: sundy minecraft <console|start|stop|restart|status> NAME")
	}
	name := args[1]
	if action == "console" {
		return minecraft.Console(name)
	}
	a, err := apps.Get(name)
	if err != nil {
		return err
	}
	switch action {
	case "start", "stop", "restart", "status":
		return serviceAction(action, a.Service)
	default:
		return fmt.Errorf("unknown minecraft action %q", action)
	}
}
func minecraftMenu() error {
	list, _ := apps.List()
	var mc []apps.App
	for _, a := range list {
		if a.Kind == "minecraft" {
			mc = append(mc, a)
		}
	}
	items := []string{"Install/register Minecraft server"}
	for _, a := range mc {
		items = append(items, fmt.Sprintf("%s (%d)", a.Name, a.Port))
	}
	items = append(items, "Back")
	n, err := ui.Menu("Minecraft Manager", items)
	if err != nil {
		return err
	}
	if n == 0 {
		return installMinecraftInteractive()
	}
	if n == len(items)-1 {
		return nil
	}
	a := mc[n-1]
	act, err := ui.Menu(a.Name, []string{"Console", "Status", "Start", "Restart", "Stop", "Back"})
	if err != nil {
		return err
	}
	switch act {
	case 0:
		return minecraft.Console(a.Name)
	case 1:
		return serviceAction("status", a.Service)
	case 2:
		return serviceAction("start", a.Service)
	case 3:
		return serviceAction("restart", a.Service)
	case 4:
		return serviceAction("stop", a.Service)
	}
	return nil
}

func cmdService(args []string) error {
	if len(args) == 0 {
		return servicesMenu()
	}
	if len(args) < 2 {
		return fmt.Errorf("usage: sundy service ACTION NAME")
	}
	return serviceAction(args[0], args[1])
}
func serviceAction(action, name string) error {
	sm := platform.Services()
	if action == "status" {
		state := sm.State(name)
		if state == "active" {
			ui.Success(name + " is active")
		} else {
			ui.Warn(name + " is " + state)
		}
		if platform.Detect().Init == "systemd" {
			r := util.Run(8*time.Second, "systemctl", "status", name, "--no-pager", "-n", "15")
			fmt.Println(r.Stdout)
		}
		return nil
	}
	if err := util.RequireRoot(); err != nil {
		return err
	}
	if err := sm.Action(action, name); err != nil {
		return err
	}
	ui.Success(action + " completed for " + name)
	return nil
}
func servicesMenu() error {
	if platform.Detect().Init != "systemd" {
		ui.Warn("Interactive service browser currently provides rich listing on systemd; direct service actions support additional init systems.")
		return nil
	}
	r := util.Run(10*time.Second, "systemctl", "list-units", "--type=service", "--state=running,failed", "--no-legend", "--no-pager")
	ui.Header("Running / failed services")
	fmt.Println(r.Stdout)
	name, _ := ui.Prompt("Service to inspect (blank = back)", "")
	if name == "" {
		return nil
	}
	return serviceAction("status", name)
}

func cmdReport(args []string) error {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	anon := fs.Bool("anonymous", false, "")
	out := fs.String("out", "", "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	path, err := report.Create(*out, *anon)
	if err != nil {
		return err
	}
	ui.Success("Support report created: " + path)
	return nil
}
func cmdRuntime(args []string) error {
	if len(args) == 2 && args[0] == "minecraft" {
		return minecraft.RunSupervisor(args[1])
	}
	return fmt.Errorf("internal runtime command")
}

func contains(a []string, v string) bool {
	for _, x := range a {
		if x == v {
			return true
		}
	}
	return false
}
func oneLine(s string, n int) string {
	s = strings.ReplaceAll(strings.TrimSpace(s), "\n", " | ")
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

func init() { _ = filepath.Separator }
