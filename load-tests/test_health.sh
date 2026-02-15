#!/bin/bash
# Health Check Endpoint Load Test
# Tests GET /auth-server/v1/oauth/ endpoint
# Expected: Fast response (no database queries), ~31k req/sec

BASE_URL="http://localhost:8080"
ENDPOINT="/auth-server/v1/oauth/"
REQUESTS=100000
CONCURRENCY=100

echo "=========================================="
echo "Health Check Load Test"
echo "=========================================="
echo "Endpoint: GET $ENDPOINT"
echo "Requests: $REQUESTS"
echo "Concurrency: $CONCURRENCY"
echo "URL: $BASE_URL$ENDPOINT"
echo ""

hey -n $REQUESTS -c $CONCURRENCY -m GET "$BASE_URL$ENDPOINT"
