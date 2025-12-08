#!/bin/sh
set -e

REPO="kessler-frost/styx"
INSTALL_DIR="${STYX_INSTALL_DIR:-$HOME/.local/bin}"
PLUGIN_DIR="${STYX_PLUGIN_DIR:-$HOME/.local/lib/styx/plugins}"

# Detect architecture
ARCH=$(uname -m)
case $ARCH in
  arm64|aarch64) ARCH="arm64" ;;
  x86_64) ARCH="amd64" ;;
  *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

# Detect OS
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
if [ "$OS" != "darwin" ]; then
  echo "Styx only supports macOS (darwin). Detected: $OS"
  exit 1
fi

# Get latest version
VERSION=$(curl -sL "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name"' | cut -d'"' -f4)
if [ -z "$VERSION" ]; then
  echo "Failed to get latest version"
  exit 1
fi

echo "Installing styx $VERSION for $OS/$ARCH..."

# Download and extract
TARBALL="styx_${VERSION#v}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/$REPO/releases/download/$VERSION/$TARBALL"

TMP_DIR=$(mktemp -d)
trap "rm -rf $TMP_DIR" EXIT

curl -sL "$URL" -o "$TMP_DIR/styx.tar.gz"
tar -xzf "$TMP_DIR/styx.tar.gz" -C "$TMP_DIR"

mkdir -p "$INSTALL_DIR" "$PLUGIN_DIR"
mv "$TMP_DIR/styx" "$INSTALL_DIR/"
mv "$TMP_DIR/apple-container" "$PLUGIN_DIR/"
chmod +x "$INSTALL_DIR/styx" "$PLUGIN_DIR/apple-container"

echo ""
echo "Installed styx to $INSTALL_DIR/styx"
echo "Installed plugin to $PLUGIN_DIR/apple-container"
echo ""
if ! echo "$PATH" | grep -q "$INSTALL_DIR"; then
  echo "Add this to your shell profile:"
  echo "  export PATH=\"\$PATH:$INSTALL_DIR\""
fi
