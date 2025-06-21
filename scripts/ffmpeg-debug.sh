#!/bin/bash
# FFmpeg debugging script for development

set -e

echo "=== FFmpeg Debugging Tool ==="
echo

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Check if running in container
if [ -f /.dockerenv ]; then
    echo -e "${GREEN}Running inside Docker container${NC}"
else
    echo -e "${YELLOW}Running on host - executing in backend container${NC}"
    docker-compose exec backend /app/scripts/ffmpeg-debug.sh "$@"
    exit 0
fi

# Functions
check_ffmpeg_processes() {
    echo -e "\n${BLUE}=== Active FFmpeg Processes ===${NC}"
    ps aux | grep -E "(ffmpeg|transcode)" | grep -v grep || echo "No FFmpeg processes found"
}

check_plugin_processes() {
    echo -e "\n${BLUE}=== Plugin Processes ===${NC}"
    ps aux | grep plugin | grep -v grep || echo "No plugin processes found"
}

check_system_resources() {
    echo -e "\n${BLUE}=== System Resources ===${NC}"
    echo "Memory:"
    free -h
    echo -e "\nDisk:"
    df -h /app/viewra-data
    echo -e "\nCPU:"
    top -bn1 | head -5
}

check_process_limits() {
    echo -e "\n${BLUE}=== Process Limits ===${NC}"
    ulimit -a
}

check_recent_sessions() {
    echo -e "\n${BLUE}=== Recent Transcode Sessions ===${NC}"
    find /app/viewra-data/transcoding -type d -name "dash_*" -o -name "hls_*" -mtime -1 2>/dev/null | head -10 || echo "No recent sessions"
}

check_debug_logs() {
    echo -e "\n${BLUE}=== Recent Debug Logs ===${NC}"
    find /app/viewra-data/transcoding -name "*.log" -mtime -1 2>/dev/null | while read log; do
        echo -e "${YELLOW}Log: $log${NC}"
        tail -20 "$log"
        echo "---"
    done || echo "No debug logs found"
}

test_ffmpeg_directly() {
    echo -e "\n${BLUE}=== Testing FFmpeg Directly ===${NC}"
    local test_file="$1"
    
    if [ -z "$test_file" ]; then
        # Find a test file
        test_file=$(sqlite3 /app/viewra-data/viewra.db "SELECT path FROM media_files LIMIT 1;" 2>/dev/null || echo "")
    fi
    
    if [ -n "$test_file" ] && [ -f "$test_file" ]; then
        echo "Testing with: $test_file"
        # Simple 5-second test
        timeout 10s ffmpeg -i "$test_file" -t 5 -f null - 2>&1 | head -30
        echo -e "${GREEN}FFmpeg test completed${NC}"
    else
        echo -e "${RED}No test file available${NC}"
    fi
}

monitor_session() {
    local session_id="$1"
    echo -e "\n${BLUE}=== Monitoring Session: $session_id ===${NC}"
    
    local session_dir="/app/viewra-data/transcoding/*${session_id}*"
    
    # Watch the directory
    echo "Watching directory changes..."
    watch -n 1 "ls -la $session_dir 2>/dev/null || echo 'Session directory not found'"
}

enable_verbose_mode() {
    echo -e "\n${BLUE}=== Enabling Verbose FFmpeg Logging ===${NC}"
    export FFMPEG_DEBUG=true
    export FFMPEG_LOG_LEVEL=debug
    echo "Debug mode enabled. Next transcode will create detailed logs."
}

# Main menu
case "${1:-help}" in
    ps|processes)
        check_ffmpeg_processes
        check_plugin_processes
        ;;
    
    resources|res)
        check_system_resources
        check_process_limits
        ;;
    
    logs)
        check_debug_logs
        ;;
    
    sessions)
        check_recent_sessions
        ;;
    
    test)
        test_ffmpeg_directly "$2"
        ;;
    
    monitor)
        if [ -z "$2" ]; then
            echo -e "${RED}Usage: $0 monitor <session_id>${NC}"
            exit 1
        fi
        monitor_session "$2"
        ;;
    
    verbose)
        enable_verbose_mode
        ;;
    
    all)
        check_ffmpeg_processes
        check_plugin_processes
        check_system_resources
        check_recent_sessions
        check_debug_logs
        ;;
    
    help|*)
        echo "Usage: $0 <command> [args]"
        echo
        echo "Commands:"
        echo "  ps|processes     - Show FFmpeg and plugin processes"
        echo "  resources|res    - Show system resources and limits"
        echo "  logs            - Show recent debug logs"
        echo "  sessions        - List recent transcode sessions"
        echo "  test [file]     - Test FFmpeg directly with a file"
        echo "  monitor <id>    - Monitor a specific session"
        echo "  verbose         - Enable verbose FFmpeg logging"
        echo "  all             - Show all information"
        echo
        echo "Examples:"
        echo "  $0 ps"
        echo "  $0 test /media/movie.mkv"
        echo "  $0 monitor 7583695c-14bc-412c"
        ;;
esac