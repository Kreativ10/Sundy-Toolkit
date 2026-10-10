package snapshot

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/SundySystems/sundy-toolkit/internal/platform"
	"github.com/SundySystems/sundy-toolkit/internal/util"
)

type Meta struct {
	ID         string        `json:"id"`
	Name       string        `json:"name"`
	Type       string        `json:"type"`
	Created    time.Time     `json:"created"`
	Hostname   string        `json:"hostname"`
	Platform   platform.Info `json:"platform"`
	Components []string      `json:"components"`
	Automatic  bool          `json:"automatic"`
	Notes      string        `json:"notes,omitempty"`
}

type Store struct{ Base string }

func NewStore() Store { return Store{Base: stateDir()} }
func stateDir() string {
	if v := os.Getenv("SUNDY_STATE_DIR"); v != "" {
		return v
	}
	if os.Geteuid() == 0 {
		return "/var/lib/sundy"
	}
	h, _ := os.UserHomeDir()
	return filepath.Join(h, ".local", "share", "sundy")
}
func (s Store) SnapDir(id string) string { return filepath.Join(s.Base, "snapshots", id) }

var snapshotIDRx = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

func (s Store) Create(name, typ string, components []string, automatic bool) (Meta, error) {
	if !snapshotIDRx.MatchString(typ) {
		return Meta{}, fmt.Errorf("invalid snapshot type %q", typ)
	}
	host, _ := os.Hostname()
	id := fmt.Sprintf("%s-%s", strings.ReplaceAll(typ, "/", "-"), time.Now().Format("20060102-150405.000000000"))
	m := Meta{ID: id, Name: name, Type: typ, Created: time.Now(), Hostname: host, Platform: platform.Detect(), Components: components, Automatic: automatic}
	d := s.SnapDir(id)
	if err := os.MkdirAll(d, 0700); err != nil {
		return m, err
	}
	if err := util.WriteJSON(filepath.Join(d, "meta.json"), m, 0600); err != nil {
		return m, err
	}
	return m, nil
}
func (s Store) Update(m Meta) error {
	if !snapshotIDRx.MatchString(m.ID) {
		return fmt.Errorf("invalid snapshot ID")
	}
	return util.WriteJSON(filepath.Join(s.SnapDir(m.ID), "meta.json"), m, 0600)
}
func (s Store) Get(idOrName string) (Meta, error) {
	list, err := s.List()
	if err != nil {
		return Meta{}, err
	}
	for _, m := range list {
		if m.ID == idOrName || m.Name == idOrName {
			return m, nil
		}
	}
	return Meta{}, fmt.Errorf("snapshot %q not found", idOrName)
}
func (s Store) List() ([]Meta, error) {
	root := filepath.Join(s.Base, "snapshots")
	ents, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Meta
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		var m Meta
		if util.ReadJSON(filepath.Join(root, e.Name(), "meta.json"), &m) == nil && m.ID == e.Name() && snapshotIDRx.MatchString(m.ID) {
			out = append(out, m)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Created.After(out[j].Created) })
	return out, nil
}

func ArchivePaths(dst string, paths []string) error {
	f, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	if err := f.Chmod(0600); err != nil {
		f.Close()
		return err
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	closeAll := func(e error) error {
		e1 := tw.Close()
		e2 := gz.Close()
		e3 := f.Close()
		if e != nil {
			return e
		}
		if e1 != nil {
			return e1
		}
		if e2 != nil {
			return e2
		}
		return e3
	}
	seen := map[string]bool{}
	for _, root := range paths {
		root = filepath.Clean(root)
		if seen[root] {
			continue
		}
		seen[root] = true
		if _, err := os.Lstat(root); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return closeAll(err)
		}
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			hdr, err := tar.FileInfoHeader(info, "")
			if err != nil {
				return err
			}
			hdr.Name = strings.TrimPrefix(filepath.Clean(path), "/")
			if info.Mode()&os.ModeSymlink != 0 {
				target, _ := os.Readlink(path)
				hdr.Linkname = target
			}
			if err := tw.WriteHeader(hdr); err != nil {
				return err
			}
			if info.Mode().IsRegular() {
				rf, err := os.Open(path)
				if err != nil {
					return err
				}
				_, err = io.Copy(tw, rf)
				rf.Close()
				return err
			}
			return nil
		})
		if err != nil {
			return closeAll(err)
		}
	}
	return closeAll(nil)
}

func ExtractArchive(src string) error { return util.ExtractTGZ(src, "/") }

func SaveText(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0600)
}
