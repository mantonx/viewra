#!/bin/bash

# Test video playback with a real episode from the database

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${GREEN}=== Video Playback Test ===${NC}"
echo ""

# Episode details (Louie S01E00 - small H264 file for testing)
MEDIA_FILE_ID="5d3dae1e-b4d9-4876-9a9a-930bc62ac1da"
FILE_PATH="/media/tv/Louie (2010)/Season 01/Louie (2010) - S01E00 - .mkv"

echo -e "${YELLOW}Testing with episode:${NC}"
echo "File ID: $MEDIA_FILE_ID"
echo "Path: $FILE_PATH"
echo "Size: 22MB"
echo "Codec: H264"
echo ""

# Step 1: Check if file exists
echo -e "${YELLOW}Step 1: Checking if file exists...${NC}"
if docker exec viewra-backend-1 test -f "$FILE_PATH"; then
    echo -e "${GREEN}✓ File exists${NC}"
else
    echo -e "${RED}✗ File not found! Please ensure your media is mounted.${NC}"
    exit 1
fi

# Step 2: Get playback decision
echo -e "\n${YELLOW}Step 2: Getting playback decision...${NC}"
DECISION_RESPONSE=$(curl -s -X POST http://localhost:8080/api/playback/decide \
  -H "Content-Type: application/json" \
  -d '{
    "file_id": "'$MEDIA_FILE_ID'",
    "device_profile": {
      "user_agent": "Test Client",
      "supported_codecs": ["h264", "aac"],
      "max_resolution": "1080p",
      "max_bitrate": 8000,
      "supports_hevc": false
    }
  }')

echo "Response: $DECISION_RESPONSE"

# Check if transcoding is needed
SHOULD_TRANSCODE=$(echo $DECISION_RESPONSE | jq -r '.should_transcode')
REASON=$(echo $DECISION_RESPONSE | jq -r '.reason')

echo -e "${GREEN}Should transcode: $SHOULD_TRANSCODE${NC}"
echo -e "${GREEN}Reason: $REASON${NC}"

# Step 3: Start transcoding session
echo -e "\n${YELLOW}Step 3: Starting transcoding session...${NC}"
SESSION_RESPONSE=$(curl -s -X POST http://localhost:8080/api/playback/start \
  -H "Content-Type: application/json" \
  -d '{
    "media_file_id": "'$MEDIA_FILE_ID'",
    "container": "dash",
    "enable_abr": false
  }')

echo "Response: $SESSION_RESPONSE"

SESSION_ID=$(echo $SESSION_RESPONSE | jq -r '.id')
MANIFEST_URL=$(echo $SESSION_RESPONSE | jq -r '.manifest_url')
CONTENT_HASH=$(echo $SESSION_RESPONSE | jq -r '.content_hash')

if [ "$SESSION_ID" = "null" ]; then
    echo -e "${RED}✗ Failed to create session!${NC}"
    echo "Full response: $SESSION_RESPONSE"
    exit 1
fi

echo -e "${GREEN}✓ Session created:${NC}"
echo "  Session ID: $SESSION_ID"
echo "  Manifest URL: $MANIFEST_URL"
echo "  Content Hash: $CONTENT_HASH"

# Step 4: Monitor transcoding progress
echo -e "\n${YELLOW}Step 4: Waiting for manifest to be ready...${NC}"
MAX_ATTEMPTS=30
ATTEMPT=0

while [ $ATTEMPT -lt $MAX_ATTEMPTS ]; do
    MANIFEST_CHECK=$(curl -s -o /dev/null -w "%{http_code}" "http://localhost:8080$MANIFEST_URL")
    
    if [ "$MANIFEST_CHECK" = "200" ]; then
        echo -e "\n${GREEN}✓ Manifest is ready!${NC}"
        break
    fi
    
    echo -n "."
    sleep 1
    ((ATTEMPT++))
done

if [ $ATTEMPT -eq $MAX_ATTEMPTS ]; then
    echo -e "\n${RED}✗ Manifest not ready after 30 seconds${NC}"
    
    # Check session status
    SESSION_STATUS=$(curl -s "http://localhost:8080/api/playback/session/$SESSION_ID")
    echo "Session status: $SESSION_STATUS"
    
    # Check backend logs
    echo -e "\n${YELLOW}Recent backend logs:${NC}"
    docker logs viewra-backend-1 --tail 50 | grep -E "(ERROR|WARN|$SESSION_ID)"
    
    exit 1
fi

# Step 5: Verify manifest content
echo -e "\n${YELLOW}Step 5: Checking manifest content...${NC}"
MANIFEST_CONTENT=$(curl -s "http://localhost:8080$MANIFEST_URL")
echo "First 500 chars of manifest:"
echo "$MANIFEST_CONTENT" | head -c 500
echo "..."

# Check if it's valid DASH manifest
if echo "$MANIFEST_CONTENT" | grep -q "<MPD"; then
    echo -e "\n${GREEN}✓ Valid DASH manifest detected${NC}"
else
    echo -e "\n${RED}✗ Invalid manifest content${NC}"
fi

# Step 6: Check for segments
echo -e "\n${YELLOW}Step 6: Checking for video segments...${NC}"
# Extract base URL from manifest URL
BASE_URL=$(echo $MANIFEST_URL | sed 's/manifest.mpd//')

# Try common segment patterns
SEGMENT_URLS=(
    "${BASE_URL}init-0.m4s"
    "${BASE_URL}chunk-stream0-00001.m4s"
    "${BASE_URL}segment-0.m4s"
)

SEGMENT_FOUND=false
for SEGMENT_URL in "${SEGMENT_URLS[@]}"; do
    SEGMENT_CHECK=$(curl -s -o /dev/null -w "%{http_code}" "http://localhost:8080$SEGMENT_URL")
    if [ "$SEGMENT_CHECK" = "200" ]; then
        echo -e "${GREEN}✓ Found segment at: $SEGMENT_URL${NC}"
        SEGMENT_FOUND=true
        break
    fi
done

if [ "$SEGMENT_FOUND" = false ]; then
    echo -e "${RED}✗ No segments found${NC}"
fi

# Step 7: Get session details
echo -e "\n${YELLOW}Step 7: Getting session details...${NC}"
SESSION_DETAILS=$(curl -s "http://localhost:8080/api/playback/session/$SESSION_ID")
echo "Session details:"
echo "$SESSION_DETAILS" | jq '.'

# Summary
echo -e "\n${GREEN}=== Test Summary ===${NC}"
echo "Session ID: $SESSION_ID"
echo "Manifest URL: http://localhost:8080$MANIFEST_URL"
echo "Content Hash: $CONTENT_HASH"
echo ""
echo -e "${YELLOW}To play this video:${NC}"
echo "1. Open your browser to: http://localhost:3000"
echo "2. Navigate to the episode"
echo "3. Or use a DASH player with URL: http://localhost:8080$MANIFEST_URL"
echo ""
echo -e "${YELLOW}To stop the session:${NC}"
echo "curl -X DELETE http://localhost:8080/api/playback/session/$SESSION_ID"