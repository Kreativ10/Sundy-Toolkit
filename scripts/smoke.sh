#!/bin/sh
set -eu
BIN="${1:-./bin/sundy}"
NO_COLOR=1 "$BIN" version
NO_COLOR=1 "$BIN" overview >/dev/null
NO_COLOR=1 "$BIN" audit >/dev/null
NO_COLOR=1 "$BIN" doctor >/dev/null || true
NO_COLOR=1 "$BIN" install list >/dev/null
printf 'smoke tests passed\n'
