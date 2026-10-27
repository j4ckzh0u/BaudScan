#!/usr/bin/env bash
# Build the x64 release artifacts: macOS DMG, Windows EXE and Linux binary.
# Requires macOS for the DMG (hdiutil) and the Go toolchain in go.mod.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT"

VERSION="$(sed -n 's/.*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' mygo.json | head -n1)"
if [[ -z "$VERSION" ]]; then
  echo "error: cannot read version from mygo.json" >&2
  exit 1
fi

if [[ "$(uname -s)" != "Darwin" ]]; then
  echo "error: building the macOS DMG needs macOS (hdiutil); run this script on macOS." >&2
  exit 1
fi

for cmd in go hdiutil file; do
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "error: $cmd is required" >&2
    exit 1
  fi
done

OUT="$ROOT/build/release"
DIST="$ROOT/dist"
PLATFORMS="darwin/amd64,windows/amd64,linux/amd64"
START="$(date +%s)"

echo "BaudScan $VERSION · x64 release"
echo "  macOS   darwin/amd64"
echo "  Windows windows/amd64"
echo "  Linux   linux/amd64"
echo

rm -rf "$OUT" "$DIST"
mkdir -p "$OUT" "$DIST"

CGO_ENABLED=0 go tool mygo build -platform "$PLATFORMS" -o "$OUT"

DMG="$OUT/darwin-amd64/baudscan-${VERSION}-amd64.dmg"
EXE="$OUT/windows-amd64/BaudScan.exe"
LINUX="$OUT/linux-amd64/baudscan"

for artifact in "$DMG" "$EXE" "$LINUX"; do
  if [[ ! -f "$artifact" ]]; then
    echo "error: expected artifact not found: $artifact" >&2
    exit 1
  fi
done

DMG_NAME="BaudScan-${VERSION}-macos-x64.dmg"
EXE_NAME="BaudScan-${VERSION}-windows-x64.exe"
LINUX_NAME="BaudScan-${VERSION}-linux-x64"

cp "$DMG" "$DIST/$DMG_NAME"
cp "$EXE" "$DIST/$EXE_NAME"
cp "$LINUX" "$DIST/$LINUX_NAME"
chmod +x "$DIST/$LINUX_NAME"

echo "Verifying artifacts…"
hdiutil verify "$DIST/$DMG_NAME" >/dev/null

if ! file "$DIST/$EXE_NAME" | grep -q "PE32+ executable (GUI) x86-64"; then
  echo "error: Windows artifact is not a GUI x64 PE executable" >&2
  exit 1
fi
if ! file "$DIST/$LINUX_NAME" | grep -q "ELF 64-bit LSB executable, x86-64"; then
  echo "error: Linux artifact is not an x64 ELF executable" >&2
  exit 1
fi

(
  cd "$DIST"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$DMG_NAME" "$EXE_NAME" "$LINUX_NAME" > SHA256SUMS
  else
    shasum -a 256 "$DMG_NAME" "$EXE_NAME" "$LINUX_NAME" > SHA256SUMS
  fi
)

echo
echo "Done in $(( $(date +%s) - START ))s · artifacts in dist/"
for f in "$DIST/$DMG_NAME" "$DIST/$EXE_NAME" "$DIST/$LINUX_NAME" "$DIST/SHA256SUMS"; do
  printf '  %-42s %s\n' "$(basename "$f")" "$(du -h "$f" | cut -f1)"
done
