#!/bin/bash

# Script to ensure FFmpeg plugin is enabled on startup
# This should be called after the backend starts

set -euo pipefail

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${YELLOW}Ensuring FFmpeg plugin is enabled...${NC}"

# Wait for backend to be ready
MAX_ATTEMPTS=30
ATTEMPT=0

while [ $ATTEMPT -lt $MAX_ATTEMPTS ]; do
    if curl -s http://localhost:8080/api/v1/plugins/ >/dev/null 2>&1; then
        break
    fi
    echo "Waiting for backend to be ready... (attempt $((ATTEMPT+1))/$MAX_ATTEMPTS)"
    sleep 2
    ATTEMPT=$((ATTEMPT+1))
done

if [ $ATTEMPT -eq $MAX_ATTEMPTS ]; then
    echo -e "${YELLOW}Backend not ready after $MAX_ATTEMPTS attempts${NC}"
    exit 1
fi

# Enable the plugin
echo "Enabling FFmpeg plugin..."
curl -X POST http://localhost:8080/api/v1/plugins/ffmpeg_software/enable >/dev/null 2>&1 || true

# Refresh playback module
echo "Refreshing playback module..."
sleep 2
curl -X POST http://localhost:8080/api/playback/plugins/refresh >/dev/null 2>&1 || true

# Verify it's working
sleep 1
if curl -s http://localhost:8080/api/playback/stats | grep -q "ffmpeg_software"; then
    echo -e "${GREEN}✓ FFmpeg plugin is enabled and ready${NC}"
else
    echo -e "${YELLOW}⚠ FFmpeg plugin may not be ready${NC}"
fi