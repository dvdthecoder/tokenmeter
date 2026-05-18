#!/usr/bin/env sh
# curl -fsSL https://raw.githubusercontent.com/dvdthecoder/tokenmeter/main/scripts/install.sh | sh
set -e

REPO="dvdthecoder/tokenmeter"
BIN="tokenmeter"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH" && exit 1 ;;
esac

LATEST=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
URL="https://github.com/${REPO}/releases/download/${LATEST}/${BIN}-${OS}-${ARCH}"

echo "Downloading ${BIN} ${LATEST} for ${OS}/${ARCH}..."
curl -fsSL "$URL" -o "/tmp/${BIN}"
chmod +x "/tmp/${BIN}"
mv "/tmp/${BIN}" "${INSTALL_DIR}/${BIN}"

echo "Installed to ${INSTALL_DIR}/${BIN}"
echo "Run 'tokenmeter install' to start the daemon and configure your AI tools."
