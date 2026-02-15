#!/bin/bash

# HTTPS Load Test - Token Revocation Endpoint
# Tests: POST /revoke on HTTPS port 8443
# Expected: High throughput (fast DB writes, no async delays)

source ./config.sh

HTTPS_URL="https://localhost:8443/auth-server/v1/oauth/revoke"
RESULTS_FILE="./results/https_token_revocation_results.txt"

mkdir -p results

echo "Starting HTTPS Token Revocation Load Test..."
echo "URL: $HTTPS_URL"
echo "Requests: $REQUESTS"
echo "Concurrency: $CONCURRENCY"
echo ""

# Revocation endpoint payload
PAYLOAD='{"token_id":"test-token-123"}'

# Run hey with HTTPS (skip certificate verification with -i flag)
hey -n $REQUESTS -c $CONCURRENCY -m POST -H "Content-Type: application/json" -d "$PAYLOAD" -i "$HTTPS_URL" > "$RESULTS_FILE" 2>&1

# Extract and display results
echo "=== HTTPS Token Revocation Results ==="
grep -E "Requests/sec:|Average:|Status|Total" "$RESULTS_FILE"
echo ""
echo "Full results saved to: $RESULTS_FILE"
