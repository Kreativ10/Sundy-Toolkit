package selfupdate

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const repo = "https://github.com/Kreativ10/Sundy-Toolkit"

func Run() error {
	arch := runtime.GOARCH
	if arch != "amd64" && arch != "arm64" && arch != "arm" && arch != "riscv64" {
		return fmt.Errorf("unsupported architecture %s", arch)
	}
	asset := "sundy-linux-" + arch
	base := repo + "/releases/latest/download/"
	tmp, err := os.MkdirTemp("", "sundy-update-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	bin := filepath.Join(tmp, asset)
	sums := filepath.Join(tmp, "checksums.txt")
	if err := download(base+asset, bin); err != nil {
		return err
	}
	if err := download(base+"checksums.txt", sums); err != nil {
		return err
	}
	want, err := findChecksum(sums, asset)
	if err != nil {
		return err
	}
	got, err := sha256File(bin)
	if err != nil {
		return err
	}
	if !strings.EqualFold(want, got) {
		return fmt.Errorf("checksum mismatch: expected %s got %s", want, got)
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return err
	}
	dir := filepath.Dir(exe)

	src, err := os.Open(bin)
	if err != nil {
		return err
	}
	defer src.Close()
	f, err := os.CreateTemp(dir, ".sundy-update-*")
	if err != nil {
		return fmt.Errorf("cannot write next to %s; re-run `sudo sundy update` or use the installer: %w", exe, err)
	}
	targetTmp := f.Name()
	defer os.Remove(targetTmp)
	_, cp := io.Copy(f, src)
	ce := f.Close()
	if cp != nil {
		_ = os.Remove(targetTmp)
		return cp
	}
	if ce != nil {
		_ = os.Remove(targetTmp)
		return ce
	}
	if err := os.Chmod(targetTmp, 0755); err != nil {
		_ = os.Remove(targetTmp)
		return err
	}
	if err := os.Rename(targetTmp, exe); err != nil {
		_ = os.Remove(targetTmp)
		return fmt.Errorf("cannot replace %s: %w", exe, err)
	}
	return nil
}
func download(url, dst string) error {
	c := &http.Client{Timeout: 90 * time.Second}
	r, e := c.Get(url)
	if e != nil {
		return e
	}
	defer r.Body.Close()
	if r.StatusCode/100 != 2 {
		if r.StatusCode == http.StatusNotFound {
			return fmt.Errorf("release asset not found (404): %s; check %s/releases for a release with this architecture and checksums.txt, or reinstall using installer/install.sh", url, repo)
		}
		return fmt.Errorf("download %s returned %s", url, r.Status)
	}
	f, e := os.Create(dst)
	if e != nil {
		return e
	}
	_, cp := io.Copy(f, r.Body)
	ce := f.Close()
	if cp != nil {
		return cp
	}
	return ce
}
func findChecksum(path, asset string) (string, error) {
	f, e := os.Open(path)
	if e != nil {
		return "", e
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		p := strings.Fields(s.Text())
		if len(p) >= 2 && strings.TrimPrefix(p[len(p)-1], "*") == asset {
			return p[0], nil
		}
	}
	if err := s.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("checksum for %s not found", asset)
}
func sha256File(path string) (string, error) {
	f, e := os.Open(path)
	if e != nil {
		return "", e
	}
	defer f.Close()
	h := sha256.New()
	if _, e := io.Copy(h, f); e != nil {
		return "", e
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
