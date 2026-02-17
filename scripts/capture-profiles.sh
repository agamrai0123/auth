#!/bin/bash

PROFILE_DIR="./profiles"
mkdir -p $PROFILE_DIR
HTTPS_HOST="https://localhost:8443"
BASE_PATH="/auth-server/v1/oauth"

# Generate a token for testing
TOKEN=$(curl -sk -X POST "${HTTPS_HOST}${BASE_PATH}/token" \
  -H "Content-Type: application/json" \
  -d '{"client_id":"test-client","client_secret":"test-secret-123","grant_type":"client_credentials"}' 2>/dev/null | grep -o '"access_token":"[^"]*"' | cut -d'"' -f4)

echo "Token: ${TOKEN:0:30}..."
echo ""
echo "[1] Capturing CPU Profile for Token Generation (30 sec)..."
curl -s "http://localhost:7071/debug/pprof/profile?seconds=30" > $PROFILE_DIR/cpu_token.prof &
sleep 1

# Heavy load on token endpoint
hey -n 30000 -c 100 -m POST -H "Content-Type: application/json" \
  -d '{"client_id"st:7071/debug/pprof.prof &
sleep 1

# Heavy load on validate endpoint
hey -n 30000 -c 100 -m POST -H "Authorization: Bearer $TOKEN" \
  -H "X-Forwarded-For: http:{BASE_PATH}/validate" >o ""
echo "[3] Generating Flame Graphs..."
cd $PROFILE_DIR

# Generate SVG flame graphs
go tool pprof -http=:8888 cpu_token.prof &
sleep 2
echo "✓ Token flame graph available at http://localhost:8888"

echo ""
echo "[4] ANDPOINT - Top 15 CPU Functions ==="
go tool pprof -top -nodecount=15 cpu_token.prof 2>/dev/null | head -20

echo ""
echo "=== VALIDATE ENDPOINT - Top 15 CPU Functions ==="
go tool pprof -top -nodecount=15 cpu_validate.prof 2>/dev/null | head -20

