#!/bin/bash

echo "🚨 EMERGENCY FFMPEG CLEANUP SCRIPT 🚨"

# Function to kill all FFmpeg processes
kill_all_ffmpeg() {
    echo "Killing all FFmpeg processes..."
    
    # Kill in container
    docker exec viewra-backend-1 sh -c "pkill -9 -f ffmpeg" 2>/dev/null
    
    # Kill on host
    sudo pkill -9 -f ffmpeg 2>/dev/null
    
    # Kill zombies by restarting their parent
    docker exec viewra-backend-1 sh -c "pkill -9 -f ffmpeg_software" 2>/dev/null
    
    echo "Waiting for cleanup..."
    sleep 2
}

# Function to stop all sessions via API
stop_all_sessions() {
    echo "Stopping all transcoding sessions..."
    curl -X DELETE http://localhost:8080/api/playback/sessions/all 2>/dev/null | jq .
}

# Function to restart backend
restart_backend() {
    echo "Restarting backend container..."
    docker-compose restart backend
    echo "Waiting for backend to start..."
    sleep 10
}

# Main cleanup
echo "1. Stopping all sessions..."
stop_all_sessions

echo -e "\n2. Killing FFmpeg processes..."
kill_all_ffmpeg

echo -e "\n3. Checking remaining processes..."
REMAINING=$(ps aux | grep ffmpeg | grep -v grep | wc -l)
echo "Remaining FFmpeg processes: $REMAINING"

if [ "$REMAINING" -gt 0 ]; then
    echo -e "\n4. Zombie processes detected, restarting backend..."
    restart_backend
fi

echo -e "\n5. Final check..."
ps aux | grep ffmpeg | grep -v grep || echo "✅ No FFmpeg processes found!"

echo -e "\n6. Checking orphaned sessions..."
curl -s http://localhost:8080/api/playback/sessions/orphaned | jq .

echo -e "\n✅ Cleanup complete!"