#!/bin/bash
# Plugin Development Helper Script
# Ensures plugins are built correctly for the container environment

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Function to display usage
usage() {
    echo "Usage: $0 [COMMAND] [OPTIONS]"
    echo ""
    echo "Commands:"
    echo "  build <plugin>     Build a specific plugin for container"
    echo "  rebuild <plugin>   Clean and rebuild a plugin"
    echo "  watch <plugin>     Build plugin and watch for changes"
    echo "  test <plugin>      Test if plugin works in container"
    echo "  list              List all available plugins"
    echo "  verify            Verify all plugin binaries"
    echo ""
    echo "Options:"
    echo "  -h, --help        Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0 build ffmpeg_software"
    echo "  $0 watch ffmpeg_software"
    echo "  $0 verify"
}

# Function to build a plugin
build_plugin() {
    local plugin=$1
    echo -e "${GREEN}Building plugin: $plugin${NC}"
    
    # Ensure plugin directory exists
    if [ ! -d "plugins/$plugin" ]; then
        echo -e "${RED}Error: Plugin directory not found: plugins/$plugin${NC}"
        exit 1
    fi
    
    # Create output directory
    mkdir -p viewra-data/plugins/$plugin
    
    # Build in container with same environment as runtime
    echo -e "${YELLOW}Building for Alpine Linux (container environment)...${NC}"
    docker run --rm \
        -v "$(pwd):/workspace" \
        -w /workspace \
        --platform linux/amd64 \
        golang:1.24-alpine \
        sh -c "
            set -e
            apk add --no-cache gcc musl-dev git
            cd plugins/$plugin
            echo 'Building with CGO_ENABLED=1 for Alpine...'
            CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build \
                -ldflags '-extldflags \"-static\"' \
                -o /workspace/viewra-data/plugins/$plugin/$plugin .
            chmod +x /workspace/viewra-data/plugins/$plugin/$plugin
        "
    
    # Copy plugin.cue if it exists
    if [ -f "plugins/$plugin/plugin.cue" ]; then
        cp "plugins/$plugin/plugin.cue" "viewra-data/plugins/$plugin/"
    fi
    
    # Verify the build
    verify_plugin "$plugin"
    
    # Auto-enable the plugin if backend is running
    if command -v curl >/dev/null 2>&1 && curl -s http://localhost:8080/api/health >/dev/null 2>&1; then
        echo -e "${YELLOW}Auto-enabling plugin in backend...${NC}"
        curl -s -X POST http://localhost:8080/api/plugin-manager/external/$plugin/enable >/dev/null
        curl -s -X POST http://localhost:8080/api/plugin-manager/external/$plugin/load >/dev/null
        echo -e "${GREEN}✅ Plugin enabled and loaded${NC}"
    fi
}

# Function to rebuild a plugin (clean first)
rebuild_plugin() {
    local plugin=$1
    echo -e "${YELLOW}Cleaning old build...${NC}"
    rm -rf "viewra-data/plugins/$plugin"
    build_plugin "$plugin"
}

# Function to verify a plugin
verify_plugin() {
    local plugin=$1
    local binary="viewra-data/plugins/$plugin/$plugin"
    
    echo -e "${YELLOW}Verifying plugin: $plugin${NC}"
    
    if [ ! -f "$binary" ]; then
        echo -e "${RED}❌ Binary not found: $binary${NC}"
        return 1
    fi
    
    # Check if executable
    if [ ! -x "$binary" ]; then
        echo -e "${RED}❌ Binary not executable: $binary${NC}"
        return 1
    fi
    
    # Check file type
    file_info=$(file "$binary" 2>/dev/null || echo "unknown")
    echo "File type: $file_info"
    
    # Test in container
    echo -e "${YELLOW}Testing in container...${NC}"
    if docker run --rm \
        -v "$(pwd)/viewra-data/plugins/$plugin:/plugin:ro" \
        alpine:latest \
        /plugin/$plugin --version 2>&1 | grep -q "version"; then
        echo -e "${GREEN}✅ Plugin verified successfully${NC}"
    else
        echo -e "${YELLOW}⚠️  Plugin may not run correctly in container${NC}"
    fi
}

# Function to watch and rebuild on changes
watch_plugin() {
    local plugin=$1
    
    # Initial build
    build_plugin "$plugin"
    
    echo -e "${GREEN}Watching for changes in plugins/$plugin...${NC}"
    echo -e "${YELLOW}Press Ctrl+C to stop${NC}"
    
    # Use fswatch if available, otherwise fall back to simple loop
    if command -v fswatch >/dev/null 2>&1; then
        fswatch -o "plugins/$plugin" | while read _; do
            echo -e "${YELLOW}Changes detected, rebuilding...${NC}"
            build_plugin "$plugin"
            echo -e "${GREEN}Rebuild complete. Watching for changes...${NC}"
        done
    else
        # Simple polling fallback
        local last_modified=$(stat -c %Y "plugins/$plugin" 2>/dev/null || echo 0)
        while true; do
            sleep 2
            local current_modified=$(stat -c %Y "plugins/$plugin" 2>/dev/null || echo 0)
            if [ "$current_modified" != "$last_modified" ]; then
                echo -e "${YELLOW}Changes detected, rebuilding...${NC}"
                build_plugin "$plugin"
                echo -e "${GREEN}Rebuild complete. Watching for changes...${NC}"
                last_modified=$current_modified
            fi
        done
    fi
}

# Function to list plugins
list_plugins() {
    echo -e "${GREEN}Available plugins:${NC}"
    find plugins -maxdepth 1 -type d -name "*_*" -printf "  %f\n" 2>/dev/null | sort
}

# Function to verify all plugins
verify_all() {
    echo -e "${GREEN}Verifying all plugin binaries...${NC}"
    for plugin_dir in viewra-data/plugins/*/; do
        if [ -d "$plugin_dir" ]; then
            plugin=$(basename "$plugin_dir")
            verify_plugin "$plugin" || true
            echo ""
        fi
    done
}

# Function to test plugin in container
test_plugin() {
    local plugin=$1
    local binary="viewra-data/plugins/$plugin/$plugin"
    
    if [ ! -f "$binary" ]; then
        echo -e "${RED}Error: Plugin not built yet. Run: $0 build $plugin${NC}"
        exit 1
    fi
    
    echo -e "${GREEN}Testing plugin in container: $plugin${NC}"
    docker run --rm -it \
        -v "$(pwd)/viewra-data:/app/viewra-data:ro" \
        -w /app \
        alpine:latest \
        sh -c "
            echo 'Testing plugin binary...'
            /app/viewra-data/plugins/$plugin/$plugin --version || echo 'Failed to run'
            echo ''
            echo 'Checking dependencies...'
            ldd /app/viewra-data/plugins/$plugin/$plugin 2>&1 || echo 'Statically linked or error'
        "
}

# Main script logic
case "$1" in
    build)
        if [ -z "$2" ]; then
            echo -e "${RED}Error: Plugin name required${NC}"
            usage
            exit 1
        fi
        build_plugin "$2"
        ;;
    rebuild)
        if [ -z "$2" ]; then
            echo -e "${RED}Error: Plugin name required${NC}"
            usage
            exit 1
        fi
        rebuild_plugin "$2"
        ;;
    watch)
        if [ -z "$2" ]; then
            echo -e "${RED}Error: Plugin name required${NC}"
            usage
            exit 1
        fi
        watch_plugin "$2"
        ;;
    test)
        if [ -z "$2" ]; then
            echo -e "${RED}Error: Plugin name required${NC}"
            usage
            exit 1
        fi
        test_plugin "$2"
        ;;
    list)
        list_plugins
        ;;
    verify)
        verify_all
        ;;
    -h|--help|help)
        usage
        ;;
    *)
        echo -e "${RED}Error: Unknown command: $1${NC}"
        usage
        exit 1
        ;;
esac