#!/bin/bash

set -e

echo "=== Viewra Plugin Build Script ==="
echo "This script ensures all plugins are built with the correct architecture."
echo ""

# Configuration
PLUGINS_DIR="${PLUGINS_DIR:-./backend/data/plugins}"
TARGET_OS="${TARGET_OS:-linux}"
TARGET_ARCH="${TARGET_ARCH:-amd64}"

# Set build environment
export CGO_ENABLED=1  # Enable CGO for SQLite support
export GOOS=$TARGET_OS
export GOARCH=$TARGET_ARCH

echo "Build Configuration:"
echo "  Target OS/Arch: $TARGET_OS/$TARGET_ARCH"
echo "  Plugins Dir: $PLUGINS_DIR"
echo ""

# Check if plugins directory exists
if [ ! -d "$PLUGINS_DIR" ]; then
    echo "❌ Plugins directory not found: $PLUGINS_DIR"
    echo "Please run this script from the project root or set PLUGINS_DIR environment variable."
    exit 1
fi

# Save current directory and change to plugins directory
ORIGINAL_DIR=$(pwd)
cd "$PLUGINS_DIR" || {
    echo "❌ Failed to change to plugins directory: $PLUGINS_DIR"
    exit 1
}

# Find all plugin directories
PLUGIN_DIRS=($(find . -name "*_enricher" -o -name "*_transcoder" -type d | head -10))

if [ ${#PLUGIN_DIRS[@]} -eq 0 ]; then
    echo "❌ No plugin directories found in $PLUGINS_DIR"
    cd "$ORIGINAL_DIR"
    exit 1
fi

echo "Found ${#PLUGIN_DIRS[@]} plugin(s): ${PLUGIN_DIRS[*]}"
echo ""

BUILT_PLUGINS=()
FAILED_PLUGINS=()

# Build each plugin
for plugin_dir in "${PLUGIN_DIRS[@]}"; do
    plugin_name=$(basename "$plugin_dir")
    echo "🔨 Building $plugin_name..."
    
    if [ ! -d "$plugin_dir" ]; then
        echo "  ❌ Directory not found: $plugin_dir"
        FAILED_PLUGINS+=("$plugin_name: directory not found")
        continue
    fi
    
    if [ ! -f "$plugin_dir/main.go" ]; then
        echo "  ❌ main.go not found in $plugin_dir"
        FAILED_PLUGINS+=("$plugin_name: main.go not found")
        continue
    fi
    
    # Remove old binary
    if [ -f "$plugin_dir/$plugin_name" ]; then
        rm "$plugin_dir/$plugin_name"
        echo "  🗑️  Removed old binary"
    fi
    
    # Build the plugin
    cd "$plugin_dir" || {
        echo "  ❌ Failed to change to $plugin_dir"
        FAILED_PLUGINS+=("$plugin_name: cd failed")
        continue
    }
    
    if go build -ldflags='-s -w' -o "$plugin_name" .; then
        echo "  ✅ Built successfully"
        
        # Verify the binary
        if [ -x "$plugin_name" ]; then
            echo "  ✅ Binary is executable"
            
            # Get binary info
            if command -v file >/dev/null 2>&1; then
                file_info=$(file "$plugin_name" 2>/dev/null || echo "file command failed")
                echo "  ℹ️  Binary info: $file_info"
            fi
            
            BUILT_PLUGINS+=("$plugin_name")
        else
            echo "  ❌ Binary is not executable"
            FAILED_PLUGINS+=("$plugin_name: not executable")
        fi
    else
        echo "  ❌ Build failed"
        FAILED_PLUGINS+=("$plugin_name: build failed")
    fi
    
    cd "$PLUGINS_DIR" || {
        echo "  ⚠️  Failed to return to plugins directory"
    }
    echo ""
done

# Return to original directory
cd "$ORIGINAL_DIR"

echo "=== Build Summary ==="
echo "Successfully built: ${#BUILT_PLUGINS[@]} plugin(s)"
for plugin in "${BUILT_PLUGINS[@]}"; do
    echo "  ✅ $plugin"
done

if [ ${#FAILED_PLUGINS[@]} -gt 0 ]; then
    echo ""
    echo "Failed builds: ${#FAILED_PLUGINS[@]}"
    for failure in "${FAILED_PLUGINS[@]}"; do
        echo "  ❌ $failure"
    done
    echo ""
    echo "⚠️  Some plugins failed to build. Please check the errors above."
    exit 1
fi

echo ""
echo "=== Final Verification ==="
echo "All plugin binaries:"
cd "$PLUGINS_DIR"
find . \( -name "*_enricher" -o -name "*_transcoder" \) -type f -executable -exec ls -la {} \;
cd "$ORIGINAL_DIR"

echo ""
echo "✅ All plugins built successfully with $TARGET_OS/$TARGET_ARCH architecture!"
echo ""
echo "Next steps:"
echo "1. If running in Docker, restart the backend service:"
echo "   docker-compose restart backend"
echo "2. Check logs to verify plugins load correctly:"
echo "   docker-compose logs backend | grep -i plugin" 