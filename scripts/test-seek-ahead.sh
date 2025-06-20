#!/bin/bash

# Test seek-ahead functionality

echo "🎬 Testing Seek-Ahead Functionality"
echo "==================================="

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

# Check if session has request field
echo -n "4. Checking session has Request field... "
HAS_REQUEST=$(curl -s http://localhost:8080/api/playback/session/$SESSION_ID | jq -r '.request')
if [ "$HAS_REQUEST" != "null" ]; then
    echo "✅ Session has Request field"
else
    echo "❌ Session missing Request field - restart playback!"
    exit 1
fi

echo "5. Calling seek-ahead API (seeking to 10 minutes)... "
RESPONSE=$(curl -s -X POST http://localhost:8080/api/playback/seek-ahead \
  -H "Content-Type: application/json" \
  -d "{\"session_id\": \"$SESSION_ID\", \"seek_time\": 600}")

# Check if response is an error
if echo "$RESPONSE" | jq -e '.error' > /dev/null 2>&1; then
    ERROR=$(echo "$RESPONSE" | jq -r '.error')
    echo "❌ Seek-ahead failed: $ERROR"
    exit 1
else
    echo "✅ API call successful"
fi

# Extract new session ID from response
NEW_SESSION=$(echo "$RESPONSE" | jq -r '.session_id // empty')
if [ -n "$NEW_SESSION" ]; then
    echo "6. ✅ Created new session: $NEW_SESSION"
    
    # Check manifest URL
    MANIFEST=$(echo "$RESPONSE" | jq -r '.manifest_url // empty')
    if [ -n "$MANIFEST" ]; then
        echo "7. ✅ Manifest URL: $MANIFEST"
    else
        echo "7. ⚠️  No manifest URL in response"
    fi
else
    echo "6. ❌ Failed to create new session"
fi

echo ""
echo "Test complete! Check your browser console for frontend logs."
echo ""
echo "To test in the UI:"
echo "1. Play a video and wait ~20 seconds for buffering"
echo "2. Click on the blue-tinted area in the progress bar (unbuffered content)"
echo "3. You should see 'Buffering for seek-ahead...' message"
echo "4. Video should resume from the seeked position" 