package install

import (
	"archive/tar"
	"compress/gzip"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/SundySystems/sundy-toolkit/internal/platform"
	"github.com/SundySystems/sundy-toolkit/internal/util"
)

func InstallWings() error {
	if err := util.RequireRoot(); err != nil {
		return err
	}
	p := platform.Detect()
	if p.ID != "ubuntu" && p.ID != "debian" {
		return fmt.Errorf("Pterodactyl Wings is officially supported on Ubuntu/Debian; detected %s", p.Name)
	}
	if !util.Exists("docker") {
		dp, _ := Find("docker")
		if err := InstallPreset(dp); err != nil {
			return fmt.Errorf("Docker installation failed: %w", err)
		}
	}
	arch := "amd64"
	if runtime.GOARCH == "arm64" {
		arch = "arm64"
	} else if runtime.GOARCH != "amd64" {
		return fmt.Errorf("Wings preset currently supports amd64/arm64")
	}
	url := "https://github.com/pterodactyl/wings/releases/latest/download/wings_linux_" + arch
	if err := download(url, "/usr/local/bin/wings", 0755); err != nil {
		return err
	}
	if err := os.MkdirAll("/etc/pterodactyl", 0755); err != nil {
		return err
	}
	unit := `[Unit]
Description=Pterodactyl Wings Daemon
After=docker.service
Requires=docker.service
PartOf=docker.service

[Service]
User=root
WorkingDirectory=/etc/pterodactyl
LimitNOFILE=4096
PIDFile=/var/run/wings/daemon.pid
ExecStart=/usr/local/bin/wings
Restart=on-failure
StartLimitInterval=180
StartLimitBurst=30
RestartSec=5s

[Install]
WantedBy=multi-user.target
`
	if p.Init == "systemd" {
		if err := os.WriteFile("/etc/systemd/system/wings.service", []byte(unit), 0644); err != nil {
			return err
		}
		r := util.Run(10*time.Second, "systemctl", "daemon-reload")
		if r.Code != 0 {
			return fmt.Errorf("daemon-reload: %s", r.Stderr)
		}
		r = util.Run(10*time.Second, "systemctl", "enable", "wings")
		if r.Code != 0 {
			return fmt.Errorf("enable wings: %s", r.Stderr)
		}
	}
	return nil
}

// InstallPanelStable installs the current stable 1.x release on Ubuntu 24.04 using native packages.
// It creates the database, installs dependencies, configures NGINX/queue worker, and leaves only
// account creation and optional TLS to the operator.
func InstallPanelStable(domain, email string) (string, error) {
	if err := util.RequireRoot(); err != nil {
		return "", err
	}
	p := platform.Detect()
	if p.ID != "ubuntu" || !strings.HasPrefix(p.Version, "24.04") {
		return "", fmt.Errorf("the fully automated native Panel preset is intentionally limited to Ubuntu 24.04; use the guided/manual preset on other supported systems")
	}
	pm := platform.Packages()
	deps := []string{"php8.3", "php8.3-common", "php8.3-cli", "php8.3-gd", "php8.3-mysql", "php8.3-mbstring", "php8.3-bcmath", "php8.3-xml", "php8.3-fpm", "php8.3-curl", "php8.3-zip", "php8.3-intl", "mariadb-server", "redis-server", "tar", "unzip", "curl", "nginx"}
	if err := pm.Install(deps...); err != nil {
		return "", err
	}
	_ = platform.Services().Action("enable", "mariadb")
	_ = platform.Services().Action("start", "mariadb")
	_ = platform.Services().Action("enable", "redis-server")
	_ = platform.Services().Action("start", "redis-server")
	if !util.Exists("composer") {
		tmp := "/tmp/sundy-composer-setup.php"
		if err := download("https://getcomposer.org/installer", tmp, 0644); err != nil {
			return "", err
		}
		if err := util.RunStreaming("php", tmp, "--install-dir=/usr/local/bin", "--filename=composer"); err != nil {
			return "", err
		}
		_ = os.Remove(tmp)
	}
	root := "/var/www/pterodactyl"
	if err := os.MkdirAll(root, 0755); err != nil {
		return "", err
	}
	tgz := "/tmp/pterodactyl-panel.tar.gz"
	panelURL, panelVersion, err := githubReleaseAsset("pterodactyl/panel", "v1.", "panel.tar.gz")
	if err != nil {
		return "", fmt.Errorf("resolve stable Pterodactyl 1.x release: %w", err)
	}
	if err := download(panelURL, tgz, 0644); err != nil {
		return "", err
	}
	_ = panelVersion
	if err := extractTGZ(tgz, root); err != nil {
		return "", err
	}
	_ = os.Remove(tgz)
	if _, err := os.Stat(filepath.Join(root, ".env")); os.IsNotExist(err) {
		if err := util.CopyFile(filepath.Join(root, ".env.example"), filepath.Join(root, ".env"), 0640); err != nil {
			return "", err
		}
	}
	if err := util.RunStreamingDir(root, "env", "COMPOSER_ALLOW_SUPERUSER=1", "composer", "install", "--no-dev", "--optimize-autoloader", "--no-interaction"); err != nil {
		return "", err
	}
	if err := util.RunStreamingDir(root, "php", "artisan", "key:generate", "--force"); err != nil {
		return "", err
	}
	pass, err := randomPassword()
	if err != nil {
		return "", err
	}
	sql := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS panel; CREATE USER IF NOT EXISTS 'pterodactyl'@'127.0.0.1' IDENTIFIED BY '%s'; GRANT ALL PRIVILEGES ON panel.* TO 'pterodactyl'@'127.0.0.1'; FLUSH PRIVILEGES;", pass)
	dbcli := "mysql"
	if util.Exists("mariadb") {
		dbcli = "mariadb"
	}
	r := util.Run(10*time.Second, dbcli, "-u", "root", "-e", sql)
	if r.Code != 0 {
		return "", fmt.Errorf("database setup failed: %s", r.Stderr)
	}
	setupArgs := []string{"artisan", "p:environment:setup", "--author=" + email, "--url=https://" + domain, "--timezone=UTC", "--cache=redis", "--session=redis", "--queue=redis", "--redis-host=127.0.0.1", "--redis-port=6379", "--settings-ui=true"}
	if err := util.RunStreamingDir(root, "php", setupArgs...); err != nil {
		return "", err
	}
	dbArgs := []string{"artisan", "p:environment:database", "--host=127.0.0.1", "--port=3306", "--database=panel", "--username=pterodactyl", "--password=" + pass}
	if err := util.RunStreamingDir(root, "php", dbArgs...); err != nil {
		return "", err
	}
	if err := util.RunStreamingDir(root, "php", "artisan", "migrate", "--seed", "--force"); err != nil {
		return "", err
	}
	_ = os.Chmod(filepath.Join(root, "storage"), 0755)
	_ = os.Chmod(filepath.Join(root, "bootstrap", "cache"), 0755)
	nginx := fmt.Sprintf(`server {
    listen 80;
    server_name %s;
    root /var/www/pterodactyl/public;
    index index.php;
    client_max_body_size 100m;
    location / { try_files $uri $uri/ /index.php?$query_string; }
    location ~ \.php$ {
        include snippets/fastcgi-php.conf;
        fastcgi_pass unix:/run/php/php8.3-fpm.sock;
    }
    location ~ /\.ht { deny all; }
}
`, domain)
	if err := os.WriteFile("/etc/nginx/sites-available/pterodactyl.conf", []byte(nginx), 0644); err != nil {
		return "", err
	}
	_ = os.Remove("/etc/nginx/sites-enabled/default")
	_ = os.Remove("/etc/nginx/sites-enabled/pterodactyl.conf")
	if err := os.Symlink("/etc/nginx/sites-available/pterodactyl.conf", "/etc/nginx/sites-enabled/pterodactyl.conf"); err != nil && !os.IsExist(err) {
		return "", err
	}
	worker := `[Unit]
Description=Pterodactyl Queue Worker
After=redis-server.service

[Service]
User=www-data
Group=www-data
Restart=always
ExecStart=/usr/bin/php /var/www/pterodactyl/artisan queue:work --queue=high,standard,low --sleep=3 --tries=3
StartLimitInterval=180
StartLimitBurst=30
RestartSec=5s

[Install]
WantedBy=multi-user.target
`
	if err := os.WriteFile("/etc/systemd/system/pteroq.service", []byte(worker), 0644); err != nil {
		return "", err
	}
	cron := "* * * * * www-data php /var/www/pterodactyl/artisan schedule:run >> /dev/null 2>&1\n"
	if err := os.WriteFile("/etc/cron.d/pterodactyl", []byte(cron), 0644); err != nil {
		return "", err
	}
	_ = util.RunStreaming("chown", "-R", "www-data:www-data", root)
	_ = util.RunStreaming("systemctl", "daemon-reload")
	_ = util.RunStreaming("systemctl", "enable", "--now", "pteroq")
	_ = util.RunStreaming("systemctl", "enable", "--now", "nginx")
	if r := util.Run(8*time.Second, "nginx", "-t"); r.Code != 0 {
		return "", fmt.Errorf("nginx validation failed: %s", r.Stderr)
	}
	return pass, nil
}

func githubReleaseAsset(repo, tagPrefix, assetName string) (string, string, error) {
	type asset struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	}
	type release struct {
		TagName    string  `json:"tag_name"`
		Draft      bool    `json:"draft"`
		Prerelease bool    `json:"prerelease"`
		Assets     []asset `json:"assets"`
	}
	req, _ := http.NewRequest("GET", "https://api.github.com/repos/"+repo+"/releases?per_page=30", nil)
	req.Header.Set("User-Agent", "Sundy-Toolkit")
	c := &http.Client{Timeout: 30 * time.Second}
	resp, err := c.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return "", "", fmt.Errorf("GitHub API returned %s", resp.Status)
	}
	var releases []release
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return "", "", err
	}
	for _, r := range releases {
		if r.Draft || r.Prerelease || !strings.HasPrefix(r.TagName, tagPrefix) {
			continue
		}
		for _, a := range r.Assets {
			if a.Name == assetName {
				return a.BrowserDownloadURL, r.TagName, nil
			}
		}
	}
	return "", "", fmt.Errorf("no stable %s release asset %s found", tagPrefix, assetName)
}

func download(url, dst string, mode os.FileMode) error {
	c := &http.Client{Timeout: 2 * time.Minute}
	r, e := c.Get(url)
	if e != nil {
		return e
	}
	defer r.Body.Close()
	if r.StatusCode/100 != 2 {
		return fmt.Errorf("download %s returned %s", url, r.Status)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	tmp := dst + ".part"
	f, e := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if e != nil {
		return e
	}
	_, cp := io.Copy(f, r.Body)
	ce := f.Close()
	if cp != nil {
		return cp
	}
	if ce != nil {
		return ce
	}
	if err := os.Chmod(tmp, mode); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}
func extractTGZ(src, dst string) error {
	f, e := os.Open(src)
	if e != nil {
		return e
	}
	defer f.Close()
	gz, e := gzip.NewReader(f)
	if e != nil {
		return e
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		h, e := tr.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			return e
		}
		target := filepath.Join(dst, filepath.Clean(h.Name))
		if !strings.HasPrefix(target, filepath.Clean(dst)+string(os.PathSeparator)) && target != filepath.Clean(dst) {
			return fmt.Errorf("unsafe archive path")
		}
		switch h.Typeflag {
		case tar.TypeDir:
			if e := os.MkdirAll(target, os.FileMode(h.Mode)); e != nil {
				return e
			}
		case tar.TypeReg:
			if e := os.MkdirAll(filepath.Dir(target), 0755); e != nil {
				return e
			}
			w, e := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(h.Mode))
			if e != nil {
				return e
			}
			_, e = io.Copy(w, tr)
			w.Close()
			if e != nil {
				return e
			}
		}
	}
	return nil
}
func randomPassword() (string, error) {
	b := make([]byte, 24)
	if _, e := rand.Read(b); e != nil {
		return "", e
	}
	return strings.TrimRight(base64.RawURLEncoding.EncodeToString(b), "="), nil
}
