#!/bin/bash
# Token Revocation Endpoint Load Test
# Tests POST /auth-server/v1/oauth/revoke endpoint
# Expected: ~21k req/sec, revokes tokens
# Note: Using test token ID (will return 404 or 401)

BASE_URL="http://localhost:8080"
ENDPOINT="/auth-server/v1/oauth/revoke"
REQUESTS=100000
CONCURRENCY=100

echo "=========================================="
echo "Token Revocation Load Test"
echo "=========================================="
echo "Endpoint: POST $ENDPOINT"
echo "Requests: $REQUESTS"
echo "Concurrency: $CONCURRENCY"
echo "URL: $BASE_URL$ENDPOINT"
echo ""
echo "Test parameters:"
echo "  token_id: test-token-123"
echo ""
echo "Note: This test uses a test token ID that doesn't exist"
echo "Expected responses: 404 (token not found) or 401 (unauthorized)"
echo "In production, you would use real token IDs from /token endpoint"
echo ""

hey -n $REQUESTS -c $CONCURRENCY -m POST \
  -H "Content-Type: application/json" \
  -d '{"token_id":"test-token-123"}' \
  "$BASE_URL$ENDPOINT"
