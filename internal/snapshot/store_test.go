package snapshot

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStoreCreateList(t *testing.T) {
	t.Setenv("SUNDY_STATE_DIR", t.TempDir())
	s := NewStore()
	m, err := s.Create("test", "unit", []string{"x"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "test" {
		t.Fatal("name")
	}
	list, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Name != "test" {
		t.Fatalf("unexpected list: %#v", list)
	}
	_ = os.RemoveAll(s.Base)
}

func TestArchiveRoundTripAndPermissions(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "original.conf")
	os.WriteFile(file, []byte("saved config"), 0600)
	archive := filepath.Join(t.TempDir(), "snapshot.tar.gz")
	if err := ArchivePaths(archive, []string{file}); err != nil {
		t.Fatal(err)
	}
	st, _ := os.Stat(archive)
	if st.Mode().Perm() != 0600 {
		t.Fatal("snapshot is publicly readable")
	}
	os.WriteFile(file, []byte("changed"), 0600)
	if err := ExtractArchive(archive); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(file)
	if string(data) != "saved config" {
		t.Fatal("restore lost saved data")
	}
}

func TestStoreRejectsUnsafeIDs(t *testing.T) {
	t.Setenv("SUNDY_STATE_DIR", t.TempDir())
	store := NewStore()
	if _, err := store.Create("bad", "../escape", nil, false); err == nil {
		t.Fatal("accepted unsafe type")
	}
	if err := store.Update(Meta{ID: "../../escape"}); err == nil {
		t.Fatal("accepted unsafe ID")
	}
}
