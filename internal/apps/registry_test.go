package apps

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestRegistryConcurrentUpdates(t *testing.T) {
	t.Setenv("SUNDY_STATE_DIR", t.TempDir())
	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			if err := Save(App{ID: fmt.Sprint(id), Name: fmt.Sprintf("server-%d", id)}); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	list, err := List()
	if err != nil || len(list) != 30 {
		t.Fatalf("lost registry entries: %d %v", len(list), err)
	}
	a, err := Get("server-12")
	if err != nil || a.ID != "12" {
		t.Fatal(a, err)
	}
	a.Port = 25565
	if err := Save(a); err != nil {
		t.Fatal(err)
	}
	if err := Delete("server-12"); err != nil {
		t.Fatal(err)
	}
	list, _ = List()
	if len(list) != 29 {
		t.Fatal("delete or upsert failed")
	}
	st, _ := os.Stat(path())
	if st.Mode().Perm() != 0600 {
		t.Fatal("registry permissions")
	}
}

func TestRegistryCorruptionIsNotOverwritten(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SUNDY_STATE_DIR", dir)
	path := filepath.Join(dir, "apps.json")
	os.WriteFile(path, []byte("broken json"), 0600)
	if err := Save(App{ID: "test"}); err == nil {
		t.Fatal("overwrote corrupt registry")
	}
	data, _ := os.ReadFile(path)
	if string(data) != "broken json" {
		t.Fatal("lost original registry")
	}
}
