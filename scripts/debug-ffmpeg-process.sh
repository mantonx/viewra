#\!/bin/bash

# Script to debug FFmpeg process execution issues

echo "=== FFmpeg Process Debug Script ==="
echo "Time: $(date)"
echo ""

# Check system limits
echo "=== System Resource Limits ==="
ulimit -a
echo ""

# Check memory
echo "=== Memory Status ==="
free -h
echo ""

# Check disk space
echo "=== Disk Space ==="
df -h /app/viewra-data
echo ""

# Monitor FFmpeg processes
echo "=== Monitoring FFmpeg Processes ==="
echo "Watching for FFmpeg processes..."

# Function to check process
check_process() {
    local pid=$1
    if [ -d "/proc/$pid" ]; then
        echo "Process $pid info:"
        cat /proc/$pid/status  < /dev/null |  grep -E "Name:|State:|VmSize:|VmRSS:|Threads:"
        echo "Command line: $(cat /proc/$pid/cmdline | tr '\0' ' ')"
        echo "CGroup: $(cat /proc/$pid/cgroup)"
        echo ""
    fi
}

# Watch for new FFmpeg processes
while true; do
    pids=$(pgrep -f "ffmpeg.*transcode" 2>/dev/null)
    if [ -n "$pids" ]; then
        echo "Found FFmpeg process(es): $pids"
        for pid in $pids; do
            check_process $pid
            
            # Try to trace the process
            if command -v strace >/dev/null 2>&1; then
                echo "Attempting to trace process $pid..."
                timeout 2 strace -p $pid 2>&1 | head -20
            fi
        done
    fi
    sleep 0.1
done
