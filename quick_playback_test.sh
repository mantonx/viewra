#!/bin/bash
# Quick playback integration test
# Tests the basic video playback flow without waiting for full transcoding

set -e

API_BASE="http://localhost:8080"
echo "🎬 Quick Playback Integration Test"
echo "================================="

# 1. Check backend health
echo -n "✓ Checking backend... "
if ! curl -s "${API_BASE}/api/health" | grep -q '"status":"ok"'; then
    echo "❌ Backend not running!"
    exit 1
fi
echo "OK"

# 2. Get an episode
echo -n "✓ Finding test episode... "
EPISODE=$(curl -s "${API_BASE}/api/media/files?limit=10" | jq -r '.media_files[] | select(.media_type == "episode") | {id, path} | @base64' | head -1)
if [ -z "$EPISODE" ]; then
    echo "❌ No episodes found!"
    exit 1
fi

MEDIA_ID=$(echo "$EPISODE" | base64 -d | jq -r '.id')
MEDIA_PATH=$(echo "$EPISODE" | base64 -d | jq -r '.path' | cut -c1-50)
echo "${MEDIA_PATH}..."

# 3. Test playback decision
echo -n "✓ Testing playback decision... "
DECISION=$(curl -s -X POST "${API_BASE}/api/playback/decide" \
    -H "Content-Type: application/json" \
    -d "{\"media_file_id\": \"$MEDIA_ID\"}")

SHOULD_TRANSCODE=$(echo "$DECISION" | jq -r '.should_transcode')
REASON=$(echo "$DECISION" | jq -r '.reason')
echo "$REASON"

# 4. Start playback session
echo -n "✓ Starting playback... "
SESSION=$(curl -s -X POST "${API_BASE}/api/playback/start" \
    -H "Content-Type: application/json" \
    -d "{\"media_file_id\": \"$MEDIA_ID\", \"container\": \"dash\", \"enable_abr\": true}")

SESSION_ID=$(echo "$SESSION" | jq -r '.id')
if [ "$SESSION_ID" = "null" ]; then
    echo "❌ Failed!"
    echo "$SESSION"
    exit 1
fi
echo "Session: $SESSION_ID"

# 5. Quick status check (don't wait for full transcoding)
echo "✓ Checking initial status..."
sleep 2
STATUS=$(curl -s "${API_BASE}/api/playback/session/$SESSION_ID")
TRANSCODE_STATUS=$(echo "$STATUS" | jq -r '.status')
PROGRESS=$(echo "$STATUS" | jq -r '.progress')
MANIFEST_URL=$(echo "$STATUS" | jq -r '.manifest_url // empty')

echo "  - Status: $TRANSCODE_STATUS"
echo "  - Progress: ${PROGRESS}%"
if [ -n "$MANIFEST_URL" ]; then
    echo "  - Manifest: $MANIFEST_URL"
fi

# 6. Test session-based URL (should work immediately)
echo -n "✓ Testing session URL... "
SESSION_MANIFEST="${API_BASE}/api/v1/sessions/${SESSION_ID}/manifest.mpd"
SESSION_STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$SESSION_MANIFEST")
if [ "$SESSION_STATUS" = "200" ]; then
    echo "✅ Working!"
else
    echo "❌ Not ready (HTTP $SESSION_STATUS)"
fi

# 7. Check if content hash is generated quickly
echo -n "✓ Checking for content hash... "
CONTENT_HASH=$(echo "$STATUS" | jq -r '.content_hash // empty')
if [ -n "$CONTENT_HASH" ]; then
    echo "Found: $CONTENT_HASH"
    CONTENT_URL="${API_BASE}/api/v1/content/${CONTENT_HASH}/manifest.mpd"
    echo "  - Content URL: $CONTENT_URL"
else
    echo "Not yet available"
fi

# 8. Clean up
echo -n "✓ Stopping session... "
curl -s -X DELETE "${API_BASE}/api/playback/session/${SESSION_ID}" > /dev/null
echo "Done"

echo ""
echo "================================="
echo "✅ Playback test completed!"
echo ""
echo "Key findings:"
echo "  - Transcoding required: $SHOULD_TRANSCODE"
echo "  - Session-based URLs: $([ "$SESSION_STATUS" = "200" ] && echo "Working" || echo "Not working")"
echo "  - Content hash: $([ -n "$CONTENT_HASH" ] && echo "Available" || echo "Not yet generated")"
echo ""