package minecraft

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func jarFixture(t *testing.T, entries map[string][]byte) []byte {
	t.Helper()
	var out bytes.Buffer
	z := zip.NewWriter(&out)
	for name, data := range entries {
		file, err := z.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func TestBundlerJavaRequirement(t *testing.T) {
	inner := jarFixture(t, map[string][]byte{"version.json": []byte(`{"java_version":25}`)})
	outer := jarFixture(t, map[string][]byte{"META-INF/versions/game/server.jar": inner, "META-INF/MANIFEST.MF": []byte("Main-Class: net.minecraft.bundler.Main\r\n"), "net/minecraft/bundler/Main.class": {0xca, 0xfe, 0xba, 0xbe, 0, 0, 0, 52}})
	file := filepath.Join(t.TempDir(), "server.jar")
	os.WriteFile(file, outer, 0600)
	major, err := jarJavaMajor(file)
	if err != nil || major != 25 {
		t.Fatalf("used launcher Java requirement instead of bundled game: %d %v", major, err)
	}
}

func TestJavaSelectionRejectsOutdatedRuntime(t *testing.T) {
	java := filepath.Join(t.TempDir(), "java")
	os.WriteFile(java, []byte("#!/bin/sh\nprintf 'openjdk version \"17.0.12\"\\n' >&2\n"), 0755)
	if _, err := ensureJava(java, 21); err == nil {
		t.Fatal("accepted Java 17 for a Java 21 server")
	}
	if got, err := ensureJava(java, 17); err != nil || got != java {
		t.Fatalf("valid Java rejected: %s %v", got, err)
	}
}
