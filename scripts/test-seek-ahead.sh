#!/bin/bash

# Test seek-ahead functionality

echo "🎬 Testing Seek-Ahead Functionality"
echo "=================================="

# Check if FFmpeg plugin is running
echo -n "1. Checking FFmpeg plugin status... "
BACKENDS=$(curl -s http://localhost:8080/api/playback/stats | jq -r '.backends | keys[]')
if [[ "$BACKENDS" == *"ffmpeg_transcoder"* ]]; then
    echo "✅ FFmpeg plugin is registered"
else
    echo "❌ FFmpeg plugin not found!"
    exit 1
fi

# Get active sessions
echo -n "2. Getting active sessions... "
SESSIONS=$(curl -s http://localhost:8080/api/playback/sessions | jq -r '.sessions[] | .id')
if [ -z "$SESSIONS" ]; then
    echo "⚠️  No active sessions. Start playing a video first!"
    exit 1
else
    echo "✅ Found sessions: $(echo $SESSIONS | wc -w)"
fi

# Test seek-ahead for the first session
SESSION_ID=$(echo $SESSIONS | awk '{print $1}')
echo "3. Testing seek-ahead for session: $SESSION_ID"

# Call seek-ahead API
echo -n "4. Calling seek-ahead API (seeking to 10 minutes)... "
RESPONSE=$(curl -s -X POST http://localhost:8080/api/playback/seek-ahead \
    -H "Content-Type: application/json" \
    -d '{
        "session_id": "'$SESSION_ID'",
        "seek_time": 600
    }')

if [ $? -eq 0 ]; then
    echo "✅ API call successful"
    echo "   Response: $RESPONSE" | jq '.'
    
    # Extract new session ID
    NEW_SESSION=$(echo $RESPONSE | jq -r '.session_id')
    if [ "$NEW_SESSION" != "null" ]; then
        echo "5. ✅ New session created: $NEW_SESSION"
        echo "   Manifest URL: $(echo $RESPONSE | jq -r '.manifest_url')"
    else
        echo "5. ❌ Failed to create new session"
    fi
else
    echo "❌ API call failed"
fi

echo ""
echo "Test complete! Check your browser console for frontend logs." 