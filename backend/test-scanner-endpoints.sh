#!/bin/bash

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}Testing Scanner Module Endpoints${NC}"

# Determine which port to use
echo -e "\n${YELLOW}Checking server availability...${NC}"
PRIMARY_PORT=8080
FALLBACK_PORT=8081

if curl -s http://localhost:$PRIMARY_PORT/api/hello > /dev/null; then
    PORT=$PRIMARY_PORT
    echo -e "${GREEN}Server found on primary port $PORT${NC}"
elif curl -s http://localhost:$FALLBACK_PORT/api/hello > /dev/null; then
    PORT=$FALLBACK_PORT
    echo -e "${GREEN}Server found on fallback port $PORT${NC}"
else
    echo -e "${RED}Server not found on ports $PRIMARY_PORT or $FALLBACK_PORT. Make sure the server is running.${NC}"
    exit 1
fi

# Test scanner status endpoint
echo -e "\n${YELLOW}Testing Scanner Status Endpoint...${NC}"
curl -s http://localhost:$PORT/api/scanner/status | jq || echo "Failed to get scanner status"

# Test scanner config endpoint
echo -e "\n${YELLOW}Testing Scanner Config Endpoint...${NC}"
curl -s http://localhost:$PORT/api/scanner/config | jq || echo "Failed to get scanner config"

# Test list scan jobs endpoint
echo -e "\n${YELLOW}Testing List Scan Jobs Endpoint...${NC}"
curl -s http://localhost:$PORT/api/scanner/jobs | jq || echo "Failed to list scan jobs"

# Test start scan endpoint
echo -e "\n${YELLOW}Testing Start Scan Endpoint...${NC}"
curl -s -X POST http://localhost:$PORT/api/scanner/scan | jq || echo "Failed to start scan"

echo -e "\n${GREEN}Scanner Module Tests Complete${NC}"
