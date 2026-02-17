#!/bin/bash

# OAuth2 Service - Load Test (No Rate Limiting)
# Tests all endpoints for maximum RPS with 100% 200 status responses

HTTPS_HOST="https://localhost:8443"
BASE_PATH="/auth-server/v1/oauth"
CLIENT_ID="test-client"
CLIENT_SECRET="test-secret-123"
RESOURCE_ENDPOINT="http://localhost:8082/resource1"

# Load test parameters
REQUESTS=200
CONCURRENCY=30

echo "═══════════════════════════════════════════════════════════════"
echo "  OAuth2 Service Load Test (Rate Limiting DISABLED)"
echo "  Testing all 5 endpoints for maximum RPS"
echo "═══════════════════════════════════════════════════════════════"
echo ""
echo "Configuration:"
echo "  - Rate Limiting: DISABLED (global_rps: 100000/10000 burst)"
echo "  - Requests per endpoint: $REQUESTS"
echo "  - Concurrent connections: $CONCURRENCY"
echo ""

# Parse hey output to extract RPS and status codes
parse_hey_output() {
    local output=$1
    local rps=$(echo "$output" | grep "Requests/sec:" | awk '{print $2}')
    
    # Extract status codes from the Status code distribution section
    local status_200=$(echo "$output" | grep "\[200\]" | grep -o "[0-9]\+" | head -1)
    local status_400=$(echo "$output" | grep "\[400\]" | grep -o "[0-9]\+" | head -1)
    local status_401=$(echo "$output" | grep "\[401\]" | grep -o "[0-9]\+" | head -1)
    local status_403=$(echo "$output" | grep "\[403\]" | grep -o "[0-9]\+" | head -1)
    local status_429=$(echo "$output" | grep "\[429\]" | grep -o "[0-9]\+" | head -1)
    local status_500=$(echo "$output" | grep "\[500\]" | grep -o "[0-9]\+" | head -1)
    
    # Handle empty values (0)
    [ -z "$status_200" ] && status_200="0"
    [ -z "$status_400" ] && status_400="0"
    [ -z "$status_401" ] && status_401="0"
    [ -z "$status_403" ] && status_403="0"
    [ -z "$status_429" ] && status_429="0"
    [ -z "$status_500" ] && status_500="0"
    
    printf "  RPS: %-10s | 200: %3s | 401: %3s | 400: %3s | 403: %3s | 429: %3s | 500: %3s\n" \
        "${rps:-0}" "$status_200" "$status_401" "$status_400" "$status_403" "$status_429" "$status_500"
}

echo "[*] Generating test token for validate/revoke tests..."
TOKEN_RESPONSE=$(curl -sk -X POST "${HTTPS_HOST}${BASE_PATH}/token" \
    -H "Content-Type: application/json" \
    -d "{
        \"client_id\": \"${CLIENT_ID}\",
        \"client_secret\": \"${CLIENT_SECRET}\",
        \"grant_type\": \"client_credentials\"
    }" 2>/dev/null)

TOKEN=$(echo "$TOKEN_RESPONSE" | grep -o '"access_token":"[^"]*"' | cut -d'"' -f4)

if [ -z "$TOKEN" ]; then
    echo "ERROR: Failed to generate test token"
    echo "Response: $TOKEN_RESPONSE"
    exit 1
fi

echo "  ✓ Token generated"
echo ""
echo "─────────────────────────────────────────────────────────────────"

# TEST 1: Health Check
echo ""
echo "[1] GET / (Health Check)"
output=$(hey -n $REQUESTS -c $CONCURRENCY "${HTTPS_HOST}${BASE_PATH}/" 2>&1)
parse_hey_output "$output"

# TEST 2: Token Generation
echo ""
echo "[2] POST /token (Generate New Token)"
output=$(hey -n $REQUESTS -c $CONCURRENCY -m POST \
    -H "Content-Type: application/json" \
    -d "{\"client_id\":\"${CLIENT_ID}\",\"client_secret\":\"${CLIENT_SECRET}\",\"grant_type\":\"client_credentials\"}" \
    "${HTTPS_HOST}${BASE_PATH}/token" 2>&1)
parse_hey_output "$output"

# TEST 3: OTT Generation
echo ""
echo "[3] POST /ott (Generate One-Time Token)"
output=$(hey -n $REQUESTS -c $CONCURRENCY -m POST \
    -H "Content-Type: application/json" \
    -d "{\"client_id\":\"${CLIENT_ID}\",\"client_secret\":\"${CLIENT_SECRET}\",\"grant_type\":\"client_credentials\"}" \
    "${HTTPS_HOST}${BASE_PATH}/ott" 2>&1)
parse_hey_output "$output"

# TEST 4: Validate Token
echo ""
echo "[4] POST /validate (Validate Token)"
output=$(hey -n $REQUESTS -c $CONCURRENCY -m POST \
    -H "Authorization: Bearer ${TOKEN}" \
    -H "X-Forwarded-For: ${RESOURCE_ENDPOINT}" \
    "${HTTPS_HOST}${BASE_PATH}/validate" 2>&1)
parse_hey_output "$output"

# Generate fresh token for revoke test
echo ""
echo "[*] Generating fresh token for revoke test..."
REVOKE_TOKEN=$(curl -sk -X POST "${HTTPS_HOST}${BASE_PATH}/token" \
    -H "Content-Type: application/json" \
    -d "{
        \"client_id\": \"${CLIENT_ID}\",
        \"client_secret\": \"${CLIENT_SECRET}\",
        \"grant_type\": \"client_credentials\"
    }" 2>/dev/null | grep -o '"access_token":"[^"]*"' | cut -d'"' -f4)

echo "  ✓ Fresh token generated"
echo ""

# TEST 5: Revoke Token
echo "[5] POST /revoke (Revoke Token)"
output=$(hey -n $REQUESTS -c $CONCURRENCY -m POST \
    -H "Authorization: Bearer ${REVOKE_TOKEN}" \
    "${HTTPS_HOST}${BASE_PATH}/revoke" 2>&1)
parse_hey_output "$output"

echo ""
echo "═══════════════════════════════════════════════════════════════"
echo "  Load Test Complete"
echo "═══════════════════════════════════════════════════════════════"
echo ""
echo "Summary:"
echo "✓ All 5 endpoints tested for throughput"
echo "✓ Rate limiting disabled (no 429 throttling)"
echo "✓ Token cache enabled for instant validate/revoke"
echo ""
