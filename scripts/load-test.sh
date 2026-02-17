#!/bin/bash

# Simple OAuth2 Load Test - RPS for 200 status only

HTTPS_HOST="https://localhost:8443"
BASE_PATH="/auth-server/v1/oauth"
CLIENT_ID="test-client"
CLIENT_SECRET="test-secret-123"
RESOURCE_ENDPOINT="http://localhost:8082/resource1"
REQUESTS=50

echo "════════════════════════════════════════════════════════"
echo "   OAuth2 Service Load Test - RPS for 200 Status Only"  
echo "════════════════════════════════════════════════════════"
echo ""

# Generate test token
echo "[*] Generating test token..."
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
    echo "[X] Failed to generate token"
    exit 1
fi
echo "[✓] Token generated: ${TOKEN:0:30}..."
echo ""

# Function to calculate RPS
calc_rps() {
    local success=$1
    local duration=$2
    awk "BEGIN {printf \"%.2f\", $success / $duration}"
}

# Test 1: Health Check
echo "[*] Testing: GET / (Health Check)"
start=$(date +%s%N)
success=0
for ((i=0; i<REQUESTS; i++)); do
    status=$(curl -s -w "%{http_code}" -o /dev/null -k "${HTTPS_HOST}${BASE_PATH}/")
    [ "$status" = "200" ] && ((success++))
done
end=$(date +%s%N)
duration=$(awk "BEGIN {printf \"%.3f\", ($end - $start) / 1000000000}")
rps=$(calc_rps $success $duration)
echo "    RPS: $rps (200: $success/$REQUESTS) [${duration}s]"
echo ""

# Test 2: Token Generation
echo "[*] Testing: POST /token (Generate Token)"
start=$(date +%s%N)
success=0
for ((i=0; i<REQUESTS; i++)); do
    status=$(curl -s -w "%{http_code}" -o /dev/null -X POST "${HTTPS_HOST}${BASE_PATH}/token" \
        -k \
        -H "Content-Type: application/json" \
        -d "{
            \"client_id\": \"${CLIENT_ID}\",
            \"client_secret\": \"${CLIENT_SECRET}\",
            \"grant_type\": \"client_credentials\"
        }")
    [ "$status" = "200" ] && ((success++))
done
end=$(date +%s%N)
duration=$(awk "BEGIN {printf \"%.3f\", ($end - $start) / 1000000000}")
rps=$(calc_rps $success $duration)
echo "    RPS: $rps (200: $success/$REQUESTS) [${duration}s]"
echo ""

# Test 3: OTT Endpoint
echo "[*] Testing: POST /ott (One-Time Token)"
start=$(date +%s%N)
success=0
for ((i=0; i<REQUESTS; i++)); do
    status=$(curl -s -w "%{http_code}" -o /dev/null -X POST "${HTTPS_HOST}${BASE_PATH}/ott" \
        -k \
        -H "Content-Type: application/json" \
        -d "{
            \"client_id\": \"${CLIENT_ID}\",
            \"client_secret\": \"${CLIENT_SECRET}\",
            \"grant_type\": \"client_credentials\"
        }")
    [ "$status" = "200" ] && ((success++))
done
end=$(date +%s%N)
duration=$(awk "BEGIN {printf \"%.3f\", ($end - $start) / 1000000000}")
rps=$(calc_rps $success $duration)
echo "    RPS: $rps (200: $success/$REQUESTS) [${duration}s]"
echo ""

# Test 4: Validate Token - Note: Requires authorization scope
echo "[*] Testing: POST /validate (Token Validation) *"
start=$(date +%s%N)
success=0
total_200=0
for ((i=0; i<REQUESTS; i++)); do
    status=$(curl -s -w "%{http_code}" -o /dev/null -X POST "${HTTPS_HOST}${BASE_PATH}/validate" \
        -k \
        -H "Authorization: Bearer ${TOKEN}" \
        -H "X-Forwarded-For: ${RESOURCE_ENDPOINT}")
    [ "$status" = "200" ] && ((success++))
    [ "$status" = "200" ] && ((total_200++))
done
end=$(date +%s%N)
duration=$(awk "BEGIN {printf \"%.3f\", ($end - $start) / 1000000000}")
rps=$(calc_rps $total_200 $duration)
echo "    RPS: $rps (200: $total_200/$REQUESTS) [${duration}s]"
echo "    * Requires specific endpoint authorization scope"
echo ""

# Test 5: Revoke Token
echo "[*] Testing: POST /revoke (Token Revocation)"
start=$(date +%s%N)
success=0
for ((i=0; i<REQUESTS; i++)); do
    # Generate fresh token for revoke test
    revoke_token=$(curl -s -X POST "${HTTPS_HOST}${BASE_PATH}/token" \
        -k \
        -H "Content-Type: application/json" \
        -d "{
            \"client_id\": \"${CLIENT_ID}\",
            \"client_secret\": \"${CLIENT_SECRET}\",
            \"grant_type\": \"client_credentials\"
        }" | grep -o '"access_token":"[^"]*"' | cut -d'"' -f4)
    
    if [ -n "$revoke_token" ]; then
        status=$(curl -s -w "%{http_code}" -o /dev/null -X POST "${HTTPS_HOST}${BASE_PATH}/revoke" \
            -k \
            -H "Authorization: Bearer $revoke_token")
        [ "$status" = "200" ] && ((success++))
    fi
done
end=$(date +%s%N)
duration=$(awk "BEGIN {printf \"%.3f\", ($end - $start) / 1000000000}")
rps=$(calc_rps $success $duration)
echo "    RPS: $rps (200: $success/$REQUESTS) [${duration}s]"
echo ""

echo "════════════════════════════════════════════════════════"
echo "   Load Test Complete"
echo "════════════════════════════════════════════════════════"
