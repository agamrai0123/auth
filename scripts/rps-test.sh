#!/bin/bash
HTTPS_HOST="https://localhost:8443"
BASE_PATH="/auth-server/v1/oauth"

echo "RPS Test Results"
echo "========================================"

# Test 1: Health
echo ""
echo "[1] Health Endpoint (300 requests, 50 concurrent):"
hey -n 300 -c 50 "$HTTPS_HOST$BASE_PATH/" 2>/dev/null | grep "Requests/sec"

# Test 2: Token 
echo ""
echo "[2] Token Generation Endpoint (300 requests, 50 concurrent):"
hey -n 300 -c 50 -m POST \
  -H "Content-Type: application/json" \
  -d '{"client_id":"test-client","client_secret":"test-secret-123","grant_type":"client_credentials"}' \
  "$HTTPS_HOST$BASE_PATH/token" 2>/dev/null | grep "Requests/sec"

# Test 3: OTT
echo ""
echo "[3] OTT Endpoint (300 requests, 50 concurrent):"
hey -n 300 -c 50 -m POST \
  -H "Content-Type: application/json" \
  -d '{"client_id":"test-client","client_secret":"test-secret-123"}' \
  "$HTTPS_HOST$BASE_PATH/ott" 2>/dev/null | grep "Requests/sec"

# Get token for validate/revoke tests
TOKEN=$(curl -sk -X POST "$HTTPS_HOST$BASE_PATH/token" \
  -H "Content-Type: application/json" \
  -d '{"client_id":"test-client","client_secret":"test-secret-123","grant_type":"client_credentials"}' 2>/dev/null | grep -o '"access_token":"[^"]*"' | cut -d'"' -f4)

if [ -n "$TOKEN" ]; then
  # Test 4: Validate
  echo ""
  echo "[4] Validate Token Endpoint (300 requests, 50 concurrent):"
  hey -n 300 -c 50 -m POST \
    -H "Authorization: Bearer $TOKEN" \
    -H "X-Forwarded-For: http://localhost:8082/resource1" \
    "$HTTPS_HOST$BASE_PATH/validate" 2>/dev/null | grep "Requests/sec"

  # Test 5: Revoke
  echo ""
  echo "[5] Revoke Token Endpoint (300 requests, 50 concurrent):"
  hey -n 300 -c 50 -m POST \
    -H "Authorization: Bearer $TOKEN" \
    "$HTTPS_HOST$BASE_PATH/revoke" 2>/dev/null | grep "Requests/sec"
fi

echo ""
echo "========================================"
echo "Note: RPS Target is 5000+ per endpoint"
echo "========================================"
