#!/bin/bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BIN_DIR="${HOME}/bin"

echo "[install] Building cursor-tools..."
cd "$REPO_ROOT"

VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=${VERSION}" \
    -o bin/cursor-tools ./cmd/cursor-tools/

mkdir -p "$BIN_DIR"
cp bin/cursor-tools "$BIN_DIR/cursor-tools"
chmod +x "$BIN_DIR/cursor-tools"

echo "[install] Installed cursor-tools ${VERSION} to ${BIN_DIR}/cursor-tools"
echo "[install] Verify: cursor-tools version"
