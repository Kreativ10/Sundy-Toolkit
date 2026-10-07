#!/bin/sh
set -eu
BIN="${SUNDY_INSTALL_DIR:-/usr/local/bin}/sundy"
printf 'This removes only the Sundy binary. Managed services, snapshots and application data are left intact.\n'
if [ "$(id -u)" -eq 0 ]; then rm -f "$BIN"; else sudo rm -f "$BIN"; fi
printf 'Removed %s\n' "$BIN"
