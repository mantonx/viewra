#!/bin/bash

# Comprehensive Media Scanner Test Suite
# This script provides complete testing for the Viewra media scanner functionality
# including pause/resume, progress tracking, and endpoint validation

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
PRIMARY_PORT=8080
FALLBACK_PORT=8081
BASE_URL=""

# Functions
print_header() {
    echo -e "\n${BLUE}========================================${NC}"
    echo -e "${BLUE}  $1${NC}"
    echo -e "${BLUE}========================================${NC}\n"
}

print_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

print_error() {
    echo -e "${RED}✗ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠ $1${NC}"
}

print_info() {
    echo -e "${BLUE}ℹ $1${NC}"
}

# Check server availability
check_server() {
    print_info "Checking server availability..."
    
    if curl -s http://localhost:$PRIMARY_PORT/api/health > /dev/null 2>&1; then
        BASE_URL="http://localhost:$PRIMARY_PORT/api"
        print_success "Server found on primary port $PRIMARY_PORT"
        return 0
    elif curl -s http://localhost:$FALLBACK_PORT/api/health > /dev/null 2>&1; then
        BASE_URL="http://localhost:$FALLBACK_PORT/api"
        print_success "Server found on fallback port $FALLBACK_PORT"
        return 0
    else
        print_error "Server not found on ports $PRIMARY_PORT or $FALLBACK_PORT"
        echo "Please make sure the server is running with: docker-compose up -d"
        return 1
    fi
}

# Test basic endpoints
test_endpoints() {
    print_header "Testing Scanner Endpoints"
    
    print_info "Testing health endpoint..."
    if curl -s "${BASE_URL}/health" > /dev/null; then
        print_success "Health endpoint working"
    else
        print_error "Health endpoint failed"
    fi
    
    print_info "Testing scanner status endpoint..."
    if curl -s "${BASE_URL}/scanner/status" | jq . > /dev/null 2>&1; then
        print_success "Scanner status endpoint working"
    else
        print_error "Scanner status endpoint failed"
    fi
    
    print_info "Testing scan jobs list endpoint..."
    if curl -s "${BASE_URL}/scanner/jobs" | jq . > /dev/null 2>&1; then
        print_success "Scan jobs list endpoint working"
    else
        print_error "Scan jobs list endpoint failed"
    fi
}

# Helper functions for scan job management
get_scan_status() {
    local job_id=$1
    curl -s "${BASE_URL}/scanner/jobs/${job_id}" 2>/dev/null
}

get_job_field() {
    local job_id=$1
    local field=$2
    echo $(get_scan_status $job_id) | jq -r ".${field}" 2>/dev/null || echo "0"
}

wait_for_status() {
    local job_id=$1
    local expected_status=$2
    local timeout=$3
    local start_time=$(date +%s)
    
    print_info "Waiting for job $job_id to reach status: $expected_status (timeout: ${timeout}s)"
    
    while true; do
        local current_time=$(date +%s)
        local elapsed=$((current_time - start_time))
        
        if [ $elapsed -ge $timeout ]; then
            print_error "Timeout waiting for status $expected_status"
            return 1
        fi
        
        local status=$(get_job_field $job_id "status")
        echo "  Current status: $status (elapsed: ${elapsed}s)"
        
        if [ "$status" = "$expected_status" ]; then
            print_success "Job $job_id reached status $expected_status after ${elapsed}s"
            return 0
        fi
        
        sleep 2
    done
}

# Test pause/resume functionality
test_pause_resume() {
    local library_id=$1
    
    print_header "Testing Pause/Resume Functionality"
    
    # Start a new scan
    print_info "Starting new scan for library $library_id..."
    local start_response=$(curl -s -X POST "${BASE_URL}/scanner/scan" -H "Content-Type: application/json" -d "{\"library_id\": ${library_id}}")
    local job_id=$(echo $start_response | jq -r '.scan_job.id' 2>/dev/null)
    
    if [ -z "$job_id" ] || [ "$job_id" = "null" ]; then
        print_error "Failed to start scan job"
        echo "Response: $start_response"
        return 1
    fi
    
    print_success "Started scan job with ID: $job_id"
    
    # Wait for job to start running
    if ! wait_for_status $job_id "running" 30; then
        print_error "Job failed to start running"
        return 1
    fi
    
    # Wait for some progress
    print_info "Waiting for initial progress (15 seconds)..."
    sleep 15
    
    local files_before_pause=$(get_job_field $job_id "files_processed")
    local files_found=$(get_job_field $job_id "files_found")
    local progress_before=$(get_job_field $job_id "progress")
    
    print_info "Progress before pause:"
    echo "  Files processed: $files_before_pause"
    echo "  Files found: $files_found"
    echo "  Progress: $progress_before%"
    
    # Pause the scan
    print_info "Pausing scan job $job_id..."
    local pause_response=$(curl -s -X DELETE "${BASE_URL}/scanner/jobs/${job_id}")
    
    if ! wait_for_status $job_id "paused" 20; then
        print_error "Job failed to pause"
        return 1
    fi
    
    local files_after_pause=$(get_job_field $job_id "files_processed")
    
    # Verify progress was preserved
    if [ "$files_before_pause" = "$files_after_pause" ]; then
        print_success "Files processed count preserved during pause ($files_after_pause files)"
    else
        print_error "Files processed count changed during pause: $files_before_pause -> $files_after_pause"
    fi
    
    # Resume the scan
    print_info "Resuming scan job $job_id..."
    local resume_response=$(curl -s -X POST "${BASE_URL}/scanner/resume/${job_id}")
    
    if ! wait_for_status $job_id "running" 20; then
        print_error "Job failed to resume"
        return 1
    fi
    
    local files_after_resume=$(get_job_field $job_id "files_processed")
    
    # Verify progress was preserved during resume
    if [ "$files_after_pause" = "$files_after_resume" ]; then
        print_success "Files processed count preserved during resume ($files_after_resume files)"
    else
        print_error "Files processed count changed during resume: $files_after_pause -> $files_after_resume"
    fi
    
    # Wait for continued progress
    print_info "Monitoring for continued progress (15 seconds)..."
    sleep 15
    
    local files_later=$(get_job_field $job_id "files_processed")
    local progress_later=$(get_job_field $job_id "progress")
    
    print_info "Progress after resume:"
    echo "  Files processed: $files_later"
    echo "  Progress: $progress_later%"
    
    # Verify new progress is being made
    if [ "$files_later" -gt "$files_after_resume" ]; then
        print_success "Scan making progress after resume! ($files_after_resume → $files_later files)"
    else
        print_warning "Scan may not be making progress after resume ($files_after_resume → $files_later files)"
    fi
    
    # Pause the job again to clean up
    print_info "Pausing job for cleanup..."
    curl -s -X DELETE "${BASE_URL}/scanner/jobs/${job_id}" > /dev/null
    
    print_success "Pause/Resume test completed!"
    return 0
}

# Test existing job resume
test_existing_job_resume() {
    local job_id=$1
    
    print_header "Testing Resume of Existing Job"
    
    print_info "Checking status of job $job_id..."
    local status=$(get_job_field $job_id "status")
    local files_before=$(get_job_field $job_id "files_processed")
    
    print_info "Current job status: $status"
    print_info "Files processed: $files_before"
    
    if [ "$status" != "paused" ]; then
        print_warning "Job is not paused, attempting to pause first..."
        curl -s -X DELETE "${BASE_URL}/scanner/jobs/${job_id}" > /dev/null
        sleep 3
    fi
    
    # Resume the job
    print_info "Resuming job $job_id..."
    local resume_response=$(curl -s -X POST "${BASE_URL}/scanner/resume/${job_id}")
    
    if ! wait_for_status $job_id "running" 20; then
        print_error "Job failed to resume"
        return 1
    fi
    
    # Monitor progress
    print_info "Monitoring progress for 10 seconds..."
    sleep 10
    
    local files_after=$(get_job_field $job_id "files_processed")
    local progress=$(get_job_field $job_id "progress")
    
    print_info "Progress after resume:"
    echo "  Files processed: $files_after"
    echo "  Progress: $progress%"
    
    if [ "$files_after" -gt "$files_before" ]; then
        print_success "Job resumed successfully and is making progress!"
    else
        print_warning "Job resumed but may not be making progress"
    fi
    
    return 0
}

# Show usage
show_usage() {
    echo "Usage: $0 [command] [options]"
    echo ""
    echo "Commands:"
    echo "  endpoints                    - Test all scanner endpoints"
    echo "  pause-resume <library_id>    - Test pause/resume with new scan"
    echo "  resume <job_id>              - Test resume of existing job"
    echo "  full <library_id>            - Run all tests"
    echo ""
    echo "Examples:"
    echo "  $0 endpoints"
    echo "  $0 pause-resume 19"
    echo "  $0 resume 56"
    echo "  $0 full 19"
}

# Main script
main() {
    print_header "Viewra Media Scanner Test Suite"
    
    if ! check_server; then
        exit 1
    fi
    
    case "${1:-}" in
        "endpoints")
            test_endpoints
            ;;
        "pause-resume")
            if [ -z "$2" ]; then
                print_error "Library ID required for pause-resume test"
                show_usage
                exit 1
            fi
            test_pause_resume "$2"
            ;;
        "resume")
            if [ -z "$2" ]; then
                print_error "Job ID required for resume test"
                show_usage
                exit 1
            fi
            test_existing_job_resume "$2"
            ;;
        "full")
            if [ -z "$2" ]; then
                print_error "Library ID required for full test"
                show_usage
                exit 1
            fi
            test_endpoints
            test_pause_resume "$2"
            ;;
        *)
            show_usage
            exit 1
            ;;
    esac
    
    print_header "Test Suite Complete"
}

# Check dependencies
if ! command -v jq &> /dev/null; then
    print_error "jq is required but not installed. Please install jq first."
    exit 1
fi

if ! command -v curl &> /dev/null; then
    print_error "curl is required but not installed. Please install curl first."
    exit 1
fi

# Run main function
main "$@" 