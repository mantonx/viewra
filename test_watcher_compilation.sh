#!/bin/bash
# Temporary script to test if the new watcher tests compile correctly

cd /home/fictional/Projects/viewra2

# Try to build just checking syntax
go test -c -o /dev/null ./internal/infrastructure/transcoding/... 2>&1 | grep -A5 "session_segment_watcher_test.go" || echo "No errors in session_segment_watcher_test.go"
