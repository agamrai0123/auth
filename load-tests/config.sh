# Load Test Configuration
# This file contains configurable parameters for all load tests

# Server Configuration
BASE_URL=http://localhost:8080
SERVER_PORT=8080

# Load Test Parameters
# These can be customized based on your needs

# Standard test configuration (100k requests, 100 concurrent)
REQUESTS=100000
CONCURRENCY=100

# Alternative configurations (uncomment to use)
# Light load test
# REQUESTS=10000
# CONCURRENCY=10

# Medium load test
# REQUESTS=50000
# CONCURRENCY=50

# Heavy load test (might cause server issues)
# REQUESTS=500000
# CONCURRENCY=500

# Database Configuration
DB_HOST=localhost
DB_PORT=1521
DB_SERVICE=XE
DB_USER=system
DB_PASSWORD=abcd1234

# Test Client Credentials (from schema.sql)
CLIENT_ID=test-client
CLIENT_SECRET=test-secret-123

# Timeouts (in seconds)
REQUEST_TIMEOUT=30
DATABASE_TIMEOUT=5

# Output Configuration
SAVE_RESULTS=true
RESULTS_DIR=results
TIMESTAMP_RESULTS=false

# Performance Thresholds (for automated reporting)
HEALTH_CHECK_EXPECTED_THROUGHPUT=30000
TOKEN_GENERATION_EXPECTED_THROUGHPUT=8000
OTT_EXPECTED_THROUGHPUT=1200
VALIDATE_EXPECTED_THROUGHPUT=18000
REVOKE_EXPECTED_THROUGHPUT=21000

# Alert thresholds (percentage below expected)
ALERT_THRESHOLD=20  # Alert if 20% below expected throughput
