#!/bin/bash
# FFmpeg process monitoring script for testing resource cleanup

clear
echo "========================================="
echo "FFmpeg Process Monitor"
echo "========================================="
echo ""
echo "Legend:"
echo "  Active  = Normal running process"
echo "  Zombie  = <defunct> process (should be 0)"
echo ""
echo "Press Ctrl+C to exit"
echo "========================================="
echo ""

while true; do
    # Move cursor to top of results area
    tput cup 9 0

    # Count processes
    ACTIVE=$(ps aux | grep ffmpeg | grep -v grep | grep -v defunct | wc -l)
    ZOMBIE=$(ps aux | grep ffmpeg | grep defunct | wc -l)

    # Display counts with color
    echo "Active FFmpeg Processes: $ACTIVE"
    echo "Zombie FFmpeg Processes: $ZOMBIE"
    echo ""
    echo "========================================="

    # Show detailed process list
    echo "Process Details:"
    echo "----------------------------------------"
    ps aux | grep ffmpeg | grep -v grep | grep -v monitor-ffmpeg | head -10

    # Clear rest of screen
    tput ed

    sleep 2
done
