package systeminfo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestOverviewCollection(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "ip"), []byte("#!/bin/sh\ncase \"$1\" in -4) printf '2: eth0 inet 192.0.2.1/24 scope global eth0\\n';; -6) printf '2: eth0 inet6 2001:db8::1/64 scope global\\n';; esac\n"), 0755)
	os.WriteFile(filepath.Join(dir, "ss"), []byte("#!/bin/sh\nprintf 'tcp LISTEN 0 4096 0.0.0.0:22 0.0.0.0:*\\n'\n"), 0755)
	t.Setenv("PATH", dir)
	overview := Collect()
	if overview.Hostname == "" || overview.Uptime == "" || overview.Cores < 1 || overview.Memory == "" {
		t.Fatalf("missing host info: %+v", overview)
	}
	if overview.IPv4 != "192.0.2.1" || overview.IPv6 != "2001:db8::1" || overview.Listening != 1 {
		t.Fatalf("network parsing failed: %+v", overview)
	}
	if !json.Valid([]byte(overview.JSON())) {
		t.Fatal("overview JSON invalid")
	}
}
