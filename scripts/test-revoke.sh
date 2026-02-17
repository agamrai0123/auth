#!/bin/bash

# OAuth2 Token Revocation Test Script
# This script tests the complete token lifecycle: generate -> validate -> revoke

set -e

# Configuration
HTTPS_HOST="https://localhost:8443"
HTTP_HOST="http://localhost:8080"
CLIENT_ID="test-client"
CLIENT_SECRET="test-secret-123"
RESOURCE_ENDPOINT="http://localhost:8082/resource1"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Counters
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

# Functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

test_token_generation() {
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    log_info "Test $TOTAL_TESTS: Generating token..."
    
    RESPONSE=$(curl -s -X POST "${HTTPS_HOST}/auth-server/v1/oauth/token" \
        -k \
        -H "Content-Type: application/json" \
        -d "{
            \"client_id\": \"${CLIENT_ID}\",
            \"client_secret\": \"${CLIENT_SECRET}\",
            \"grant_type\": \"client_credentials\"
        }")
    
    # Extract token from response
    TOKEN=$(echo "$RESPONSE" | grep -o '"access_token":"[^"]*"' | cut -d'"' -f4)
    
    if [ -z "$TOKEN" ]; then
        log_error "Failed to generate token"
        log_warn "Response: $RESPONSE"
        FAILED_TESTS=$((FAILED_TESTS + 1))
        return 1
    fi
    
    log_success "Token generated: ${TOKEN:0:20}..."
    PASSED_TESTS=$((PASSED_TESTS + 1))
    return 0
}

test_token_validation() {
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    log_info "Test $TOTAL_TESTS: Validating token..."
    
    RESPONSE=$(curl -s -X POST "${HTTPS_HOST}/auth-server/v1/oauth/validate" \
        -k \
        -H "Authorization: Bearer ${TOKEN}" \
        -H "X-Forwarded-For: ${RESOURCE_ENDPOINT}")
    
    VALID=$(echo "$RESPONSE" | grep -o '"valid":[^,}]*' | cut -d':' -f2)
    
    if [ "$VALID" = "true" ]; then
        log_success "Token validated successfully"
        PASSED_TESTS=$((PASSED_TESTS + 1))
        return 0
    else
        log_error "Token validation failed"
        log_warn "Response: $RESPONSE"
        FAILED_TESTS=$((FAILED_TESTS + 1))
        return 1
    fi
}

test_token_revocation() {
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    log_info "Test $TOTAL_TESTS: Revoking token..."
    
    RESPONSE=$(curl -s -X POST "${HTTPS_HOST}/auth-server/v1/oauth/revoke" \
        -k \
        -H "Authorization: Bearer ${TOKEN}")
    
    MESSAGE=$(echo "$RESPONSE" | grep -o '"message":"[^"]*"')
    
    if [ -n "$MESSAGE" ]; then
        log_success "Token revoked successfully"
        PASSED_TESTS=$((PASSED_TESTS + 1))
        return 0
    else
        log_error "Token revocation failed"
        log_warn "Response: $RESPONSE"
        FAILED_TESTS=$((FAILED_TESTS + 1))
        return 1
    fi
}

test_revoked_token_validation() {
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    log_info "Test $TOTAL_TESTS: Validating revoked token (should fail)..."
    
    RESPONSE=$(curl -s -X POST "${HTTPS_HOST}/auth-server/v1/oauth/validate" \
        -k \
        -H "Authorization: Bearer ${TOKEN}" \
        -H "X-Forwarded-For: ${RESOURCE_ENDPOINT}")
    
    # Should receive an error now
    if echo "$RESPONSE" | grep -q "error"; then
        log_success "Revoked token correctly rejected"
        PASSED_TESTS=$((PASSED_TESTS + 1))
        return 0
    else
        log_warn "Expected error for revoked token, but got: $RESPONSE"
        FAILED_TESTS=$((FAILED_TESTS + 1))
        return 1
    fi
}

load_test_revocation() {
    local iterations=$1
    local concurrent=$2
    
    log_info "Starting load test: $iterations iterations with $concurrent concurrent requests"
    
    local counter=0
    for ((i=1; i<=iterations; i++)); do
        for ((j=0; j<concurrent; j++)); do
            (
                log_info "Load test [$i/$iterations, worker $j]: Starting..."
                
                # Generate token
                TOKEN_RESPONSE=$(curl -s -X POST "${HTTPS_HOST}/auth-server/v1/oauth/token" \
                    -k \
                    -H "Content-Type: application/json" \
                    -d "{
                        \"client_id\": \"${CLIENT_ID}\",
                        \"client_secret\": \"${CLIENT_SECRET}\",
                        \"grant_type\": \"client_credentials\"
                    }")
                
                TEST_TOKEN=$(echo "$TOKEN_RESPONSE" | grep -o '"access_token":"[^"]*"' | cut -d'"' -f4)
                
                if [ -z "$TEST_TOKEN" ]; then
                    log_error "Load test [$i/$iterations, worker $j]: Failed to generate token"
                    return 1
                fi
                
                # Revoke token
                REVOKE_RESPONSE=$(curl -s -X POST "${HTTPS_HOST}/auth-server/v1/oauth/revoke" \
                    -k \
                    -H "Authorization: Bearer ${TEST_TOKEN}")
                
                if echo "$REVOKE_RESPONSE" | grep -q "message"; then
                    log_success "Load test [$i/$iterations, worker $j]: Revocation successful"
                else
                    log_error "Load test [$i/$iterations, worker $j]: Revocation failed"
                    return 1
                fi
            ) &
            
            counter=$((counter + 1))
        done
        
        # Wait for all background jobs in this batch
        wait
    done
    
    log_success "Load test completed: $counter total revocations"
}

print_summary() {
    echo ""
    echo "=========================================="
    echo "        Test Summary"
    echo "=========================================="
    echo "Total Tests:  $TOTAL_TESTS"
    echo -e "Passed:       ${GREEN}$PASSED_TESTS${NC}"
    echo -e "Failed:       ${RED}$FAILED_TESTS${NC}"
    echo "=========================================="
    
    if [ $FAILED_TESTS -eq 0 ]; then
        log_success "All tests passed!"
        return 0
    else
        log_error "$FAILED_TESTS test(s) failed"
        return 1
    fi
}

# Main execution
main() {
    local test_mode=$1
    local param1=$2
    local param2=$3
    
    echo -e "${BLUE}╔════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║  OAuth2 Token Revocation Test Script   ║${NC}"
    echo -e "${BLUE}╚════════════════════════════════════════╝${NC}"
    echo ""
    
    case $test_mode in
        "single"|"")
            log_info "Running single token lifecycle test..."
            echo ""
            test_token_generation && \
            test_token_validation && \
            test_token_revocation && \
            test_revoked_token_validation
            ;;
        "load")
            local iterations=${param1:-10}
            local concurrent=${param2:-5}
            log_info "Running load test..."
            load_test_revocation "$iterations" "$concurrent"
            ;;
        "help"|"-h"|"--help")
            echo "Usage: $0 [MODE] [PARAM1] [PARAM2]"
            echo ""
            echo "Modes:"
            echo "  single       - Run complete token lifecycle test (default)"
            echo "  load         - Run load test with concurrent revocations"
            echo "  help         - Show this help message"
            echo ""
            echo "Parameters for load mode:"
            echo "  PARAM1       - Number of iterations (default: 10)"
            echo "  PARAM2       - Concurrent requests per iteration (default: 5)"
            echo ""
            echo "Examples:"
            echo "  $0 single"
            echo "  $0 load 20 10      # 20 iterations with 10 concurrent requests each"
            echo "  $0 load 100 50     # Heavy load test"
            return 0
            ;;
        *)
            log_error "Unknown mode: $test_mode"
            echo "Use '$0 help' for usage information"
            return 1
            ;;
    esac
    
    echo ""
    print_summary
}

# Run main function with arguments
main "$@"
