#!/bin/sh
# daimon installer for macOS and Linux
# Usage: curl -fsSL https://raw.githubusercontent.com/Kishanmp3/daimon/main/install.sh | sh

set -e

REPO="Kishanmp3/daimon"
INSTALL_DIR="/usr/local/bin"
BINARY="daimon"

# ── Detect platform ──────────────────────────────────────────────────────────
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$OS" in
  darwin)
    case "$ARCH" in
      arm64)   ASSET="daimon-mac-arm64" ;;
      x86_64)  ASSET="daimon-mac-amd64" ;;
      *)
        echo "Unsupported architecture: $ARCH"
        exit 1
        ;;
    esac
    ;;
  linux)
    case "$ARCH" in
      x86_64)  ASSET="daimon-linux-amd64" ;;
      *)
        echo "Unsupported architecture: $ARCH"
        echo "Only linux/amd64 is currently supported."
        exit 1
        ;;
    esac
    ;;
  *)
    echo "Unsupported OS: $OS"
    echo "For Windows use: irm https://raw.githubusercontent.com/Kishanmp3/daimon/main/install.ps1 | iex"
    exit 1
    ;;
esac

# ── Resolve latest release tag ───────────────────────────────────────────────
echo "→ Fetching latest daimon release..."

if command -v curl >/dev/null 2>&1; then
  TAG=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
       | grep '"tag_name":' | cut -d '"' -f4)
elif command -v wget >/dev/null 2>&1; then
  TAG=$(wget -qO- "https://api.github.com/repos/$REPO/releases/latest" \
       | grep '"tag_name":' | cut -d '"' -f4)
else
  echo "Error: curl or wget is required."
  exit 1
fi

if [ -z "$TAG" ]; then
  echo "Error: could not determine latest release tag."
  exit 1
fi

URL="https://github.com/$REPO/releases/download/$TAG/$ASSET"
TMP=$(mktemp)

# ── Download ─────────────────────────────────────────────────────────────────
echo "→ Downloading daimon $TAG ($ASSET)..."

if command -v curl >/dev/null 2>&1; then
  curl -fsSL "$URL" -o "$TMP"
else
  wget -qO "$TMP" "$URL"
fi

chmod +x "$TMP"

# ── Install ──────────────────────────────────────────────────────────────────
echo "→ Installing to $INSTALL_DIR/$BINARY..."

if [ -w "$INSTALL_DIR" ]; then
  mv "$TMP" "$INSTALL_DIR/$BINARY"
else
  echo "  (requires sudo to write to $INSTALL_DIR)"
  sudo mv "$TMP" "$INSTALL_DIR/$BINARY"
fi

echo "→ daimon $TAG installed at $INSTALL_DIR/$BINARY"
echo ""

# ── First-time setup ─────────────────────────────────────────────────────────
"$INSTALL_DIR/$BINARY" summon
