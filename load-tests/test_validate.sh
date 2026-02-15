#!/bin/bash
# Token Validation Endpoint Load Test
# Tests POST /auth-server/v1/oauth/validate endpoint
# Expected: ~18k req/sec, validates token and checks access
# Note: Using invalid token for testing (returns 400)

BASE_URL="http://localhost:8080"
ENDPOINT="/auth-server/v1/oauth/validate"
REQUESTS=100000
CONCURRENCY=100

echo "=========================================="
echo "Token Validation Load Test"
echo "=========================================="
echo "Endpoint: POST $ENDPOINT"
echo "Requests: $REQUESTS"
echo "Concurrency: $CONCURRENCY"
echo "URL: $BASE_URL$ENDPOINT"
echo ""
echo "Test parameters:"
echo "  access_token: invalid_token (for testing)"
echo "  endpoint: /test/resource"
echo ""
echo "Note: This test uses an invalid token to simulate validation failure"
echo "In production, you would use a real JWT token from /token endpoint"
echo ""

hey -n $REQUESTS -c $CONCURRENCY -m POST \
  -H "Content-Type: application/json" \
  -d '{"access_token":"invalid_token","endpoint":"/test/resource"}' \
  "$BASE_URL$ENDPOINT"
