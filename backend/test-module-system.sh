#!/bin/bash

# This script tests the Viewra backend server with the module system

echo "===================================="
echo "  Testing Viewra Module System      "
echo "===================================="

# Change to backend directory
cd "$(dirname "$0")/../backend" || exit 1

# Build test scanner
echo "Building test-scanner..."
go build -o ./tmp/test-scanner ./cmd/test-scanner/main.go

# Run test scanner (module test)
echo -e "\nTesting module system..."
./tmp/test-scanner

# Start server in background
echo -e "\nStarting server in background..."
SQLITE_PATH=./data/viewra.db PORT=8081 go run cmd/viewra/main.go &
SERVER_PID=$!

# Give the server time to start
echo "Waiting for server to start..."
sleep 3

# Test server endpoints
echo -e "\nTesting server endpoints..."
echo "Testing /api/hello endpoint..."
curl -s http://localhost:8081/api/hello
echo -e "\n\nTesting /api/db-status endpoint..."
curl -s http://localhost:8081/api/db-status
echo -e "\n\nTesting /api/scanner/status endpoint..."
curl -s http://localhost:8081/api/scanner/status
echo -e "\n\nTesting /api/scanner/jobs endpoint..."
curl -s http://localhost:8081/api/scanner/jobs

# Kill server
echo -e "\n\nStopping server..."
kill $SERVER_PID

echo -e "\n===================================="
echo "  Test completed  "
echo "===================================="
