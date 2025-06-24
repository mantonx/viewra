#!/bin/bash

# =============================================================================
# Unified Plugin Build Script for Viewra
# =============================================================================
# Consolidates all plugin build functionality with optimizations

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
PLUGINS_DIR="$PROJECT_ROOT/plugins"
DATA_DIR="$PROJECT_ROOT/viewra-data"
CACHE_DIR="$PROJECT_ROOT/.build-cache"

# Build configuration
BUILD_MODE="${1:-build}"
PLUGIN_NAME="${2:-}"
USE_CACHE="${USE_CACHE:-true}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# Persistent build container
BUILD_CONTAINER="viewra-plugin-builder"

# Print functions
print_status() { echo -e "${BLUE}[BUILD]${NC} $1"; }
print_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
print_warning() { echo -e "${YELLOW}[WARNING]${NC} $1"; }
print_error() { echo -e "${RED}[ERROR]${NC} $1"; }
print_info() { echo -e "${CYAN}[INFO]${NC} $1"; }

show_usage() {
    echo "Usage: $0 [command] [plugin_name]"
    echo ""
    echo "Commands:"
    echo "  build <plugin>     Build a specific plugin (default)"
    echo "  all                Build all plugins"
    echo "  watch <plugin>     Watch and auto-rebuild on changes"
    echo "  clean              Clean build caches"
    echo "  setup              Setup build environment"
    echo "  list               List available plugins"
    echo ""
    echo "Examples:"
    echo "  $0 build ffmpeg_software    # Build specific plugin"
    echo "  $0 all                      # Build all plugins"
    echo "  $0 watch ffmpeg_software    # Auto-rebuild on changes"
}

# Setup build environment with caching
setup_build_env() {
    print_status "Setting up build environment..."
    
    # Create cache directories
    mkdir -p "${CACHE_DIR}/go-mod"
    mkdir -p "${CACHE_DIR}/go-build"
    
    # Create Docker volumes for persistent caching
    docker volume create viewra-go-cache >/dev/null 2>&1 || true
    
    print_success "Build environment ready"
}

# Check if persistent build container exists and is running
check_build_container() {
    if ! docker ps -a --format "{{.Names}}" | grep -q "^${BUILD_CONTAINER}$"; then
        return 1
    fi
    
    if [ "$(docker inspect -f '{{.State.Running}}' ${BUILD_CONTAINER} 2>/dev/null)" != "true" ]; then
        docker start ${BUILD_CONTAINER} >/dev/null 2>&1
        sleep 1
    fi
    
    return 0
}

# Create persistent build container for faster builds
create_build_container() {
    print_info "Creating persistent build container..."
    
    docker run -d \
        --name ${BUILD_CONTAINER} \
        -v "${PROJECT_ROOT}:/workspace" \
        -v "viewra-go-cache:/go" \
        -v "${CACHE_DIR}/go-mod:/go/pkg/mod" \
        -v "${CACHE_DIR}/go-build:/root/.cache/go-build" \
        -w /workspace \
        -e GOCACHE=/root/.cache/go-build \
        -e GOMODCACHE=/go/pkg/mod \
        --platform linux/amd64 \
        golang:1.24-alpine \
        sleep infinity >/dev/null
    
    # Install build dependencies once
    docker exec ${BUILD_CONTAINER} sh -c "apk add --no-cache gcc musl-dev git" >/dev/null
    
    # Pre-download SDK dependencies
    if [ -d "${PROJECT_ROOT}/sdk" ]; then
        docker exec ${BUILD_CONTAINER} sh -c "cd /workspace/sdk && go mod download" >/dev/null 2>&1 || true
    fi
    
    print_success "Build container created"
}

# Build a single plugin
build_plugin() {
    local plugin_name="$1"
    local plugin_dir="${PLUGINS_DIR}/${plugin_name}"
    local output_dir="${DATA_DIR}/plugins/${plugin_name}"
    
    # Extract the actual binary name from the plugin path
    local binary_name=$(basename "$plugin_name")
    
    # Validate plugin exists
    if [ ! -d "$plugin_dir" ]; then
        print_error "Plugin directory not found: $plugin_dir"
        return 1
    fi
    
    if [ ! -f "$plugin_dir/main.go" ]; then
        print_error "main.go not found in $plugin_dir"
        return 1
    fi
    
    print_status "Building $plugin_name..."
    local BUILD_START=$(date +%s)
    
    # Create output directory
    mkdir -p "$output_dir"
    
    # Get current user info for proper file ownership
    local HOST_UID=$(id -u)
    local HOST_GID=$(id -g)
    
    # Use persistent container if available for speed
    if [ "$USE_CACHE" == "true" ] && check_build_container; then
        # Fast build with persistent container
        docker exec ${BUILD_CONTAINER} sh -c "
            cd /workspace/plugins/${plugin_name} && \
            CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build \
                -buildvcs=false \
                -ldflags='-s -w' \
                -trimpath \
                -o /workspace/viewra-data/plugins/${plugin_name}/${binary_name} . && \
            chown ${HOST_UID}:${HOST_GID} /workspace/viewra-data/plugins/${plugin_name}/${binary_name}
        "
    else
        # Standard Docker build (slower but always works)
        docker run --rm \
            -v "${PROJECT_ROOT}:/workspace" \
            -w /workspace \
            --platform linux/amd64 \
            golang:1.24-alpine \
            sh -c "
                apk add --no-cache gcc musl-dev git && \
                cd plugins/${plugin_name} && \
                CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build \
                    -buildvcs=false \
                    -ldflags='-s -w' \
                    -o /workspace/viewra-data/plugins/${plugin_name}/${binary_name} . && \
                chown ${HOST_UID}:${HOST_GID} /workspace/viewra-data/plugins/${plugin_name}/${binary_name}
            "
    fi
    
    local BUILD_END=$(date +%s)
    local BUILD_TIME=$((BUILD_END - BUILD_START))
    
    if [ $? -eq 0 ]; then
        # Copy plugin configuration
        if [ -f "$plugin_dir/plugin.cue" ]; then
            cp "$plugin_dir/plugin.cue" "$output_dir/"
        fi
        
        # Make executable
        chmod +x "$output_dir/$binary_name"
        
        # Get file size
        local SIZE=$(du -h "$output_dir/$binary_name" | cut -f1)
        
        print_success "Built $plugin_name in ${BUILD_TIME}s (${SIZE})"
        
        # Hot reload if backend is running
        if docker ps -q -f name=viewra-backend >/dev/null 2>&1; then
            docker exec viewra-backend-1 pkill -f "$plugin_name" 2>/dev/null || true
            print_info "Plugin process terminated, triggering refresh..."
            
            # Trigger plugin refresh in plugin module
            if curl -s -X POST http://localhost:8080/api/v1/plugins/external/refresh >/dev/null 2>&1; then
                print_info "Plugin module refreshed"
            fi
            
            # Trigger plugin refresh in playback module for transcoding plugins
            if [[ "$plugin_name" == *"_transcoder" ]] || [[ "$plugin_name" == "ffmpeg_"* ]]; then
                if curl -s -X POST http://localhost:8080/api/playback/plugins/refresh >/dev/null 2>&1; then
                    print_info "Playback module refreshed for transcoding plugin"
                fi
            fi
            
            # Enable the plugin if it was previously enabled
            if curl -s -X POST "http://localhost:8080/api/v1/plugins/${plugin_name}/reload" >/dev/null 2>&1; then
                print_success "Plugin ${plugin_name} reloaded and enabled"
            fi
        fi
        
        return 0
    else
        print_error "Failed to build $plugin_name"
        return 1
    fi
}

# Build all plugins
build_all_plugins() {
    print_status "Building all plugins..."
    
    # Setup persistent container for faster builds
    if [ "$USE_CACHE" == "true" ] && ! check_build_container; then
        create_build_container
    fi
    
    local plugins=()
    
    # Function to find plugins recursively
    find_plugins() {
        local search_dir="$1"
        local prefix="$2"
        
        for dir in "$search_dir"/*; do
            if [ ! -d "$dir" ]; then
                continue
            fi
            
            local dir_name=$(basename "$dir")
            local full_path="${prefix}${dir_name}"
            
            if [ -f "$dir/main.go" ]; then
                # This directory contains a plugin
                plugins+=("$full_path")
            else
                # Check subdirectories (but only one level deep to avoid infinite recursion)
                if [ -z "$prefix" ]; then
                    find_plugins "$dir" "${dir_name}/"
                fi
            fi
        done
    }
    
    # Find all plugins starting from plugins directory
    find_plugins "$PLUGINS_DIR" ""
    
    if [ ${#plugins[@]} -eq 0 ]; then
        print_warning "No plugins found to build"
        return 0
    fi
    
    print_info "Found ${#plugins[@]} plugin(s) to build"
    
    local success=0
    local failed=0
    
    for plugin in "${plugins[@]}"; do
        if build_plugin "$plugin"; then
            ((success++))
        else
            ((failed++))
        fi
    done
    
    print_status "Build Summary:"
    print_success "Successfully built: $success plugin(s)"
    if [ $failed -gt 0 ]; then
        print_error "Failed: $failed plugin(s)"
        return 1
    fi
    
    return 0
}

# Watch plugin for changes and rebuild
watch_plugin() {
    local plugin_name="$1"
    
    if [ -z "$plugin_name" ]; then
        print_error "Plugin name required for watch mode"
        show_usage
        exit 1
    fi
    
    # Initial build
    build_plugin "$plugin_name"
    
    print_info "Watching for changes in $plugin_name..."
    print_info "Press Ctrl+C to stop"
    
    # Use fswatch if available, otherwise inotifywait
    if command -v fswatch >/dev/null 2>&1; then
        fswatch -o "$PLUGINS_DIR/$plugin_name" "$PROJECT_ROOT/sdk/" | while read; do
            print_warning "Changes detected, rebuilding..."
            build_plugin "$plugin_name"
        done
    elif command -v inotifywait >/dev/null 2>&1; then
        while inotifywait -r -e modify,create,delete "$PLUGINS_DIR/$plugin_name" "$PROJECT_ROOT/sdk/"; do
            print_warning "Changes detected, rebuilding..."
            build_plugin "$plugin_name"
        done
    else
        print_error "Install fswatch or inotify-tools for file watching"
        print_info "macOS: brew install fswatch"
        print_info "Linux: apt-get install inotify-tools"
        exit 1
    fi
}

# Clean build caches
clean_caches() {
    print_warning "Cleaning build caches..."
    
    # Stop and remove build container
    docker stop ${BUILD_CONTAINER} 2>/dev/null || true
    docker rm ${BUILD_CONTAINER} 2>/dev/null || true
    
    # Remove cache directory
    rm -rf "${CACHE_DIR}"
    
    # Remove Docker volumes
    docker volume rm viewra-go-cache 2>/dev/null || true
    
    print_success "Build caches cleaned"
}

# List available plugins
list_plugins() {
    print_info "Available plugins:"
    
    # Function to list plugins recursively
    list_plugins_recursive() {
        local search_dir="$1"
        local prefix="$2"
        
        for dir in "$search_dir"/*; do
            if [ ! -d "$dir" ]; then
                continue
            fi
            
            local dir_name=$(basename "$dir")
            local full_path="${prefix}${dir_name}"
            
            if [ -f "$dir/main.go" ]; then
                # This directory contains a plugin
                local binary_name=$(basename "$dir")
                local binary="$DATA_DIR/plugins/$full_path/$binary_name"
                
                if [ -x "$binary" ]; then
                    echo "  ✅ $full_path (built)"
                else
                    echo "  ⭕ $full_path (not built)"
                fi
            else
                # Check subdirectories (but only one level deep)
                if [ -z "$prefix" ]; then
                    list_plugins_recursive "$dir" "${dir_name}/"
                fi
            fi
        done
    }
    
    # List all plugins starting from plugins directory
    list_plugins_recursive "$PLUGINS_DIR" ""
}

# Main execution
main() {
    case "$BUILD_MODE" in
        build)
            if [ -z "$PLUGIN_NAME" ]; then
                print_error "Plugin name required"
                show_usage
                exit 1
            fi
            build_plugin "$PLUGIN_NAME"
            ;;
        
        all)
            build_all_plugins
            ;;
        
        watch)
            watch_plugin "$PLUGIN_NAME"
            ;;
        
        setup)
            setup_build_env
            ;;
        
        clean)
            clean_caches
            ;;
        
        list)
            list_plugins
            ;;
        
        -h|--help|help)
            show_usage
            exit 0
            ;;
        
        *)
            # Default to build if plugin name provided as first argument
            if [ -n "$BUILD_MODE" ] && [ -d "$PLUGINS_DIR/$BUILD_MODE" ]; then
                build_plugin "$BUILD_MODE"
            else
                print_error "Unknown command: $BUILD_MODE"
                show_usage
                exit 1
            fi
            ;;
    esac
}

# Run main function
main