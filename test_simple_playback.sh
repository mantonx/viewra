#!/bin/bash

# Simple test to check if basic transcoding works

echo "Testing simple transcoding without DASH packaging..."

# Find a small H264 file
MEDIA_FILE_ID="5d3dae1e-b4d9-4876-9a9a-930bc62ac1da"

# Start a simple MP4 transcode (no DASH/HLS)
echo "Starting MP4 transcoding..."
SESSION_RESPONSE=$(curl -s -X POST http://localhost:8080/api/playback/start \
  -H "Content-Type: application/json" \
  -d '{
    "media_file_id": "'$MEDIA_FILE_ID'",
    "container": "mp4",
    "enable_abr": false
  }')

echo "Response: $SESSION_RESPONSE"

SESSION_ID=$(echo $SESSION_RESPONSE | jq -r '.id')

if [ "$SESSION_ID" != "null" ]; then
    echo "Session created: $SESSION_ID"
    
    # Check session status
    sleep 5
    SESSION_STATUS=$(curl -s "http://localhost:8080/api/playback/session/$SESSION_ID")
    echo "Session status after 5 seconds:"
    echo "$SESSION_STATUS" | jq '.'
    
    # Check for errors in logs
    echo -e "\nRecent FFmpeg logs:"
    docker logs viewra-backend-1 --tail 30 2>&1 | grep -E "(FFmpeg|ffmpeg|segment|error)" | tail -10
else
    echo "Failed to create session"
fi