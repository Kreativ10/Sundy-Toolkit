# Changelog

## Unreleased

- Add simultaneous Minecraft instance regression checks for registration, menu selection, independent consoles and stopping one server while another remains available.
- Explain release download failures with the requested URL and recovery instructions; document migration from the broken 0.1.1 updater in both languages.

- Fix Minecraft relative JAR paths, version-aware Java checks, explicit Java selection, heap validation and existing-instance protection.
- Split Minecraft installation, downloads, Java detection, service generation and runtime management into focused modules.
- Add graceful shutdown, duplicate-supervisor locking, bounded console queues and complete stdout/stderr capture.
- Correct systemd path formatting and OpenRC background/crash supervision; retain custom state/runtime locations.
- Fix terminal borders for ANSI colors, Unicode and narrow terminals; preserve buffered prompt answers and reject EOF confirmations.
- Surface dashboard, repair, package/service and snapshot failures; validate service actions and restore components.
- Protect concurrent registry writes, temporary files and archive extraction; sanitize JSON and IPv6 diagnostics.
- Correct the self-update repository and ARMv7 bootstrap detection.
- Guard existing Panel installations, validate inputs, verify Composer, disable Panel telemetry and check required services.
- Add English/Russian README navigation, regression coverage, real Vanilla lifecycle and OpenRC container tests.

## 0.1.0 — 2026-10-07

Initial public-ready implementation:

- orange interactive Sundy Toolkit dashboard and ASCII branding;
- system overview and JSON output;
- audit, doctor and selective controlled repair flow;
- network inspection and diagnostics;
- selective network and system configuration snapshots;
- guarded network restore with emergency rollback snapshot and connectivity verification;
- install preset catalog with package/init adapters;
- managed Minecraft direct mode, Vanilla downloader, service supervisor and Unix-socket console;
- Pterodactyl stable 1.x Panel preset and Wings preset;
- application registry and service controls;
- redacted/anonymous support bundles;
- self-update with SHA-256 verification;
- one-command GitHub Release bootstrap installer;
- CI, multi-architecture release workflow and unit tests.
