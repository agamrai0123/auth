#!/bin/bash

# HTTPS Load Test - Token Validation Endpoint
# Tests: POST /validate on HTTPS port 8443
# Expected: High throughput (mostly cached, no writes)

source ./config.sh

HTTPS_URL="https://localhost:8443/auth-server/v1/oauth/validate"
RESULTS_FILE="./results/https_token_validation_results.txt"

mkdir -p results

echo "Starting HTTPS Token Validation Load Test..."
echo "URL: $HTTPS_URL"
echo "Requests: $REQUESTS"
echo "Concurrency: $CONCURRENCY"
echo ""

# Validation endpoint payload (use invalid token to test error handling)
PAYLOAD='{"token":"invalid-token-for-testing"}'

# Run hey with HTTPS (skip certificate verification with -i flag)
hey -n $REQUESTS -c $CONCURRENCY -m POST -H "Content-Type: application/json" -d "$PAYLOAD" -i "$HTTPS_URL" > "$RESULTS_FILE" 2>&1

# Extract and display results
echo "=== HTTPS Token Validation Results ==="
grep -E "Requests/sec:|Average:|Status|Total" "$RESULTS_FILE"
echo ""
echo "Full results saved to: $RESULTS_FILE"
