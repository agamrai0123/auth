#!/bin/bash

# Performance Comparison Tool
# Compares current test results against baseline metrics

set -e

BASELINE_HEALTH_RPS=30996
BASELINE_TOKEN_RPS=8155
BASELINE_OTT_RPS=1254
BASELINE_VALIDATE_RPS=18606
BASELINE_REVOKE_RPS=21190

RESULTS_DIR="./results"
THRESHOLD_PERCENT=20  # Alert if performance drops more than 20%

# Colors for output
RED='\033[0;31m'
YELLOW='\033[1;33m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

# Function to extract throughput from hey output
extract_throughput() {
    local file=$1
    grep "Requests/sec:" "$file" | awk '{print $NF}' | sed 's/\.0$//'
}

# Function to extract average latency from hey output
extract_latency() {
    local file=$1
    grep "Average:" "$file" | head -1 | awk '{print $NF}' | sed 's/ms$//'
}

# Function to calculate percentage difference
calc_diff() {
    local baseline=$1
    local current=$2
    echo "scale=2; (($baseline - $current) / $baseline) * 100" | bc
}

# Function to print comparison
print_comparison() {
    local endpoint=$1
    local baseline_rps=$2
    local current_rps=$3
    local baseline_lat=$4
    local current_lat=$5

    printf "%-15s | " "$endpoint"
    printf "RPS: %-8s -> %-8s | " "$baseline_rps" "$current_rps"
    printf "Lat: %-8s -> %-8s | " "$baseline_lat" "$current_lat"

    local rps_diff=$(calc_diff "$baseline_rps" "$current_rps")
    
    if (( $(echo "$rps_diff > $THRESHOLD_PERCENT" | bc -l) )); then
        printf "${RED}⚠️  Performance down ${rps_diff}%%${NC}\n"
    elif (( $(echo "$rps_diff < -5" | bc -l) )); then
        printf "${GREEN}✓ Performance up ${rps_diff#-}%%${NC}\n"
    else
        printf "${GREEN}✓ Stable${NC}\n"
    fi
}

# Check if results exist
if [ ! -d "$RESULTS_DIR" ]; then
    echo "Error: No results found. Run tests first with: bash run_all_tests.sh"
    exit 1
fi

echo "=========================================="
echo "Performance Comparison Report"
echo "=========================================="
echo ""

# Initialize comparison data
health_rps=0
token_rps=0
ott_rps=0
validate_rps=0
revoke_rps=0

health_lat=0
token_lat=0
ott_lat=0
validate_lat=0
revoke_lat=0

# Extract metrics from results files
if [ -f "$RESULTS_DIR/health_check_results.txt" ]; then
    health_rps=$(extract_throughput "$RESULTS_DIR/health_check_results.txt")
    health_lat=$(extract_latency "$RESULTS_DIR/health_check_results.txt")
fi

if [ -f "$RESULTS_DIR/token_generation_results.txt" ]; then
    token_rps=$(extract_throughput "$RESULTS_DIR/token_generation_results.txt")
    token_lat=$(extract_latency "$RESULTS_DIR/token_generation_results.txt")
fi

if [ -f "$RESULTS_DIR/one_time_token_results.txt" ]; then
    ott_rps=$(extract_throughput "$RESULTS_DIR/one_time_token_results.txt")
    ott_lat=$(extract_latency "$RESULTS_DIR/one_time_token_results.txt")
fi

if [ -f "$RESULTS_DIR/token_validation_results.txt" ]; then
    validate_rps=$(extract_throughput "$RESULTS_DIR/token_validation_results.txt")
    validate_lat=$(extract_latency "$RESULTS_DIR/token_validation_results.txt")
fi

if [ -f "$RESULTS_DIR/token_revocation_results.txt" ]; then
    revoke_rps=$(extract_throughput "$RESULTS_DIR/token_revocation_results.txt")
    revoke_lat=$(extract_latency "$RESULTS_DIR/token_revocation_results.txt")
fi

# Print header
printf "%-15s | %-30s | %-30s | Status\n" "Endpoint" "Throughput" "Latency"
printf "%-15s | %-30s | %-30s | -------\n" "---" "---" "---"

# Print comparisons
print_comparison "Health" "$BASELINE_HEALTH_RPS" "$health_rps" "3.2ms" "${health_lat}ms"
print_comparison "Token" "$BASELINE_TOKEN_RPS" "$token_rps" "12.2ms" "${token_lat}ms"
print_comparison "OTT" "$BASELINE_OTT_RPS" "$ott_rps" "95.6ms" "${ott_lat}ms"
print_comparison "Validate" "$BASELINE_VALIDATE_RPS" "$validate_rps" "5.3ms" "${validate_lat}ms"
print_comparison "Revoke" "$BASELINE_REVOKE_RPS" "$revoke_rps" "4.7ms" "${revoke_lat}ms"

echo ""
echo "=========================================="
echo "Interpretation:"
echo "  ✓ Green = Performance maintained (±5%)"
echo "  ✓ Green = Performance improved (>5%)"
echo "  ⚠️ Red = Performance degraded (>20%)"
echo "=========================================="
echo ""
echo "Threshold: ${THRESHOLD_PERCENT}% degradation triggers alert"
echo ""
echo "For detailed results, see:"
echo "  - $RESULTS_DIR/health_check_results.txt"
echo "  - $RESULTS_DIR/token_generation_results.txt"
echo "  - $RESULTS_DIR/one_time_token_results.txt"
echo "  - $RESULTS_DIR/token_validation_results.txt"
echo "  - $RESULTS_DIR/token_revocation_results.txt"
echo ""
