#!/bin/bash
# Token Generation Endpoint Load Test
# Tests POST /auth-server/v1/oauth/token endpoint
# Expected: ~8k req/sec, generates real JWT tokens
# Credentials: test-client / test-secret-123

BASE_URL="http://localhost:8080"
ENDPOINT="/auth-server/v1/oauth/token"
REQUESTS=100000
CONCURRENCY=100

echo "=========================================="
echo "Token Generation Load Test"
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

hey -n $REQUESTS -c $CONCURRENCY -m POST \
  -H "Content-Type: application/json" \
  -d '{"grant_type":"client_credentials","client_id":"test-client","client_secret":"test-secret-123"}' \
  "$BASE_URL$ENDPOINT"
