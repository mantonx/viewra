#!/bin/bash

# Test Shaka Packager integration
echo "Testing Shaka Packager integration..."

# Get a media file ID
MEDIA_ID="4c182292-552d-4c79-a9e7-97b8fecd0f18"

echo "Using media file ID: $MEDIA_ID"

# Start transcoding with Shaka enabled
RESPONSE=$(curl -s -X POST http://localhost:8080/api/playback/start \
    -H "Content-Type: application/json" \
    -d "{\"media_file_id\": \"$MEDIA_ID\", \"container\": \"dash\"}")

echo "Response: $RESPONSE"

# Extract session ID
SESSION_ID=$(echo "$RESPONSE" | grep -o '"id":"[^"]*' | cut -d'"' -f4)

if [ -z "$SESSION_ID" ]; then
    echo "Failed to get session ID"
    exit 1
fi

echo "Session ID: $SESSION_ID"

# Wait a bit for processing
sleep 5

# Check logs for our fixed stream selectors
echo "Checking for fixed Shaka arguments..."
docker-compose logs backend | grep -E "stream=video|stream=audio|Starting Shaka" | tail -20

# Also check for any errors
echo -e "\nChecking for errors..."
docker-compose logs backend | grep -i "shaka.*error\|shaka.*failed" | tail -10