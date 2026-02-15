#!/bin/bash

# Master HTTPS Load Test Runner
# Runs all 5 endpoint tests on HTTPS and archives results

echo "=========================================="
echo "HTTPS Load Test Suite - All Endpoints"
echo "=========================================="
echo "Date: $(date)"
echo "Requests: 100,000 per endpoint"
echo "Concurrency: 100"
echo "Total requests: 500,000"
echo ""

mkdir -p results

# Function to run test and report
run_test() {
    local test_name=$1
    local script=$2
    echo ""
    echo "Running $test_name..."
    bash "$script"
    echo "✓ $test_name completed"
}

# Run all tests sequentially
START_TIME=$(date +%s)

run_test "HTTPS Health Check" "./test_https_health.sh"
run_test "HTTPS Token Generation" "./test_https_token.sh"
run_test "HTTPS One-Time Token" "./test_https_ott.sh"
run_test "HTTPS Token Validation" "./test_https_validate.sh"
run_test "HTTPS Token Revocation" "./test_https_revoke.sh"

END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))

echo ""
echo "=========================================="
echo "HTTPS Load Test Suite Complete"
echo "=========================================="
echo "Total Duration: ${DURATION}s"
echo ""
echo "Results Summary:"
echo "  Health Check:      $(grep 'Requests/sec:' results/https_health_check_results.txt | awk '{print $NF}')"
echo "  Token Generation:  $(grep 'Requests/sec:' results/https_token_generation_results.txt | awk '{print $NF}')"
echo "  One-Time Token:    $(grep 'Requests/sec:' results/https_one_time_token_results.txt | awk '{print $NF}')"
echo "  Token Validation:  $(grep 'Requests/sec:' results/https_token_validation_results.txt | awk '{print $NF}')"
echo "  Token Revocation:  $(grep 'Requests/sec:' results/https_token_revocation_results.txt | awk '{print $NF}')"
echo ""
echo "Results saved to: results/"
