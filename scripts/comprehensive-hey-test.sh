#!/bin/bash

# OAuth2 Service - Comprehensive Hey Load Test (No Rate Limiting)
# Tests all endpoints for 100% 200 status responses

HTTPS_HOST="https://localhost:8443"
BASE_PATH="/auth-server/v1/oauth"
CLIENT_ID="test-client"
CLIENT_SECRET="test-secret-123"
RESOURCE_ENDPOINT="http://localhost:8082/resource1"

REQUESTS=100
CONCURRENCY=20

echo "═══════════════════════════════════════════════════════════════"
echo "  OAuth2 Service Load Test (No Rate Limiting)"
echo "  100% 200 Status Code Responses"
echo "═══════════════════════════════════════════════════════════════"
echo ""
echo "Settings: $REQUESTS requests, $CONCURRENCY concurrent"
echo ""

# Helper function to extract data from hey output
parse_hey_output() {
    local output=$1
    local endpoint=$2
    
    local rps=$(echo "$output" | grep "Requests/sec" | awk '{print $2}')
    local status_200=$(echo "$output" | grep -m1 "Status code distribution:" -A 5 | grep "200" | awk '{print $2}' | tr -d '"')
    local status_401=$(echo "$output" | grep -m1 "Status code distribution:" -A 5 | grep "401" | awk '{print $2}' | tr -d '"')
    local status_400=$(echo "$output" | grep -m1 "Status code distribution:" -A 5 | grep "400" | awk '{print $2}' | tr -d '"')
    local status_500=$(echo "$output" | grep -m1 "Status code distribution:" -A 5 | grep "500" | awk '{print $2}' | tr -d '"')
    
    if [ -z "$status_200" ]; then status_200="0"; fi
    if [ -z "$status_401" ]; then status_401="0"; fi
    if [ -z "$status_400" ]; then status_400="0"; fi
    if [ -z "$status_500" ]; then status_500="0"; fi
    
    echo "RPS: ${rps:-N/A} | 200: $status_200, 401: $status_401, 400: $status_400, 500: $status_500"
    
    # Return error if not all 200
    if [ "$status_200" != "$REQUESTS" ] && [ "$status_200" -ne "$REQUESTS" ]; then
        return 1
    fi
    return 0
}

# TEST 1: Health Check
echo "[1] GET / (Health Check)"
output=$(hey -n $REQUESTS -c $CONCURRENCY "${HTTPS_HOST}${BASE_PATH}/" 2>&1)
parse_hey_output "$output" "Health"
echo ""

# Generate token for other tests
echo "[*] Generating test token..."
TOKEN=$(curl -s -X POST "${HTTPS_HOST}${BASE_PATH}/token" \
    -H "Content-Type: application/json" \
    -d "{
        \"client_id\": \"${CLIENT_ID}\",
        \"client_secret\": \"${CLIENT_SECRET}\",
        \"grant_type\": \"client_credentials\"
    }" | grep -o '"access_token":"[^"]*"' | cut -d'"' -f4)

echo "Token: ${TOKEN:0:30}..."
echo ""

# TEST 2: Token Generation
echo "[2] POST /token (Generate Token)"
output=$(hey -n $REQUESTS -c $CONCURRENCY -m POST \
    -H "Content-Type: application/json" \
    -d "{\"client_id\":\"${CLIENT_ID}\",\"client_secret\":\"${CLIENT_SECRET}\",\"grant_type\":\"client_credentials\"}" \
    "${HTTPS_HOST}${BASE_PATH}/token" 2>&1)
parse_hey_output "$output" "Token"
echo ""

# TEST 3: OTT Generation
echo "[3] POST /ott (One-Time Token)"
output=$(hey -n $REQUESTS -c $CONCURRENCY -m POST \
    -H "Content-Type: application/json" \
    -d "{\"client_id\":\"${CLIENT_ID}\",\"client_secret\":\"${CLIENT_SECRET}\",\"grant_type\":\"client_credentials\"}" \
    "${HTTPS_HOST}${BASE_PATH}/ott" 2>&1)
parse_hey_output "$output" "OTT"
echo ""

# TEST 4: Validate Token
echo "[4] POST /validate (Token Validation)"
output=$(hey -n $REQUESTS -c $CONCURRENCY -m POST \
    -H "Authorization: Bearer ${TOKEN}" \
    -H "X-Forwarded-For: ${RESOURCE_ENDPOINT}" \
    "${HTTPS_HOST}${BASE_PATH}/validate" 2>&1)
status_dist=$(echo "$output" | grep -A 5 "Status code distribution:")
result=$(parse_hey_output "$output" "Validate")
echo "$result"
echo "Full status dist:"
echo "$status_dist"
echo ""

# TEST 5: Revoke Token  
echo "[5] POST /revoke (Token Revocation)"
output=$(hey -n $REQUESTS -c $CONCURRENCY -m POST \
    -H "Authorization: Bearer ${TOKEN}" \
    "${HTTPS_HOST}${BASE_PATH}/revoke" 2>&1)
status_dist=$(echo "$output" | grep -A 5 "Status code distribution:")
result=$(parse_hey_output "$output" "Revoke")
echo "$result"
echo "Full status dist:"
echo "$status_dist"
echo ""

echo "═══════════════════════════════════════════════════════════════"
echo "  Load Test Complete"
echo "═══════════════════════════════════════════════════════════════"
