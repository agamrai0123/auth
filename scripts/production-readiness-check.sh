#!/bin/bash

# Production Readiness Checklist for Auth Service
# Tests security, performance, error handling, and configuration

set -e

BASE_URL="https://localhost:8443"
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Counters
PASS=0
FAIL=0
WARN=0

# Helper functions
result() {
    if [ $1 -eq 0 ]; then
        echo -e "${GREEN}✓ PASS${NC}: $2"
        ((PASS++))
    else
        echo -e "${RED}✗ FAIL${NC}: $2"
        ((FAIL++))
    fi
}

warning() {
    echo -e "${YELLOW}⚠ WARN${NC}: $1"
    ((WARN++))
}

section() {
    echo -e "\n${BLUE}=== $1 ===${NC}"
}

# ============================================================================
# 1. CONFIGURATION & ENVIRONMENT CHECKS
# ============================================================================
section "Configuration & Environment Checks"

# Check JWT_SECRET
if [ -z "$JWT_SECRET" ]; then
    echo -e "${YELLOW}ℹ INFO${NC}: JWT_SECRET not loaded in current shell (expected - loaded in service)"
else
    if [ ${#JWT_SECRET} -ge 32 ]; then
        result 0 "JWT_SECRET length meets minimum (${#JWT_SECRET} chars)"
    else
        result 1 "JWT_SECRET too short (${#JWT_SECRET} < 32 chars)"
    fi
fi

# Check TLS certificates
if [ -f "certs/server.crt" ] && [ -f "certs/server.key" ]; then
    result 0 "TLS certificates present"
else
    result 1 "TLS certificates missing"
fi

# Check configuration file
if [ -f "config/auth-server-config.json" ]; then
    if grep -q '"global_rps"' config/auth-server-config.json; then
        result 0 "Rate limiting configuration present"
    else
        result 1 "Rate limiting configuration missing"
    fi
else
    result 1 "Configuration file missing"
fi

# ============================================================================
# 2. SECURITY CHECKS
# ============================================================================
section "Security Checks"

# Check HTTPS is available
status_code=$(curl -sk -o /dev/null -w "%{http_code}" "$BASE_URL/auth-server/v1/oauth/" 2>/dev/null)
if [ "$status_code" -eq 200 ]; then
    result 0 "HTTPS endpoint accessible"
else
    result 1 "HTTPS endpoint returned $status_code"
fi

# Check HTTP redirect (if applicable)
status_code=$(curl -si http://localhost:8080/auth-server/v1/oauth/ 2>/dev/null | head -1 | grep -o '[0-9]\{3\}' | head -1)
if [ -n "$status_code" ]; then
    if [[ "$status_code" =~ ^(301|302|303|307|308)$ ]]; then
        result 0 "HTTP redirects to HTTPS (status: $status_code)"
    elif [[ "$status_code" =~ ^(200|404)$ ]]; then
        warning "HTTP not redirecting to HTTPS (status: $status_code)"
    fi
else
    echo -e "${YELLOW}ℹ INFO${NC}: HTTP redirect test skipped (HTTP server may be disabled)"
fi

# Test missing Authorization header
response=$(curl -sk -X POST "$BASE_URL/auth-server/v1/oauth/token" \
    -H "Content-Type: application/json" \
    -d '{"client_id":"test","client_secret":"test","grant_type":"client_credentials"}' \
    2>/dev/null | grep -o error | head -1)
if [ "$response" = "error" ]; then
    result 0 "Invalid requests return error responses"
else
    result 1 "Invalid requests not properly handled"
fi

# Test invalid grant type
response=$(curl -sk -X POST "$BASE_URL/auth-server/v1/oauth/token" \
    -H "Content-Type: application/json" \
    -d '{"client_id":"test-client","client_secret":"test-secret-123","grant_type":"invalid"}' \
    2>/dev/null)
if echo "$response" | grep -q "error"; then
    result 0 "Invalid grant type rejected"
else
    result 1 "Invalid grant type not properly rejected"
fi

# Test valid token generation
response=$(curl -sk -X POST "$BASE_URL/auth-server/v1/oauth/token" \
    -H "Content-Type: application/json" \
    -d '{"client_id":"test-client","client_secret":"test-secret-123","grant_type":"client_credentials"}' \
    2>/dev/null)
if echo "$response" | grep -q "access_token"; then
    TOKEN=$(echo "$response" | grep -o '"access_token":"[^"]*"' | cut -d'"' -f4)
    result 0 "Valid token generated"
else
    result 1 "Token generation failed"
    TOKEN=""
fi

# Test validate endpoint with valid token
if [ -n "$TOKEN" ]; then
    status=$(curl -sk -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/auth-server/v1/oauth/validate" \
        -H "Authorization: Bearer $TOKEN" \
        -H "Content-Type: application/json" \
        -d '{"endpoint_url":"http://test.com/api"}' \
        2>/dev/null)
    if [ "$status" -eq 200 ]; then
        result 0 "Valid token accepted by validate endpoint"
    else
        result 1 "Valid token rejected (status: $status)"
    fi
else
    warning "Skipping validate test (no token generated)"
fi

# Test invalid token rejection
status=$(curl -sk -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/auth-server/v1/oauth/validate" \
    -H "Authorization: Bearer invalid.token.here" \
    -H "Content-Type: application/json" \
    -d '{"endpoint_url":"http://test.com/api"}' \
    2>/dev/null)
if [ "$status" -eq 401 ] || [ "$status" -eq 400 ]; then
    result 0 "Invalid token rejected (status: $status)"
else
    result 1 "Invalid token not properly rejected (status: $status)"
fi

# Test malformed JSON handling
response=$(curl -sk -X POST "$BASE_URL/auth-server/v1/oauth/token" \
    -H "Content-Type: application/json" \
    -d '{invalid json}' \
    2>/dev/null)
if echo "$response" | grep -q "error" || echo "$response" | grep -q "Invalid"; then
    result 0 "Malformed JSON handled gracefully"
else
    result 1 "Malformed JSON not handled"
fi

# ============================================================================
# 3. PERFORMANCE CHECKS
# ============================================================================
section "Performance Checks"

# Health endpoint performance (baseline)
echo -n "Testing health endpoint RPS (20 concurrent, 100 requests)... "
if command -v hey &> /dev/null; then
    health_rps=$(hey -n 100 -c 20 -q "$BASE_URL/auth-server/v1/oauth/" 2>/dev/null | grep "Requests/sec" | awk '{print $2}')
    if (( $(echo "$health_rps > 1000" | bc -l) )); then
        result 0 "Health endpoint RPS: $health_rps"
    else
        warning "Health endpoint RPS low: $health_rps (target: >1000)"
    fi
else
    echo -e "${YELLOW}⚠ WARN${NC}: 'hey' not installed, skipping RPS test"
fi

# Connection reuse test (HTTP/2)
response=$(curl -sk -I "$BASE_URL/auth-server/v1/oauth/" 2>/dev/null)
if echo "$response" | grep -qi "HTTP/2"; then
    result 0 "HTTP/2 support enabled"
elif echo "$response" | grep -qi "HTTP/1.1"; then
    warning "Using HTTP/1.1 instead of HTTP/2"
else
    result 1 "HTTP version unknown"
fi

# ============================================================================
# 4. ERROR HANDLING CHECKS
# ============================================================================
section "Error Handling Checks"

# Test 404 on invalid endpoint
status=$(curl -sk -o /dev/null -w "%{http_code}" "$BASE_URL/invalid/endpoint" 2>/dev/null)
if [ "$status" -eq 404 ]; then
    result 0 "404 errors properly returned"
else
    result 1 "404 not properly returned (got $status)"
fi

# Test method not allowed
status=$(curl -sk -o /dev/null -w "%{http_code}" -X DELETE "$BASE_URL/auth-server/v1/oauth/" 2>/dev/null)
if [ "$status" -eq 405 ] || [ "$status" -eq 404 ]; then
    result 0 "Unsupported HTTP methods rejected (status: $status)"
else
    result 1 "Unsupported methods not properly rejected (status: $status)"
fi

# Test empty request body
response=$(curl -sk -X POST "$BASE_URL/auth-server/v1/oauth/token" \
    -H "Content-Type: application/json" \
    -d '' \
    2>/dev/null)
if echo "$response" | grep -q "error" || [ -z "$response" ]; then
    result 0 "Empty request body handled"
else
    result 1 "Empty request body not handled"
fi

# Test missing required fields
response=$(curl -sk -X POST "$BASE_URL/auth-server/v1/oauth/token" \
    -H "Content-Type: application/json" \
    -d '{"client_id":"test"}' \
    2>/dev/null)
if echo "$response" | grep -q "error"; then
    result 0 "Missing required fields validation works"
else
    result 1 "Missing required fields not validated"
fi

# ============================================================================
# 5. LOGGING & MONITORING CHECKS
# ============================================================================
section "Logging & Monitoring Checks"

# Check log file exists
if [ -f "log/auth-server.log" ]; then
    result 0 "Log file created"
    
    # Check for error logs count
    error_count=$(grep -c '"level":"error"' log/auth-server.log 2>/dev/null || echo 0)
    if [ "$error_count" -lt 100 ]; then
        result 0 "Error log count reasonable: $error_count errors"
    else
        warning "High error count in logs: $error_count"
    fi
    
    # Check for debug logs (many is OK for operational visibility)
    info_count=$(grep -c '"level":"info"' log/auth-server.log 2>/dev/null || echo 0)
    if [ "$info_count" -gt 0 ]; then
        result 0 "Info logs present for monitoring"
    fi
    
    # Check for sensitive data in logs (client secrets should NOT be logged)
    if grep -q "test-secret-123" log/auth-server.log 2>/dev/null; then
        result 1 "SECURITY: Secrets found in logs!"
    else
        result 0 "No secrets exposed in logs"
    fi
else
    result 1 "Log file not created"
fi

# Check metrics endpoint
status=$(curl -sk -o /dev/null -w "%{http_code}" "$BASE_URL/metrics" 2>/dev/null)
if [ "$status" -eq 200 ]; then
    result 0 "Prometheus metrics endpoint available"
else
    result 1 "Metrics endpoint not accessible (status: $status)"
fi

# Check pprof endpoints (profiling)
status=$(curl -sk -o /dev/null -w "%{http_code}" "$BASE_URL/debug/pprof/" 2>/dev/null)
if [ "$status" -eq 200 ]; then
    warning "pprof profiling endpoints exposed (should be disabled in production)"
else
    result 0 "pprof endpoints properly disabled or restricted"
fi

# ============================================================================
# 6. ENDPOINT AVAILABILITY CHECKS
# ============================================================================
section "Endpoint Availability Checks"

# Check all critical endpoints
endpoints=(
    "/auth-server/v1/oauth/"
    "/auth-server/v1/oauth/token"
    "/auth-server/v1/oauth/validate"
    "/auth-server/v1/oauth/revoke"
    "/health"
    "/metrics"
)

for endpoint in "${endpoints[@]}"; do
    status=$(curl -sk -o /dev/null -w "%{http_code}" "$BASE_URL$endpoint" 2>/dev/null)
    if [ "$status" -eq 200 ] || [ "$status" -eq 400 ] || [ "$status" -eq 401 ] || [ "$status" -eq 404 ]; then
        result 0 "Endpoint $endpoint accessible"
    else
        result 1 "Endpoint $endpoint returned $status"
    fi
done

# ============================================================================
# SUMMARY
# ============================================================================
section "Production Readiness Summary"

TOTAL=$((PASS + FAIL + WARN))
echo -e "\n${BLUE}Results:${NC}"
echo -e "  ${GREEN}✓ Passed: $PASS${NC}"
echo -e "  ${RED}✗ Failed: $FAIL${NC}"
echo -e "  ${YELLOW}⚠ Warnings: $WARN${NC}"
echo -e "  Total: $TOTAL\n"

if [ $FAIL -eq 0 ]; then
    echo -e "${GREEN}✓ Service is ready for production${NC}"
    exit 0
elif [ $FAIL -lt 3 ]; then
    echo -e "${YELLOW}⚠ Service has minor issues to address${NC}"
    exit 1
else
    echo -e "${RED}✗ Service has critical issues - NOT ready for production${NC}"
    exit 1
fi
