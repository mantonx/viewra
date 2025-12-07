#!/bin/bash
# Test script to verify ffmpeg_encoding_profiles_test.go

set -e

echo "Testing encoding profile functions..."
echo ""

# Run specific tests from our new test file
echo "Running H.265 profile tests..."
go test -v -run "TestAddH265Profile" ./internal/infrastructure/transcoding/ 2>&1 | grep -E "(PASS|FAIL|RUN)" || true

echo ""
echo "Running VP9 profile tests..."
go test -v -run "TestAddVP9Profile" ./internal/infrastructure/transcoding/ 2>&1 | grep -E "(PASS|FAIL|RUN)" || true

echo ""
echo "Running AV1 profile tests..."
go test -v -run "TestAddAV1Profile" ./internal/infrastructure/transcoding/ 2>&1 | grep -E "(PASS|FAIL|RUN)" || true

echo ""
echo "Running codec profile routing tests..."
go test -v -run "TestAddCodecProfile" ./internal/infrastructure/transcoding/ 2>&1 | grep -E "(PASS|FAIL|RUN)" || true

echo ""
echo "Test execution complete."
