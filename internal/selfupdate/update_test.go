package selfupdate

import (
	"os"
	"path/filepath"
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
