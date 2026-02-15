#!/bin/bash

# HTTPS Load Test - One-Time Token Endpoint
# Tests: POST /ott on HTTPS port 8443
# Expected: Low throughput (async operations, slowest endpoint)

source ./config.sh

HTTPS_URL="https://localhost:8443/auth-server/v1/oauth/ott"
RESULTS_FILE="./results/https_one_time_token_results.txt"

mkdir -p results

echo "Starting HTTPS One-Time Token Load Test..."
echo "URL: $HTTPS_URL"
echo "Requests: $REQUESTS"
echo "Concurrency: $CONCURRENCY"
echo ""

# OTT endpoint payload (client credentials grant)
PAYLOAD='{"grant_type":"client_credentials", "client_id":"'"$CLIENT_ID"'", "client_secret":"'"$CLIENT_SECRET"'"}'

# Run hey with HTTPS (skip certificate verification with -i flag)
hey -n $REQUESTS -c $CONCURRENCY -m POST -H "Content-Type: application/json" -d "$PAYLOAD" -i "$HTTPS_URL" > "$RESULTS_FILE" 2>&1

# Extract and display results
echo "=== HTTPS One-Time Token Results ==="
grep -E "Requests/sec:|Average:|Status|Total" "$RESULTS_FILE"
echo ""
echo "Full results saved to: $RESULTS_FILE"
