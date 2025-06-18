#!/bin/bash

# Viewra E2E Transcoding Test Runner
# This script runs comprehensive end-to-end tests for DASH/HLS transcoding

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}🎬 Viewra E2E Transcoding Test Suite${NC}"
echo "=============================================="

# Check prerequisites
echo -e "${YELLOW}📋 Checking prerequisites...${NC}"

# Check if FFmpeg is available
if ! command -v ffmpeg &> /dev/null; then
    echo -e "${RED}❌ FFmpeg not found in PATH${NC}"
    echo "FFmpeg is required for transcoding tests"
    echo "Please install FFmpeg and ensure it's in your PATH"
    exit 1
fi

FFMPEG_VERSION=$(ffmpeg -version | head -n1)
echo -e "${GREEN}✅ FFmpeg found: ${FFMPEG_VERSION}${NC}"

# Check Go test environment
if ! command -v go &> /dev/null; then
    echo -e "${RED}❌ Go not found in PATH${NC}"
    exit 1
fi

GO_VERSION=$(go version)
echo -e "${GREEN}✅ Go found: ${GO_VERSION}${NC}"

# Navigate to backend directory
cd "$(dirname "$0")/.."
echo -e "${GREEN}✅ Working directory: $(pwd)${NC}"

# Test configuration
TEST_PACKAGE="./internal/modules/playbackmodule"
TEST_TIMEOUT="300s" # 5 minutes timeout for E2E tests
VERBOSE_FLAG=""
COVERAGE_FLAG=""
BENCHMARK_FLAG=""
RUN_PATTERN=""

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -v|--verbose)
            VERBOSE_FLAG="-v"
            shift
            ;;
        -c|--coverage)
            COVERAGE_FLAG="-coverprofile=coverage_e2e.out"
            shift
            ;;
        -b|--benchmark)
            BENCHMARK_FLAG="-bench=."
            shift
            ;;
        -r|--run)
            RUN_PATTERN="-run=$2"
            shift 2
            ;;
        -h|--help)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  -v, --verbose     Enable verbose test output"
            echo "  -c, --coverage    Generate coverage report"
            echo "  -b, --benchmark   Run performance benchmarks"
            echo "  -r, --run PATTERN Run only tests matching pattern"
            echo "  -h, --help        Show this help message"
            echo ""
            echo "Examples:"
            echo "  $0                          # Run all E2E tests"
            echo "  $0 -v                       # Run with verbose output"
            echo "  $0 -r TestE2ETranscodingDASH # Run only DASH tests"
            echo "  $0 -b                       # Run benchmarks"
            echo "  $0 -c -v                    # Run with coverage and verbose output"
            exit 0
            ;;
        *)
            echo -e "${RED}❌ Unknown option: $1${NC}"
            echo "Use -h or --help for usage information"
            exit 1
            ;;
    esac
done

# Set default run pattern if none specified
if [[ -z "$RUN_PATTERN" ]]; then
    RUN_PATTERN="-run=TestE2E"
fi

echo ""
echo -e "${YELLOW}🔧 Test Configuration:${NC}"
echo "  Package: $TEST_PACKAGE"
echo "  Timeout: $TEST_TIMEOUT"
echo "  Pattern: $RUN_PATTERN"
echo "  Verbose: $([ -n "$VERBOSE_FLAG" ] && echo "Yes" || echo "No")"
echo "  Coverage: $([ -n "$COVERAGE_FLAG" ] && echo "Yes" || echo "No")"
echo "  Benchmarks: $([ -n "$BENCHMARK_FLAG" ] && echo "Yes" || echo "No")"

echo ""
echo -e "${YELLOW}🧪 Running E2E tests...${NC}"

# Build the test command
TEST_CMD="go test $TEST_PACKAGE -timeout=$TEST_TIMEOUT $RUN_PATTERN $VERBOSE_FLAG $COVERAGE_FLAG $BENCHMARK_FLAG"

echo "Command: $TEST_CMD"
echo ""

# Run the tests
if eval $TEST_CMD; then
    echo ""
    echo -e "${GREEN}✅ All E2E tests passed successfully!${NC}"
    
    # Show coverage report if generated
    if [[ -n "$COVERAGE_FLAG" ]]; then
        echo ""
        echo -e "${YELLOW}📊 Coverage Report:${NC}"
        go tool cover -func=coverage_e2e.out | tail -1
        
        # Generate HTML coverage report
        go tool cover -html=coverage_e2e.out -o coverage_e2e.html
        echo -e "${GREEN}✅ HTML coverage report generated: coverage_e2e.html${NC}"
    fi
    
    # Show test artifacts
    echo ""
    echo -e "${YELLOW}📁 Test artifacts:${NC}"
    find /tmp -name "viewra_e2e_test_*" -type d 2>/dev/null | head -5 | while read dir; do
        echo "  - $dir"
        ls -la "$dir" 2>/dev/null | head -3 | tail -2 || true
    done
    
    echo ""
    echo -e "${GREEN}🎉 E2E test suite completed successfully!${NC}"
    
else
    echo ""
    echo -e "${RED}❌ E2E tests failed!${NC}"
    echo ""
    echo -e "${YELLOW}🔍 Debugging tips:${NC}"
    echo "  1. Check FFmpeg installation and permissions"
    echo "  2. Ensure sufficient disk space for test files"
    echo "  3. Run with -v flag for detailed output"
    echo "  4. Check system logs for transcoding errors"
    echo ""
    echo -e "${YELLOW}📁 Check test artifacts in /tmp/viewra_e2e_test_*${NC}"
    
    exit 1
fi 