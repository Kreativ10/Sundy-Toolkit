package minecraft

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/SundySystems/sundy-toolkit/internal/platform"
	"github.com/SundySystems/sundy-toolkit/internal/util"
)

var javaVersionRx = regexp.MustCompile(`(?:openjdk|java) version "(?:1\.)?([0-9]+)`)

func javaMajor(output string) (int, error) {
	match := javaVersionRx.FindStringSubmatch(output)
	if len(match) != 2 {
		return 0, fmt.Errorf("could not parse Java version: %s", strings.TrimSpace(output))
	}
	return strconv.Atoi(match[1])
}

func ensureJava(command string, required int) (string, error) {
	explicit := command != "" && command != "java"
	if command == "" {
		command = "java"
	}
	path, err := exec.LookPath(command)
	if err != nil {
		if explicit {
			return "", fmt.Errorf("Java executable %q is unavailable", command)
		}
		if err := installJava(required); err != nil {
			return "", err
		}
		path, err = exec.LookPath(command)
		if err != nil {
			return "", err
		}
	}
	r := util.Run(10*time.Second, path, "-version")
	if r.Code != 0 {
		return "", fmt.Errorf("cannot run Java %s: %s", path, r.Stderr)
	}
	major, err := javaMajor(r.Stdout + "\n" + r.Stderr)
	if err != nil {
		return "", err
	}
	if major < required {
		return "", fmt.Errorf("server requires Java %d+, but %s is Java %d; install a compatible runtime and select its executable in the wizard", required, path, major)
	}
	return path, nil
}

func installJava(major int) error {
	if major < 8 {
		major = 8
	}
	pm := platform.Packages()
	var pkg string
	switch pm.Name {
	case "apt-get":
		pkg = fmt.Sprintf("openjdk-%d-jre-headless", major)
	case "dnf", "yum", "zypper":
		pkg = fmt.Sprintf("java-%d-openjdk-headless", major)
	case "pacman":
		pkg = fmt.Sprintf("jre%d-openjdk-headless", major)
	case "apk":
		pkg = fmt.Sprintf("openjdk%d-jre-headless", major)
	default:
		return fmt.Errorf("automatic Java installation is unavailable for %s; install Java %d+ and select its executable", pm.Name, major)
	}
	if err := pm.Install(pkg); err != nil {
		return fmt.Errorf("install Java %d (%s): %w; install the required runtime manually if unavailable in distribution repositories", major, pkg, err)
	}
	return nil
}

// Read the launcher's class version and any embedded game version metadata.
// Vanilla creation uses Mojang's javaVersion directly (the bundler may target an older Java).
func jarJavaMajor(path string) (int, error) {
	jar, err := zip.OpenReader(path)
	if err != nil {
		return 0, fmt.Errorf("invalid server JAR: %w", err)
	}
	defer jar.Close()
	return archiveJavaMajor(jar.File, 0)
}

func archiveJavaMajor(files []*zip.File, depth int) (int, error) {
	required := 8
	mainClass := ""
	for _, file := range files {
		if depth == 0 && strings.HasPrefix(file.Name, "META-INF/versions/") && strings.HasSuffix(file.Name, ".jar") {
			// Vanilla's launcher may target Java 8 while its bundled game needs Java 21+.
			if file.UncompressedSize64 > 256<<20 {
				return 0, fmt.Errorf("bundled server JAR is too large to inspect")
			}
			r, err := file.Open()
			if err != nil {
				return 0, err
			}
			data, err := io.ReadAll(io.LimitReader(r, 256<<20))
			r.Close()
			if err != nil {
				return 0, err
			}
			nested, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
			if err != nil {
				return 0, fmt.Errorf("invalid bundled server JAR: %w", err)
			}
			major, err := archiveJavaMajor(nested.File, depth+1)
			if err != nil {
				return 0, err
			}
			if major > required {
				required = major
			}
		}

		if file.Name != "META-INF/MANIFEST.MF" && file.Name != "version.json" {
			continue
		}
		r, err := file.Open()
		if err != nil {
			return 0, err
		}
		b, err := io.ReadAll(io.LimitReader(r, 1<<20))
		r.Close()
		if err != nil {
			return 0, err
		}
		if file.Name == "version.json" {
			var version struct {
				Java int `json:"java_version"`
			}
			if json.Unmarshal(b, &version) == nil && version.Java > required {
				required = version.Java
			}
		} else {
			for _, line := range strings.Split(string(b), "\n") {
				if strings.HasPrefix(line, "Main-Class:") {
					mainClass = strings.ReplaceAll(strings.TrimSpace(strings.TrimPrefix(line, "Main-Class:")), ".", "/") + ".class"
				}
			}
		}
	}
	for _, file := range files {
		if file.Name != mainClass {
			continue
		}
		r, err := file.Open()
		if err != nil {
			return 0, err
		}
		var header [8]byte
		_, err = io.ReadFull(r, header[:])
		r.Close()
		if err != nil {
			return 0, err
		}
		if binary.BigEndian.Uint32(header[:4]) != 0xcafebabe {
			return 0, fmt.Errorf("invalid Java class header")
		}
		if major := int(binary.BigEndian.Uint16(header[6:])) - 44; major > required {
			required = major
		}
	}
	return required, nil
}
