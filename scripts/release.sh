#!/usr/bin/env bash
set -euo pipefail

VERSION=$(git describe --tags --abbrev=0 2>/dev/null || echo "dev")
BINARY_NAME="harness"
INSTALL_DIR="$HOME/.local/bin"

echo "Building ${BINARY_NAME} ${VERSION}..."
go build -ldflags "-X main.version=${VERSION}" -o "${BINARY_NAME}" .

echo "Installing to ${INSTALL_DIR}..."
mv "${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"

echo "Done. Installed:"
"${INSTALL_DIR}/${BINARY_NAME}" --version 2>/dev/null || echo "(no --version flag wired up yet)"
