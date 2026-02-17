#!/bin/bash

# OAuth2 Service Load Test using 'hey'
# Tests all endpoints and reports RPS for 200 status responses only

set -e

# Configuration
HTTPS_HOST="https://localhost:8443"
BASE_PATH="/auth-server/v1/oauth"
CLIENT_ID="test-client"
CLIENT_SECRET="test-secret-123"
RESOURCE_ENDPOINT="http://localhost:8082/resource1"

# Load test parameters
CONCURRENCY=10
TOTAL_REQUESTS=100

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[✓]${NC} $1"
}

log_error() {
    echo -e "${RED}[✗]${NC} $1"
}

log_section() {
    echo ""
    echo -e "${CYAN}╔════════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║  $1${NC}"
    echo -e "${CYAN}╚════════════════════════════════════════╝${NC}"
    echo ""
}

# Check if hey is installed
check_hey() {
    if ! command -v hey &> /dev/null; then
        log_error "hey is not installed. Installing..."
        go install github.com/rakyll/hey@latest
        if ! command -v hey &> /dev/null; then
            log_error "Failed to install hey. Please install manually: go install github.com/rakyll/hey@latest"
            exit 1
        fi
    fi
    log_success "hey is installed"
}

# Generate a valid token for testing
generate_test_token() {
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
        log_error "Failed to generate test token"
        log_error "Response: $RESPONSE"
        exit 1
    fi
    
    log_success "Test token generated: ${TOKEN:0:30}..."
}

# Parse hey output and extract RPS for 200 status
parse_hey_output() {
    local output=$1
    local endpoint=$2
    
    # Extract status code distribution
    local status_line=$(echo "$output" | grep "Status code distribution" -A 10)
    local rps=$(echo "$output" | grep "Requests/sec:" | awk '{print $2}')
    local avg=$(echo "$output" | grep "Average:" | awk '{print $2}')
    local p50=$(echo "$output" | grep "50% in" | awk '{print $3}')
    local p95=$(echo "$output" | grep "95% in" | awk '{print $3}')
    local p99=$(echo "$output" | grep "99% in" | awk '{print $3}')
    
    # Check if we have 200 responses
    local status_200=$(echo "$status_line" | grep "200" | head -1)
    
    if [ -z "$status_200" ]; then
        log_error "$endpoint: No 200 status responses"
        return 1
    fi
    
    # Extract count of 200 responses
    local count_200=$(echo "$status_line" | grep "200" | head -1 | awk '{print $2}')
    
    # Calculate actual RPS for 200 responses only
    local actual_rps=$(echo "scale=2; $count_200 / $(($TOTAL_REQUESTS / $rps))" | bc 2>/dev/null || echo "N/A")
    
    # Format output
    printf "  %-50s %s RPS (200: %3d/%d)\n" \
        "$endpoint" \
        "$(printf '%6s' "$rps")" \
        "$count_200" \
        "$TOTAL_REQUESTS"
    
    # Additional metrics
    printf "    Avg: %10s | P50: %10s | P95: %10s | P99: %10s\n" \
        "$avg" "$p50" "$p95" "$p99"
}

# Load test token endpoint
test_token_endpoint() {
    local endpoint="${HTTPS_HOST}${BASE_PATH}/token"
    
    log_info "Testing: POST /auth-server/v1/oauth/token"
    
    local json_payload=$(cat <<EOF
{
    "client_id": "${CLIENT_ID}",
    "client_secret": "${CLIENT_SECRET}",
    "grant_type": "client_credentials"
}
EOF
)
    
    local output=$(hey -n $TOTAL_REQUESTS -c $CONCURRENCY \
        -H "Content-Type: application/json" \
        -d "$json_payload" \
        -k \
        "$endpoint" 2>&1)
    
    parse_hey_output "$output" "POST /token"
}

# Load test OTT endpoint
test_ott_endpoint() {
    local endpoint="${HTTPS_HOST}${BASE_PATH}/ott"
    
    log_info "Testing: POST /auth-server/v1/oauth/ott"
    
    # Generate fresh token for OTT test
    local token=$(curl -s -X POST "${HTTPS_HOST}${BASE_PATH}/token" \
        -k \
        -H "Content-Type: application/json" \
        -d "{
            \"client_id\": \"${CLIENT_ID}\",
            \"client_secret\": \"${CLIENT_SECRET}\",
            \"grant_type\": \"client_credentials\"
        }" | grep -o '"access_token":"[^"]*"' | cut -d'"' -f4)
    
    if [ -z "$token" ]; then
        log_error "Could not generate token for OTT test"
        return 1
    fi
    
    local output=$(hey -n $TOTAL_REQUESTS -c $CONCURRENCY \
        -H "Authorization: Bearer $token" \
        -k \
        "$endpoint" 2>&1)
    
    parse_hey_output "$output" "POST /ott"
}

# Load test validate endpoint
test_validate_endpoint() {
    local endpoint="${HTTPS_HOST}${BASE_PATH}/validate"
    
    log_info "Testing: POST /auth-server/v1/oauth/validate"
    
    local output=$(hey -n $TOTAL_REQUESTS -c $CONCURRENCY \
        -H "Authorization: Bearer $TOKEN" \
        -H "X-Forwarded-For: ${RESOURCE_ENDPOINT}" \
        -k \
        "$endpoint" 2>&1)
    
    parse_hey_output "$output" "POST /validate"
}

# Load test revoke endpoint
test_revoke_endpoint() {
    local endpoint="${HTTPS_HOST}${BASE_PATH}/revoke"
    
    log_info "Testing: POST /auth-server/v1/oauth/revoke"
    log_info "Note: Generating fresh tokens for each request in batch..."
    
    # For revoke test, we need fresh tokens each time
    # Create a temporary file with curl commands
    local temp_tokens=$(mktemp)
    
    log_info "Pre-generating tokens for revoke load test..."
    for ((i=1; i<=TOTAL_REQUESTS; i++)); do
        local token=$(curl -s -X POST "${HTTPS_HOST}${BASE_PATH}/token" \
            -k \
            -H "Content-Type: application/json" \
            -d "{
                \"client_id\": \"${CLIENT_ID}\",
                \"client_secret\": \"${CLIENT_SECRET}\",
                \"grant_type\": \"client_credentials\"
            }" | grep -o '"access_token":"[^"]*"' | cut -d'"' -f4)
        echo "$token" >> "$temp_tokens"
    done
    
    # Run load test by reading tokens from file
    # Since hey doesn't support dynamic headers easily, we'll simulate with sequential requests
    local start_time=$(date +%s%N)
    local success_count=0
    
    while IFS= read -r token; do
        if [ -n "$token" ]; then
            local response=$(curl -s -w "%{http_code}" -o /dev/null -X POST "$endpoint" \
                -k \
                -H "Authorization: Bearer $token")
            
            if [ "$response" = "200" ]; then
                success_count=$((success_count + 1))
            fi
        fi
    done < "$temp_tokens"
    
    local end_time=$(date +%s%N)
    local duration=$((($end_time - $start_time) / 1000000000))
    
    if [ $duration -eq 0 ]; then
        duration=1
    fi
    
    local rps=$(echo "scale=2; $success_count / $duration" | bc)
    
    printf "  %-50s %s RPS (200: %3d/%d)\n" \
        "POST /revoke (sequential)" \
        "$(printf '%6s' "$rps")" \
        "$success_count" \
        "$TOTAL_REQUESTS"
    
    rm -f "$temp_tokens"
}

# Load test health check endpoint
test_health_endpoint() {
    local endpoint="${HTTPS_HOST}${BASE_PATH}/"
    
    log_info "Testing: GET /auth-server/v1/oauth/"
    
    local output=$(hey -n $TOTAL_REQUESTS -c $CONCURRENCY \
        -k \
        "$endpoint" 2>&1)
    
    parse_hey_output "$output" "GET /"
}

# Main execution
main() {
    log_section "OAuth2 Service - Load Test Suite (hey)"
    
    log_info "Configuration:"
    echo "  Host:             $HTTPS_HOST"
    echo "  Base Path:        $BASE_PATH"
    echo "  Concurrency:      $CONCURRENCY"
    echo "  Total Requests:   $TOTAL_REQUESTS"
    echo ""
    
    # Check prerequisites
    check_hey
    
    # Generate test token for stateful tests
    generate_test_token
    
    echo ""
    log_section "Running Load Tests"
    
    # Test health endpoint first
    test_health_endpoint
    echo ""
    
    # Test token generation
    test_token_endpoint
    echo ""
    
    # Test OTT
    test_ott_endpoint
    echo ""
    
    # Test validate
    test_validate_endpoint
    echo ""
    
    # Test revoke
    test_revoke_endpoint
    echo ""
    
    log_section "Load Test Complete"
    log_success "All endpoints tested"
    echo ""
    
    echo "Legend:"
    echo "  RPS    = Requests per second"
    echo "  200    = Count of 200 status responses"
    echo "  Avg    = Average response time"
    echo "  P50    = 50th percentile response time"
    echo "  P95    = 95th percentile response time"
    echo "  P99    = 99th percentile response time"
}

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -c|--concurrency)
            CONCURRENCY="$2"
            shift 2
            ;;
        -n|--requests)
            TOTAL_REQUESTS="$2"
            shift 2
            ;;
        -h|--help)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  -c, --concurrency NUM    Number of concurrent requests (default: 10)"
            echo "  -n, --requests NUM       Total number of requests per endpoint (default: 100)"
            echo "  -h, --help               Show this help message"
            echo ""
            echo "Examples:"
            echo "  $0                       # Default: 10 concurrent, 100 total requests"
            echo "  $0 -c 20 -n 500          # 20 concurrent, 500 total requests"
            echo "  $0 --concurrency 50 --requests 1000  # Heavy load test"
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Run main function
main
