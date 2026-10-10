package minecraft

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const manifestURL = "https://piston-meta.mojang.com/mc/game/version_manifest_v2.json"

var httpClient = http.Client{Timeout: 5 * time.Minute}

type versionMeta struct {
	ID          string `json:"id"`
	JavaVersion struct {
		MajorVersion int `json:"majorVersion"`
	} `json:"javaVersion"`
	Downloads struct {
		Server struct {
			URL, SHA1 string
			Size      int64
		} `json:"server"`
	} `json:"downloads"`
}

func DownloadVanilla(version, dst string) (string, error) {
	meta, err := resolveVanilla(&httpClient, version)
	if err != nil {
		return "", err
	}
	if err := downloadServer(&httpClient, meta, dst); err != nil {
		return "", err
	}
	return meta.ID, nil
}

func resolveVanilla(client *http.Client, version string) (versionMeta, error) {
	var manifest struct {
		Latest struct {
			Release string `json:"release"`
		} `json:"latest"`
		Versions []struct{ ID, URL string }
	}
	var meta versionMeta
	if err := getJSON(client, manifestURL, &manifest); err != nil {
		return meta, err
	}
	if version == "" || version == "latest" {
		version = manifest.Latest.Release
	}
	for _, v := range manifest.Versions {
		if v.ID != version {
			continue
		}
		if err := getJSON(client, v.URL, &meta); err != nil {
			return meta, err
		}
		if meta.ID == "" {
			meta.ID = version
		}
		if meta.JavaVersion.MajorVersion == 0 {
			meta.JavaVersion.MajorVersion = 8
		}
		if meta.Downloads.Server.URL == "" {
			return meta, fmt.Errorf("server download is unavailable for %s", version)
		}
		return meta, nil
	}
	return meta, fmt.Errorf("Minecraft version %s not found", version)
}

func downloadServer(client *http.Client, meta versionMeta, dst string) error {
	server := meta.Downloads.Server
	checksum, err := hex.DecodeString(server.SHA1)
	if err != nil || len(checksum) != sha1.Size || server.Size <= 0 {
		return fmt.Errorf("missing or invalid server checksum/size in Mojang metadata")
	}
	resp, err := client.Get(server.URL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("download returned %s", resp.Status)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(dst), ".server-*.part")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	h := sha1.New()
	n, copyErr := io.Copy(io.MultiWriter(f, h), io.LimitReader(resp.Body, server.Size+1))
	closeErr := f.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if n != server.Size {
		return fmt.Errorf("download size mismatch: expected %d, got %d", server.Size, n)
	}
	if !strings.EqualFold(hex.EncodeToString(h.Sum(nil)), server.SHA1) {
		return fmt.Errorf("download checksum mismatch")
	}
	if err := os.Chmod(f.Name(), 0644); err != nil {
		return err
	}
	return os.Rename(f.Name(), dst)
}

func getJSON(c *http.Client, url string, v any) error {
	r, err := c.Get(url)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	if r.StatusCode/100 != 2 {
		return fmt.Errorf("%s returned %s", url, r.Status)
	}
	return json.NewDecoder(io.LimitReader(r.Body, 8<<20)).Decode(v)
}
