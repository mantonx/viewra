#!/bin/bash

set -euo pipefail

# =============================================================================
# Unified Plugin Build Script for Viewra
# =============================================================================
# This script consolidates all plugin build functionality and ensures plugins
# are built with the correct Docker environment compatibility.

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
PLUGINS_DIR="$PROJECT_ROOT/backend/data/plugins"

# Build modes
BUILD_MODE="${1:-auto}"  # auto, docker, host
SPECIFIC_PLUGIN="${2:-}" # Optional: build only specific plugin

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${BLUE}[BUILD]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_info() {
    echo -e "${CYAN}[INFO]${NC} $1"
}

# Function to show usage
show_usage() {
    echo "Usage: $0 [build_mode] [plugin_name]"
    echo ""
    echo "Build modes:"
    echo "  auto     - Automatically detect best build method (default)"
    echo "  docker   - Force Docker container build"
    echo "  host     - Force host system build"
    echo ""
    echo "Examples:"
    echo "  $0                           # Build all plugins (auto mode)"
    echo "  $0 docker                    # Build all plugins in Docker"
    echo "  $0 auto ffmpeg_transcoder    # Build only FFmpeg transcoder (auto mode)"
    echo "  $0 docker ffmpeg_transcoder  # Build only FFmpeg transcoder in Docker"
}

# Function to check if Docker is available and running
check_docker() {
    if ! command -v docker >/dev/null 2>&1; then
        return 1
    fi
    
    if ! docker ps >/dev/null 2>&1; then
        return 1
    fi
    
    return 0
}

# Function to get backend container ID
get_backend_container() {
    local container_id
    container_id=$(docker ps --filter "name=viewra.*backend" --format "{{.ID}}" | head -1)
    
    if [[ -z "$container_id" ]]; then
        # Try alternative naming patterns
        container_id=$(docker ps --filter "expose=8080" --format "{{.ID}}" | head -1)
    fi
    
    if [[ -z "$container_id" ]]; then
        return 1
    fi
    
    echo "$container_id"
}

# Function to validate plugins directory
validate_environment() {
    if [[ ! -d "$PLUGINS_DIR" ]]; then
        print_error "Plugins directory not found: $PLUGINS_DIR"
        return 1
    fi
    
    print_info "Project root: $PROJECT_ROOT"
    print_info "Plugins directory: $PLUGINS_DIR"
}

# Function to find plugin directories
find_plugin_dirs() {
    if [[ -n "$SPECIFIC_PLUGIN" ]]; then
        if [[ -d "$PLUGINS_DIR/$SPECIFIC_PLUGIN" ]]; then
            echo "$PLUGINS_DIR/$SPECIFIC_PLUGIN"
        else
            print_error "Plugin directory not found: $PLUGINS_DIR/$SPECIFIC_PLUGIN"
            return 1
        fi
    else
        find "$PLUGINS_DIR" -maxdepth 1 -type d \( -name "*_transcoder" -o -name "*_enricher" -o -name "*_scanner" \) | sort
    fi
}

# Function to detect if plugin needs CGO
detect_cgo_requirement() {
    local plugin_dir="$1"
    
    # Check for CGO-requiring imports in Go files
    local cgo_imports=(
        "github.com/mattn/go-sqlite3"
        "database/sql"
        "_cgo"
        "#cgo"
        "gorm.io/driver/sqlite"
    )
    
    for import in "${cgo_imports[@]}"; do
        if grep -r "$import" "$plugin_dir"/*.go >/dev/null 2>&1; then
            echo "true"
            return
        fi
    done
    
    # Check go.mod for CGO dependencies
    if [[ -f "$plugin_dir/go.mod" ]]; then
        local cgo_modules=(
            "github.com/mattn/go-sqlite3"
            "modernc.org/sqlite"
            "gorm.io/driver/sqlite"
        )
        
        for module in "${cgo_modules[@]}"; do
            if grep "$module" "$plugin_dir/go.mod" >/dev/null 2>&1; then
                echo "true"
                return
            fi
        done
    fi
    
    echo "false"
}

# Function to build plugin in Docker container
build_plugin_docker() {
    local plugin_dir="$1"
    local plugin_name="$2"
    local container_id="$3"
    
    print_status "Building $plugin_name in Docker container..."
    
    # Get relative path from project root for Docker context
    local relative_plugin_dir="data/plugins/$plugin_name"
    
    # Clean old binaries first
    print_info "Cleaning old binaries for $plugin_name"
    docker exec "$container_id" find "/app/$relative_plugin_dir" -name "$plugin_name" -type f -exec rm -f {} \; 2>/dev/null || true
    
    # Build the plugin inside the container
    print_info "Running go build for $plugin_name..."
    local build_cmd="cd /app/$relative_plugin_dir && CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -ldflags='-s -w' -o $plugin_name ."
    
    if docker exec "$container_id" sh -c "$build_cmd"; then
        print_success "Built $plugin_name successfully"
        
        # Verify the binary exists and is executable
        if docker exec "$container_id" test -x "/app/$relative_plugin_dir/$plugin_name"; then
            print_success "Binary $plugin_name is executable"
            
            # Get binary info
            local file_info
            file_info=$(docker exec "$container_id" file "/app/$relative_plugin_dir/$plugin_name" 2>/dev/null || echo "file command failed")
            print_info "Binary info: $file_info"
            
            return 0
        else
            print_error "Binary $plugin_name is not executable"
            return 1
        fi
    else
        print_error "Failed to build $plugin_name"
        return 1
    fi
}

# Function to build plugin on host
build_plugin_host() {
    local plugin_dir="$1"
    local plugin_name="$2"
    
    print_status "Building $plugin_name on host..."
    
    # Detect CGO requirement
    local needs_cgo
    needs_cgo=$(detect_cgo_requirement "$plugin_dir")
    
    # Set build environment
    export GOOS=linux
    export GOARCH=amd64
    export CGO_ENABLED="0"
    
    if [[ "$needs_cgo" == "true" ]]; then
        export CGO_ENABLED="1"
        print_info "CGO enabled for $plugin_name"
    fi
    
    # Change to plugin directory
    cd "$plugin_dir"
    
    # Clean old binaries
    rm -f "$plugin_name" *.exe 2>/dev/null || true
    
    # Build with specific flags
    local build_flags=(
        "-ldflags=-s -w"
        "-trimpath"
        "-o" "$plugin_name"
        "."
    )
    
    if go build "${build_flags[@]}"; then
        print_success "Built $plugin_name successfully"
        
        # Verify the binary
        if [[ -x "$plugin_name" ]]; then
            print_success "Binary $plugin_name is executable"
            
            # Get binary info
            if command -v file >/dev/null 2>&1; then
                local file_info
                file_info=$(file "$plugin_name" 2>/dev/null || echo "file command failed")
                print_info "Binary info: $file_info"
            fi
            
            return 0
        else
            print_error "Binary $plugin_name is not executable"
            return 1
        fi
    else
        print_error "Failed to build $plugin_name"
        return 1
    fi
}

# Function to enable FFmpeg transcoder by default
enable_ffmpeg_transcoder() {
    local container_id="$1"
    
    print_status "Ensuring FFmpeg transcoder is enabled by default..."
    
    # The plugin is already configured with enabled_by_default: true in plugin.cue
    # and special handling in external_manager.go, but let's ensure database consistency
    
    local sql_commands=(
        "UPDATE plugins SET status = 'enabled' WHERE plugin_id = 'ffmpeg_transcoder' AND status != 'enabled';"
        "UPDATE plugins SET enabled_at = datetime('now') WHERE plugin_id = 'ffmpeg_transcoder' AND enabled_at IS NULL;"
    )
    
    local has_updates=false
    for sql_cmd in "${sql_commands[@]}"; do
        if docker exec "$container_id" sqlite3 /app/viewra-data/database.db "$sql_cmd" 2>/dev/null; then
            local changes
            changes=$(docker exec "$container_id" sqlite3 /app/viewra-data/database.db "SELECT changes();" 2>/dev/null || echo "0")
            if [[ "$changes" != "0" ]]; then
                print_info "Database updated: $sql_cmd"
                has_updates=true
            fi
        else
            print_warning "Could not execute SQL (database may not exist yet): $sql_cmd"
        fi
    done
    
    if [[ "$has_updates" == "true" ]]; then
        print_success "FFmpeg transcoder enabled in database"
    else
        print_info "FFmpeg transcoder already enabled in database"
    fi
}

# Function to determine best build mode
determine_build_mode() {
    if [[ "$BUILD_MODE" != "auto" ]]; then
        echo "$BUILD_MODE"
        return
    fi
    
    # Auto-detect best build mode
    if check_docker; then
        local container_id
        if container_id=$(get_backend_container); then
            echo "docker"
            return
        else
            print_warning "Docker available but backend container not running"
        fi
    fi
    
    echo "host"
}

# Main build function
main() {
    print_status "Starting unified plugin build process..."
    
    # Show usage if help requested
    if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
        show_usage
        exit 0
    fi
    
    # Validate environment
    validate_environment || exit 1
    
    # Determine build mode
    local effective_build_mode
    effective_build_mode=$(determine_build_mode)
    
    # Show build mode with appropriate context
    if [[ "$effective_build_mode" == "docker" ]]; then
        print_info "Build mode: docker (container detected)"
    elif [[ "$effective_build_mode" == "host" ]]; then
        print_info "Build mode: host (no container or forced host mode)"
    else
        print_info "Build mode: $effective_build_mode"
    fi
    
    # Find plugins to build
    local plugin_dirs
    mapfile -t plugin_dirs < <(find_plugin_dirs)
    
    if [[ ${#plugin_dirs[@]} -eq 0 ]]; then
        print_error "No plugin directories found"
        exit 1
    fi
    
    print_info "Found ${#plugin_dirs[@]} plugin(s) to build"
    for dir in "${plugin_dirs[@]}"; do
        print_info "  - $(basename "$dir")"
    done
    
    # Prepare for Docker build if needed
    local container_id=""
    if [[ "$effective_build_mode" == "docker" ]]; then
        if ! check_docker; then
            print_error "Docker is not available or not running"
            exit 1
        fi
        
        container_id=$(get_backend_container)
        if [[ -z "$container_id" ]]; then
            print_error "Backend container not running. Please start with: docker-compose up -d"
            exit 1
        fi
        
        print_info "Using Docker container: $container_id"
    fi
    
    # Build each plugin
    local built_plugins=()
    local failed_plugins=()
    
    for plugin_dir in "${plugin_dirs[@]}"; do
        local plugin_name
        plugin_name=$(basename "$plugin_dir")
        
        # Validate plugin structure
        if [[ ! -f "$plugin_dir/main.go" ]]; then
            print_error "main.go not found in $plugin_dir"
            failed_plugins+=("$plugin_name: main.go not found")
            continue
        fi
        
        if [[ ! -f "$plugin_dir/go.mod" ]]; then
            print_error "go.mod not found in $plugin_dir"
            failed_plugins+=("$plugin_name: go.mod not found")
            continue
        fi
        
        # Build the plugin
        if [[ "$effective_build_mode" == "docker" ]]; then
            if build_plugin_docker "$plugin_dir" "$plugin_name" "$container_id"; then
                built_plugins+=("$plugin_name")
            else
                failed_plugins+=("$plugin_name: Docker build failed")
            fi
        else
            if build_plugin_host "$plugin_dir" "$plugin_name"; then
                built_plugins+=("$plugin_name")
            else
                failed_plugins+=("$plugin_name: Host build failed")
            fi
        fi
    done
    
    # Enable FFmpeg transcoder if we built it and we're using Docker
    if [[ "$effective_build_mode" == "docker" && -n "$container_id" ]]; then
        for plugin in "${built_plugins[@]}"; do
            if [[ "$plugin" == "ffmpeg_transcoder" ]]; then
                enable_ffmpeg_transcoder "$container_id"
                break
            fi
        done
    fi
    
    # Print build summary
    print_status "Build Summary"
    print_success "Successfully built: ${#built_plugins[@]} plugin(s)"
    for plugin in "${built_plugins[@]}"; do
        print_success "  ✅ $plugin"
    done
    
    if [[ ${#failed_plugins[@]} -gt 0 ]]; then
        print_error "Failed builds: ${#failed_plugins[@]}"
        for failure in "${failed_plugins[@]}"; do
            print_error "  ❌ $failure"
        done
        echo ""
        print_error "Some plugins failed to build. Check the errors above."
        exit 1
    fi
    
    # Show next steps
    print_success "All plugins built successfully!"
    echo ""
    print_info "Next steps:"
    if [[ "$effective_build_mode" == "docker" ]]; then
        print_info "1. Plugins are ready for use in the Docker environment"
        print_info "2. The backend will automatically detect and load enabled plugins"
        print_info "3. Check logs with: docker-compose logs backend | grep -i plugin"
    else
        print_info "1. If using Docker, restart the backend: docker-compose restart backend"
        print_info "2. If running natively, restart the backend process"
        print_info "3. Check logs for plugin loading confirmation"
    fi
    
    print_success "Plugin build completed successfully!"
}

# Run main function
main "$@" 