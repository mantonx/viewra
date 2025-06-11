#!/bin/bash

set -euo pipefail

# Bulletproof Plugin Build Script for Viewra
# Usage: ./build-plugin.sh <plugin_name> [build_mode] [target_arch]
# Build modes: auto, host, container

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
PLUGINS_DIR="$PROJECT_ROOT/backend/data/plugins"

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

# Function to detect if plugin needs CGO
detect_cgo_requirement() {
    local plugin_dir="$1"
    
    print_info "Analyzing CGO requirements for plugin..." >&2
    
    # Check for CGO-requiring imports in Go files
    local cgo_imports=(
        "github.com/mattn/go-sqlite3"
        "database/sql"
        "_cgo"
        "#cgo"
        "gorm.io/driver/sqlite"
    )
    
    local needs_cgo=false
    
    for import in "${cgo_imports[@]}"; do
        if grep -r "$import" "$plugin_dir"/*.go >/dev/null 2>&1; then
            print_info "Found CGO dependency in Go files: $import" >&2
            needs_cgo=true
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
                print_info "Found CGO dependency in go.mod: $module" >&2
                needs_cgo=true
            fi
        done
        
        # Also check go.sum for transitive dependencies
        if [[ -f "$plugin_dir/go.sum" ]]; then
            if grep "github.com/mattn/go-sqlite3" "$plugin_dir/go.sum" >/dev/null 2>&1; then
                print_info "Found transitive CGO dependency in go.sum: github.com/mattn/go-sqlite3" >&2
                needs_cgo=true
            fi
        fi
    fi
    
    if $needs_cgo; then
        print_warning "Plugin requires CGO - will use container build for compatibility" >&2
        echo "cgo"
    else
        print_info "Plugin does not require CGO - can use host build" >&2
        echo "no-cgo"
    fi
}

# Function to detect target architecture
detect_target_arch() {
    local container_arch=""
    local host_arch=""
    
    # Detect host architecture
    case "$(uname -m)" in
        x86_64) host_arch="amd64" ;;
        aarch64|arm64) host_arch="arm64" ;;
        *) host_arch="amd64" ;; # Default fallback
    esac
    
    # Check if we're building for Docker container
    if command -v docker >/dev/null 2>&1; then
        # Check if backend container is running
        if docker ps --filter "expose=8080" --format "{{.ID}}" | head -1 >/dev/null 2>&1; then
            container_id=$(docker ps --filter "expose=8080" --format "{{.ID}}" | head -1)
            container_arch=$(docker exec "$container_id" uname -m 2>/dev/null || echo "x86_64")
            case "$container_arch" in
                x86_64) container_arch="amd64" ;;
                aarch64|arm64) container_arch="arm64" ;;
                *) container_arch="amd64" ;;
            esac
        else
            print_warning "Backend container not running, defaulting to amd64"
            container_arch="amd64"
        fi
    else
        container_arch="$host_arch"
    fi
    
    echo "$container_arch"
}

# Function to get container ID
get_container_id() {
    local container_id
    container_id=$(docker ps --filter "expose=8080" --format "{{.ID}}" | head -1)
    
    if [[ -z "$container_id" ]]; then
        print_error "Backend container not running. Please start the backend with: docker-compose up -d"
        return 1
    fi
    
    echo "$container_id"
}

# Function to validate plugin directory
validate_plugin() {
    local plugin_name="$1"
    local plugin_dir="$PLUGINS_DIR/$plugin_name"
    
    if [[ ! -d "$plugin_dir" ]]; then
        print_error "Plugin directory not found: $plugin_dir"
        return 1
    fi
    
    if [[ ! -f "$plugin_dir/main.go" ]]; then
        print_error "main.go not found in plugin directory: $plugin_dir"
        return 1
    fi
    
    if [[ ! -f "$plugin_dir/go.mod" ]]; then
        print_error "go.mod not found in plugin directory: $plugin_dir"
        return 1
    fi
    
    return 0
}

# Function to clean old binaries
clean_old_binaries() {
    local plugin_dir="$1"
    local plugin_name="$2"
    
    print_status "Cleaning old binaries in $plugin_dir"
    
    # Remove any existing binaries that might cause confusion
    find "$plugin_dir" -name "${plugin_name}*" -type f -executable -delete 2>/dev/null || true
    find "$plugin_dir" -name "*.exe" -type f -delete 2>/dev/null || true
    
    # Clean Go build cache for this module
    (cd "$plugin_dir" && go clean -cache 2>/dev/null || true)
}

# Function to build plugin on host
build_plugin_host() {
    local plugin_name="$1"
    local target_arch="$2"
    local cgo_enabled="$3"
    local plugin_dir="$PLUGINS_DIR/$plugin_name"
    
    print_status "Building plugin on host: $plugin_name for architecture: $target_arch (CGO: $cgo_enabled)"
    
    # Set build environment
    export GOOS=linux
    export GOARCH="$target_arch"
    export CGO_ENABLED="$cgo_enabled"
    
    # Build with specific flags for consistency
    local build_flags=(
        -ldflags="-s -w"  # Strip debug info and symbol table
        -trimpath         # Remove build paths from binary
        -o "$plugin_name"
        main.go
    )
    
    print_status "Build command: CGO_ENABLED=$CGO_ENABLED GOOS=$GOOS GOARCH=$GOARCH go build ${build_flags[*]}"
    
    # Change to plugin directory and build
    if (cd "$plugin_dir" && go build "${build_flags[@]}"); then
        print_success "Plugin binary built successfully on host"
        return 0
    else
        print_error "Host build failed"
        return 1
    fi
}

# Function to build plugin in container
build_plugin_container() {
    local plugin_name="$1"
    local target_arch="$2"
    local cgo_enabled="$3"
    local container_id="$4"
    local plugin_dir="$PLUGINS_DIR/$plugin_name"
    
    print_status "Building plugin in container: $plugin_name for architecture: $target_arch (CGO: $cgo_enabled)"
    
    # Build command to run in container
    local build_cmd="cd /app/data/plugins/$plugin_name && CGO_ENABLED=$cgo_enabled GOOS=linux GOARCH=$target_arch GOCACHE=/tmp go build -ldflags='-s -w' -trimpath -o $plugin_name main.go"
    
    print_status "Container build command: $build_cmd"
    
    # Execute build in container as root to avoid permission issues
    if docker exec --user root "$container_id" sh -c "$build_cmd"; then
        print_success "Plugin binary built successfully in container"
        
        # Fix ownership of the binary
        local host_uid=$(id -u)
        local host_gid=$(id -g)
        docker exec --user root "$container_id" chown "$host_uid:$host_gid" "/app/data/plugins/$plugin_name/$plugin_name" 2>/dev/null || true
        
        return 0
    else
        print_error "Container build failed"
        return 1
    fi
}

# Function to verify binary compatibility
verify_binary() {
    local plugin_name="$1"
    local target_arch="$2"
    local build_strategy="$3"
    local container_id="$4"
    local plugin_dir="$PLUGINS_DIR/$plugin_name"
    local binary_path="$plugin_dir/$plugin_name"
    
    if [[ "$build_strategy" == "container" && -n "$container_id" ]]; then
        # For container builds, verify the binary inside the container
        print_status "Verifying binary in container..."
        
        if ! docker exec "$container_id" test -f "/app/data/plugins/$plugin_name/$plugin_name"; then
            print_error "Binary was not created in container: /app/data/plugins/$plugin_name/$plugin_name"
            return 1
        fi
        
        # Make executable in container
        docker exec "$container_id" chmod +x "/app/data/plugins/$plugin_name/$plugin_name"
        
        # Check if binary is executable and has reasonable size
        local file_size
        file_size=$(docker exec "$container_id" sh -c "du -h /app/data/plugins/$plugin_name/$plugin_name | cut -f1")
        print_info "Binary size: $file_size"
        
        # Try to read the ELF header to verify it's a valid binary
        local elf_header
        elf_header=$(docker exec "$container_id" sh -c "head -c 16 /app/data/plugins/$plugin_name/$plugin_name | hexdump -C | head -1" 2>/dev/null || echo "")
        
        if echo "$elf_header" | grep -q "7f 45 4c 46"; then
            print_success "Binary verified as ELF file in container"
        else
            print_warning "Could not verify ELF header, but binary exists and is executable"
        fi
        
        # Test if the binary can be executed (basic smoke test)
        if docker exec "$container_id" test -x "/app/data/plugins/$plugin_name/$plugin_name"; then
            print_success "Binary is executable in container"
        else
            print_error "Binary is not executable in container"
            return 1
        fi
        
        print_success "Binary verified in container"
        return 0
        
    else
        # For host builds, verify the binary on host
        if [[ ! -f "$binary_path" ]]; then
            print_error "Binary was not created: $binary_path"
            return 1
        fi
        
        # Make executable
        chmod +x "$binary_path"
        
        # Get binary info
        local binary_info
        binary_info=$(file "$binary_path" 2>/dev/null || echo "unknown")
        print_status "Binary info: $binary_info"
        
        # Verify it's a Linux binary
        if ! echo "$binary_info" | grep -q "ELF.*LSB\|ELF.*MSB\|ELF.*Linux"; then
            print_error "Binary is not a Linux ELF file"
            return 1
        fi
        
        # Get file size
        local file_size
        file_size=$(du -h "$binary_path" | cut -f1)
        print_info "Binary size: $file_size"
        
        print_success "Binary verified on host"
        return 0
    fi
    
    # Verify architecture (if possible)
    case "$target_arch" in
        amd64)
            if echo "$binary_info" | grep -q "x86-64\|x86_64"; then
                print_success "Binary architecture verified: $target_arch"
            else
                print_warning "Binary architecture verification inconclusive for $target_arch"
            fi
            ;;
        arm64)
            if echo "$binary_info" | grep -q "aarch64\|ARM"; then
                print_success "Binary architecture verified: $target_arch"
            else
                print_warning "Binary architecture verification inconclusive for $target_arch"
            fi
            ;;
    esac
}

# Function to test binary in container
test_binary_in_container() {
    local plugin_name="$1"
    local container_id="$2"
    
    print_status "Testing binary compatibility in container..."
    
    # Test if binary can be executed in container
    if docker exec "$container_id" test -x "/app/data/plugins/$plugin_name/$plugin_name"; then
        print_success "Binary is executable in container"
        
        # Try to get version or help output (non-blocking test)
        local test_output
        test_output=$(docker exec "$container_id" timeout 2s "/app/data/plugins/$plugin_name/$plugin_name" --help 2>&1 || echo "no_help")
        
        if [[ "$test_output" != "no_help" ]]; then
            print_success "Binary responds to --help in container"
        else
            print_info "Binary doesn't support --help (normal for plugins)"
        fi
        
        return 0
    else
        print_error "Binary is not executable in container"
        return 1
    fi
}

# Function to deploy to container
deploy_to_container() {
    local plugin_name="$1"
    local plugin_dir="$PLUGINS_DIR/$plugin_name"
    local container_id="$2"
    
    print_status "Deploying to container: $container_id"
    
    if docker cp "$plugin_dir/$plugin_name" "$container_id:/app/data/plugins/$plugin_name/"; then
        print_success "Plugin deployed to container"
        
        # Test binary in container
        test_binary_in_container "$plugin_name" "$container_id"
        
        return 0
    else
        print_error "Failed to deploy plugin to container"
        return 1
    fi
}

# Function to determine optimal build strategy (DOCKER ONLY)
determine_build_strategy() {
    local plugin_dir="$1"
    local cgo_requirement="$2"
    local build_mode="$3"
    
    case "$build_mode" in
        host)
            print_error "Host builds are no longer supported for consistency and reliability"
            print_error "All plugin builds now use Docker containers"
            return 1
            ;;
        container)
            echo "container"
            ;;
        auto)
            # Always use container builds for consistency
            print_info "Auto mode: using container build for maximum consistency"
            echo "container"
            ;;
        *)
            print_error "Invalid build mode: $build_mode (valid: container, auto)"
            return 1
            ;;
    esac
}

# Main function
main() {
    if [[ $# -lt 1 ]]; then
        echo "Usage: $0 <plugin_name> [build_mode] [target_arch]"
        echo ""
        echo "Build modes:"
        echo "  auto      - Use Docker container build (default, recommended)"
        echo "  container - Build inside Docker container (maximum compatibility)"
        echo "  host      - DEPRECATED: No longer supported"
        echo ""
        echo "Target architectures:"
        echo "  auto      - Auto-detect target architecture (default)"
        echo "  amd64     - Build for x86_64"
        echo "  arm64     - Build for ARM64"
        echo ""
        echo "Available plugins:"
        find "$PLUGINS_DIR" -maxdepth 1 -type d -name "*_*" -printf "  %f\n" 2>/dev/null | sort
        exit 1
    fi
    
    local plugin_name="$1"
    local build_mode="${2:-auto}"
    local target_arch="${3:-$(detect_target_arch)}"
    
    print_status "🚀 Starting bulletproof build process for plugin: $plugin_name"
    print_status "Build mode: $build_mode"
    print_status "Target architecture: $target_arch"
    print_status "Project root: $PROJECT_ROOT"
    print_status "Plugins directory: $PLUGINS_DIR"
    
    # Validation
    if ! validate_plugin "$plugin_name"; then
        exit 1
    fi
    
    local plugin_dir="$PLUGINS_DIR/$plugin_name"
    
    # Detect CGO requirements
    local cgo_requirement
    cgo_requirement=$(detect_cgo_requirement "$plugin_dir")
    
    # Determine build strategy
    local build_strategy
    build_strategy=$(determine_build_strategy "$plugin_dir" "$cgo_requirement" "$build_mode")
    
    # Log the strategy selection
    case "$build_strategy" in
        container)
            if [[ "$cgo_requirement" == "cgo" ]]; then
                print_info "CGO required - using container build for maximum compatibility"
            else
                print_info "Container build requested"
            fi
            ;;
        host)
            if [[ "$cgo_requirement" == "cgo" ]]; then
                print_warning "CGO required but using host build - may have compatibility issues"
            else
                print_info "No CGO required - using host build for speed"
            fi
            ;;
    esac
    
    print_info "Selected build strategy: $build_strategy"
    
    # Get container ID if needed
    local container_id=""
    if [[ "$build_strategy" == "container" ]] || [[ "$build_mode" != "host" ]]; then
        if ! container_id=$(get_container_id); then
            if [[ "$build_strategy" == "container" ]]; then
                print_error "Container build required but container not available"
                exit 1
            else
                print_warning "Container not available, falling back to host build"
                build_strategy="host"
            fi
        fi
    fi
    
    # Clean old binaries
    clean_old_binaries "$plugin_dir" "$plugin_name"
    
    # Determine CGO setting
    local cgo_enabled
    if [[ "$cgo_requirement" == "cgo" ]]; then
        cgo_enabled="1"
    else
        cgo_enabled="0"
    fi
    
    # Build plugin based on strategy
    case "$build_strategy" in
        host)
            if ! build_plugin_host "$plugin_name" "$target_arch" "$cgo_enabled"; then
                exit 1
            fi
            ;;
        container)
            if ! build_plugin_container "$plugin_name" "$target_arch" "$cgo_enabled" "$container_id"; then
                exit 1
            fi
            ;;
    esac
    
    # Verify binary
    if ! verify_binary "$plugin_name" "$target_arch" "$build_strategy" "$container_id"; then
        exit 1
    fi
    
    # Deploy to container if we have one and didn't build there
    if [[ -n "$container_id" ]] && [[ "$build_strategy" != "container" ]]; then
        deploy_to_container "$plugin_name" "$container_id"
    elif [[ "$build_strategy" == "container" ]]; then
        # Test the binary that was built in container
        test_binary_in_container "$plugin_name" "$container_id"
    fi
    
    print_success "🎉 Plugin build process completed successfully!"
    print_success "Binary location: $plugin_dir/$plugin_name"
    print_info "Build summary:"
    print_info "  Plugin: $plugin_name"
    print_info "  Strategy: $build_strategy"
    print_info "  CGO: $cgo_requirement"
    print_info "  Architecture: $target_arch"
    print_info "  Container deployed: $([ -n "$container_id" ] && echo "yes" || echo "no")"
}

# Run main function
main "$@" 