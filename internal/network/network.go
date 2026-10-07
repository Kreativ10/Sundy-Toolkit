package network

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/SundySystems/sundy-toolkit/internal/util"
)

type State struct{ Interfaces, Routes, Rules, DNS, Listening, Connections string }

type Check struct{ Name, Status, Detail, Suggestion string }

func Inspect() State {
	return State{Interfaces: run("ip", "-br", "addr"), Routes: run("ip", "route", "show", "table", "all"), Rules: run("ip", "rule", "show"), DNS: dns(), Listening: run("ss", "-lntup"), Connections: run("ss", "-ntup")}
}
func run(name string, args ...string) string {
	if !util.Exists(name) {
		return "unavailable"
	}
	r := util.Run(5*time.Second, name, args...)
	if r.Code != 0 {
		return strings.TrimSpace(r.Stderr)
	}
	return r.Stdout
}
func dns() string {
	r := util.Run(2*time.Second, "cat", "/etc/resolv.conf")
	if r.Code != 0 {
		return "unavailable"
	}
	return r.Stdout
}

func Doctor() []Check {
	var out []Check
	if !util.Exists("ip") {
		return []Check{{"iproute2", "fail", "ip command is missing", "Install iproute2"}}
	}
	link := util.Run(3*time.Second, "ip", "-br", "link")
	if link.Code == 0 {
		out = append(out, Check{"Network interfaces", "ok", fmt.Sprintf("%d interface rows detected", lineCount(link.Stdout)), ""})
	} else {
		out = append(out, Check{"Network interfaces", "fail", link.Stderr, "Check iproute2/network namespace"})
	}
	def := util.Run(3*time.Second, "ip", "route", "show", "default")
	if def.Code == 0 && strings.TrimSpace(def.Stdout) != "" {
		out = append(out, Check{"Default route", "ok", firstLine(def.Stdout), ""})
	} else {
		out = append(out, Check{"Default route", "fail", "No IPv4 default route detected", "Configure a gateway or routing policy"})
	}
	gateway := gatewayIP(def.Stdout)
	if gateway != "" && util.Exists("ping") {
		p := util.Run(5*time.Second, "ping", "-c", "1", "-W", "2", gateway)
		if p.Code == 0 {
			out = append(out, Check{"Gateway reachability", "ok", gateway + " reachable", ""})
		} else {
			out = append(out, Check{"Gateway reachability", "fail", gateway + " did not reply", "Check L2, VLAN, bridge, ARP and gateway"})
		}
	}
	if util.Exists("ping") {
		p := util.Run(6*time.Second, "ping", "-c", "1", "-W", "3", "1.1.1.1")
		if p.Code == 0 {
			out = append(out, Check{"IPv4 Internet", "ok", "1.1.1.1 reachable", ""})
		} else {
			out = append(out, Check{"IPv4 Internet", "fail", "External IPv4 ping failed", "Check upstream routing/firewall; ICMP may also be filtered"})
		}
	}
	if util.Exists("getent") {
		r := util.Run(5*time.Second, "getent", "ahostsv4", "example.com")
		if r.Code == 0 && r.Stdout != "" {
			out = append(out, Check{"DNS resolution", "ok", firstLine(r.Stdout), ""})
		} else {
			out = append(out, Check{"DNS resolution", "fail", "example.com could not be resolved", "Inspect /etc/resolv.conf or local resolver"})
		}
	}
	if util.Exists("ping") {
		p := util.Run(6*time.Second, "ping", "-6", "-c", "1", "-W", "3", "2606:4700:4700::1111")
		if p.Code == 0 {
			out = append(out, Check{"IPv6 Internet", "ok", "Cloudflare IPv6 reachable", ""})
		} else {
			out = append(out, Check{"IPv6 Internet", "warn", "IPv6 connectivity was not confirmed", "Ignore if IPv6 is intentionally disabled"})
		}
	}
	return out
}
func JSON(v any) string { b, _ := json.MarshalIndent(v, "", "  "); return string(b) }
func lineCount(s string) int {
	if strings.TrimSpace(s) == "" {
		return 0
	}
	return len(strings.Split(strings.TrimSpace(s), "\n"))
}
func firstLine(s string) string {
	p := strings.Split(strings.TrimSpace(s), "\n")
	if len(p) > 0 {
		return p[0]
	}
	return ""
}
func gatewayIP(s string) string {
	f := strings.Fields(firstLine(s))
	for i, v := range f {
		if v == "via" && i+1 < len(f) {
			return f[i+1]
		}
	}
	return ""
}
