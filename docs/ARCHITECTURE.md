# Sundy Toolkit Architecture

## Core principles

Sundy Toolkit is split into four product concepts rather than one overloaded "plugin" system.

### Modules

Internal capabilities such as packages, services, networking, storage and auditing. Presets call modules and should not directly care whether the machine uses APT or Pacman.

### Presets

Opinionated installation workflows. They may have stricter support matrices than the Sundy core. Examples: Minecraft, Docker and Pterodactyl.

### Managed applications

Software instances Sundy knows how to start, stop, inspect and repair. Minecraft direct-mode servers are the first implementation.

### Snapshots

Configuration recovery points. A snapshot is not a full machine backup and never claims to replace application/database/world backups.

## Transaction model

Every future mutating subsystem should fit this lifecycle:

```text
DISCOVER
   ↓
PLAN
   ↓
SNAPSHOT
   ↓
APPLY
   ↓
VERIFY
  ↙   ↘
ROLLBACK COMMIT
```

Read-only discovery and plan generation should remain callable without root whenever technically possible.

## Platform adapters

`internal/platform` is the compatibility boundary. New package managers/init systems belong there instead of inside every preset.

## State

Root-managed state: `/var/lib/sundy`

Runtime sockets: `/run/sundy`

User-local fallback: `~/.local/share/sundy`

The app registry is declarative metadata, not the authoritative state of a service. Runtime status is always re-queried from the host.
