#!/bin/bash

# Tilt Installation Script for Viewra
# Installs Tilt CLI on Linux systems

echo "🛠️  Installing Tilt for Viewra Development"
echo "=========================================="

# Check if Tilt is already installed
if command -v tilt >/dev/null 2>&1; then
    echo "✅ Tilt is already installed: $(tilt version)"
    exit 0
fi

# Detect OS
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

# Map architecture names
case $ARCH in
    x86_64)
        ARCH="x86_64"
        ;;
    aarch64|arm64)
        ARCH="arm64"
        ;;
    *)
        echo "❌ Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

echo "Detected OS: $OS, Architecture: $ARCH"

# Download and install Tilt
TILT_VERSION="0.33.20"
DOWNLOAD_URL="https://github.com/tilt-dev/tilt/releases/download/v${TILT_VERSION}/tilt.${TILT_VERSION}.${OS}.${ARCH}.tar.gz"

echo "Downloading Tilt v${TILT_VERSION}..."
curl -fsSL "$DOWNLOAD_URL" | tar -xzv tilt

# Move to system path
if [ -w /usr/local/bin ]; then
    mv tilt /usr/local/bin/
    INSTALL_DIR="/usr/local/bin"
elif [ -w "$HOME/.local/bin" ]; then
    mkdir -p "$HOME/.local/bin"
    mv tilt "$HOME/.local/bin/"
    INSTALL_DIR="$HOME/.local/bin"
else
    echo "Installing to current directory (add to PATH manually)"
    INSTALL_DIR="$(pwd)"
fi

echo "✅ Tilt installed to $INSTALL_DIR"

# Verify installation
if command -v tilt >/dev/null 2>&1; then
    echo "🚀 Installation successful!"
    echo "Tilt version: $(tilt version)"
    echo ""
    echo "Next steps:"
    echo "1. cd /home/fictional/Projects/viewra"
    echo "2. tilt up"
    echo "3. Open http://localhost:10350 for Tilt UI"
else
    echo "⚠️  Tilt installed but not in PATH"
    echo "Add $INSTALL_DIR to your PATH:"
    echo "export PATH=\"$INSTALL_DIR:\$PATH\""
fi
