#!/bin/bash

# Auto cleanup script that runs every 30 seconds
while true; do
    # Count FFmpeg processes
    FFMPEG_COUNT=$(ps aux | grep ffmpeg | grep -v grep | wc -l)
    
    if [ "$FFMPEG_COUNT" -gt 2 ]; then
        echo "[$(date)] WARNING: Found $FFMPEG_COUNT FFmpeg processes, cleaning up..."
        
        # Stop all sessions via API
        curl -s -X DELETE http://localhost:8080/api/playback/sessions/all > /dev/null
        
        # Kill processes in container
        docker exec viewra-backend-1 sh -c "pkill -9 -f ffmpeg" 2>/dev/null
        
        # Kill zombie parents
        docker exec viewra-backend-1 sh -c "pkill -9 -f ffmpeg_software" 2>/dev/null
        
        echo "[$(date)] Cleanup complete"
    fi
    
    sleep 30
done