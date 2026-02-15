#!/bin/bash

# Performance Test Results Archive
# This script creates a timestamped archive of load test results for historical tracking

RESULTS_DIR="./results"
ARCHIVE_DIR="./results/archive"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
ARCHIVE_PATH="${ARCHIVE_DIR}/results_${TIMESTAMP}"

# Create directories
mkdir -p "${ARCHIVE_DIR}"

# Check if results exist
if [ ! -d "${RESULTS_DIR}" ]; then
    echo "Error: No results directory found. Run load tests first."
    exit 1
fi

# Create timestamped archive
mkdir -p "${ARCHIVE_PATH}"

# Copy all results
if [ -f "${RESULTS_DIR}/health_check_results.txt" ]; then
    cp "${RESULTS_DIR}/health_check_results.txt" "${ARCHIVE_PATH}/"
fi

if [ -f "${RESULTS_DIR}/token_generation_results.txt" ]; then
    cp "${RESULTS_DIR}/token_generation_results.txt" "${ARCHIVE_PATH}/"
fi

if [ -f "${RESULTS_DIR}/one_time_token_results.txt" ]; then
    cp "${RESULTS_DIR}/one_time_token_results.txt" "${ARCHIVE_PATH}/"
fi

if [ -f "${RESULTS_DIR}/token_validation_results.txt" ]; then
    cp "${RESULTS_DIR}/token_validation_results.txt" "${ARCHIVE_PATH}/"
fi

if [ -f "${RESULTS_DIR}/token_revocation_results.txt" ]; then
    cp "${RESULTS_DIR}/token_revocation_results.txt" "${ARCHIVE_PATH}/"
fi

# Create metadata file
cat > "${ARCHIVE_PATH}/metadata.txt" << EOF
Test Run: ${TIMESTAMP}
Hostname: $(hostname)
System: $(uname -s)
Load Test Configuration:
  - Requests: 100000
  - Concurrency: 100
  - Server: http://localhost:8080

Running Environment:
  - Go Version: $(go version 2>/dev/null || echo "Not found")
  - Database: Oracle XE 11.2
  - Driver: go-ora/v2 (pure Go)
EOF

echo "✓ Results archived to: ${ARCHIVE_PATH}"
echo "✓ Total runs archived: $(ls -d "${ARCHIVE_DIR}"/results_* 2>/dev/null | wc -l)"
