#!/bin/bash
# Debug script for playback issues

set -e

echo "=== Viewra Playback Debugging Tool ==="
echo

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check if backend is running
echo -n "Checking backend status... "
if curl -s http://localhost:8080/api/health > /dev/null; then
    echo -e "${GREEN}✓ Running${NC}"
else
    echo -e "${RED}✗ Not running${NC}"
    echo "Start with: docker-compose up -d"
    exit 1
fi

# Check plugin status
echo -e "\n${YELLOW}Plugin Status:${NC}"
echo "Checking available plugins..."
curl -s http://localhost:8080/api/admin/plugins/ | jq -r '.[] | "- \(.name) [\(.status)]"' || echo "Failed to get plugins"

# Check transcoding plugins specifically
echo -e "\n${YELLOW}Transcoding Providers:${NC}"
curl -s http://localhost:8080/api/playback/stats | jq '.backends' || echo "No providers found"

# Test media file lookup
if [ "$1" != "" ]; then
    echo -e "\n${YELLOW}Testing media file lookup:${NC}"
    echo "Looking up media file: $1"
    docker-compose exec backend sh -c "sqlite3 /app/viewra-data/viewra.db \"SELECT id, path, container FROM media_files WHERE id='$1' OR path LIKE '%$1%' LIMIT 5;\""
fi

# Show recent logs
echo -e "\n${YELLOW}Recent playback logs:${NC}"
docker-compose logs --tail=20 backend | grep -E "(playback|transcode|ffmpeg|plugin)" || echo "No recent playback logs"

# Test playback endpoint
if [ "$2" == "test" ]; then
    echo -e "\n${YELLOW}Testing playback endpoint:${NC}"
    MEDIA_ID="${1:-ffff2929-a038-46ba-a4ed-739dd08b88a2}"
    echo "Testing with media_file_id: $MEDIA_ID"
    
    echo -e "\nRequest:"
    echo '{"media_file_id": "'$MEDIA_ID'", "container": "dash"}'
    
    echo -e "\nResponse:"
    curl -X POST http://localhost:8080/api/playback/start \
        -H "Content-Type: application/json" \
        -d '{"media_file_id": "'$MEDIA_ID'", "container": "dash"}' \
        -s | jq . || echo "Request failed"
fi

# Show help
if [ "$1" == "" ]; then
    echo -e "\n${YELLOW}Usage:${NC}"
    echo "  ./debug-playback.sh                    # Show status"
    echo "  ./debug-playback.sh <media_id>         # Look up media file"
    echo "  ./debug-playback.sh <media_id> test    # Test playback endpoint"
    echo "  ./debug-playback.sh help              # Show debugging tips"
fi

if [ "$1" == "help" ]; then
    echo -e "\n${YELLOW}Common debugging commands:${NC}"
    echo "# Force rebuild without cache:"
    echo "  make clean && docker-compose build --no-cache backend"
    echo
    echo "# Watch logs in real-time:"
    echo "  docker-compose logs -f backend | grep -E '(playback|transcode|plugin)'"
    echo
    echo "# Check FFmpeg installation:"
    echo "  docker-compose exec backend ffmpeg -version"
    echo
    echo "# List transcoding directories:"
    echo "  docker-compose exec backend ls -la /app/viewra-data/transcoding/"
    echo
    echo "# Refresh plugins:"
    echo "  curl -X POST http://localhost:8080/api/playback/plugins/refresh"
fi