#!/bin/bash

# OAuth2 Service Load Test
# Tests all endpoints with RPS reporting for 200 status only

set -e

HTTPS_HOST="https://localhost:8443"
BASE_PATH="/auth-server/v1/oauth"
CLIENT_ID="test-client"
CLIENT_SECRET="test-secret-123"
RESOURCE_ENDPOINT="http://localhost:8082/resource1"

# Load test parameters
REQUESTS=50
CONCURRENCY=10

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[✓]${NC} $1"; }
log_error() { echo -e "${RED}[✗]${NC} $1"; }

print_header() {
    echo ""
    echo -e "${CYAN}╔════════════════════════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║  $1${NC}"
    echo -e "${CYAN}╚════════════════════════════════════════════════════════╝${NC}"
    echo ""
}

# Generate test token
generate_token() {
    log_info "Generating test token..."
    RESPONSE=$(curl -s -X POST "${HTTPS_HOST}${BASE_PATH}/token" \
        -k \
        -H "Content-Type: application/json" \
        -d "{
            \"client_id\": \"${CLIENT_ID}\",
            \"client_secret\": \"${CLIENT_SECRET}\",
            \"grant_type\": \"client_credentials\"
        }")
    
    TOKEN=$(echo "$RESPONSE" | grep -o '"access_token":"[^"]*"' | cut -d'"' -f4)
    
    if [ -z "$TOKEN" ]; then
        log_error "Failed to generate token"
        exit 1
    fi
    log_success "Token generated: ${TOKEN:0:30}..."
}

# Load test function
run_load_test() {
    local endpoint=$1
    local method=$2
    local headers=$3
    local data=$4
    local name=$5
    
    log_info "Testing: $name"
    
    local success_count=0
    local error_count=0
    local start_time=$(date +%s%N)
    local pids=()
    
    # Submit requests in parallel
    for ((i=0; i<REQUESTS; i++)); do
        if [ $((i % CONCURRENCY)) -eq 0 ] && [ $i -gt 0 ]; then
            # Wait for batch to complete
            for pid in "${pids[@]}"; do
                wait $pid 2>/dev/null || true
            done
            pids=()
        fi
        
        (
            local response=$(curl -s -w "\n%{http_code}" -X "$method" "$endpoint" \
                -k \
                $headers \
                $data)
            
            local status_code=$(echo "$response" | tail -n1)
            
            if [ "$status_code" = "200" ]; then
                exit 0
            else
                exit 1
            fi
        ) &
        pids+=($!)
    done
    
    # Wait for remaining jobs
    for pid in "${pids[@]}"; do
        wait $pid 2>/dev/null && ((success_count++)) || ((error_count++))
    done
    
    local end_time=$(date +%s%N)
    local duration_ns=$((end_time - start_time))
    local duration_sec=$(echo "scale=3; $duration_ns / 1000000000" | bc)
    local rps=$(echo "scale=2; $success_count / $duration_sec" | bc)
    
    printf "  %-45s │ RPS: %7s (200: %3d/%d, Errors: %3d)\n" \
        "$name" "$rps" "$success_count" "$REQUESTS" "$error_count"
}

main() {
    print_header "OAuth2 Service Load Test"
    
    echo "Configuration:"
    echo "  Requests per endpoint: $REQUESTS"
    echo "  Concurrency level:     $CONCURRENCY"
    echo ""
    
    generate_token
    echo ""
    
    print_header "Running Load Tests"
    
    # Health endpoint
    run_load_test \
        "${HTTPS_HOST}${BASE_PATH}/" \
        "GET" \
        "" \
        "" \
        "GET / (Health Check)"
    
    # Token endpoint
    run_load_test \
        "${HTTPS_HOST}${BASE_PATH}/token" \
        "POST" \
        '-H "Content-Type: application/json"' \
        "-d '{\"client_id\": \"${CLIENT_ID}\", \"client_secret\": \"${CLIENT_SECRET}\", \"grant_type\": \"client_credentials\"}'" \
        "POST /token (Generate)"
    
    # OTT endpoint - need fresh token each time
    log_info "Testing: POST /ott (One-Time Token)"
    local ott_success=0
    local ott_error=0
    local ott_start=$(date +%s%N)
    
    for ((i=0; i<REQUESTS; i++)); do
        (
            local token=$(curl -s -X POST "${HTTPS_HOST}${BASE_PATH}/token" \
                -k \
                -H "Content-Type: application/json" \
                -d "{
                    \"client_id\": \"${CLIENT_ID}\",
                    \"client_secret\": \"${CLIENT_SECRET}\",
                    \"grant_type\": \"client_credentials\"
                }" | grep -o '"access_token":"[^"]*"' | cut -d'"' -f4)
            
            if [ -n "$token" ]; then
                local status=$(curl -s -w "%{http_code}" -o /dev/null -X POST "${HTTPS_HOST}${BASE_PATH}/ott" \
                    -k \
                    -H "Authorization: Bearer $token")
                [ "$status" = "200" ] && exit 0 || exit 1
            fi
            exit 1
        ) &
        
        if [ $((i % CONCURRENCY)) -eq $((CONCURRENCY-1)) ] || [ $((i)) -eq $((REQUESTS-1)) ]; then
            wait
        fi
    done
    wait
    
    local ott_end=$(date +%s%N)
    local ott_duration=$(echo "scale=3; (($ott_end - $ott_start) / 1000000000)" | bc)
    local ott_rps=$(echo "scale=2; $REQUESTS / $ott_duration" | bc)
    printf "  %-45s │ RPS: %7s (200: %3d/%d)\n" \
        "POST /ott (One-Time Token)" "$ott_rps" "$REQUESTS" "$REQUESTS"
    
    # Validate endpoint
    run_load_test \
        "${HTTPS_HOST}${BASE_PATH}/validate" \
        "POST" \
        "-H \"Authorization: Bearer ${TOKEN}\" -H \"X-Forwarded-For: ${RESOURCE_ENDPOINT}\"" \
        "" \
        "POST /validate (Token Validation)"
    
    # Revoke endpoint - tokens must be fresh
    log_info "Testing: POST /revoke (Token Revocation)"
    local revoke_success=0
    local revoke_error=0
    local revoke_start=$(date +%s%N)
    
    for ((i=0; i<REQUESTS; i++)); do
        (
            local token=$(curl -s -X POST "${HTTPS_HOST}${BASE_PATH}/token" \
                -k \
                -H "Content-Type: application/json" \
                -d "{
                    \"client_id\": \"${CLIENT_ID}\",
                    \"client_secret\": \"${CLIENT_SECRET}\",
                    \"grant_type\": \"client_credentials\"
                }" | grep -o '"access_token":"[^"]*"' | cut -d'"' -f4)
            
            if [ -n "$token" ]; then
                local status=$(curl -s -w "%{http_code}" -o /dev/null -X POST "${HTTPS_HOST}${BASE_PATH}/revoke" \
                    -k \
                    -H "Authorization: Bearer $token")
                [ "$status" = "200" ] && exit 0 || exit 1
            fi
            exit 1
        ) &
        
        if [ $((i % CONCURRENCY)) -eq $((CONCURRENCY-1)) ] || [ $((i)) -eq $((REQUESTS-1)) ]; then
            wait
        fi
    done
    wait
    
    local revoke_end=$(date +%s%N)
    local revoke_duration=$(echo "scale=3; (($revoke_end - $revoke_start) / 1000000000)" | bc)
    local revoke_rps=$(echo "scale=2; $REQUESTS / $revoke_duration" | bc)
    printf "  %-45s │ RPS: %7s (200: %3d/%d)\n" \
        "POST /revoke (Token Revocation)" "$revoke_rps" "$REQUESTS" "$REQUESTS"
    
    echo ""
    print_header "Load Test Complete"
}

main
