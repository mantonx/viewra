#!/bin/bash
# Test session-based URLs during transcoding
# This tests the temporary URLs used before content is fully packaged

set -e

API_BASE="http://localhost:8080"
echo "🎬 Testing Session-Based URLs"
echo "============================"

# 1. Get a media file
echo -n "✓ Finding test media... "
MEDIA_FILES=$(curl -s "${API_BASE}/api/media/files?limit=10")
MEDIA_ID=$(echo "$MEDIA_FILES" | jq -r '.media_files[] | select(.media_type == "episode") | .id' | head -1)
MEDIA_PATH=$(echo "$MEDIA_FILES" | jq -r '.media_files[] | select(.id == "'$MEDIA_ID'") | .path' | cut -c1-50)
echo "${MEDIA_PATH}..."

# 2. Start transcoding
echo -n "✓ Starting transcoding... "
SESSION_RESPONSE=$(curl -s -X POST "${API_BASE}/api/playback/start" \
    -H "Content-Type: application/json" \
    -d "{\"media_file_id\": \"$MEDIA_ID\", \"container\": \"dash\", \"enable_abr\": true}")

SESSION_ID=$(echo "$SESSION_RESPONSE" | jq -r '.id')
echo "Session: $SESSION_ID"

# 3. Wait for encoding to produce some output
echo -n "✓ Waiting for encoding to start... "
MAX_WAIT=30
WAIT=0
while [ $WAIT -lt $MAX_WAIT ]; do
    # Check if intermediate.mp4 exists in encoded directory
    if curl -s -f "${API_BASE}/api/v1/sessions/${SESSION_ID}/intermediate.mp4" > /dev/null 2>&1; then
        echo "Encoding started!"
        break
    fi
    sleep 1
    WAIT=$((WAIT + 1))
    echo -n "."
done

if [ $WAIT -eq $MAX_WAIT ]; then
    echo "❌ Timeout waiting for encoding"
    exit 1
fi

# 4. Test session-based URL access
echo "✓ Testing session-based file access..."

# Try to access the intermediate encoded file
echo -n "  - Intermediate file... "
INTERMEDIATE_STATUS=$(curl -s -o /dev/null -w "%{http_code}" "${API_BASE}/api/v1/sessions/${SESSION_ID}/intermediate.mp4")
if [ "$INTERMEDIATE_STATUS" = "200" ]; then
    echo "✅ Accessible"
else
    echo "❌ Not accessible (HTTP $INTERMEDIATE_STATUS)"
fi

# 5. Wait a bit for packaging to start
echo -n "✓ Waiting for packaging... "
sleep 5

# Check for manifest
echo -n "  - DASH manifest... "
MANIFEST_STATUS=$(curl -s -o /dev/null -w "%{http_code}" "${API_BASE}/api/v1/sessions/${SESSION_ID}/manifest.mpd")
if [ "$MANIFEST_STATUS" = "200" ]; then
    echo "✅ Available"
    # Get some manifest content
    MANIFEST_PREVIEW=$(curl -s "${API_BASE}/api/v1/sessions/${SESSION_ID}/manifest.mpd" | head -3)
    echo "    Preview: $(echo "$MANIFEST_PREVIEW" | head -1)"
else
    echo "⚠️  Not yet available (HTTP $MANIFEST_STATUS)"
fi

# 6. Check session details
echo "✓ Session details:"
SESSION_INFO=$(curl -s "${API_BASE}/api/playback/session/${SESSION_ID}")
echo "  - Status: $(echo "$SESSION_INFO" | jq -r '.status')"
echo "  - Progress: $(echo "$SESSION_INFO" | jq -r '.progress // 0')%"
echo "  - Provider: $(echo "$SESSION_INFO" | jq -r '.provider')"
echo "  - Content Hash: $(echo "$SESSION_INFO" | jq -r '.content_hash')"

# 7. Clean up
echo -n "✓ Stopping session... "
curl -s -X DELETE "${API_BASE}/api/playback/session/${SESSION_ID}" > /dev/null
echo "Done"

echo ""
echo "============================"
echo "✅ Session URL test completed!"
echo ""
echo "Summary:"
echo "  - Session ID: $SESSION_ID"
echo "  - Session URLs work for: intermediate files during encoding"
echo "  - Manifest availability: depends on packaging completion"
echo ""