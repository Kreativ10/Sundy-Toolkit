package util

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

func EnsureDir(path string, mode os.FileMode) error { return os.MkdirAll(path, mode) }

func AtomicWrite(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	// Root-run configuration edits retain an existing file's ownership.
	if os.Geteuid() == 0 {
		if previous, err := os.Stat(path); err == nil {
			if stat, ok := previous.Sys().(*syscall.Stat_t); ok {
				if err := f.Chown(int(stat.Uid), int(stat.Gid)); err != nil {
					f.Close()
					return err
				}
			}
		}
	}
	if err := f.Chmod(mode); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

func WriteJSON(path string, v any, mode os.FileMode) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return AtomicWrite(path, append(data, '\n'), mode)
}

func ReadJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

func CopyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	_, cpErr := io.Copy(out, in)
	closeErr := out.Close()
	if cpErr != nil {
		return cpErr
	}
	return closeErr
}

func FileExists(path string) bool { _, err := os.Stat(path); return err == nil }
func IsNotExist(err error) bool   { return errors.Is(err, os.ErrNotExist) }

func StateDir() string {
	if v := os.Getenv("SUNDY_STATE_DIR"); v != "" {
		return v
	}
	if os.Geteuid() == 0 {
		return "/var/lib/sundy"
	}
	h, _ := os.UserHomeDir()
	return filepath.Join(h, ".local", "share", "sundy")
}
