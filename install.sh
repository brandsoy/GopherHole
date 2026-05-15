#!/usr/bin/env bash
set -euo pipefail

BINARY_NAME="gopherhole"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"

if ! command -v go >/dev/null 2>&1; then
  echo "Error: Go is not installed or not in PATH." >&2
  exit 1
fi

echo "Building ${BINARY_NAME}..."
go build -o "${BINARY_NAME}" .

mkdir -p "${INSTALL_DIR}"
cp "${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
chmod +x "${INSTALL_DIR}/${BINARY_NAME}"

echo "Installed to ${INSTALL_DIR}/${BINARY_NAME}"

case ":$PATH:" in
  *":${INSTALL_DIR}:"*)
    echo "${INSTALL_DIR} is already in PATH."
    ;;
  *)
    echo "\nAdd this to your shell profile (~/.zshrc, ~/.bashrc, etc.):"
    echo "export PATH=\"${INSTALL_DIR}:\$PATH\""
    ;;
esac

echo "\nNext steps:"
echo "  ${BINARY_NAME} init"
echo "  ${BINARY_NAME}"
