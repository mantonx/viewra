#!/bin/bash
# Test script for content-addressable storage URLs
# This script tests the video playback flow from session URLs to content-hash URLs

set -e

API_BASE="http://localhost:8080"
echo "🎬 Testing Content-Addressable Storage Video Playback"
echo "=================================================="

# 1. Check if backend is running
echo -n "✓ Checking backend health... "
if ! curl -s "${API_BASE}/api/health" | grep -q '"status":"ok"'; then
    echo "❌ Backend not running!"
    exit 1
fi
echo "OK"

# 2. Get a test media file
echo -n "✓ Getting test media file... "
# Get an episode file (skip the test file)
MEDIA_FILES=$(curl -s "${API_BASE}/api/media/files?limit=10")
MEDIA_ID=$(echo "$MEDIA_FILES" | jq -r '.media_files[] | select(.media_type == "episode") | .id' | head -1)
MEDIA_PATH=$(echo "$MEDIA_FILES" | jq -r '.media_files[] | select(.id == "'$MEDIA_ID'") | .path')

if [ "$MEDIA_ID" = "null" ]; then
    echo "❌ No media files found!"
    exit 1
fi
echo "Found: $MEDIA_PATH"

# 3. Start a transcoding session
echo -n "✓ Starting transcoding session... "
SESSION_RESPONSE=$(curl -s -X POST "${API_BASE}/api/playback/start" \
    -H "Content-Type: application/json" \
    -d "{\"media_file_id\": \"$MEDIA_ID\", \"container\": \"dash\", \"enable_abr\": true}")

SESSION_ID=$(echo "$SESSION_RESPONSE" | jq -r '.id')
MANIFEST_URL=$(echo "$SESSION_RESPONSE" | jq -r '.manifest_url')

if [ "$SESSION_ID" = "null" ]; then
    echo "❌ Failed to start session!"
    echo "$SESSION_RESPONSE"
    exit 1
fi
echo "Session: $SESSION_ID"

# 4. Wait for content hash to be available
echo -n "✓ Waiting for content hash... "
MAX_ATTEMPTS=30
ATTEMPT=0
CONTENT_HASH=""

while [ $ATTEMPT -lt $MAX_ATTEMPTS ]; do
    SESSION_STATUS=$(curl -s "${API_BASE}/api/playback/session/$SESSION_ID")
    CONTENT_HASH=$(echo "$SESSION_STATUS" | jq -r '.content_hash // empty')
    TRANSCODE_STATUS=$(echo "$SESSION_STATUS" | jq -r '.status')
    
    if [ -n "$CONTENT_HASH" ]; then
        echo "Hash: $CONTENT_HASH"
        break
    elif [ "$TRANSCODE_STATUS" = "failed" ]; then
        echo "❌ Transcoding failed!"
        echo "$SESSION_STATUS"
        exit 1
    fi
    
    sleep 2
    ATTEMPT=$((ATTEMPT + 1))
    echo -n "."
done

if [ -z "$CONTENT_HASH" ]; then
    echo "❌ Timeout waiting for content hash!"
    exit 1
fi

# 5. Test content-addressable URLs
echo "✓ Testing content URLs..."
CONTENT_BASE="/api/v1/content/${CONTENT_HASH}"

# Test manifest URL
echo -n "  - DASH manifest... "
MANIFEST_STATUS=$(curl -s -o /dev/null -w "%{http_code}" "${API_BASE}${CONTENT_BASE}/manifest.mpd")
if [ "$MANIFEST_STATUS" = "200" ]; then
    echo "✅ OK"
    
    # Check manifest content
    MANIFEST_CONTENT=$(curl -s "${API_BASE}${CONTENT_BASE}/manifest.mpd")
    if echo "$MANIFEST_CONTENT" | grep -q "mediaPresentationDuration"; then
        DURATION=$(echo "$MANIFEST_CONTENT" | grep -oP 'mediaPresentationDuration="[^"]*"' | cut -d'"' -f2)
        echo "    Duration: $DURATION"
    fi
else
    echo "❌ Failed (HTTP $MANIFEST_STATUS)"
fi

# Test video segment
echo -n "  - Video segments... "
VIDEO_STATUS=$(curl -s -o /dev/null -w "%{http_code}" "${API_BASE}${CONTENT_BASE}/packaged/video.mp4")
if [ "$VIDEO_STATUS" = "200" ]; then
    echo "✅ OK"
else
    echo "⚠️  Not found (might be in init segment)"
fi

# Test audio segment
echo -n "  - Audio segments... "
AUDIO_STATUS=$(curl -s -o /dev/null -w "%{http_code}" "${API_BASE}${CONTENT_BASE}/packaged/audio.mp4")
if [ "$AUDIO_STATUS" = "200" ]; then
    echo "✅ OK"
else
    echo "⚠️  Not found (might be in init segment)"
fi

# 6. Test session-based URLs (fallback)
echo "✓ Testing session-based URLs..."
SESSION_BASE="/api/v1/sessions/${SESSION_ID}"

echo -n "  - Session manifest... "
SESSION_MANIFEST_STATUS=$(curl -s -o /dev/null -w "%{http_code}" "${API_BASE}${SESSION_BASE}/manifest.mpd")
if [ "$SESSION_MANIFEST_STATUS" = "200" ]; then
    echo "✅ OK"
else
    echo "❌ Failed (HTTP $SESSION_MANIFEST_STATUS)"
fi

# 7. Clean up
echo -n "✓ Cleaning up session... "
curl -s -X DELETE "${API_BASE}/api/playback/session/${SESSION_ID}" > /dev/null
echo "Done"

echo ""
echo "=================================================="
echo "✅ Content-addressable storage test completed!"
echo ""
echo "Summary:"
echo "  - Media ID: $MEDIA_ID"
echo "  - Session ID: $SESSION_ID"
echo "  - Content Hash: $CONTENT_HASH"
echo "  - Content URL: ${API_BASE}${CONTENT_BASE}/"
echo ""