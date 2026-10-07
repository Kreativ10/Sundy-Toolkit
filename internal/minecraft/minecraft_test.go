package minecraft

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetProperty(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "server.properties")
	if err := os.WriteFile(p, []byte("motd=Hi\nserver-port=25565\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := setProperty(p, "server-port", "25570"); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	if !strings.Contains(string(b), "server-port=25570") {
		t.Fatalf("unexpected: %s", b)
	}
}
