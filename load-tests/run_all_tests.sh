#!/bin/bash
# Run All Load Tests Sequentially
# Executes all endpoint load tests and generates a combined report

echo "=========================================="
echo "Auth Service - Complete Load Test Suite"
echo "=========================================="
echo "Start time: $(date)"
echo ""
echo "This will run 5 endpoints with 100,000 requests each"
echo "Total requests: 500,000"
echo "Total expected time: ~300 seconds (5 minutes)"
echo ""
echo "Press Ctrl+C to stop at any time"
echo ""

# Create results directory if it doesn't exist
mkdir -p results

# Function to run test and save results
run_test() {
    local test_name=$1
    local script=$2
    local output_file="results/${test_name}_results.txt"
    
    echo ""
    echo "=========================================="
    echo "Running: $test_name"
    echo "=========================================="
    
    bash "$script" | tee "$output_file"
    
    echo ""
    echo "Results saved to: $output_file"
    echo ""
}

# Run all tests
start_time=$(date +%s)

run_test "health_check" "test_health.sh"
run_test "token_generation" "test_token.sh"
run_test "one_time_token" "test_ott.sh"
run_test "token_validation" "test_validate.sh"
run_test "token_revocation" "test_revoke.sh"

end_time=$(date +%s)
total_time=$((end_time - start_time))

echo ""
echo "=========================================="
echo "Load Test Suite Complete"
echo "=========================================="
echo "Total execution time: ${total_time} seconds ($(($total_time / 60)) min $(($total_time % 60)) sec)"
echo "End time: $(date)"
echo ""
echo "Results saved to: results/"
echo ""
echo "Summary:"
echo "  ✓ Health Check: results/health_check_results.txt"
echo "  ✓ Token Generation: results/token_generation_results.txt"
echo "  ✓ One-Time Token: results/one_time_token_results.txt"
echo "  ✓ Token Validation: results/token_validation_results.txt"
echo "  ✓ Token Revocation: results/token_revocation_results.txt"
echo ""
