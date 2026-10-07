package snapshot

import (
	"os"
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
