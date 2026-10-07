# Sundy Toolkit

**Sundy Toolkit** is an open-source Linux administration multitool by Sundy Systems: system overview, audits, repair workflows, network diagnostics, configuration snapshots, installation presets, managed Minecraft servers, service control, and support bundles from one command.

The project is designed around a simple safety rule:

> **DISCOVER → PLAN → SNAPSHOT → APPLY → VERIFY → COMMIT / ROLLBACK**

The CLI uses an orange terminal interface and opens an interactive control center when run without arguments.

## One-command installation

After the first GitHub Release is published:

```bash
curl -fsSL https://raw.githubusercontent.com/Kreativ10/Sundy-Toolkit/main/installer/install.sh | sh
```

Then run:

```bash
sundy
```

The bootstrap downloads the correct static Linux binary for the host architecture, downloads `checksums.txt`, verifies SHA-256, and installs `sundy` into `/usr/local/bin` by default.

For a custom location:

```bash
SUNDY_INSTALL_DIR="$HOME/.local/bin" curl -fsSL https://raw.githubusercontent.com/Kreativ10/Sundy-Toolkit/main/installer/install.sh | sh
```

## Main commands

```text
sundy                          interactive control center
sundy overview                 system overview
sundy overview --json          machine-readable overview
sundy audit                    quick audit
sundy audit --full             expanded audit
sundy audit --full --fix       audit + selective Repair Center
sundy doctor                   focused health/network diagnosis

sundy network info             interfaces/routes/rules/DNS/sockets
sundy network doctor           connectivity diagnosis
sundy network save             selective network snapshot
sundy network restore NAME --apply

sundy snapshots                list recovery points
sundy snapshot show NAME       inspect metadata
sundy snapshot create          selective system snapshot
sundy snapshot restore NAME --components firewall,docker --apply

sundy install                  interactive Install Center
sundy install list             list presets
sundy install minecraft        Minecraft setup wizard
sundy install pterodactyl      Pterodactyl Panel/Wings setup

sundy apps                     managed applications
sundy minecraft console NAME   attach to managed server console
sundy minecraft start NAME
sundy minecraft stop NAME
sundy minecraft restart NAME
sundy minecraft status NAME

sundy service status nginx
sundy service restart nginx
sundy report --anonymous       sanitized support bundle
sundy update                   update binary from GitHub Release + checksum
```

## Install Center

Built-in presets in v0.1:

- Administrator essentials
- Network toolkit
- Storage toolkit
- Docker Engine
- NGINX
- Caddy
- PostgreSQL
- MariaDB
- Redis
- WireGuard
- Fail2ban
- KVM/libvirt
- Development toolchain
- Minecraft Server
- Pterodactyl Panel / Wings

Presets are **not** random shell snippets. Package-manager details are routed through the platform layer, and the user sees an installation plan before changes are applied.

### Package managers

The core platform layer recognizes:

- APT
- DNF
- YUM
- Pacman
- Zypper
- APK
- XBPS
- Portage/emerge
- Nix (`nix-env`, limited preset coverage)

A preset may intentionally support fewer distributions than the core. This is especially important for third-party server software with its own official support matrix.

### Init systems

- systemd
- OpenRC
- runit (basic service actions)

## Minecraft: panel or direct console mode

`sudo sundy install minecraft` asks how the server should be managed.

### Direct / console-managed

You can register an existing server directory or create a new Vanilla server. Sundy can:

- auto-detect common server JAR names;
- download the official Vanilla server for `latest` or a specific version;
- verify Mojang's SHA-1 for the downloaded server JAR;
- set the server port;
- manage Java memory;
- require explicit EULA acceptance;
- install Java 21 if necessary;
- create a service;
- enable autostart;
- restart after crashes;
- expose a detachable interactive console over a Unix socket.

Example:

```bash
sudo sundy install minecraft
sundy minecraft console survival
```

The managed process path is:

```text
systemd/OpenRC
   ↓
Sundy Minecraft Supervisor
   ↓
stdin/stdout Unix socket
   ↓
java -jar server.jar nogui
```

The console command `:detach` disconnects the terminal without stopping the Minecraft server.

### Pterodactyl

The interactive installer offers:

- stable Panel (native installer currently deliberately limited to Ubuntu 24.04);
- Wings node;
- Panel + Wings.

The stable Panel resolver explicitly chooses the newest stable **1.x** GitHub release instead of blindly following a future major release. Wings is downloaded from Pterodactyl's latest release for `amd64` or `arm64`.

The Panel preset configures the base MariaDB/Redis/PHP/NGINX/queue-worker installation. TLS and the first Panel account remain explicit operator steps so the toolkit does not silently make domain/certificate/account decisions.

## Audit + Repair Center

`sudo sundy audit --full --fix` separates diagnosis from remediation.

A finding includes:

- severity;
- clear explanation;
- evidence where useful;
- recommendation;
- whether a bounded automatic repair exists.

The current automatic repair engine handles failed systemd services conservatively:

1. show why the service is considered failed;
2. validate known configs such as NGINX or OpenSSH where possible;
3. offer the repair rather than immediately applying it;
4. reset the failed state;
5. perform one controlled restart;
6. verify that the service actually became active.

Sundy deliberately does **not** auto-fix ambiguous failures such as a full disk, broken application config, or unsafe SSH hardening. Those are explained instead of guessed.

## Snapshots and rollback

### Network snapshots

`sudo sundy network save` lets the operator choose all or only some of:

- interfaces/addresses;
- routes;
- policy rules;
- DNS;
- persistent network configuration;
- firewall;
- network sysctl;
- WireGuard.

Persistent configuration from NetworkManager, systemd-networkd, Netplan, ifupdown and WireGuard is captured where present.

Before a network restore, Sundy creates an emergency snapshot. It then restores persistent files, reloads the detected network backend, verifies the default route/connectivity, and attempts rollback if verification fails.

> Remote network changes can always be dangerous. `--apply` is mandatory for restore operations.

### System snapshots

`sudo sundy snapshot create` can preserve selected configuration groups:

- Network
- Firewall
- SSH (configuration only; host private keys excluded)
- Services
- System settings
- Docker
- Minecraft managed-instance configuration
- Installed package inventory

Automatic restoration is intentionally restricted for components that can easily lock out a remote administrator. v0.1 treats SSH and Minecraft snapshots as review/recovery material rather than blindly overwriting them.

Snapshots are stored under:

```text
/var/lib/sundy/snapshots/
```

For non-root local use, state falls back to:

```text
~/.local/share/sundy/
```

## Support bundle

```bash
sundy report
sundy report --anonymous
```

The bundle contains system, audit, network, storage and failed-service diagnostics. A secret redaction pass removes common password/token/API-key patterns. Anonymous mode additionally masks public IPv4 addresses and host identity fields.

It does **not** intentionally collect SSH private keys, application databases, Minecraft worlds or arbitrary `/etc` contents.

## Cross-distribution design

The core is a static Go binary (`CGO_ENABLED=0`) and does not require Python, Node.js, a shell framework or a TUI runtime.

High-level operations call adapters instead of embedding distro-specific commands everywhere:

```text
Preset / Audit / Manager
          ↓
  Sundy internal API
          ↓
Package / Service / Network backend
          ↓
apt, dnf, pacman, apk, systemd, OpenRC, ...
```

This makes unsupported combinations fail with a clear explanation instead of running guessed commands.

## Project structure

```text
cmd/sundy/               CLI and interactive dashboard
internal/ui/             orange UI, menus, ASCII branding
internal/platform/       distro/package/init adapters
internal/systeminfo/     host overview
internal/audit/          checks + Repair Center
internal/network/        inspection + doctor
internal/snapshot/       network/system recovery points
internal/install/        preset catalog + Pterodactyl installers
internal/apps/           managed application registry
internal/minecraft/      installer, service, supervisor, console
internal/report/         sanitized support bundles
internal/selfupdate/     release updater
installer/               bootstrap/uninstall shell scripts
docs/                    design and safety documentation
.github/workflows/       CI and release builds
```

## Build locally

Requirements: Go 1.23+.

```bash
make test
make build
./bin/sundy
```

Create release binaries:

```bash
make dist VERSION=0.1.0
```

Targets:

- linux/amd64
- linux/arm64
- linux/armv7
- linux/riscv64

## Publishing a release

Push a tag such as:

```bash
git tag v0.1.0
git push origin v0.1.0
```

GitHub Actions will test the project, build static binaries, generate `checksums.txt`, and attach them to the GitHub Release. Once that release exists, `installer/install.sh` and `sundy update` can use it.

## Security model

Sundy is an administration tool and some commands run as root. The project therefore follows these rules:

- read-only commands do not request root unnecessarily;
- mutating commands require explicit privilege;
- restore requires explicit `--apply`;
- repair actions are opt-in and individually selectable;
- no telemetry is enabled by default;
- secrets are excluded from configuration snapshots where practical;
- updates are checksum verified;
- third-party arbitrary plugin execution is intentionally **not** part of v0.1.

See [`docs/SECURITY-MODEL.md`](docs/SECURITY-MODEL.md).

## License

MIT — Sundy Systems, 2026.
