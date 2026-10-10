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
	if err := os.WriteFile(p, []byte("motd=Hi\nserver-port=25565\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := setProperty(p, "server-port", "25570"); err != nil {
		t.Fatal(err)
	}
	st, _ := os.Stat(p)
	if st.Mode().Perm() != 0600 {
		t.Fatalf("changed private configuration permissions: %v", st.Mode())
	}
	b, _ := os.ReadFile(p)
	if !strings.Contains(string(b), "server-port=25570") {
		t.Fatalf("unexpected: %s", b)
	}
}

func TestNormalizeConfig(t *testing.T) {
	for _, name := range []string{"../bad", "a/b", "with space", "bad\nname", "-option", ""} {
		if _, err := normalizeConfig(Config{Name: name, Directory: "server", Port: 25565}); err == nil {
			t.Errorf("accepted name %q", name)
		}
	}
	for _, memory := range []string{"-1G", "0G", "1G -XX:bad", "1.5G", "12M", "999999999999G"} {
		if _, err := normalizeConfig(Config{Name: "test", Directory: "server", Port: 25565, Memory: memory}); err == nil {
			t.Errorf("accepted memory %q", memory)
		}
	}
	c, err := normalizeConfig(Config{Name: "test", Directory: "server", Jar: "server.jar", Port: 25565, Memory: "2g"})
	if err != nil || !filepath.IsAbs(c.Directory) || c.Jar != filepath.Join(c.Directory, "server.jar") || c.Memory != "2G" {
		t.Fatalf("incorrect config: %+v %v", c, err)
	}
}

func TestJavaMajor(t *testing.T) {
	for text, want := range map[string]int{`java version "1.8.0_402"`: 8, `openjdk version "17.0.12" 2024-07-16`: 17, `openjdk version "25" 2025-09-16`: 25} {
		got, err := javaMajor(text)
		if err != nil || got != want {
			t.Errorf("%q: got %d %v", text, got, err)
		}
	}
	if _, err := javaMajor("not Java"); err == nil {
		t.Fatal("accepted unrecognized runtime")
	}
}

func TestFindJarRejectsAmbiguousDirectory(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"plugin.jar", "another.jar"} {
		os.WriteFile(filepath.Join(dir, name), nil, 0644)
	}
	if found := findJar(dir); found != "" {
		t.Fatalf("arbitrary jar selected: %s", found)
	}
	os.WriteFile(filepath.Join(dir, "server.jar"), nil, 0644)
	if found := findJar(dir); found != filepath.Join(dir, "server.jar") {
		t.Fatalf("server not detected: %s", found)
	}
}

func TestSetPropertyDoesNotOverwriteUnreadablePath(t *testing.T) {
	dir := t.TempDir()
	if err := setProperty(dir, "eula", "true"); err == nil {
		t.Fatal("read failure was ignored")
	}
}
