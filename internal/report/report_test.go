package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRedaction(t *testing.T) {
	for _, input := range []string{`password=hunter2`, `--token abc123`, `Authorization: Bearer abc.def.ghi`, `api_key='secret with spaces'`, `{"password":"secret with spaces","nested":{"token":"abc123"}}`} {
		got := redactSecrets(input)
		for _, secret := range []string{"hunter2", "abc123", "abc.def.ghi", "secret with spaces"} {
			if strings.Contains(got, secret) {
				t.Errorf("secret leaked: %s", got)
			}
		}
		if strings.HasPrefix(input, "{") && !json.Valid([]byte(got)) {
			t.Errorf("redaction broke JSON: %s", got)
		}
	}
	got := redactIPs("127.0.0.1 192.168.1.5 203.0.113.4 ::1 2001:db8::1 fe80::1%eth0")
	for _, ip := range []string{"192.168.1.5", "203.0.113.4", "2001:db8::1", "fe80::1"} {
		if strings.Contains(got, ip) {
			t.Errorf("IP leaked: %s", got)
		}
	}
	if !strings.Contains(got, "127.0.0.1") || !strings.Contains(got, "::1") {
		t.Fatal("loopback redacted")
	}
}

func TestWriteSafeAllDiagnosticTypes(t *testing.T) {
	dir := t.TempDir()
	host, _ := os.Hostname()
	if err := writeSafe(dir, "audit.json", `{"evidence":"token=abc123 host `+host+` 2001:db8::2"}`, true); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "audit.json"))
	if !json.Valid(data) {
		t.Fatalf("invalid JSON: %s", data)
	}
	if strings.Contains(string(data), "abc123") || strings.Contains(string(data), host) || strings.Contains(string(data), "2001:db8") {
		t.Fatalf("unsanitized JSON: %s", data)
	}
	if err := writeSafe(filepath.Join(dir, "missing"), "test", "test", false); err == nil {
		t.Fatal("ignored write failure")
	}
}
