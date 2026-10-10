#!/bin/sh
set -eu
BIN="${1:-./bin/sundy}"
NO_COLOR=1 "$BIN" version
NO_COLOR=1 "$BIN" help >/dev/null
NO_COLOR=1 "$BIN" overview >/dev/null
NO_COLOR=1 "$BIN" overview --json >/dev/null
NO_COLOR=1 "$BIN" audit >/dev/null
NO_COLOR=1 "$BIN" audit --full --json >/dev/null
NO_COLOR=1 "$BIN" doctor >/dev/null
NO_COLOR=1 "$BIN" network info >/dev/null
NO_COLOR=1 "$BIN" snapshots >/dev/null
NO_COLOR=1 "$BIN" install list >/dev/null
printf 'read-only CLI smoke tests passed\n'
