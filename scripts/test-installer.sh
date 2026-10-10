#!/bin/sh
# Offline bootstrap regression: successful install, checksum failure, architecture rejection.
set -eu
base=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
mkdir -p "$tmp/tools" "$tmp/release" "$tmp/install"
cat > "$tmp/release/sundy-linux-amd64" <<'BIN'
#!/bin/sh
printf 'Sundy Toolkit bootstrap-test\n'
BIN
(cd "$tmp/release" && sha256sum sundy-linux-amd64 > checksums.txt)
cat > "$tmp/tools/curl" <<'CURL'
#!/bin/sh
set -eu
while [ "$#" -gt 0 ]; do
 case "$1" in
  https://*) source=${1##*/} ;;
  -o) shift; output=$1 ;;
 esac
 shift
done
cp "$FIXTURE_RELEASE/$source" "$output"
CURL
cat > "$tmp/tools/uname" <<'UNAME'
#!/bin/sh
case "$1" in -s) printf 'Linux\n';; -m) printf '%s\n' "${FIXTURE_ARCH:-x86_64}";; esac
UNAME
chmod +x "$tmp/tools/curl" "$tmp/tools/uname"
export PATH="$tmp/tools:$PATH" FIXTURE_RELEASE="$tmp/release" SUNDY_INSTALL_DIR="$tmp/install"
sh "$base/installer/install.sh" >/dev/null
test -x "$tmp/install/sundy"
cp "$tmp/install/sundy" "$tmp/original"
printf 'tampered' > "$tmp/release/sundy-linux-amd64"
if sh "$base/installer/install.sh" > "$tmp/failure" 2>&1; then
 printf 'Bootstrap accepted invalid checksum\n' >&2
 exit 1
fi
cmp "$tmp/install/sundy" "$tmp/original"
export FIXTURE_ARCH=armv6l
if sh "$base/installer/install.sh" > "$tmp/failure" 2>&1; then
 printf 'Bootstrap accepted ARMv6 for an ARMv7 binary\n' >&2
 exit 1
fi
printf 'bootstrap regression tests passed\n'
