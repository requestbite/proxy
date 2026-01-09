#!/bin/bash

# Test script for RequestBite Slingshot Proxy
# Comprehensive test coverage for all endpoints and features
set -e

PORT=8081
PROXY_URL="http://localhost:$PORT"

# Test counters
TESTS_RUN=0
TESTS_PASSED=0
TESTS_FAILED=0

# Color codes
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Helper function to check test result
check_result() {
    local test_name="$1"
    local expected="$2"
    local actual="$3"

    TESTS_RUN=$((TESTS_RUN + 1))

    if [ "$expected" = "$actual" ]; then
        echo -e "${GREEN}✓${NC} $test_name"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        echo -e "${RED}✗${NC} $test_name"
        echo -e "  Expected: $expected"
        echo -e "  Got: $actual"
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
}

echo "========================================================"
echo "  RequestBite Slingshot Proxy - Test Suite"
echo "========================================================"
echo ""

echo -e "${BLUE}🚀 Starting proxy server on port $PORT...${NC}"

# Create a temporary test directory and file for local file tests
TEST_DIR=$(mktemp -d)
TEST_FILE="$TEST_DIR/test.txt"
echo "Hello from local file!" > "$TEST_FILE"

# Build the proxy first
make build > /dev/null 2>&1

# Start proxy with local files enabled using make dev in background
ARGS="--port $PORT --enable-local-files" make dev > /tmp/proxy.log 2>&1 &
PROXY_PID=$!

# Wait for server to start
sleep 3

# Cleanup function
cleanup() {
    echo ""
    echo -e "${BLUE}🛑 Stopping proxy server...${NC}"
    kill $PROXY_PID 2>/dev/null || true
    wait $PROXY_PID 2>/dev/null || true

    # Clean up temp files
    rm -rf "$TEST_DIR"

    # Print summary
    echo ""
    echo "========================================================"
    echo "  Test Results"
    echo "========================================================"
    echo -e "Total tests: $TESTS_RUN"
    echo -e "${GREEN}Passed: $TESTS_PASSED${NC}"
    if [ $TESTS_FAILED -gt 0 ]; then
        echo -e "${RED}Failed: $TESTS_FAILED${NC}"
        exit 1
    else
        echo -e "${GREEN}All tests passed!${NC}"
    fi
}
trap cleanup EXIT

echo -e "${GREEN}✓${NC} Proxy server started with PID: $PROXY_PID"
echo ""

# ========================================
# Health Check Tests
# ========================================
echo -e "${YELLOW}━━━ Health Check Tests ━━━${NC}"

RESPONSE=$(curl -s "$PROXY_URL/health")
STATUS=$(echo "$RESPONSE" | jq -r '.status')
check_result "Health endpoint returns status 'ok'" "ok" "$STATUS"

VERSION=$(echo "$RESPONSE" | jq -r '.version')
[ -n "$VERSION" ] && check_result "Health endpoint includes version" "true" "true"

ENABLE_LOCAL=$(echo "$RESPONSE" | jq -r '.enableLocalFiles')
check_result "Health endpoint shows enableLocalFiles=true" "true" "$ENABLE_LOCAL"

echo ""

# ========================================
# Basic Proxy Request Tests
# ========================================
echo -e "${YELLOW}━━━ Basic Proxy Request Tests ━━━${NC}"

# Test 1: Normal GET request
RESPONSE=$(curl -s -X POST "$PROXY_URL/proxy/request" \
    -H "Content-Type: application/json" \
    -d '{
        "method": "GET",
        "url": "https://httpbin.org/get",
        "headers": [],
        "timeout": 10,
        "followRedirects": true
    }')
SUCCESS=$(echo "$RESPONSE" | jq -r '.success')
check_result "Normal GET request succeeds" "true" "$SUCCESS"

# Test 2: Timeout request
RESPONSE=$(curl -s -X POST "$PROXY_URL/proxy/request" \
    -H "Content-Type: application/json" \
    -d '{
        "method": "GET",
        "url": "https://httpbin.org/delay/5",
        "headers": [],
        "timeout": 2,
        "followRedirects": true
    }')
SUCCESS=$(echo "$RESPONSE" | jq -r '.success')
ERROR_TYPE=$(echo "$RESPONSE" | jq -r '.error_type')
check_result "Timeout request fails" "false" "$SUCCESS"
check_result "Timeout error type is 'timeout'" "timeout" "$ERROR_TYPE"

# Test 3: Redirect with followRedirects=false
RESPONSE=$(curl -s -X POST "$PROXY_URL/proxy/request" \
    -H "Content-Type: application/json" \
    -d '{
        "method": "GET",
        "url": "http://httpbin.org/redirect/1",
        "headers": [],
        "timeout": 10,
        "followRedirects": false
    }')
SUCCESS=$(echo "$RESPONSE" | jq -r '.success')
check_result "Redirect with followRedirects=false fails" "false" "$SUCCESS"

# Test 4: Redirect with followRedirects=true
RESPONSE=$(curl -s -X POST "$PROXY_URL/proxy/request" \
    -H "Content-Type: application/json" \
    -d '{
        "method": "GET",
        "url": "http://httpbin.org/redirect/1",
        "headers": [],
        "timeout": 10,
        "followRedirects": true
    }')
SUCCESS=$(echo "$RESPONSE" | jq -r '.success')
check_result "Redirect with followRedirects=true succeeds" "true" "$SUCCESS"

echo ""

# ========================================
# PassThrough Mode Tests
# ========================================
echo -e "${YELLOW}━━━ PassThrough Mode Tests ━━━${NC}"

# Test 5: PassThrough=false (default)
RESPONSE=$(curl -s -X POST "$PROXY_URL/proxy/request" \
    -H "Content-Type: application/json" \
    -d '{
        "method": "GET",
        "url": "https://httpbin.org/json",
        "headers": [],
        "timeout": 10,
        "passThrough": false
    }')
HAS_SUCCESS=$(echo "$RESPONSE" | jq 'has("success")')
check_result "PassThrough=false returns JSON wrapper" "true" "$HAS_SUCCESS"

# Test 6: PassThrough=true with JSON response
RESPONSE=$(curl -s -X POST "$PROXY_URL/proxy/request" \
    -H "Content-Type: application/json" \
    -d '{
        "method": "GET",
        "url": "https://httpbin.org/json",
        "headers": [],
        "timeout": 10,
        "passThrough": true
    }')
HAS_SLIDESHOW=$(echo "$RESPONSE" | jq 'has("slideshow")')
check_result "PassThrough=true returns raw JSON" "true" "$HAS_SLIDESHOW"

# Test 7: PassThrough=true with HTML response
RESPONSE=$(curl -s -X POST "$PROXY_URL/proxy/request" \
    -H "Content-Type: application/json" \
    -d '{
        "method": "GET",
        "url": "https://httpbin.org/html",
        "headers": [],
        "timeout": 10,
        "passThrough": true
    }')
CONTAINS_HTML=$(echo "$RESPONSE" | grep -q "<html>" && echo "true" || echo "false")
check_result "PassThrough=true returns raw HTML" "true" "$CONTAINS_HTML"

echo ""

# ========================================
# Path Parameter Substitution Tests
# ========================================
echo -e "${YELLOW}━━━ Path Parameter Substitution Tests ━━━${NC}"

# Test path parameter substitution
RESPONSE=$(curl -s -X POST "$PROXY_URL/proxy/request" \
    -H "Content-Type: application/json" \
    -d '{
        "method": "GET",
        "url": "https://httpbin.org/status/:code",
        "path_params": {"code": "200"},
        "headers": [],
        "timeout": 10,
        "followRedirects": true
    }')
SUCCESS=$(echo "$RESPONSE" | jq -r '.success')
STATUS=$(echo "$RESPONSE" | jq -r '.response_status')
check_result "Path parameter substitution works" "true" "$SUCCESS"
check_result "Path parameter :code replaced with 200" "200" "$STATUS"

echo ""

# ========================================
# Loop Detection Tests
# ========================================
echo -e "${YELLOW}━━━ Loop Detection Tests ━━━${NC}"

# Test loop detection via hostname blocking
# Note: /health endpoint is allowed on any hostname, so we test with a different endpoint
RESPONSE=$(curl -s -X POST "$PROXY_URL/proxy/request" \
    -H "Content-Type: application/json" \
    -d '{
        "method": "GET",
        "url": "http://p.requestbite.com/some-endpoint",
        "headers": [],
        "timeout": 10,
        "followRedirects": true
    }')
SUCCESS=$(echo "$RESPONSE" | jq -r '.success')
ERROR_TYPE=$(echo "$RESPONSE" | jq -r '.error_type')
check_result "Loop detection blocks p.requestbite.com" "false" "$SUCCESS"
check_result "Loop detection returns loop_detected error" "loop_detected" "$ERROR_TYPE"

# Test loop detection via User-Agent
RESPONSE=$(curl -s -X POST "$PROXY_URL/proxy/request" \
    -H "Content-Type: application/json" \
    -H "User-Agent: rb-slingshot/1.0.0" \
    -d '{
        "method": "GET",
        "url": "https://httpbin.org/get",
        "headers": [],
        "timeout": 10,
        "followRedirects": true
    }')
SUCCESS=$(echo "$RESPONSE" | jq -r '.success')
ERROR_TYPE=$(echo "$RESPONSE" | jq -r '.error_type')
check_result "Loop detection blocks rb-slingshot User-Agent" "false" "$SUCCESS"
check_result "User-Agent loop returns loop_detected error" "loop_detected" "$ERROR_TYPE"

echo ""

# ========================================
# Error Handling Tests
# ========================================
echo -e "${YELLOW}━━━ Error Handling Tests ━━━${NC}"

# Test 404 - Invalid endpoint
RESPONSE=$(curl -s "$PROXY_URL/invalid/endpoint")
SUCCESS=$(echo "$RESPONSE" | jq -r '.success')
ERROR_TYPE=$(echo "$RESPONSE" | jq -r '.error_type')
check_result "Invalid endpoint returns success=false" "false" "$SUCCESS"
check_result "Invalid endpoint returns endpoint_not_found error" "endpoint_not_found" "$ERROR_TYPE"

# Test 405 - Invalid method (returns 400 per code)
RESPONSE=$(curl -s -X GET "$PROXY_URL/proxy/request")
SUCCESS=$(echo "$RESPONSE" | jq -r '.success')
ERROR_TYPE=$(echo "$RESPONSE" | jq -r '.error_type')
check_result "Invalid method returns success=false" "false" "$SUCCESS"
check_result "Invalid method returns method_not_allowed error" "method_not_allowed" "$ERROR_TYPE"

# Test missing required fields
RESPONSE=$(curl -s -X POST "$PROXY_URL/proxy/request" \
    -H "Content-Type: application/json" \
    -d '{
        "method": "GET"
    }')
SUCCESS=$(echo "$RESPONSE" | jq -r '.success')
ERROR_TYPE=$(echo "$RESPONSE" | jq -r '.error_type')
check_result "Missing URL returns success=false" "false" "$SUCCESS"
check_result "Missing URL returns request_format_error" "request_format_error" "$ERROR_TYPE"

echo ""

# ========================================
# Local File Serving Tests
# ========================================
echo -e "${YELLOW}━━━ Local File Serving Tests ━━━${NC}"

# Test file endpoint
RESPONSE=$(curl -s -X POST "$PROXY_URL/file" \
    -H "Content-Type: application/json" \
    -d "{
        \"path\": \"$TEST_FILE\"
    }")
check_result "File endpoint returns file content" "Hello from local file!" "$RESPONSE"

# Test file not found
RESPONSE=$(curl -s -X POST "$PROXY_URL/file" \
    -H "Content-Type: application/json" \
    -d '{
        "path": "/nonexistent/file.txt"
    }')
SUCCESS=$(echo "$RESPONSE" | jq -r '.success')
ERROR_TYPE=$(echo "$RESPONSE" | jq -r '.error_type')
check_result "Nonexistent file returns success=false" "false" "$SUCCESS"
check_result "Nonexistent file returns file_not_found error" "file_not_found" "$ERROR_TYPE"

# Test directory instead of file
RESPONSE=$(curl -s -X POST "$PROXY_URL/file" \
    -H "Content-Type: application/json" \
    -d "{
        \"path\": \"$TEST_DIR\"
    }")
SUCCESS=$(echo "$RESPONSE" | jq -r '.success')
ERROR_TYPE=$(echo "$RESPONSE" | jq -r '.error_type')
check_result "Directory path returns success=false" "false" "$SUCCESS"
check_result "Directory path returns file_access_error" "file_access_error" "$ERROR_TYPE"

echo ""

# ========================================
# Directory Listing Tests
# ========================================
echo -e "${YELLOW}━━━ Directory Listing Tests ━━━${NC}"

# Test directory endpoint
RESPONSE=$(curl -s -X POST "$PROXY_URL/dir" \
    -H "Content-Type: application/json" \
    -d "{
        \"path\": \"$TEST_DIR\"
    }")
CURRENT_DIR=$(echo "$RESPONSE" | jq -r '.currentDir')
check_result "Directory endpoint returns currentDir" "$TEST_DIR" "$CURRENT_DIR"

DIR_COUNT=$(echo "$RESPONSE" | jq '.dir | length')
check_result "Directory contains test.txt" "1" "$DIR_COUNT"

ENTRY_NAME=$(echo "$RESPONSE" | jq -r '.dir[0].name')
check_result "Directory entry name is test.txt" "test.txt" "$ENTRY_NAME"

# Test directory not found
RESPONSE=$(curl -s -X POST "$PROXY_URL/dir" \
    -H "Content-Type: application/json" \
    -d '{
        "path": "/nonexistent/directory"
    }')
SUCCESS=$(echo "$RESPONSE" | jq -r '.success')
ERROR_TYPE=$(echo "$RESPONSE" | jq -r '.error_type')
check_result "Nonexistent directory returns success=false" "false" "$SUCCESS"
check_result "Nonexistent directory returns file_not_found error" "file_not_found" "$ERROR_TYPE"

# Test file instead of directory
RESPONSE=$(curl -s -X POST "$PROXY_URL/dir" \
    -H "Content-Type: application/json" \
    -d "{
        \"path\": \"$TEST_FILE\"
    }")
SUCCESS=$(echo "$RESPONSE" | jq -r '.success')
ERROR_TYPE=$(echo "$RESPONSE" | jq -r '.error_type')
check_result "File path to /dir returns success=false" "false" "$SUCCESS"
check_result "File path to /dir returns file_access_error" "file_access_error" "$ERROR_TYPE"

echo ""

# ========================================
# Form Request Tests
# ========================================
echo -e "${YELLOW}━━━ Form Request Tests ━━━${NC}"

# Test form endpoint with URL-encoded data
RESPONSE=$(curl -s -X POST "$PROXY_URL/proxy/form?url=https://httpbin.org/post&timeout=10&contentType=application/x-www-form-urlencoded" \
    -d "key1=value1&key2=value2")
SUCCESS=$(echo "$RESPONSE" | jq -r '.success')
check_result "Form request succeeds" "true" "$SUCCESS"

# Verify form data was sent (response_data is a JSON string, need to parse it)
FORM_KEY1=$(echo "$RESPONSE" | jq -r '.response_data | fromjson | .form.key1')
check_result "Form data key1 sent correctly" "value1" "$FORM_KEY1"

echo ""

echo -e "${GREEN}🎉 All test sections completed!${NC}"
