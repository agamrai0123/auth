#!/bin/bash

# Quick CPU profiling during load test
HTTPS_HOST="https://localhost:8443"
BASE_PATH="/auth-server/v1/oauth"
CLIENT_ID="test-client"
CLIENT_SECRET="test-secret-123"
RESOURCE_ENDPOINT="http://localhost:8082/resource1"
PROFILE_DIR="./profiles"

mkdir -p $PROFILE_DIR

echo "[1] Generating test token..."
TOKEN=$(curl -sk -X POST "${HTTPS_HOST}${BASE_PATH}/token" \
    -H "Content-Type: application/json" \
    -d "{\"client_id\": \"${CLIENT_ID}\", \"client_secret\": \"${CLIENT_SECRET}\", \"grant_type\": \"client_credentials\"}" 2>/dev/null | grep -o '"access_token":"[^"]*"' | cut -d'"' -f4)

if [ -z "$TOKEN" ]; then
    echo "ERROR: Failed to generate token"
    exit 1
fi

echo "Token generated: ${TOKEN:0:30}..."
echo ""
echo "[2] Capturing 30-second CPU profile during validate load..."

curl -s "http://localhost:7071/debug/pprof/profile?seconds=30" > $PROFILE_DIR/cpu.prof &
sleep 1

hey -n 30000 -c 100 -m POST \
    -H "Authorization: Bearer ${TOKEN}" \
    -H "X-Forwarded-For: ${RESOURCE_ENDPOINT}" \
    "${HTTPS_HOST}${BASE_PATH}/validate" > /dev/null 2>&1 &

wait
echo "✓ Profile captured"
echo ""
echo "[3] Top 20 CPU-consuming functions:"
go tool pprof -top -nodecount=20 $PROFILE_DIR/cpu.prof 2>/dev/null | head -25
