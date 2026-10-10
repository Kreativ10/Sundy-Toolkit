package util

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ExtractTGZ restores files under root and never writes through a symlink.
// Absolute link targets are retained for system configuration (e.g. resolv.conf),
// but later entries cannot use them to escape the extraction root.
func ExtractTGZ(src, root string) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		name := strings.TrimPrefix(hdr.Name, "./")
		if name == "." || name == "" {
			continue
		}
		if filepath.IsAbs(name) {
			return fmt.Errorf("unsafe absolute archive path %q", hdr.Name)
		}
		for _, part := range strings.Split(name, "/") {
			if part == ".." {
				return fmt.Errorf("unsafe archive path %q", hdr.Name)
			}
		}
		clean := filepath.Clean(name)
		target := filepath.Join(root, clean)
		if err := checkArchiveParents(root, filepath.Dir(clean)); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		mode := os.FileMode(hdr.Mode).Perm()
		switch hdr.Typeflag {
		case tar.TypeDir:
			if st, err := os.Lstat(target); err == nil && st.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("archive directory is a symlink: %s", target)
			}
			if err := os.MkdirAll(target, mode); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if st, err := os.Lstat(target); err == nil && !st.Mode().IsRegular() {
				return fmt.Errorf("archive file is not a regular file: %s", target)
			}
			// Atomic replacement also avoids truncating an existing hard-linked file.
			out, err := os.CreateTemp(filepath.Dir(target), ".sundy-extract-*")
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(out, tr)
			modeErr := out.Chmod(mode)
			closeErr := out.Close()
			if copyErr != nil || modeErr != nil || closeErr != nil {
				os.Remove(out.Name())
				if copyErr != nil {
					return copyErr
				}
				if modeErr != nil {
					return modeErr
				}
				return closeErr
			}
			if err := os.Rename(out.Name(), target); err != nil {
				os.Remove(out.Name())
				return err
			}
		case tar.TypeSymlink:
			if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
				return err
			}
			if err := os.Symlink(hdr.Linkname, target); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported archive entry %q (type %d)", hdr.Name, hdr.Typeflag)
		}
	}
}

func checkArchiveParents(root, relative string) error {
	current := root
	for _, part := range strings.Split(relative, string(os.PathSeparator)) {
		if part == "." || part == "" {
			continue
		}
		current = filepath.Join(current, part)
		st, err := os.Lstat(current)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if !st.IsDir() {
			return fmt.Errorf("archive parent is not a directory: %s", current)
		}
	}
	return nil
}
