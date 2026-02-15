# Auth Service Load Tests

This directory contains load testing scripts for all auth service endpoints using the `hey` tool (github.com/rakyll/hey).

## Prerequisites

1. Install `hey` load testing tool:
```bash
go install github.com/rakyll/hey@latest
```

2. Start the auth server:
```bash
cd ..
./auth.exe
# Or: go run main.go
```

3. Ensure the database is running and configured:
- Oracle instance running at `localhost:1521`
- Service: `XE`
- Credentials: `system/abcd1234`

## Endpoints Overview

| Endpoint | Method | Purpose | Status Code |
|----------|--------|---------|------------|
| `/auth-server/v1/oauth/` | GET | Health check | 200 |
| `/auth-server/v1/oauth/token` | POST | Generate normal token | 200 |
| `/auth-server/v1/oauth/ott` | POST | Generate one-time token | 200 |
| `/auth-server/v1/oauth/validate` | POST | Validate token access | 200 |
| `/auth-server/v1/oauth/revoke` | POST | Revoke token | 401/404 |

## Load Test Configurations

All tests use:
- **100,000 requests** per endpoint
- **100 concurrent connections** (-c 100)
- **JSON payloads** for POST requests

## Running Tests

### Individual Endpoint Tests

Run any of the test scripts:

```bash
# Health check (fastest)
bash test_health.sh

# Token generation
bash test_token.sh

# One-time token
bash test_ott.sh

# Token validation
bash test_validate.sh

# Token revocation
bash test_revoke.sh
```

### Run All Tests

```bash
bash run_all_tests.sh
```

This will execute all endpoint tests sequentially and generate a combined report.

## Test Parameters

- `-n 100000` - Total number of requests
- `-c 100` - Number of concurrent connections
- `-m GET/POST` - HTTP method
- `-H "Content-Type: application/json"` - Header for JSON
- `-d '{...}'` - Request payload

## Credentials Used in Tests

- **Client ID**: `test-client`
- **Client Secret**: `test-secret-123` (valid credentials from schema.sql)

## Expected Results

### Health Check Performance
- **Throughput**: ~31,000 req/sec
- **Avg Latency**: 3.2ms
- **Response**: 200 OK (2 bytes)

### Token Generation Performance
- **Throughput**: ~8,000 req/sec
- **Avg Latency**: 12.2ms
- **Response**: 200 OK with JWT token

### One-Time Token Performance
- **Throughput**: ~1,200 req/sec (slowest - database writes)
- **Avg Latency**: 95.6ms
- **Response**: 200 OK with JWT token

### Token Validation Performance
- **Throughput**: ~18,000 req/sec
- **Avg Latency**: 5.3ms
- **Response**: 200 OK or validation error

### Token Revocation Performance
- **Throughput**: ~21,000 req/sec
- **Avg Latency**: 4.7ms
- **Response**: 401 Unauthorized (expected for test token)

## Performance Optimization Tips

1. **Connection Pool**: Increase `max_open_conns` in config.json
2. **Batch Writer**: Token batch size currently 1000, flush interval 5 seconds
3. **Caching**: Token cache TTL is 1 hour
4. **Database**: Use indexes on `token_id`, `client_id`, `expires_at`

## Troubleshooting

### "Connection refused" errors
- Ensure auth server is running on port 8080
- Check firewall settings

### High latency on OTT endpoint
- This is expected (database write operation)
- Use token batcher for efficiency
- Consider read replicas for load distribution

### Client authentication failures (401)
- Verify credentials: `test-client` / `test-secret-123`
- Check database has test data from schema.sql

### Server connection drops
- Increase `MaxOpenConns` in config.json
- Check system file descriptor limits
- Monitor server logs for database connection errors

## Output Files

Each test script generates results in the console. To save results:

```bash
bash test_token.sh > results/token_results.txt 2>&1
```

## Additional Resources

- hey documentation: https://github.com/rakyll/hey
- Auth service documentation: See main README.md
- Performance analysis: See BOTTLENECK_ANALYSIS.md
