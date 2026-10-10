package util

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRunErrorsAndTimeout(t *testing.T) {
	r := Run(time.Second, "sundy-command-that-does-not-exist")
	if r.Code != 127 || r.Stderr == "" {
		t.Fatalf("missing failure detail: %+v", r)
	}
	r = Run(20*time.Millisecond, "sleep", "5")
	if r.Code != 124 || !strings.Contains(r.Stderr, "timed out") {
		t.Fatalf("timeout: %+v", r)
	}
	r = Run(time.Second, "sh", "-c", "printf output; printf error >&2; exit 7")
	if r.Code != 7 || r.Stdout != "output" || r.Stderr != "error" {
		t.Fatalf("output: %+v", r)
	}
}

func TestAtomicWriteConcurrentAndMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := AtomicWrite(path, []byte(strings.Repeat("complete", 1000)), 0600); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	data, err := os.ReadFile(path)
	if err != nil || len(data) != 8000 {
		t.Fatalf("torn file: %d %v", len(data), err)
	}
	st, _ := os.Stat(path)
	if st.Mode().Perm() != 0600 {
		t.Fatalf("wrong permissions: %v", st.Mode())
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Fatal("temporary files leaked")
	}
}

func writeArchive(t *testing.T, entries []*tar.Header) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.tar.gz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for _, hdr := range entries {
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if hdr.Typeflag == tar.TypeReg && hdr.Size > 0 {
			tw.Write([]byte(strings.Repeat("x", int(hdr.Size))))
		}
	}
	tw.Close()
	gz.Close()
	f.Close()
	return path
}

func TestExtractArchiveRejectsTraversalAndSymlinkWrites(t *testing.T) {
	for _, name := range []string{"../escape", "safe/../../escape", "/absolute"} {
		archive := writeArchive(t, []*tar.Header{{Name: name, Typeflag: tar.TypeReg, Mode: 0600, Size: 1}})
		if err := ExtractTGZ(archive, t.TempDir()); err == nil {
			t.Errorf("accepted %q", name)
		}
	}
	outside := t.TempDir()
	root := t.TempDir()
	archive := writeArchive(t, []*tar.Header{{Name: "link", Typeflag: tar.TypeSymlink, Linkname: outside}, {Name: "link/escape", Typeflag: tar.TypeReg, Mode: 0600, Size: 1}})
	if err := ExtractTGZ(archive, root); err == nil {
		t.Fatal("followed archive symlink")
	}
	if _, err := os.Stat(filepath.Join(outside, "escape")); !os.IsNotExist(err) {
		t.Fatal("wrote outside extraction root")
	}
	os.WriteFile(filepath.Join(outside, "original"), []byte("keep"), 0600)
	os.Symlink(filepath.Join(outside, "original"), filepath.Join(root, "file"))
	archive = writeArchive(t, []*tar.Header{{Name: "file", Typeflag: tar.TypeReg, Mode: 0600, Size: 1}})
	if err := ExtractTGZ(archive, root); err == nil {
		t.Fatal("followed preexisting symlink")
	}
	data, _ := os.ReadFile(filepath.Join(outside, "original"))
	if string(data) != "keep" {
		t.Fatal("overwrote symlink target")
	}
}

func TestExtractArchiveRoundTrip(t *testing.T) {
	archive := writeArchive(t, []*tar.Header{{Name: "etc/hello..conf", Typeflag: tar.TypeReg, Mode: 0600, Size: 3}, {Name: "link", Typeflag: tar.TypeSymlink, Linkname: "etc/hello..conf"}})
	root := t.TempDir()
	if err := ExtractTGZ(archive, root); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "link"))
	if err != nil || string(data) != "xxx" {
		t.Fatalf("round trip: %s %v", data, err)
	}
}
