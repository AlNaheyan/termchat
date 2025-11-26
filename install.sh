#!/bin/sh

# Termchat installer script
# Usage: curl -fsSL https://raw.githubusercontent.com/AlNaheyan/termchat/main/install.sh | sh

set -e

REPO="AlNaheyan/termchat"
INSTALL_DIR="/usr/local/bin"
BINARY_NAME="termchat"

# Detect platform
OS="$(uname -s)"
ARCH="$(uname -m)"

case "$OS" in
    Darwin)
        if [ "$ARCH" = "arm64" ]; then
            PLATFORM="macos-arm64"
        else
            PLATFORM="macos-amd64"
        fi
        ;;
    Linux)
        if [ "$ARCH" = "aarch64" ] || [ "$ARCH" = "arm64" ]; then
            PLATFORM="linux-arm64"
        else
            PLATFORM="linux-amd64"
        fi
        ;;
    *)
        echo "Unsupported operating system: $OS"
        exit 1
        ;;
esac

echo "📦 Installing termchat for $PLATFORM..."

# Get latest release
LATEST_RELEASE=$(curl -sL "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "$LATEST_RELEASE" ]; then
    echo "❌ Failed to get latest release"
    exit 1
fi

echo "📥 Downloading termchat $LATEST_RELEASE..."

# Download binary
DOWNLOAD_URL="https://github.com/$REPO/releases/download/$LATEST_RELEASE/termchat-$PLATFORM"
TMP_FILE="/tmp/termchat"

if ! curl -fSL "$DOWNLOAD_URL" -o "$TMP_FILE"; then
    echo "❌ Download failed"
    exit 1
fi

# Make executable
chmod +x "$TMP_FILE"

# Install binary
if [ -w "$INSTALL_DIR" ]; then
    mv "$TMP_FILE" "$INSTALL_DIR/$BINARY_NAME"
else
    echo "🔐 Installing to $INSTALL_DIR requires sudo..."
    sudo mv "$TMP_FILE" "$INSTALL_DIR/$BINARY_NAME"
fi

# Verify installation
if command -v termchat >/dev/null 2>&1; then
    echo "✅ termchat installed successfully!"
    echo ""
    echo "Run: termchat myroom"
else
    echo "⚠️  Installation complete, but termchat not found in PATH"
    echo "You may need to add $INSTALL_DIR to your PATH"
fi
