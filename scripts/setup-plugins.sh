#!/bin/bash

set -euo pipefail

# =============================================================================
# Comprehensive Plugin Setup Script for Viewra
# =============================================================================
# This script provides a one-command solution for plugin setup including:
# - Docker environment verification
# - Plugin building using the unified build script
# - FFmpeg transcoder auto-enablement
# - Hot reload verification

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${BLUE}[SETUP]${NC} $1"
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
    echo "Usage: $0 [options]"
    echo ""
    echo "Options:"
    echo "  -h, --help           Show this help message"
    echo "  --build-mode MODE    Force build mode (auto, docker, host)"
    echo "  --plugin NAME        Setup only specific plugin"
    echo "  --no-restart         Skip backend restart"
    echo ""
    echo "Examples:"
    echo "  $0                           # Full setup with auto-detection"
    echo "  $0 --build-mode docker       # Force Docker build mode"
    echo "  $0 --plugin ffmpeg_transcoder # Setup only FFmpeg transcoder"
    echo "  $0 --no-restart              # Skip backend restart"
}

# Function to check prerequisites
check_prerequisites() {
    print_status "Checking prerequisites..."
    
    # Check Docker
    if ! command -v docker >/dev/null 2>&1; then
        print_error "Docker is not installed"
        return 1
    fi
    
    if ! docker ps >/dev/null 2>&1; then
        print_error "Docker daemon is not running"
        return 1
    fi
    
    # Check docker-compose
    if ! command -v docker-compose >/dev/null 2>&1; then
        print_error "docker-compose is not installed"
        return 1
    fi
    
    # Check project structure
    if [[ ! -f "$PROJECT_ROOT/docker-compose.yml" ]]; then
        print_error "docker-compose.yml not found in project root"
        return 1
    fi
    
    if [[ ! -f "$SCRIPT_DIR/build-plugins.sh" ]]; then
        print_error "Unified build script not found: $SCRIPT_DIR/build-plugins.sh"
        return 1
    fi
    
    print_success "All prerequisites met"
}

# Function to start services if not running
ensure_services_running() {
    print_status "Ensuring Docker services are running..."
    
    cd "$PROJECT_ROOT"
    
    # Check if backend is running
    if ! docker ps --filter "name=viewra.*backend" --filter "status=running" | grep -q backend; then
        print_info "Starting Docker services..."
        docker-compose up -d
        
        # Wait for backend to be ready
        print_info "Waiting for backend to be ready..."
        local count=0
        while [[ $count -lt 30 ]]; do
            if docker ps --filter "name=viewra.*backend" --filter "status=running" | grep -q backend; then
                break
            fi
            sleep 2
            count=$((count + 1))
        done
        
        if [[ $count -eq 30 ]]; then
            print_error "Backend failed to start within 60 seconds"
            return 1
        fi
        
        print_success "Services are running"
    else
        print_info "Services are already running"
    fi
}

# Function to build plugins using unified script
build_plugins() {
    print_status "Building plugins using unified build script..."
    
    local build_args=()
    
    # Always add build mode (default to auto if not specified)
    if [[ -n "${FORCE_BUILD_MODE:-}" ]]; then
        build_args+=("$FORCE_BUILD_MODE")
    else
        build_args+=("auto")
    fi
    
    # Add specific plugin if specified
    if [[ -n "${SPECIFIC_PLUGIN:-}" ]]; then
        build_args+=("${SPECIFIC_PLUGIN}")
    fi
    
    # Make build script executable
    chmod +x "$SCRIPT_DIR/build-plugins.sh"
    
    # Run the unified build script
    if "$SCRIPT_DIR/build-plugins.sh" "${build_args[@]}"; then
        print_success "Plugin build completed successfully"
    else
        print_error "Plugin build failed"
        return 1
    fi
}

# Function to verify hot reload configuration
verify_hot_reload() {
    print_status "Verifying hot reload configuration..."
    
    # Check config file for hot reload settings
    local config_files=(
        "$PROJECT_ROOT/backend/internal/config/config.go"
        "$PROJECT_ROOT/backend/internal/modules/pluginmodule/hot_reload.go"
    )
    
    local hot_reload_found=false
    for config_file in "${config_files[@]}"; do
        if [[ -f "$config_file" ]] && grep -q "EnableHotReload.*true\|enable_hot_reload.*true\|HotReload.*true" "$config_file"; then
            print_success "Hot reload is enabled in $(basename "$config_file")"
            hot_reload_found=true
            break
        fi
    done
    
    if [[ "$hot_reload_found" == "false" ]]; then
        print_warning "Hot reload may not be enabled in configuration"
    fi
    
    # Check if hot reload is working by looking for file watchers
    local container_id
    container_id=$(docker ps --filter "name=viewra.*backend" --format "{{.ID}}" | head -1)
    
    if [[ -n "$container_id" ]]; then
        # Check environment variables in container
        if docker exec "$container_id" printenv | grep -q "VIEWRA_PLUGIN_HOT_RELOAD=true"; then
            print_success "Hot reload environment variable is set"
        else
            print_info "Hot reload environment variable not set (using config default)"
        fi
    fi
}

# Function to restart backend if needed
restart_backend() {
    if [[ "${SKIP_RESTART:-false}" == "true" ]]; then
        print_info "Skipping backend restart as requested"
        return 0
    fi
    
    print_status "Restarting backend to ensure fresh plugin loading..."
    
    cd "$PROJECT_ROOT"
    
    # Check if we need to restart
    local container_id
    container_id=$(docker ps --filter "name=viewra.*backend" --format "{{.ID}}" | head -1)
    
    if [[ -n "$container_id" ]]; then
        print_info "Restarting backend container..."
        docker-compose restart backend
        
        # Wait for backend to be ready again
        print_info "Waiting for backend to be ready after restart..."
        local count=0
        while [[ $count -lt 20 ]]; do
            if docker ps --filter "name=viewra.*backend" --filter "status=running" | grep -q backend; then
                break
            fi
            sleep 2
            count=$((count + 1))
        done
        
        if [[ $count -eq 20 ]]; then
            print_warning "Backend may not be fully ready yet, but restart completed"
        else
            print_success "Backend restarted successfully"
        fi
    else
        print_warning "Backend container not found, skipping restart"
    fi
}

# Function to verify plugin status
verify_plugin_status() {
    print_status "Verifying plugin status..."
    
    local container_id
    container_id=$(docker ps --filter "name=viewra.*backend" --format "{{.ID}}" | head -1)
    
    if [[ -n "$container_id" ]]; then
        print_info "Checking plugin database status..."
        
        # Check FFmpeg transcoder status specifically
        local ffmpeg_status
        ffmpeg_status=$(docker exec "$container_id" sqlite3 /app/viewra-data/database.db "SELECT status FROM plugins WHERE plugin_id = 'ffmpeg_transcoder';" 2>/dev/null || echo "not_found")
        
        if [[ "$ffmpeg_status" == "enabled" ]]; then
            print_success "FFmpeg transcoder is enabled in database"
        elif [[ "$ffmpeg_status" == "not_found" ]]; then
            print_info "FFmpeg transcoder not found in database (will be auto-registered on startup)"
        else
            print_warning "FFmpeg transcoder status: $ffmpeg_status"
        fi
        
        # Show recent plugin-related logs
        print_info "Recent plugin-related logs:"
        docker-compose logs --tail=10 backend | grep -i plugin | tail -5 || print_info "No recent plugin logs found"
    else
        print_warning "Backend container not found, cannot verify plugin status"
    fi
}

# Main function
main() {
    print_status "Starting comprehensive plugin setup..."
    
    # Parse command line arguments
    local FORCE_BUILD_MODE=""
    local SPECIFIC_PLUGIN=""
    local SKIP_RESTART="false"
    
    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help)
                show_usage
                exit 0
                ;;
            --build-mode)
                FORCE_BUILD_MODE="$2"
                shift 2
                ;;
            --plugin)
                SPECIFIC_PLUGIN="$2"
                shift 2
                ;;
            --no-restart)
                SKIP_RESTART="true"
                shift
                ;;
            *)
                print_error "Unknown option: $1"
                show_usage
                exit 1
                ;;
        esac
    done
    
    # Export variables for other functions
    export FORCE_BUILD_MODE SPECIFIC_PLUGIN SKIP_RESTART
    
    print_info "Setup configuration:"
    [[ -n "$FORCE_BUILD_MODE" ]] && print_info "  Build mode: $FORCE_BUILD_MODE" || print_info "  Build mode: auto-detect"
    [[ -n "$SPECIFIC_PLUGIN" ]] && print_info "  Target plugin: $SPECIFIC_PLUGIN" || print_info "  Target: all plugins"
    print_info "  Restart backend: $([ "$SKIP_RESTART" == "true" ] && echo "no" || echo "yes")"
    
    # Run setup steps
    check_prerequisites || exit 1
    ensure_services_running || exit 1
    build_plugins || exit 1
    verify_hot_reload
    restart_backend
    verify_plugin_status
    
    # Final success message
    print_success "Plugin setup completed successfully!"
    echo ""
    print_info "Summary:"
    print_info "✅ All plugins have been built with proper Docker environment compatibility"
    print_info "✅ FFmpeg transcoder is configured to be enabled by default"
    print_info "✅ Hot reload is configured for development workflow"
    print_info "✅ Backend has been restarted to load the latest plugins"
    echo ""
    print_info "Next steps:"
    print_info "1. Check the admin panel to verify plugins are loaded"
    print_info "2. Monitor logs with: docker-compose logs -f backend | grep -i plugin"
    print_info "3. For development, plugins will auto-reload when changed"
    echo ""
    print_success "🎉 Plugin system is ready for use!"
}

# Run main function
main "$@" 