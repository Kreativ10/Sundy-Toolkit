package selfupdate

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseRepoAndChecksum(t *testing.T) {
	if repo != "https://github.com/Kreativ10/Sundy-Toolkit" {
		t.Fatalf("updater points to wrong repository: %s", repo)
	}
	dir := t.TempDir()
	file := filepath.Join(dir, "binary")
	os.WriteFile(file, []byte("abc"), 0600)
	sum, err := sha256File(file)
	if err != nil || sum != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Fatal(sum, err)
	}
	sums := filepath.Join(dir, "checksums.txt")
	os.WriteFile(sums, []byte(sum+" *sundy-linux-amd64\n"), 0600)
	got, err := findChecksum(sums, "sundy-linux-amd64")
	if err != nil || got != sum {
		t.Fatal(got, err)
	}
	if _, err := findChecksum(sums, "missing"); err == nil {
		t.Fatal("accepted missing checksum")
	}
}

func TestMissingReleaseExplainsRecovery(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	dst := filepath.Join(t.TempDir(), "sundy")
	url := server.URL + "/releases/latest/download/sundy-linux-amd64"
	err := download(url, dst)
	if err == nil || !strings.Contains(err.Error(), url) || !strings.Contains(err.Error(), "reinstall") {
		t.Fatalf("missing actionable release error: %v", err)
	}
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Fatalf("failed download created a binary: %v", err)
	}
}
