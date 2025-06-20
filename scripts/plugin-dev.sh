#!/bin/bash
# Comprehensive plugin development tool for Viewra
# Handles building, enabling, hot reloading, and testing plugins seamlessly

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default values
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PLUGIN_DIR="${PROJECT_ROOT}/backend/data/plugins"
BACKEND_CONTAINER="viewra-backend-1"

# Function to print colored output
print_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
print_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
print_error() { echo -e "${RED}[ERROR]${NC} $1"; }
print_warning() { echo -e "${YELLOW}[WARNING]${NC} $1"; }

# Function to check if Docker is running
check_docker() {
    if ! docker info >/dev/null 2>&1; then
        print_error "Docker is not running. Please start Docker first."
        exit 1
    fi
}

# Function to check if backend container is running
check_backend() {
    if ! docker ps --format '{{.Names}}' | grep -q "^${BACKEND_CONTAINER}$"; then
        print_error "Backend container is not running. Starting it..."
        docker-compose up -d backend
        sleep 5
    fi
}

# Function to list available plugins
list_plugins() {
    print_info "Available plugins:"
    for plugin_path in "$PLUGIN_DIR"/*; do
        if [ -d "$plugin_path" ] && [ -f "$plugin_path/plugin.cue" ]; then
            plugin_name=$(basename "$plugin_path")
            if [ -f "$plugin_path/$plugin_name" ]; then
                echo "  - $plugin_name (built)"
            else
                echo "  - $plugin_name (not built)"
            fi
        fi
    done
}

# Function to validate CUE file
validate_cue() {
    local plugin_name=$1
    local cue_file="$PLUGIN_DIR/$plugin_name/plugin.cue"
    
    print_info "Validating CUE file for $plugin_name..."
    
    # Extract and validate required fields
    local errors=0
    
    # Check for required fields (now inside #Plugin block)
    for field in "id" "name" "version" "type" "description"; do
        if ! grep -q "^[[:space:]]*${field}:" "$cue_file"; then
            print_error "Missing required field: $field"
            ((errors++))
        fi
    done
    
    # Check that type is a simple string, not a constraint
    if grep -q '^[[:space:]]*type:.*|' "$cue_file"; then
        print_warning "Type field contains constraints. This may cause parsing issues."
        print_info "Fixing type field to be a simple string..."
        
        # Extract the actual type from constraint (handle indentation)
        local type_line=$(grep "^[[:space:]]*type:" "$cue_file")
        if [[ $type_line =~ \"([a-z_]+)\" ]]; then
            local actual_type="${BASH_REMATCH[1]}"
            # Replace the line with simple type (preserve indentation)
            sed -i "s/^\([[:space:]]*\)type:.*$/\1type: \"$actual_type\"/" "$cue_file"
            print_success "Fixed type field to: \"$actual_type\""
        fi
    fi
    
    if [ $errors -eq 0 ]; then
        print_success "CUE file validation passed"
        return 0
    else
        print_error "CUE file validation failed with $errors errors"
        return 1
    fi
}

# Function to build a plugin
build_plugin() {
    local plugin_name=$1
    local plugin_path="$PLUGIN_DIR/$plugin_name"
    
    if [ ! -d "$plugin_path" ]; then
        print_error "Plugin directory not found: $plugin_path"
        return 1
    fi
    
    print_info "Building plugin: $plugin_name"
    
    # Validate CUE file first
    validate_cue "$plugin_name" || return 1
    
    # Build inside Docker container for Alpine Linux compatibility
    print_info "Building inside Docker container..."
    docker-compose exec -T backend sh -c "cd /app/data/plugins/$plugin_name && go build -o $plugin_name . && chmod +x $plugin_name" || {
        print_error "Build failed!"
        return 1
    }
    
    print_success "Plugin built successfully: $plugin_name"
}

# Function to enable a plugin
enable_plugin() {
    local plugin_name=$1
    
    print_info "Enabling plugin: $plugin_name"
    
    # Enable via API
    response=$(curl -s -X POST "http://localhost:8080/api/plugin-manager/external/$plugin_name/enable")
    
    if echo "$response" | grep -q "successfully"; then
        print_success "Plugin enabled successfully"
    else
        print_error "Failed to enable plugin: $response"
        return 1
    fi
}

# Function to trigger hot reload
hot_reload() {
    local plugin_name=$1
    
    print_info "Triggering hot reload for: $plugin_name"
    
    # Touch the binary to trigger hot reload
    docker-compose exec -T backend sh -c "touch /app/data/plugins/$plugin_name/$plugin_name"
    
    sleep 2
    print_success "Hot reload triggered"
}

# Function to check plugin status
check_status() {
    local plugin_name=$1
    
    print_info "Checking status for: $plugin_name"
    
    # Check if process is running
    if docker-compose exec -T backend ps aux | grep -q "$plugin_name"; then
        print_success "Plugin process is running"
    else
        print_error "Plugin process is NOT running"
    fi
    
    # Check plugin list
    local plugin_info=$(curl -s "http://localhost:8080/api/v1/plugins/" | jq -r ".data[] | select(.id == \"$plugin_name\")")
    
    if [ -n "$plugin_info" ]; then
        echo "Plugin info:"
        echo "$plugin_info" | jq '.'
    else
        print_warning "Plugin not found in API listing"
    fi
    
    # Check health
    local health=$(docker-compose logs backend --tail=50 | grep "$plugin_name.*health" | tail -1)
    if [ -n "$health" ]; then
        echo "Latest health check: $health"
    fi
}

# Function to refresh transcoding plugins
refresh_transcoding() {
    print_info "Refreshing transcoding plugins..."
    
    response=$(curl -s -X POST http://localhost:8080/api/playback/plugins/refresh)
    
    if echo "$response" | grep -q "successfully"; then
        print_success "Transcoding plugins refreshed"
    else
        print_error "Failed to refresh transcoding plugins: $response"
    fi
}

# Function to test transcoding
test_transcode() {
    print_info "Testing transcoding..."
    
    # Create test video if it doesn't exist
    if ! docker-compose exec -T backend test -f /tmp/test_video.mp4; then
        print_info "Creating test video..."
        docker-compose exec -T backend sh -c "ffmpeg -f lavfi -i testsrc=duration=10:size=1280x720:rate=30 -f lavfi -i sine=frequency=1000:duration=10 -c:v libx264 -c:a aac /tmp/test_video.mp4 -y" >/dev/null 2>&1
    fi
    
    # Test transcoding
    response=$(curl -s -X POST -H "Content-Type: application/json" \
        -d '{"media_path": "/tmp/test_video.mp4", "client_info": {"user_agent": "Mozilla/5.0 Chrome/125.0", "client_ip": "127.0.0.1"}, "force_transcode": true, "transcode_params": {"container": "dash", "video_codec": "h264", "audio_codec": "aac", "quality": 70}}' \
        http://localhost:8080/api/playback/start)
    
    if echo "$response" | grep -q "error"; then
        print_error "Transcoding test failed:"
        echo "$response" | jq '.'
    else
        print_success "Transcoding test passed!"
        echo "$response" | jq '.'
    fi
}

# Function to show plugin logs
show_logs() {
    local plugin_name=$1
    local lines=${2:-50}
    
    print_info "Showing last $lines log lines for: $plugin_name"
    docker-compose logs backend --tail=$lines | grep -i "$plugin_name"
}

# Function to perform complete plugin workflow
workflow() {
    local plugin_name=$1
    
    print_info "Running complete workflow for: $plugin_name"
    
    # 1. Build
    build_plugin "$plugin_name" || return 1
    
    # 2. Hot reload or enable
    if docker-compose exec -T backend ps aux | grep -q "$plugin_name"; then
        hot_reload "$plugin_name"
    else
        enable_plugin "$plugin_name"
        sleep 2
    fi
    
    # 3. Check status
    check_status "$plugin_name"
    
    # 4. If it's a transcoder, refresh and test
    local plugin_type=$(grep "^[[:space:]]*type:" "$PLUGIN_DIR/$plugin_name/plugin.cue" | sed 's/.*"\([^"]*\)".*/\1/')
    if [ "$plugin_type" = "transcoder" ]; then
        refresh_transcoding
        test_transcode
    fi
}

# Main script logic
case "${1:-help}" in
    list)
        list_plugins
        ;;
    build)
        if [ -z "$2" ]; then
            print_error "Usage: $0 build <plugin_name>"
            exit 1
        fi
        check_docker
        check_backend
        build_plugin "$2"
        ;;
    enable)
        if [ -z "$2" ]; then
            print_error "Usage: $0 enable <plugin_name>"
            exit 1
        fi
        check_docker
        check_backend
        enable_plugin "$2"
        ;;
    reload)
        if [ -z "$2" ]; then
            print_error "Usage: $0 reload <plugin_name>"
            exit 1
        fi
        check_docker
        check_backend
        hot_reload "$2"
        ;;
    status)
        if [ -z "$2" ]; then
            print_error "Usage: $0 status <plugin_name>"
            exit 1
        fi
        check_docker
        check_backend
        check_status "$2"
        ;;
    logs)
        if [ -z "$2" ]; then
            print_error "Usage: $0 logs <plugin_name> [lines]"
            exit 1
        fi
        check_docker
        show_logs "$2" "${3:-50}"
        ;;
    workflow)
        if [ -z "$2" ]; then
            print_error "Usage: $0 workflow <plugin_name>"
            exit 1
        fi
        check_docker
        check_backend
        workflow "$2"
        ;;
    test-transcode)
        check_docker
        check_backend
        test_transcode
        ;;
    refresh-transcode)
        check_docker
        check_backend
        refresh_transcoding
        ;;
    help|*)
        echo "Viewra Plugin Development Tool"
        echo ""
        echo "Usage: $0 <command> [args]"
        echo ""
        echo "Commands:"
        echo "  list                    List all available plugins"
        echo "  build <plugin>          Build a plugin (Docker-compatible)"
        echo "  enable <plugin>         Enable a plugin"
        echo "  reload <plugin>         Hot reload a plugin"
        echo "  status <plugin>         Check plugin status"
        echo "  logs <plugin> [lines]   Show plugin logs"
        echo "  workflow <plugin>       Run complete workflow (build, enable/reload, test)"
        echo "  test-transcode          Test transcoding functionality"
        echo "  refresh-transcode       Refresh transcoding plugin discovery"
        echo ""
        echo "Examples:"
        echo "  $0 list"
        echo "  $0 build ffmpeg_transcoder"
        echo "  $0 workflow ffmpeg_transcoder"
        echo "  $0 status ffmpeg_transcoder"
        echo "  $0 logs ffmpeg_transcoder 100"
        ;;
esac 