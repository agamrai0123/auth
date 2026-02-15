# Quick Reference & Developer Cheat Sheet

**Last Updated:** February 15, 2026

---

## 🚀 QUICK START (5 Minutes)

### 1. Environment Setup
```bash
# Copy default config
cp .env.example .env

# Edit with your values
nano .env

# Set JWT secret
export JWT_SECRET="your-secret-key-min-32-chars"
```

### 2. Start Service
```bash
# Build
go build -o auth-service

# Run
./auth-service

# Docker
docker-compose up -d
```

### 3. Verify Running
```bash
# Check health
curl http://localhost:9090/health

# Get metrics
curl http://localhost:9090/metrics | head -20
```

---

## 📡 API QUICK REFERENCE

### Get Token
```bash
curl -X POST https://localhost:8443/token \
  -H "Content-Type: application/json" \
  -d '{
    "client_id": "my-app",
    "client_secret": "secret123",
    "grant_type": "client_credentials",
    "scope": "user:read"
  }' \
  --insecure
```

**Response:**
```json
{
  "access_token": "eyJ...",
  "token_type": "Bearer",
  "expires_in": 3600
}
```

### Validate Token
```bash
TOKEN="eyJ..."  # From response above

curl -X POST https://localhost:8443/validate \
  -H "Content-Type: application/json" \
  -d "{\"token\": \"$TOKEN\"}" \
  --insecure
```

### Revoke Token
```bash
curl -X POST https://localhost:8443/revoke \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"token\": \"$TOKEN\"}" \
  --insecure
```

### Get User Permissions
```bash
curl -X GET https://localhost:8443/user-privilege \
  -H "Authorization: Bearer $TOKEN" \
  --insecure
```

---

## 🔧 COMMON TASKS

### Check Logs
```bash
# Last 20 lines
tail -20 log/auth-server.log

# Watch in real-time
tail -f log/auth-server.log

# Search for errors
grep "ERROR" log/auth-server.log

# Last hour of logs
tail -c 1M log/auth-server.log
```

### Restart Service
```bash
# Development
# 1. Press Ctrl+C
# 2. Run again: ./auth-service

# Docker
docker-compose restart

# Systemd
sudo systemctl restart auth-service
```

### Database Queries
```sql
-- Check active clients
SELECT client_id, active FROM MV_CLIENTS;

-- Check token expiry
SELECT COUNT(*) FROM MV_TTL WHERE expires_at > SYSDATE;

-- Find revoked tokens
SELECT COUNT(*) FROM MV_TTL WHERE revoked_at IS NOT NULL;

-- Check user privileges
SELECT user_id, privilege_code FROM MV_USER_PRIV;
```

### View Metrics
```bash
# All metrics
curl http://localhost:9090/metrics

# Filter by keyword
curl http://localhost:9090/metrics | grep auth_requests

# Parse with jq (if available)
curl http://localhost:9090/metrics | grep auth_ | head -10
```

### Test Database Connection
```bash
# From command line (if sqlplus available)
sqlplus system/password@localhost:1521/XE

# From Go
go test -run TestDatabaseConnection
```

---

## 🐛 DEBUG TIPS

### Enable Verbose Logging
```bash
export LOG_LEVEL=-1
./auth-service
```

### Check Port Availability
```bash
# Linux/Mac
lsof -i :8080

# Windows
netstat -ano | findstr :8080
```

### Monitor in Real-Time
```bash
# Watch logs
tail -f log/auth-server.log | grep -E "ERROR|WARN"

# Monitor metrics
watch -n 1 'curl -s http://localhost:9090/metrics | grep auth_'

# Database connections
while true; do curl -s http://localhost:9090/metrics | grep db_pool; sleep 5; done
```

### Test Token Generation Speed
```bash
# Single request with timing
curl -X POST https://localhost:8443/token \
  -H "Content-Type: application/json" \
  -d '{"client_id":"test","client_secret":"test","grant_type":"client_credentials"}' \
  -w "\nTime: %{time_total}s\n" \
  --insecure

# Load test (100 concurrent)
ab -n 100 -c 10 -p data.json -T application/json https://localhost:8443/token
```

---

## 📊 MONITORING DASHBOARD (Prometheus/Grafana)

### Key Queries for Grafana

**Token Generation Rate (req/s):**
```promql
rate(auth_requests_total{endpoint="/token"}[1m])
```

**Error Rate (%):**
```promql
100 * sum(rate(auth_errors_total[5m])) / sum(rate(auth_requests_total[5m]))
```

**Token Generation Latency (p95):**
```promql
histogram_quantile(0.95, rate(auth_request_duration_seconds_bucket{endpoint="/token"}[5m]))
```

**Cache Hit Rate (%):**
```promql
100 * sum(auth_token_cache_hits_total) / sum(auth_token_cache_misses_total)
```

**Database Query Latency (p99):**
```promql
histogram_quantile(0.99, rate(auth_db_query_duration_seconds_bucket[5m]))
```

---

## 🔐 SECURITY CHECKLIST

### Before Production Deployment

- [ ] JWT secret set in env var (not hardcoded)
- [ ] CORS origins configured (not wildcard)
- [ ] HTTPS enabled with valid certificate
- [ ] Database password in env var
- [ ] Token TTL: 3600 seconds (1 hour)
- [ ] Rate limiting: enabled
- [ ] TLS 1.2+ minimum
- [ ] Input validation: all endpoints
- [ ] Error messages: no sensitive data
- [ ] Logs: no secrets or tokens

### Environment Variables Checklist
```bash
✓ JWT_SECRET set
✓ DB_PASSWORD set
✓ CERT_FILE exists
✓ KEY_FILE exists
✓ SERVER_PORT accessible
✓ HTTPS_ENABLED=true
✓ LOG_LEVEL=0 (INFO)
```

---

## 📋 TESTING CHECKLIST

### Unit Tests
```bash
# Run all tests
go test ./... -v

# Run specific test
go test ./auth -run TestTokenGeneration -v

# With coverage
go test ./... -cover

# Coverage report
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### Integration Tests
```bash
# Start service
./auth-service &

# Run integration tests
go test -tags=integration ./... -v

# Test with real database
DB_HOST=localhost DB_PORT=1521 go test ./... -v
```

### Manual Testing
```bash
# 1. Get token
TOKEN=$(curl -s -X POST ... | jq -r '.access_token')

# 2. Validate token
curl -s -X POST ... -d "{\"token\": \"$TOKEN\"}" | jq .

# 3. Revoke token
curl -s -X POST ... -H "Authorization: Bearer $TOKEN" | jq .

# 4. Try revoked token (should fail)
curl -s -X POST ... -d "{\"token\": \"$TOKEN\"}" | jq .
```

---

## 🚨 TROUBLESHOOTING QUICK GUIDE

| Problem | Diagnosis | Solution |
|---------|-----------|----------|
| **Server won't start** | Check logs for error message | See logs section above |
| **Can't connect to database** | `DB connection failed` in logs | Verify Oracle listener, host, port |
| **Getting "token expired"** | Token TTL too short | Increase TOKEN_EXPIRES_IN in config |
| **CORS errors** | Origins not whitelisted | Add to CORS allowed_origins |
| **Slow token generation** | Check `auth_request_duration_seconds` metric | Check DB query latency, increase pool |
| **Memory growing** | Check cache stats | Reduce cache.max_entries |
| **High error rate** | Check error types in prometheus | Look at error logs for cause |
| **Port already in use** | `bind: address already in use` | Kill existing process or use different port |

---

## 💡 PERFORMANCE TUNING

### For High Load (10k req/s +)

```json
{
  "database": {
    "pool": {
      "max_open": 200,
      "max_idle": 50,
      "max_lifetime": 600
    }
  },
  "cache": {
    "ttl_seconds": 600,
    "max_entries": 100000
  },
  "batch_write": {
    "enabled": true,
    "batch_size": 5000,
    "flush_interval_ms": 1000
  }
}
```

### For Low Latency (<50ms P95)

```json
{
  "cache": {
    "enabled": true,
    "ttl_seconds": 300,
    "max_entries": 50000
  },
  "server": {
    "timeout": 10
  }
}
```

### For Limited Resources (1GB RAM)

```json
{
  "database": {
    "pool": {
      "max_open": 20,
      "max_idle": 10
    }
  },
  "cache": {
    "max_entries": 5000,
    "ttl_seconds": 120
  }
}
```

---

## 📚 FILE STRUCTURE REFERENCE

```
auth/
├── main.go                   # Server entry point
├── config.go                 # Configuration loading
├── handlers.go               # HTTP endpoint handlers (4 endpoints)
├── service.go                # Business logic layer
├── database.go               # Database operations (Oracle)
├── cache.go                  # Token TTL cache
├── tokens.go                 # JWT generation & validation
├── logger.go                 # Structured logging setup
├── metrics.go                # Prometheus metrics
├── models.go                 # Data structures
├── errors.go                 # Custom error types
├── routes.go                 # Route definitions
├── auth_test.go              # Unit + integration tests
├── config/
│   ├── config.json           # Main configuration
│   ├── server.crt            # TLS certificate
│   └── server.key            # TLS private key
├── log/                      # Log files (auto-created)
└── schema.sql                # Database DDL
```

---

## 🔗 USEFUL LINKS

- **Go Docs:** https://pkg.go.dev/
- **Gin Docs:** https://gin-gonic.com/docs/
- **Zerolog Docs:** https://pkg.go.dev/github.com/rs/zerolog
- **Prometheus Docs:** https://prometheus.io/docs/
- **OAuth2 Spec:** https://tools.ietf.org/html/rfc6749
- **JWT Spec:** https://tools.ietf.org/html/rfc7519

---

## 📞 GETTING HELP

1. **Check logs:** `tail -f log/auth-server.log`
2. **Check metrics:** `curl http://localhost:9090/metrics`
3. **Check database:** Run SQL queries to verify data
4. **Read docs:** See PROJECT_DOCUMENTATION.md
5. **Check security audit:** See SECURITY_AUDIT_REPORT.md

### Common Error Patterns

| Error | Common Causes | Fix |
|-------|---------------|-----|
| `ORA-12514` | Listener down | Start Oracle listener |
| `invalid_client` | Wrong credentials | Verify client_id/secret |
| `rate_limited` | Too many requests | Wait or increase rate limit |
| `token_expired` | TTL too short | Increase TOKEN_EXPIRES_IN |
| `insufficient_scope` | Missing permission | Add scope to client |

---

## 🎯 PERFORMANCE TARGETS

| Metric | Target | Alert Threshold |
|--------|--------|-----------------|
| Token generation latency (p95) | <100ms | >500ms |
| Token validation latency (p95) | <50ms | >200ms |
| Error rate | <0.1% | >1% |
| Database query latency (p95) | <50ms | >200ms |
| Cache hit rate | >90% | <70% |
| Availability | >99.9% | <99% |

---

**Keep this cheat sheet handy for quick reference!**
