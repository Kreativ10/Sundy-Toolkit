#!/bin/sh
set -eu

REPO="${SUNDY_REPO:-Kreativ10/Sundy-Toolkit}"
INSTALL_DIR="${SUNDY_INSTALL_DIR:-/usr/local/bin}"
VERSION="${SUNDY_VERSION:-latest}"

orange='\033[38;5;208m'
green='\033[38;5;82m'
red='\033[38;5;196m'
reset='\033[0m'

say() { printf "%b◆%b %s\n" "$orange" "$reset" "$*"; }
ok() { printf "%b✓%b %s\n" "$green" "$reset" "$*"; }
fail() { printf "%b✗%b %s\n" "$red" "$reset" "$*" >&2; exit 1; }

case "$(uname -s)" in Linux) ;; *) fail "Sundy Toolkit currently supports Linux only.";; esac
case "$(uname -m)" in
  x86_64|amd64) arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
  armv7l) arch=arm ;;
  riscv64) arch=riscv64 ;;
  *) fail "Unsupported architecture: $(uname -m)" ;;
esac

asset="sundy-linux-$arch"
if [ "$VERSION" = latest ]; then
  base="https://github.com/$REPO/releases/latest/download"
else
  base="https://github.com/$REPO/releases/download/$VERSION"
fi

tmp="$(mktemp -d 2>/dev/null || mktemp -d -t sundy)"
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

download() {
  url="$1" out="$2"
  if command -v curl >/dev/null 2>&1; then curl -fL --retry 3 --connect-timeout 10 "$url" -o "$out"
  elif command -v wget >/dev/null 2>&1; then wget -q --https-only "$url" -O "$out"
  else fail "curl or wget is required for the bootstrap installer."; fi
}

say "Downloading Sundy Toolkit ($arch)..."
download "$base/$asset" "$tmp/$asset"
download "$base/checksums.txt" "$tmp/checksums.txt"
expected="$(awk -v f="$asset" '$2==f || $2=="*"f {print $1; exit}' "$tmp/checksums.txt")"
[ -n "$expected" ] || fail "Release checksum for $asset was not found."
if command -v sha256sum >/dev/null 2>&1; then actual="$(sha256sum "$tmp/$asset" | awk '{print $1}')"
elif command -v shasum >/dev/null 2>&1; then actual="$(shasum -a 256 "$tmp/$asset" | awk '{print $1}')"
elif command -v openssl >/dev/null 2>&1; then actual="$(openssl dgst -sha256 "$tmp/$asset" | awk '{print $NF}')"
else fail "A SHA-256 utility (sha256sum, shasum, or openssl) is required."; fi
[ "$expected" = "$actual" ] || fail "Checksum verification failed."
ok "SHA-256 verified."

if [ -w "$INSTALL_DIR" ] || { [ ! -e "$INSTALL_DIR" ] && [ -w "$(dirname "$INSTALL_DIR")" ]; }; then
  mkdir -p "$INSTALL_DIR"
  install -m 0755 "$tmp/$asset" "$INSTALL_DIR/sundy"
elif command -v sudo >/dev/null 2>&1; then
  say "Administrator privileges are required to install into $INSTALL_DIR."
  sudo mkdir -p "$INSTALL_DIR"
  sudo install -m 0755 "$tmp/$asset" "$INSTALL_DIR/sundy"
else
  fail "Cannot write to $INSTALL_DIR and sudo is unavailable. Run this installer as root or set SUNDY_INSTALL_DIR."
fi

ok "Installed: $INSTALL_DIR/sundy"
"$INSTALL_DIR/sundy" version
printf "\nRun %bsundy%b to open the control center.\n" "$orange" "$reset"
