#!/usr/bin/env bash
set -euo pipefail

REPO="avicuna/ai-council"
BINARY="council-personal"
INSTALL_DIR="${HOME}/.local/bin"

# Detect platform
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64) ARCH="arm64" ;;
  arm64)   ARCH="arm64" ;;
  *)       echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

case "$OS" in
  darwin|linux) ;;
  *)            echo "Unsupported OS: $OS"; exit 1 ;;
esac

# Check if already installed and up to date
if command -v "$BINARY" &>/dev/null; then
  CURRENT=$("$BINARY" --version 2>/dev/null || echo "unknown")
  echo "council-personal already installed: $CURRENT"

  LATEST=$(curl -sSf "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
  if [ "$CURRENT" = "$LATEST" ] || [ "$CURRENT" = "${LATEST#v}" ]; then
    echo "Already up to date."
    exit 0
  fi
  echo "Updating to $LATEST..."
fi

# Get latest release URL
ASSET="${BINARY}_${OS}_${ARCH}.tar.gz"
DOWNLOAD_URL="https://github.com/${REPO}/releases/latest/download/${ASSET}"

echo "Downloading $BINARY for ${OS}/${ARCH}..."

# Create install dir
mkdir -p "$INSTALL_DIR"

# Download and extract
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

if ! curl -sSfL "$DOWNLOAD_URL" -o "$TMPDIR/$ASSET"; then
  echo ""
  echo "Pre-built binary not available for ${OS}/${ARCH}."
  echo ""
  if command -v go &>/dev/null; then
    echo "Building from source with Go..."
    GOBIN="$INSTALL_DIR" go install "github.com/${REPO}@latest"
    echo "Installed $BINARY to $INSTALL_DIR"
  else
    echo "Install options:"
    echo "  1. Install Go (https://go.dev/dl/) then re-run this script"
    echo "  2. Build manually: go build -o council-personal ."
    exit 1
  fi
else
  tar xzf "$TMPDIR/$ASSET" -C "$TMPDIR"
  mv "$TMPDIR/$BINARY" "$INSTALL_DIR/$BINARY"
  chmod +x "$INSTALL_DIR/$BINARY"
  echo "Installed $BINARY to $INSTALL_DIR/$BINARY"
fi

# Check PATH
if ! echo "$PATH" | tr ':' '\n' | grep -q "^${INSTALL_DIR}$"; then
  echo ""
  echo "NOTE: $INSTALL_DIR is not in your PATH. Add it:"
  echo "  echo 'export PATH=\"\$HOME/.local/bin:\$PATH\"' >> ~/.zshrc"
fi

echo ""
echo "Verify: $BINARY --help"
