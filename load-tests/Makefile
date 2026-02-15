# Makefile for Load Testing
# Provides convenient targets for running tests with different configurations

.PHONY: help test test-all test-light test-heavy test-health test-token test-ott test-validate test-revoke compare archive clean

help:
	@echo "Load Testing Targets"
	@echo "==================="
	@echo ""
	@echo "Default Tests (100k requests, 100 concurrency):"
	@echo "  make test-all      - Run all 5 endpoints"
	@echo "  make test-health   - Health check endpoint"
	@echo "  make test-token    - Token generation endpoint"
	@echo "  make test-ott      - One-time token endpoint"
	@echo "  make test-validate - Token validation endpoint"
	@echo "  make test-revoke   - Token revocation endpoint"
	@echo ""
	@echo "Preset Configurations:"
	@echo "  make test-light    - Light load (10k requests, 10 concurrency)"
	@echo "  make test-heavy    - Heavy load (500k requests, 500 concurrency)"
	@echo ""
	@echo "Results Management:"
	@echo "  make compare       - Compare current results to baseline"
	@echo "  make archive       - Archive current test results"
	@echo ""
	@echo "Cleanup:"
	@echo "  make clean         - Remove all test results"
	@echo ""

# Default test configuration
test-all:
	@echo "Running all endpoint tests (100k requests, 100 concurrency)..."
	@bash run_all_tests.sh
	@make compare

test-health:
	@echo "Running health check test..."
	@bash test_health.sh

test-token:
	@echo "Running token generation test..."
	@bash test_token.sh

test-ott:
	@echo "Running one-time token test..."
	@bash test_ott.sh

test-validate:
	@echo "Running token validation test..."
	@bash test_validate.sh

test-revoke:
	@echo "Running token revocation test..."
	@bash test_revoke.sh

# Light load configuration
test-light:
	@echo "Running light load tests (10k requests, 10 concurrency)..."
	@mkdir -p results
	@echo "  Testing health check..."
	@hey -n 10000 -c 10 http://localhost:8080/auth-server/v1/oauth/ > results/light_health.txt
	@echo "  Testing token generation..."
	@hey -n 10000 -c 10 -m POST -H "Content-Type: application/json" -d '{"grant_type":"client_credentials"}' http://localhost:8080/auth-server/v1/oauth/token > results/light_token.txt
	@echo "✓ Light load tests completed"

# Heavy load configuration
test-heavy:
	@echo "Running heavy load tests (500k requests, 500 concurrency)...⚠️ WARNING: This may take 5+ minutes"
	@read -p "Continue? (y/n) " -n 1 -r; \
	if [[ $$REPLY =~ ^[Yy]$$ ]]; then \
		mkdir -p results; \
		echo ""; \
		echo "  Testing health check..."; \
		hey -n 500000 -c 500 http://localhost:8080/auth-server/v1/oauth/ > results/heavy_health.txt; \
		echo ""; \
		echo "  Testing token generation..."; \
		hey -n 500000 -c 500 -m POST -H "Content-Type: application/json" -d '{"grant_type":"client_credentials"}' http://localhost:8080/auth-server/v1/oauth/token > results/heavy_token.txt; \
		echo ""; \
		echo "✓ Heavy load tests completed"; \
	else \
		echo "Cancelled"; \
	fi

# Results management
compare:
	@bash compare_performance.sh

archive:
	@bash archive_results.sh

# Cleanup
clean:
	@echo "Removing all test results..."
	@rm -rf results/
	@echo "✓ Cleaned"

# Convenience targets
test: test-all
status: compare
