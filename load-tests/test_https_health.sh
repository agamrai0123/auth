#!/bin/bash

# HTTPS Load Test - Health Check Endpoint
# Tests: GET /oauth/ on HTTPS port 8443
# Expected: High throughput (no DB queries, cached response)

source ./config.sh

HTTPS_URL="https://localhost:8443/auth-server/v1/oauth/"
RESULTS_FILE="./results/https_health_check_results.txt"

mkdir -p results

echo "Starting HTTPS Health Check Load Test..."
echo "URL: $HTTPS_URL"
echo "Requests: $REQUESTS"
echo "Concurrency: $CONCURRENCY"
echo ""

# Run hey with HTTPS (skip certificate verification with -i flag)
hey -n $REQUESTS -c $CONCURRENCY -i "$HTTPS_URL" > "$RESULTS_FILE" 2>&1

# Extract and display results
echo "=== HTTPS Health Check Results ==="
grep -E "Requests/sec:|Average:|Status|Total" "$RESULTS_FILE"
echo ""
echo "Full results saved to: $RESULTS_FILE"
