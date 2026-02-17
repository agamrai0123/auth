#!/bin/bash

# Performance analysis script for validate endpoint
# Captures CPU profile and analyzes bottlenecks

HTTPS_HOST="https://localhost:8443"
BASE_PATH="/auth-server/v1/oauth"
CLIENT_ID="test-client"
CLIENT_SECRET="test-secret-123"
RESOURCE_ENDPOINT="http://localhost:8082/resource1"
METRIC_PORT="7071"
PROFILE_DIR="./profiles"

mkdir -p $PROFILE_DIR

echo "════════════════════════════════════════════════════════════════"
echo "  Performance Analysis & Profiling"
echo "════════════════════════════════════════════════════════════════"
echo ""

# Generate token
echo "[*] Generating test token..."
TOKEN=$(curl -sk -X POST "${HTTPS_HOST}${BASE_PATH}/token" \
    -H "Content-Type: application/json" \
    -d "{
        \"client_id\": \"${CLIENT_ID}\",
        \"client_secret\": \"${CLIENT_SECRET}\",
        \"grant_type\": \"client_credentials\"
    }" 2>/dev/null | grep -o '"access_token":"[^"]*"' | cut -d'"' -f4)

echo "✓ Token generated"
echo ""

# Collect CPU profile during load test
echo "[1] Capturing 30-second CPU profile for validate endpoint..."
echo "    Running: go tool pprof http://localhost:${METRIC_PORT}/debug/pprof/profile?seconds=30"
echo ""

# Start CPU profiling in background
curl -s "http://localhost:${METRIC_PORT}/debug/pprof/profile?seconds=30" > $PROFILE_DIR/cpu.prof &
PROF_PID=$!

# Give pprof a moment to start collecting
sleep 1

# Run intensive load test during profiling
echo "[2] Running intense load test (30 seconds @ 1000 req/sec)..."
hey -n 30000 -c 100 -m POST \
    -H "Authorization: Bearer ${TOKEN}" \
    -H "X-Forwarded-For: ${RESOURCE_ENDPOINT}" \
    "${HTTPS_HOST}${BASE_PATH}/validate" > /dev/null 2>&1 &

LOAD_PID=$!

# Wait for profiling to complete
wait $PROF_PID
echo "✓ CPU profile captured"
echo ""

# Analyze CPU profile
echo "[3] CPU Profile Analysis:"
echo "────────────────────────────────────────────────────────────────"
go tool pprof -top -nodecount=20 $PROFILE_DIR/cpu.prof | head -30
echo ""

# Collect heap profile
echo "[4] Collecting heap profile..."
curl -s "http://localhost:${METRIC_PORT}/debug/pprof/heap" > $PROFILE_DIR/heap.prof
echo "✓ Heap profile captured"
echo ""

# Analyze heap profile
echo "[5] Heap Profile Analysis (Top 10 allocations):"
echo "────────────────────────────────────────────────────────────────"
go tool pprof -top -nodecount=10 $PROFILE_DIR/heap.prof | head -20
echo ""

# Goroutine analysis
echo "[6] Goroutine Count:"
curl -s "http://localhost:${METRIC_PORT}/debug/pprof/goroutine?debug=1" | head -5
echo ""

# Generate flamegraph-ready profile
echo "[7] Generating detailed CPU profile for analysis..."
go tool pprof -pdf $PROFILE_DIR/cpu.prof > $PROFILE_DIR/cpu_profile.pdf 2>/dev/null || true
echo "✓ Profile saved (view with: go tool pprof $PROFILE_DIR/cpu.prof)"
echo ""

echo "════════════════════════════════════════════════════════════════"
echo "  Profile files saved in: $PROFILE_DIR/"
echo "════════════════════════════════════════════════════════════════"
echo ""

# Wait for load test to complete
wait $LOAD_PID
