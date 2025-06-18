#!/bin/bash

# Hot Reload Plugin Development Demo Script
# This script demonstrates the hot reload functionality for plugin development
# eliminating the need to restart the backend when testing plugin changes.

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
BACKEND_URL="http://localhost:8080"
FRONTEND_URL="http://localhost:5175"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Helper functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_step() {
    echo -e "${PURPLE}[STEP]${NC} $1"
}

# API helper functions
check_backend() {
    if curl -sf "$BACKEND_URL/api/health" > /dev/null 2>&1; then
        return 0
    else
        return 1
    fi
}

get_hot_reload_status() {
    curl -sf "$BACKEND_URL/api/plugin-manager/hot-reload/status" | jq '.hot_reload' 2>/dev/null || echo "null"
}

enable_hot_reload() {
    curl -sf -X POST "$BACKEND_URL/api/plugin-manager/hot-reload/enable" > /dev/null 2>&1
}

disable_hot_reload() {
    curl -sf -X POST "$BACKEND_URL/api/plugin-manager/hot-reload/disable" > /dev/null 2>&1
}

trigger_reload() {
    local plugin_id="$1"
    curl -sf -X POST "$BACKEND_URL/api/plugin-manager/hot-reload/trigger/$plugin_id" > /dev/null 2>&1
}

list_plugins() {
    curl -sf "$BACKEND_URL/api/plugin-manager/external" | jq '.plugins' 2>/dev/null || echo "[]"
}

build_plugin() {
    local plugin_name="$1"
    log_step "Building plugin: $plugin_name"
    
    cd "$PROJECT_ROOT"
    if ./backend/scripts/build-plugin.sh "$plugin_name"; then
        log_success "Plugin $plugin_name built successfully"
        return 0
    else
        log_error "Failed to build plugin $plugin_name"
        return 1
    fi
}

watch_logs() {
    log_info "Watching backend logs (Ctrl+C to stop)..."
    docker-compose logs -f backend | grep -E "(hot.*reload|plugin.*reload|Hot reload|🔄|✅.*reload|❌.*reload)" --color=always || true
}

# Main demo functions
show_header() {
    clear
    echo -e "${CYAN}"
    echo "╔════════════════════════════════════════════════════════════════════════════════╗"
    echo "║                         🔥 VIEWRA HOT RELOAD DEMO 🔥                          ║"
    echo "║                                                                                ║"
    echo "║  This demo shows how to develop plugins without restarting the backend!      ║"
    echo "║  Changes to plugin binaries are automatically detected and reloaded.         ║"
    echo "╚════════════════════════════════════════════════════════════════════════════════╝"
    echo -e "${NC}"
    echo
}

check_prerequisites() {
    log_step "Checking prerequisites..."
    
    # Check if backend is running
    if ! check_backend; then
        log_error "Backend is not running on $BACKEND_URL"
        log_info "Please start the backend with: docker-compose up -d backend"
        exit 1
    fi
    log_success "Backend is running"
    
    # Check if required tools are available
    for tool in curl jq docker; do
        if ! command -v "$tool" > /dev/null; then
            log_error "Required tool '$tool' is not installed"
            exit 1
        fi
    done
    log_success "All required tools are available"
    
    # Check if build script exists
    if [[ ! -f "$PROJECT_ROOT/backend/scripts/build-plugin.sh" ]]; then
        log_error "Plugin build script not found at $PROJECT_ROOT/backend/scripts/build-plugin.sh"
        exit 1
    fi
    log_success "Plugin build script is available"
}

show_current_status() {
    log_step "Checking current hot reload status..."
    
    local status=$(get_hot_reload_status)
    if [[ "$status" == "null" ]]; then
        log_warning "Could not retrieve hot reload status (API may not be available)"
    else
        log_info "Hot reload status:"
        echo "$status" | jq '.' 2>/dev/null || echo "$status"
    fi
    
    echo
    log_step "Checking available plugins..."
    local plugins=$(list_plugins)
    if [[ "$plugins" == "[]" ]]; then
        log_warning "No external plugins found"
    else
        log_info "Available external plugins:"
        echo "$plugins" | jq -r '.[] | "  - \(.id) (\(.name)) - Running: \(.running)"' 2>/dev/null || echo "  Could not parse plugin list"
    fi
    echo
}

demonstrate_hot_reload() {
    log_step "Demonstrating hot reload functionality..."
    
    # Enable hot reload if not already enabled
    log_info "Ensuring hot reload is enabled..."
    enable_hot_reload
    sleep 1
    
    # Find available plugins to demonstrate with
    local plugins=$(list_plugins)
    local plugin_count=$(echo "$plugins" | jq '. | length' 2>/dev/null || echo "0")
    
    if [[ "$plugin_count" == "0" ]]; then
        log_warning "No external plugins available for demonstration"
        log_info "Let's try building the FFmpeg transcoder plugin..."
        
        if build_plugin "ffmpeg_transcoder"; then
            log_success "Plugin built successfully!"
            sleep 2
            
            # Check again
            plugins=$(list_plugins)
            plugin_count=$(echo "$plugins" | jq '. | length' 2>/dev/null || echo "0")
        fi
    fi
    
    if [[ "$plugin_count" -gt "0" ]]; then
        local first_plugin=$(echo "$plugins" | jq -r '.[0].id' 2>/dev/null)
        if [[ "$first_plugin" != "null" && "$first_plugin" != "" ]]; then
            log_info "Demonstrating hot reload with plugin: $first_plugin"
            
            # Show the reload process
            log_info "Triggering manual reload..."
            if trigger_reload "$first_plugin"; then
                log_success "Hot reload triggered successfully!"
                log_info "Check the backend logs to see the reload process"
            else
                log_warning "Failed to trigger hot reload (this is normal if the plugin binary hasn't changed)"
            fi
        fi
    fi
}

interactive_mode() {
    echo -e "${CYAN}🔥 Interactive Hot Reload Mode${NC}"
    echo "You can now modify plugins and see them reload automatically!"
    echo
    echo "Available commands:"
    echo "  s) Show current status"
    echo "  l) List plugins"
    echo "  b) Build a specific plugin"
    echo "  r) Manually trigger reload for a plugin"
    echo "  w) Watch backend logs for reload events"
    echo "  e) Enable hot reload"
    echo "  d) Disable hot reload"
    echo "  q) Quit"
    echo
    
    while true; do
        echo -n -e "${BLUE}hot-reload> ${NC}"
        read -r choice
        
        case "$choice" in
            s|status)
                show_current_status
                ;;
            l|list)
                log_info "Available plugins:"
                list_plugins | jq -r '.[] | "  - \(.id) (\(.name)) - Running: \(.running)"' 2>/dev/null || echo "  Could not parse plugin list"
                ;;
            b|build)
                echo -n "Enter plugin name to build: "
                read -r plugin_name
                if [[ -n "$plugin_name" ]]; then
                    build_plugin "$plugin_name"
                else
                    log_warning "No plugin name provided"
                fi
                ;;
            r|reload)
                echo -n "Enter plugin ID to reload: "
                read -r plugin_id
                if [[ -n "$plugin_id" ]]; then
                    if trigger_reload "$plugin_id"; then
                        log_success "Hot reload triggered for $plugin_id"
                    else
                        log_error "Failed to trigger reload for $plugin_id"
                    fi
                else
                    log_warning "No plugin ID provided"
                fi
                ;;
            w|watch)
                watch_logs
                ;;
            e|enable)
                enable_hot_reload
                log_success "Hot reload enabled"
                ;;
            d|disable)
                disable_hot_reload
                log_success "Hot reload disabled"
                ;;
            q|quit|exit)
                log_info "Exiting interactive mode..."
                break
                ;;
            "")
                # Empty input, just continue
                ;;
            *)
                log_warning "Unknown command: $choice"
                ;;
        esac
        echo
    done
}

show_tips() {
    echo -e "${CYAN}💡 Hot Reload Development Tips:${NC}"
    echo
    echo "1. 🔧 Plugin Development Workflow:"
    echo "   - Make changes to your plugin source code"
    echo "   - Run: ./backend/scripts/build-plugin.sh <plugin_name>"
    echo "   - The plugin will automatically reload (if hot reload is enabled)"
    echo "   - Test your changes immediately!"
    echo
    echo "2. 🔍 Monitoring Changes:"
    echo "   - Watch logs: docker-compose logs -f backend | grep reload"
    echo "   - Check status via API: curl $BACKEND_URL/api/plugin-manager/hot-reload/status"
    echo "   - View plugin list: curl $BACKEND_URL/api/plugin-manager/external"
    echo
    echo "3. 🚀 Best Practices:"
    echo "   - Hot reload watches for binary changes, not source code changes"
    echo "   - Changes are debounced (500ms delay) to avoid multiple reloads"
    echo "   - Plugin state is preserved across reloads when possible"
    echo "   - Large files and temporary files are excluded from watching"
    echo
    echo "4. 🛠️ Configuration:"
    echo "   - Hot reload is enabled by default in development"
    echo "   - Configure debounce delay, watch patterns, etc. in plugin config"
    echo "   - Can be enabled/disabled at runtime via API"
    echo
    echo "5. 🐛 Troubleshooting:"
    echo "   - If reload fails, check plugin binary exists and is executable"
    echo "   - Watch backend logs for detailed error messages"
    echo "   - Manual reload: curl -X POST $BACKEND_URL/api/plugin-manager/hot-reload/trigger/<plugin_id>"
    echo
}

# Main execution
main() {
    local mode="${1:-demo}"
    
    show_header
    check_prerequisites
    show_current_status
    
    case "$mode" in
        "demo"|"d")
            demonstrate_hot_reload
            echo
            show_tips
            ;;
        "interactive"|"i")
            interactive_mode
            ;;
        "watch"|"w")
            watch_logs
            ;;
        "status"|"s")
            show_current_status
            ;;
        "tips"|"t")
            show_tips
            ;;
        *)
            log_error "Unknown mode: $mode"
            echo "Usage: $0 [demo|interactive|watch|status|tips]"
            echo "  demo        - Run the hot reload demonstration (default)"
            echo "  interactive - Enter interactive mode for manual testing"
            echo "  watch       - Watch backend logs for reload events"
            echo "  status      - Show current hot reload status"
            echo "  tips        - Show development tips and best practices"
            exit 1
            ;;
    esac
}

# Handle script interruption gracefully
trap 'echo -e "\n${YELLOW}Demo interrupted by user${NC}"; exit 0' INT

# Run the script
main "$@" 