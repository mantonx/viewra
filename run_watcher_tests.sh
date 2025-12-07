#!/bin/bash
# Script to run the new segment watcher tests once compilation issues are resolved
#
# This script runs the comprehensive tests for watchSegments and watchSegmentsPolling
# functions in session_segment_watcher.go

echo "Running segment watcher tests..."
echo "================================"
echo ""

# Run only the new watcher tests
go test -v -run "^TestWatchSegments|^TestWatchSegmentsPolling" ./internal/infrastructure/transcoding/

# Check exit code
if [ $? -eq 0 ]; then
    echo ""
    echo "================================"
    echo "All segment watcher tests passed!"
    echo "================================"
else
    echo ""
    echo "================================"
    echo "Some tests failed - see output above"
    echo "================================"
    exit 1
fi
