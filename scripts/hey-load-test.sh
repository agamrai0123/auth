#!/bin/bash

# OAuth2 Service - Hey Load Test without Rate Limiting
# Tests all endpoints and shows max RPS for 200 status only

HTTPS_HOST="https://localhost:8443"
BASE_PATH="/auth-server/v1/oauth"
CLIENT_ID="test-client"
CLIENT_SECRET="test-secret-123"
RESOURCE_ENDPOINT="http://localhost:8082/resource1"

# Load test parameters
REQUESTS=500
CONCURRENCY=100

echo "════════════════════════════════════════════════════════════"
echo "   OAuth2 Service Load Test (No Rate Limiting)"
echo "   Testing max RPS with 100% 200 status responses"
echo "════════════════════════════════════════════════════════════"
echo ""
echo "Configuration:"
echo "  Total Requests:   $REQUESTS per endpoint"
echo "  Concurrency:      $CONCURRENCY"
echo "  Host:             $HTTPS_HOST"
echo ""

# Generate test token
echo "[*] Generating test token..."
TOKEN_RESPONSE=$(curl -s -X POST "${HTTPS_HOST}${BASE_PATH}/token" \
    -k \
    -H "Content-Type: application/json" \
    -d "{
        \"client_id\": \"${CLIENT_ID}\",
        \"client_secret\": \"${CLIENT_SECRET}\",
        \"grant_type\": \"client_credentials\"
    }")

TOKEN=$(echo "$TOKEN_RESPONSE" | grep -o '"access_token":"[^"]*"' | cut -d'"' -f4)

if [ -z "$TOKEN" ]; then
    echo "[X] Failed to generate token"
    echo "Response: $TOKEN_RESPONSE"
    exit 1
fi
echo "[✓] Token generated: ${TOKEN:0:30}..."
echo ""

echo "════════════════════════════════════════════════════════════"
echo "   Running Load Tests"
echo "════════════════════════════════════════════════════════════"
echo ""

# Test 1: Health Check
echo "[TEST 1] GET / (Health Check)"
output=$(hey -n $REQUESTS -c $CONCURRENCY -k "${HTTPS_HOST}${BASE_PATH}/" 2>&1)
rps=$(echo "$output" | grep "Requests/sec" | awk '{print $2}')
status_200=$(echo "$output" | grep "200" | head -1 | awk '{print $2}')
if [ -z "$status_200" ]; then
  status_200="0"
fi
echo "  RPS: $rps (200 responses: $status_200/$REQUESTS)"
echo ""

# Test 2: Token Generation
echo "[TEST 2] POST /token (Generate Token)"
json_payload="{\"client_id\":\"${CLIENT_ID}\",\"client_secret\":\"${CLIENT_SECRET}\",\"grant_type\":\"client_credentials\"}"
output=$(hey -n $REQUESTS -c $CONCURRENCY -m POST -H "Content-Type: application/json" -d "$json_payload" -k "${HTTPS_HOST}${BASE_PATH}/token" 2>&1)
rps=$(echo "$output" | grep "Requests/sec" | awk '{print $2}')
status_200=$(echo "$output" | grep "200" | head -1 | awk '{print $2}')
if [ -z "$status_200" ]; then
  status_200="0"
fi
echo "  RPS: $rps (200 responses: $status_200/$REQUESTS)"
echo ""

# Test 3: OTT Generation
echo "[TEST 3] POST /ott (One-Time Token)"
json_payload="{\"client_id\":\"${CLIENT_ID}\",\"client_secret\":\"${CLIENT_SECRET}\",\"grant_type\":\"client_credentials\"}"
output=$(hey -n $REQUESTS -c $CONCURRENCY -m POST -H "Content-Type: application/json" -d "$json_payload" -k "${HTTPS_HOST}${BASE_PATH}/ott" 2>&1)
rps=$(echo "$output" | grep "Requests/sec" | awk '{print $2}')
status_200=$(echo "$output" | grep "200" | head -1 | awk '{print $2}')
if [ -z "$status_200" ]; then
  status_200="0"
fi
echo "  RPS: $rps (200 responses: $status_200/$REQUESTS)"
echo ""

# Test 4: Validate Token
echo "[TEST 4] POST /validate (Token Validation)"
output=$(hey -n $REQUESTS -c $CONCURRENCY -m POST \
    -H "Authorization: Bearer ${TOKEN}" \
    -H "X-Forwarded-For: ${RESOURCE_ENDPOINT}" \
    -k "${HTTPS_HOST}${BASE_PATH}/validate" 2>&1)
rps=$(echo "$output" | grep "Requests/sec" | awk '{print $2}')
status_200=$(echo "$output" | grep "200" | head -1 | awk '{print $2}')
status_401=$(echo "$output" | grep "401" | head -1 | awk '{print $2}')
if [ -z "$status_200" ]; then
  status_200="0"
fi
if [ -z "$status_401" ]; then
  status_401="0"
fi
echo "  RPS: $rps (200: $status_200, 401: $status_401/$REQUESTS)"
if [ "$status_401" -gt "0" ] && [ "$status_200" -eq "0" ]; then
  echo "  ⚠  ISSUE: Validate returning 401 instead of 200"
fi
echo ""

# Test 5: Revoke Token
echo "[TEST 5] POST /revoke (Token Revocation)"
output=$(hey -n $REQUESTS -c $CONCURRENCY -m POST \
    -H "Authorization: Bearer ${TOKEN}" \
    -k "${HTTPS_HOST}${BASE_PATH}/revoke" 2>&1)
rps=$(echo "$output" | grep "Requests/sec" | awk '{print $2}')
status_200=$(echo "$output" | grep "200" | head -1 | awk '{print $2}')
status_401=$(echo "$output" | grep "401" | head -1 | awk '{print $2}')
if [ -z "$status_200" ]; then
  status_200="0"
fi
if [ -z "$status_401" ]; then
  status_401="0"
fi
echo "  RPS: $rps (200: $status_200, 401: $status_401/$REQUESTS)"
if [ "$status_401" -gt "0" ] && [ "$status_200" -eq "0" ]; then
  echo "  ⚠  ISSUE: Revoke returning 401 instead of 200"
fi
echo ""

echo "════════════════════════════════════════════════════════════"
echo "   Load Test Complete"
echo "════════════════════════════════════════════════════════════"
