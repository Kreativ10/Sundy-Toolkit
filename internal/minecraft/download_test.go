package minecraft

import (
	"crypto/sha1"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDownloadServerValidationAndCleanup(t *testing.T) {
	data := []byte("verified jar data")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(data) }))
	defer server.Close()
	for _, kind := range []string{"valid", "checksum", "size", "missing metadata"} {
		t.Run(kind, func(t *testing.T) {
			meta := versionMeta{}
			meta.Downloads.Server.URL = server.URL
			meta.Downloads.Server.SHA1 = fmt.Sprintf("%x", sha1.Sum(data))
			meta.Downloads.Server.Size = int64(len(data))
			switch kind {
			case "checksum":
				meta.Downloads.Server.SHA1 = strings.Repeat("0", 40)
			case "size":
				meta.Downloads.Server.Size++
			case "missing metadata":
				meta.Downloads.Server.SHA1 = ""
			}
			dst := filepath.Join(t.TempDir(), "server.jar")
			os.WriteFile(dst, []byte("original"), 0644)
			err := downloadServer(server.Client(), meta, dst)
			if (err == nil) != (kind == "valid") {
				t.Fatalf("unexpected error: %v", err)
			}
			content, _ := os.ReadFile(dst)
			if kind != "valid" && string(content) != "original" {
				t.Fatal("failed download replaced server")
			}
			parts, _ := filepath.Glob(filepath.Join(filepath.Dir(dst), "*.part"))
			if len(parts) > 0 {
				t.Fatalf("left temporary files: %v", parts)
			}
		})
	}
}
