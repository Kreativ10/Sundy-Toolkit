<div align="center">

# Sundy Toolkit

**One control center for your Linux server.**

System diagnostics · Configuration recovery · Service management · Minecraft

[![CI](https://github.com/Kreativ10/Sundy-Toolkit/actions/workflows/ci.yml/badge.svg)](https://github.com/Kreativ10/Sundy-Toolkit/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.23%2B-00ADD8?logo=go&logoColor=white)](go.mod)
[![Linux](https://img.shields.io/badge/Platform-Linux-F97316?logo=linux&logoColor=white)](#compatibility)
[![License](https://img.shields.io/badge/License-MIT-64748B)](LICENSE)

**English** · [Русский](README.ru.md)

[Quick start](#quick-start) · [Minecraft](#minecraft) · [Commands](#commands) · [Development](#development)

</div>

---

Sundy Toolkit is a Linux administration CLI with an orange interactive dashboard. Run `sundy` to inspect the host, diagnose failures, install supported packages, manage Minecraft instances and save selected configuration recovery points.

| Area | What you can do |
| :--- | :--- |
| **Diagnostics** | Host overview, JSON output, system audit, network checks, failed-service inspection |
| **Repair** | Select controlled service repairs; validate known configs before restarting |
| **Recovery** | Save selected system/network configuration; restore with explicit `--apply` |
| **Install Center** | Administrator tools, web servers, databases, Docker, VPN tools and more |
| **Minecraft** | Create Vanilla or register a JAR server; manage services and a detachable console |
| **Pterodactyl** | Install a new stable 1.x Panel on Ubuntu 24.04 or prepare a Wings node |
| **Support** | Export a redacted diagnostic bundle; update using release SHA-256 checksums |

## Quick start

### Install from a release

The bootstrap requires a GitHub Release containing the binary for your architecture and `checksums.txt`.

```sh
curl -fsSL https://raw.githubusercontent.com/Kreativ10/Sundy-Toolkit/main/installer/install.sh | sh
sundy
```

It verifies SHA-256 and installs into `/usr/local/bin`, requesting `sudo` when needed. A user-local installation is also available:

```sh
curl -fsSL https://raw.githubusercontent.com/Kreativ10/Sundy-Toolkit/main/installer/install.sh \
  | SUNDY_INSTALL_DIR="$HOME/.local/bin" sh
"$HOME/.local/bin/sundy" version
```

Set `SUNDY_VERSION=v0.1.0` on the **`sh` side of the pipe** to choose a release, or `SUNDY_REPO=owner/repository` to use another release repository. Add your installation directory to `PATH` when using a custom location.

### Build from source

Requires Go 1.23+ and Make. The resulting binary does not need a Go installation.

```sh
git clone https://github.com/Kreativ10/Sundy-Toolkit.git
cd Sundy-Toolkit
make check
make build
./bin/sundy
```

For managed services, install the binary at a permanent location first:

```sh
sudo install -m 0755 bin/sundy /usr/local/bin/sundy
sudo sundy install minecraft
```

Service definitions record the binary's absolute path. Keep it available; `go run` uses a temporary executable and cannot install a persistent Minecraft service.

## Minecraft

### Create or register a server

```sh
sudo sundy install minecraft
```

Choose **Direct / console managed**, then a new Vanilla server or an existing server directory. The wizard asks for a unique instance name, directory, port, maximum heap, Java executable, autostart, crash restart policy and explicit [Minecraft EULA](https://aka.ms/MinecraftEULA) acceptance.

- New Vanilla downloads use Mojang's version manifest, Java requirement, download size and SHA-1. The destination directory must be empty.
- Existing installations keep their world and other configuration. Sundy updates the selected port and, when accepted, EULA. Specify the JAR when automatic detection is ambiguous.
- Names use 1–48 letters, digits, `_` or `-`, starting with a letter or digit. Registered names, directories and ports cannot be reused.
- Heap values use `M` or `G`, such as `2048M` or `2G`. The initial heap is capped at 512M; leave memory for the OS, Java native allocations and other services.
- Vanilla's Java requirement comes from Mojang metadata. Minecraft 1.20.5 requires Java 21; 26.1 requires Java 25. An older installed runtime produces an actionable error. [Minecraft 1.20.5 notes](https://www.minecraft.net/pt-pt/article/minecraft-java-edition-1-20-5), [26.1 notes](https://feedback.minecraft.net/hc/en-us/articles/42011663817357-Minecraft-Java-Edition-26-1-Snapshot-1).
- For existing JARs, embedded version metadata and launcher class versions provide a minimum Java requirement. Modpacks may need a specific Java release, loader or launch script; only direct `java -jar … nogui` servers are supported.
- Select an absolute Java path when multiple runtimes are installed. Automatic Java package installation is attempted only when the default `java` command is missing; package availability depends on the distribution.

### Manage an instance

```sh
sudo sundy apps
sudo sundy minecraft status survival
sudo sundy minecraft start survival
sudo sundy minecraft console survival
sudo sundy minecraft stop survival
sudo sundy minecraft restart survival
```

Type `:detach` to leave the console without stopping the server. `stop` in the console stops Minecraft itself. On service stop, the supervisor requests a graceful shutdown and allows up to 90 seconds before forcing termination.

```text
systemd / OpenRC → Sundy supervisor → Java server
                         ↕
                   Unix socket console
```

A supervisor lock prevents duplicate launches. Console clients have bounded output queues so a stalled terminal cannot block server output. systemd uses `on-failure` when crash restarts are enabled; OpenRC uses `supervise-daemon` with bounded respawning. See the [OpenRC service manual](https://github.com/OpenRC/openrc/blob/master/man/openrc-run.8).

### Diagnose a failed start

```sh
sudo sundy minecraft status survival
sudo tail -n 100 /var/lib/sundy/minecraft/survival/console.log
# systemd hosts:
sudo journalctl -u sundy-minecraft-survival.service -n 100 --no-pager
```

Check the selected Java version, EULA, JAR path, port conflicts and memory limits. An OS/container OOM kill can occur below the configured heap limit because Java uses memory outside the heap. If service setup fails after registration, Sundy reports the saved instance and the error; inspect its service before retrying a start.

## Commands

| Task | Command |
| :--- | :--- |
| Interactive dashboard | `sundy` |
| Host overview | `sundy overview [--json]` |
| Quick / full audit | `sundy audit [--full] [--json]` |
| Select repairs | `sudo sundy audit --full --fix` |
| Health diagnostics | `sundy doctor` |
| Interfaces, routes, DNS, sockets | `sundy network info` |
| Save network configuration | `sudo sundy network save` |
| Restore network configuration | `sudo sundy network restore NAME --apply` |
| List / inspect snapshots | `sundy snapshots` / `sundy snapshot show NAME` |
| Save system configuration | `sudo sundy snapshot create` |
| Restore selected files | `sudo sundy snapshot restore NAME --components firewall,docker --apply` |
| List / install presets | `sundy install list` / `sudo sundy install PRESET` |
| Pterodactyl wizard | `sudo sundy install pterodactyl` |
| Service actions | `sudo sundy service status|start|stop|restart|enable|disable NAME` |
| Support bundle | `sundy report --anonymous [--out FILE]` |
| Update binary | `sudo sundy update` |

The dashboard reports nested operation errors. Prompts support pasted/piped answers and do not treat EOF as permission to apply a default confirmation. Boxes fit the terminal width and account for Unicode and ANSI colors. Set `NO_COLOR=1` to disable styling; `COLUMNS` supplies a width when terminal detection is unavailable.

## Compatibility

| Layer | Scope |
| :--- | :--- |
| Release binaries | Linux amd64, arm64, ARMv7, riscv64; static builds |
| Package adapters | APT, DNF, YUM, Pacman, Zypper, APK, XBPS, emerge, limited Nix |
| Service adapters | systemd, OpenRC; basic runit start/stop/restart/status |
| Minecraft services | systemd or OpenRC; a runnable JAR and compatible Java |
| Native Panel | New installation only; Ubuntu 24.04 + systemd; stable 1.x release |
| Wings preset | Ubuntu/Debian + systemd; amd64 or arm64; configure the node afterward |

Adapter detection does not guarantee that a preset's packages exist on every distribution. Some database presets require distribution-specific initialization; service failures are reported instead of being labelled successful.

The Panel installer refuses to overwrite an existing installation or regenerate its application key. It verifies the Composer installer checksum, configures local HTTP, PHP-FPM, MariaDB, Redis, NGINX, cron and the queue worker, and disables Panel telemetry. Create the first account and configure TLS and the matching application URL before public use. Existing NGINX/default-site configuration should be reviewed before installing this preset. [Composer checksum procedure](https://getcomposer.org/doc/faqs/how-to-install-composer-programmatically.md), [Panel environment command](https://github.com/pterodactyl/panel/blob/1.0-develop/app/Console/Commands/Environment/AppSettingsCommand.php).

## Recovery and support data

Snapshots contain selected **configuration**, not Minecraft worlds, application databases or full disks. Back up those separately. SSH and Minecraft system-snapshot components are review-only; the package list is an inventory. System restore restores configuration files, not all running firewall/service state.

Network restore requires `--apply`, saves an emergency snapshot, reloads a supported active network backend and checks connectivity. Failed application/verification triggers a rollback attempt. Recovery cannot be guaranteed after power loss, external changes or SSH disconnection.

| Data | Default location |
| :--- | :--- |
| Root-managed state and registry | `/var/lib/sundy/` |
| Configuration snapshots | `/var/lib/sundy/snapshots/` |
| Minecraft console logs | `/var/lib/sundy/minecraft/NAME/console.log` |
| Minecraft runtime sockets | `/run/sundy/minecraft/` |
| Non-root local state | `~/.local/share/sundy/` |

`SUNDY_STATE_DIR` and `SUNDY_RUNTIME_DIR` override the state and socket locations. Generated Minecraft services retain their selected locations. Use `sudo` consistently for root-managed applications and snapshots.

Support bundles apply common secret-pattern redaction to text and JSON diagnostics. Anonymous mode also masks host identity and non-loopback IP addresses. Review a bundle before sharing it: pattern redaction cannot recognize every application-specific secret. Archives and registry files use private permissions. See the [security model](docs/SECURITY-MODEL.md).

## Development

```sh
make check       # vet, race tests, shell syntax and offline bootstrap regressions
make smoke       # build and exercise read-only CLI commands
make dist VERSION=0.1.0
```

Opt-in real Minecraft test: downloads an official server and accepts EULA only in its disposable localhost test fixture.

```sh
SUNDY_MC_E2E_VERSION=latest go test -tags=integration \
  -run TestRealVanillaLifecycle -v ./internal/minecraft -timeout 8m
```

OpenRC installation and lifecycle test in a disposable container:

```sh
make build
docker build -f scripts/integration/Dockerfile.openrc -t sundy-openrc-test .
docker run --rm --memory=3g --cpus=2 sundy-openrc-test
```

See [validation results and limits](docs/VALIDATION.md), [architecture](docs/ARCHITECTURE.md), [contributing](CONTRIBUTING.md) and [changelog](CHANGELOG.md).

```text
cmd/sundy/          CLI and dashboard
internal/minecraft/ installation, Java checks, downloads, services, supervisor, console
internal/ui/        terminal layout and prompts
internal/platform/  package and service adapters
internal/snapshot/  configuration snapshots and restore
internal/audit/     diagnostics and controlled repairs
internal/install/   presets and Pterodactyl
internal/apps/      managed application registry
internal/report/    redacted support bundles
internal/selfupdate/ release updater
installer/          bootstrap and uninstall scripts
scripts/            smoke, bootstrap and container integration tests
```

Tagging `v*` triggers release builds and checksum generation. Bootstrap installation and self-update verify checksums from the release; they do not verify cryptographic release signatures. The uninstall script removes only the binary; application data and services remain and need the binary to run.

## License

[MIT](LICENSE) · Sundy Systems, 2026.
