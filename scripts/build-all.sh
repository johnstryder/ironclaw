#!/usr/bin/env bash
# Cross-compile ironclaw for Linux, macOS, and Windows to verify build integrity.
set -e
cd "$(dirname "$0")/.."
VERSION="${1:-$(cat VERSION 2>/dev/null | tr -d '\n')}"
OUTDIR="${OUTDIR:-dist}"
mkdir -p "$OUTDIR"

targets=(
  "linux/amd64"
  "linux/arm64"
  "darwin/amd64"
  "darwin/arm64"
  "windows/amd64"
)

for t in "${targets[@]}"; do
  os="${t%%/*}"
  arch="${t##*/}"
  name="ironclaw"
  [[ "$os" == "windows" ]] && name="ironclaw.exe"
  echo "Building $os/$arch -> $OUTDIR/${os}-${arch}/$name"
  GOOS="$os" GOARCH="$arch" go build -ldflags="-s -w -X main.version=$VERSION" -o "$OUTDIR/${os}-${arch}/$name" ./cmd/ironclaw
done

echo "Done. Binaries in $OUTDIR/"
