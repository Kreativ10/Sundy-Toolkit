package platform

import (
	"os"
	"path/filepath"
	"testing"
)

func TestServiceArgumentValidation(t *testing.T) {
	for _, name := range []string{"", "--all", "nginx\nssh", "a/b"} {
		if err := (ServiceManager{Init: "systemd"}).Action("stop", name); err == nil {
			t.Errorf("accepted %q", name)
		}
	}
	if err := (ServiceManager{Init: "systemd"}).Action("reboot", "nginx"); err == nil {
		t.Fatal("accepted unsupported action")
	}
	if err := (ServiceManager{Init: "runit"}).Action("enable", "nginx"); err == nil {
		t.Fatal("claimed runit service enable support")
	}
}

func TestRunitDownIsNotActive(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "sv"), []byte("#!/bin/sh\nprintf 'down: nginx: 3s, normally up\\n'\n"), 0755)
	t.Setenv("PATH", dir)
	if state := (ServiceManager{Init: "runit"}).State("nginx"); state != "inactive" {
		t.Fatalf("down service reported as %s", state)
	}
}

func TestPackageManagerArguments(t *testing.T) {
	for _, manager := range []string{"apt-get", "dnf", "yum", "pacman", "zypper", "apk", "xbps-install", "emerge", "nix-env"} {
		t.Run(manager, func(t *testing.T) {
			dir := t.TempDir()
			log := filepath.Join(dir, "args")
			os.WriteFile(filepath.Join(dir, manager), []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$ARGS_LOG\"\n"), 0755)
			t.Setenv("PATH", dir)
			t.Setenv("ARGS_LOG", log)
			if err := (PackageManager{Name: manager}).Install("safe-package"); err != nil {
				t.Fatal(err)
			}
			data, _ := os.ReadFile(log)
			if len(data) == 0 {
				t.Fatal("package command did not execute")
			}
		})
	}
}
