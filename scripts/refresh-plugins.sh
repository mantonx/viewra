#!/bin/bash

# =============================================================================
# Plugin Refresh Script for Viewra
# =============================================================================
# Manually refresh plugins after build or when encountering issues

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# Print functions
print_status() { echo -e "${BLUE}[REFRESH]${NC} $1"; }
print_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
print_warning() { echo -e "${YELLOW}[WARNING]${NC} $1"; }
print_error() { echo -e "${RED}[ERROR]${NC} $1"; }
print_info() { echo -e "${CYAN}[INFO]${NC} $1"; }

show_usage() {
    echo "Usage: $0 [command]"
    echo ""
    echo "Commands:"
    echo "  all                Refresh all plugin systems (default)"
    echo "  external           Refresh external plugins only"
    echo "  playback           Refresh playback/transcoding plugins only"
    echo "  reload <plugin>    Reload a specific plugin"
    echo "  status             Show plugin status"
    echo ""
    echo "Examples:"
    echo "  $0                          # Refresh all plugins"
    echo "  $0 external                 # Refresh external plugins"
    echo "  $0 reload ffmpeg_software   # Reload specific plugin"
}

# Check if backend is running
check_backend() {
    if ! docker ps -q -f name=viewra-backend >/dev/null 2>&1; then
        print_error "Backend container is not running"
        print_info "Start it with: docker-compose up -d"
        return 1
    fi
    return 0
}

# Refresh external plugins
refresh_external_plugins() {
    print_status "Refreshing external plugins..."
    
    if curl -s -X POST http://localhost:8080/api/v1/plugins/external/refresh >/dev/null 2>&1; then
        print_success "External plugins refreshed"
        return 0
    else
        print_error "Failed to refresh external plugins"
        return 1
    fi
}

# Refresh playback/transcoding plugins
refresh_playback_plugins() {
    print_status "Refreshing playback/transcoding plugins..."
    
    if curl -s -X POST http://localhost:8080/api/playback/plugins/refresh >/dev/null 2>&1; then
        print_success "Playback plugins refreshed"
        return 0
    else
        print_error "Failed to refresh playback plugins"
        return 1
    fi
}

# Reload a specific plugin
reload_plugin() {
    local plugin_name="$1"
    
    if [ -z "$plugin_name" ]; then
        print_error "Plugin name required"
        return 1
    fi
    
    print_status "Reloading plugin: $plugin_name"
    
    # First, trigger plugin refresh to ensure it's discovered
    refresh_external_plugins
    
    # Then reload the specific plugin
    if curl -s -X POST "http://localhost:8080/api/v1/plugins/${plugin_name}/reload" >/dev/null 2>&1; then
        print_success "Plugin ${plugin_name} reloaded"
        
        # If it's a transcoding plugin, also refresh playback module
        if [[ "$plugin_name" == *"_transcoder" ]] || [[ "$plugin_name" == "ffmpeg_"* ]]; then
            refresh_playback_plugins
        fi
        
        return 0
    else
        print_error "Failed to reload plugin ${plugin_name}"
        return 1
    fi
}

# Show plugin status
show_plugin_status() {
    print_status "Fetching plugin status..."
    
    # Get all plugins
    local plugins_json=$(curl -s http://localhost:8080/api/v1/plugins 2>/dev/null)
    if [ $? -ne 0 ] || [ -z "$plugins_json" ]; then
        print_error "Failed to fetch plugin status"
        return 1
    fi
    
    # Extract and display plugin info
    echo ""
    echo "External Plugins:"
    echo "$plugins_json" | jq -r '.external[]? | "  \(.id): \(if .enabled then "✅ Enabled" else "❌ Disabled" end) - \(.status)"' 2>/dev/null || echo "  No external plugins found"
    
    echo ""
    echo "Core Plugins:"
    echo "$plugins_json" | jq -r '.core[]? | "  \(.id): \(if .enabled then "✅ Enabled" else "❌ Disabled" end)"' 2>/dev/null || echo "  No core plugins found"
    
    # Get transcoding providers
    echo ""
    echo "Transcoding Providers:"
    local providers_json=$(curl -s http://localhost:8080/api/playback/stats 2>/dev/null)
    if [ $? -eq 0 ] && [ -n "$providers_json" ]; then
        echo "$providers_json" | jq -r '.available_providers[]?' 2>/dev/null | sed 's/^/  /' || echo "  No providers found"
    else
        echo "  Unable to fetch provider status"
    fi
    
    echo ""
}

# Enable hot reload
enable_hot_reload() {
    print_status "Enabling hot reload..."
    
    if curl -s -X POST http://localhost:8080/api/v1/plugins/system/hot-reload/enable >/dev/null 2>&1; then
        print_success "Hot reload enabled"
        return 0
    else
        print_warning "Could not enable hot reload (may already be enabled)"
        return 0
    fi
}

# Main refresh all function
refresh_all() {
    print_info "Refreshing all plugin systems..."
    
    # Enable hot reload first
    enable_hot_reload
    
    # Refresh external plugins
    refresh_external_plugins
    
    # Refresh playback plugins
    refresh_playback_plugins
    
    # Show status
    show_plugin_status
    
    print_success "All plugin systems refreshed"
}

# Main execution
main() {
    # Check backend is running
    if ! check_backend; then
        exit 1
    fi
    
    local command="${1:-all}"
    
    case "$command" in
        all)
            refresh_all
            ;;
        
        external)
            refresh_external_plugins
            ;;
        
        playback)
            refresh_playback_plugins
            ;;
        
        reload)
            reload_plugin "$2"
            ;;
        
        status)
            show_plugin_status
            ;;
        
        -h|--help|help)
            show_usage
            exit 0
            ;;
        
        *)
            print_error "Unknown command: $command"
            show_usage
            exit 1
            ;;
    esac
}

# Run main function
main "$@"