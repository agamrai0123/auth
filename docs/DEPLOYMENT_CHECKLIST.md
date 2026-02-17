# Production Deployment Checklist
## Auth Service v1.0

---

## Pre-Deployment Phase (1-2 days before)

### Security Hardening
- [ ] **TLS Certificates** 
  - Generate or procure CA-signed certificates
  - Update server.crt path in config
  - Update server.key path in config
  - Verify certificate chain with: `openssl verify -CAfile ca-bundle.crt server.crt`
  - Set certificate expiration alerts (60 days before expiry)

- [ ] **JWT_SECRET Configuration**
  - Generate strong secret: `openssl rand -base64 32` (minimum 32 chars)
  - Store in secure vault (AWS Secrets Manager, HashiCorp Vault, etc.)
  - Document rotation schedule (recommend: every 90 days)
  - Test secret loading at startup

- [ ] **Remove Debug Endpoints**
  - Disable pprof endpoints in production build
  - Verify `/debug/pprof/` returns 404
  - Strip debug symbols from binary: `go build -ldflags="-s -w"`

### Configuration Review
- [ ] **Database Configuration**
  - Verify connection string
  - Confirm database is accessible from deployment environment
  - Test with read-only account if possible
  - Set appropriate connection pool limits (200 max recommended)
  - Enable connection timeout (5 seconds default)

- [ ] **Rate Limiting**
  - Set global_rps based on infrastructure (default: 100,000)
  - Set client_rps based on expected client load
  - Test rate limiting boundaries
  - Configure alerts for rate limit violations

- [ ] **Logging Configuration**
  - Set log level to INFO (not DEBUG)
  - Configure log rotation (e.g., 50MB per file)
  - Set up log aggregation (ELK, Splunk, Datadog, etc.)
  - Verify sensitive data not in logs (test with curl requests)
  - Configure log retention policy

### Monitoring & Alerting Setup
- [ ] **Prometheus Integration**
  - Configure Prometheus scrape job for `/metrics`
  - Set up recording rules for performance
  - Create dashboards for:
    - RPS metrics
    - Error rates
    - Latency distributions
    - DB connection pool usage

- [ ] **Alert Rules**
  - Error rate > 1% → Page on-call
  - p99 latency > 500ms → Page on-call
  - DB connections > 180 (90% of 200) → Alert
  - Service down/unreachable → Page immediately

- [ ] **Health Checks**
  - Configure endpoint: `GET /health`
  - Set timeout: 5 seconds
  - Set interval: 30 seconds
  - Failure threshold: 3 consecutive failures

### Testing
- [ ] **Load Testing**
  - Test with 100+ RPS for 5 minutes
  - Verify no hung connections
  - Monitor memory usage
  - Check error rates remain < 0.1%

- [ ] **Security Testing**
  - Verify HTTPS/TLS is enforced
  - Test invalid token rejection
  - Test malformed JSON handling
  - Verify security headers present
  - Run basic penetration testing

- [ ] **Database Testing**
  - Test connection failover
  - Test query timeout behavior
  - Test connection pool recovery
  - Verify data consistency

---

## Deployment Phase (Release Day)

### Pre-Deployment
- [ ] **Backup Current State**
  - Backup database
  - Backup current service configuration
  - Keep previous binary for rollback

- [ ] **Notify Team**
  - Notify support team of deployment window
  - Set up war room (Slack, call bridge)
  - Identify on-call engineer
  - Prepare rollback plan

### Deployment Steps
- [ ] **Stop Old Service**
  - Gracefully shut down current instance
  - Wait for in-flight requests to complete (graceful shutdown timeout: 30s)
  - Verify service stopped: `ps aux | grep auth-service`

- [ ] **Deploy New Binary**
  - Copy new auth-service.exe to deployment directory
  - Verify file permissions (executable)
  - Verify source code version matches: `git log -1 --format="%h %s"`

- [ ] **Set Environment Variables**
  ```bash
  export JWT_SECRET="<strong-secret-from-vault>"
  export DB_PASSWORD="<db-password-from-vault>"
  export ENVIRONMENT="production"
  ```

- [ ] **Start Service**
  - Start service: `./auth-service.exe`
  - Verify startup logs: `tail -100 log/auth-server.log`
  - Check for startup errors

- [ ] **Verify Service**
  - Health check: `curl https://localhost:8443/health` → 200
  - Token generation: `curl -X POST https://localhost:8443/auth-server/v1/oauth/token ...`
  - Metrics available: `curl https://localhost:8443/metrics` → 200

### Post-Deployment Validation
- [ ] **Endpoint Tests**
  ```bash
  # Health
  curl -sk https://localhost:8443/auth-server/v1/oauth/
  
  # Token generation
  curl -sk -X POST https://localhost:8443/auth-server/v1/oauth/token \
    -H "Content-Type: application/json" \
    -d '{"client_id":"test-client","client_secret":"test-secret-123","grant_type":"client_credentials"}'
  
  # Validation
  curl -sk -X POST https://localhost:8443/auth-server/v1/oauth/validate \
    -H "Authorization: Bearer <token>" \
    -H "Content-Type: application/json" \
    -d '{"endpoint_url":"http://api.example.com/v1/users"}'
  ```

- [ ] **Monitoring Checks**
  - Metrics flowing into Prometheus
  - No errors in logs (scan for ERROR level)
  - RPS metrics showing traffic
  - Response times normal

- [ ] **Load Testing** (light)
  - 50 concurrent requests
  - Verify <1% error rate
  - Check response times

---

## Post-Deployment Phase (24 hours)

### Monitoring
- [ ] **24-Hour Observation**
  - Monitor error rates
  - Monitor p50/p95/p99 latencies
  - Monitor DB connection pool usage
  - Check for memory leaks

- [ ] **Log Review**
  - Scan logs for unexpected errors
  - Verify no security incidents
  - Check rate limiting in action
  - Confirm no sensitive data exposed

### Stability Checks
- [ ] **Production Validation**
  - Run integration tests
  - Test all endpoints with real clients
  - Verify token expiration works
  - Test revocation endpoint
  - Verify caching is working (check latency improvements)

- [ ] **Client Communication**
  - Notify API consumers of new service
  - Provide status page link
  - Confirm clients are generating tokens successfully
  - Monitor client error rates

### Documentation
- [ ] **Update Documentation**
  - Document actual RPS achieved
  - Update runbook with new procedures
  - Document any configuration changes
  - Create post-mortem if any issues occurred

---

## Rollback Plan (If Needed)

### Quick Rollback (< 5 minutes)
```bash
# 1. Stop new service
pkill -f auth-service

# 2. Restore environment variables
export JWT_SECRET="<previous-secret>"

# 3. Restore previous binary
cp auth-service.exe.backup auth-service.exe

# 4. Start previous service
./auth-service.exe &

# 5. Verify service
curl -sk https://localhost:8443/health
```

### Data Recovery (If Database Modified)
- [ ] Restore database from pre-deployment backup
- [ ] Verify data consistency
- [ ] Test with sample queries

---

## Ongoing Operations

### Weekly
- [ ] Check error rates
- [ ] Review critical logs
- [ ] Verify backups completed

### Monthly
- [ ] Review security logs
- [ ] Check certificate expiration dates
- [ ] Analyze performance trends
- [ ] Review rate limiting effectiveness

### Quarterly
- [ ] Security audit
- [ ] Performance optimization review
- [ ] Dependency updates
- [ ] Disaster recovery test
- [ ] JWT secret rotation

---

## Incident Response

### Service Down
1. Check service status: `systemctl status auth-service` or `ps aux | grep auth`
2. Check logs: `tail -100 log/auth-server.log`
3. Verify dependencies: Database, network connectivity
4. Execute rollback procedure if needed

### High Error Rate
1. Check recent code changes
2. Check database connectivity
3. Check rate limiting status
4. Execute rollback if persistent

### Performance Degradation
1. Check database connection pool usage
2. Monitor CPU usage
3. Check for memory leaks
4. Review slow queries
5. Consider scaling if necessary

---

## Contact & Support

- **On-Call Engineer**: TBD
- **Database Team**: TBD
- **Security Team**: TBD
- **War Room**: TBD
- **Escalation**: TBD

---

**Prepared By**: [Engineer Name]  
**Date**: February 17, 2026  
**Version**: 1.0  
**Status**: READY FOR DEPLOYMENT ✅
