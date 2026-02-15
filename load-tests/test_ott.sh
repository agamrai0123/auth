#!/bin/bash
# One-Time Token (OTT) Endpoint Load Test
# Tests POST /auth-server/v1/oauth/ott endpoint
# Expected: ~1.2k req/sec (slowest due to async OTT operations)
# Credentials: test-client / test-secret-123

BASE_URL="http://localhost:8080"
ENDPOINT="/auth-server/v1/oauth/ott"
REQUESTS=100000
CONCURRENCY=100

echo "=========================================="
echo "One-Time Token (OTT) Load Test"
echo "=========================================="
echo "Endpoint: POST $ENDPOINT"
echo "Requests: $REQUESTS"
echo "Concurrency: $CONCURRENCY"
echo "URL: $BASE_URL$ENDPOINT"
echo ""
echo "Using credentials:"
echo "  Client ID: test-client"
echo "  Client Secret: test-secret-123"
echo ""
echo "Note: This endpoint is slower due to:"
echo "  - OTT token generation and storage"
echo "  - Async revocation handling"
echo "  - Token batch writer processing"
echo ""

hey -n $REQUESTS -c $CONCURRENCY -m POST \
  -H "Content-Type: application/json" \
  -d '{"grant_type":"client_credentials","client_id":"test-client","client_secret":"test-secret-123"}' \
  "$BASE_URL$ENDPOINT"
