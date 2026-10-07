# Security Model

Sundy Toolkit often runs with administrative privileges, so convenience must never outrank recoverability.

## Rules

1. Read-only commands do not mutate the host.
2. Mutations require root and an explicit operator action.
3. Network restore requires `--apply` and takes an emergency snapshot first.
4. Audit findings do not automatically repair themselves.
5. Repair actions are bounded; ambiguous root causes remain advisory.
6. SSH host private keys are excluded from Sundy system snapshots.
7. Support bundles redact common secret patterns and offer anonymous mode.
8. The project has no runtime third-party plugin execution in v0.1.
9. Release binaries are verified against `checksums.txt` by the bootstrap and self-updater.
10. Full application data backups are outside the scope of configuration snapshots.

## Threat boundaries

Sundy cannot guarantee recovery if an operator changes routing/firewall configuration outside the tool, if the machine loses connectivity/power during a mutation, or if a service has destructive behavior of its own. Snapshots improve recovery; they are not transactional filesystem snapshots.

## Future hardening

Planned release-hardening can add signed attestations/checksums (for example Sigstore/Cosign), SBOM generation, and integration tests inside distro containers/VMs.
