# Performance Optimization Guide

This guide provides strategies to optimize auth service performance based on baseline metrics and identified bottlenecks.

## Quick Reference: Current Baseline

| Endpoint | Current | Target | Gap |
|----------|---------|--------|-----|
| Health | 31k r/s | 50k r/s | +61% |
| Token | 8.2k r/s | 15k r/s | +83% |
| OTT | 1.3k r/s | 5k r/s | +300% |
| Validate | 18.6k r/s | 30k r/s | +61% |
| Revoke | 21.2k r/s | 30k r/s | +41% |

## Level 1: Low-Effort, High-Impact Optimizations

### 1. Increase Connection Pool Size

**Current Config:**
```go
MaxOpenConns:    1000
MaxIdleConns:    500
MaxLifetime:     15 seconds
MaxIdleTime:     5 minutes
```

**Issue**: OTT endpoint saturates at 79k requests (server drops connections)

**Solution**: Increase limits incrementally

**In auth/service.go:**
```go
db.SetMaxOpenConns(2000)     // Was 1000
db.SetMaxIdleConns(1000)     // Was 500
db.SetMaxLifetime(30 * time.Second)   // Was 15s
db.SetConnMaxIdleTime(10 * time.Minute) // Was 5m
```

**Expected Impact**: 
- OTT: +200% (5k r/s)
- Token: +25% (10.2k r/s)
- Overall: +60% throughput

**Effort**: 5 minutes

---

### 2. Optimize Token Batch Size and Flush Interval

**Current Config (auth/cache.go):**
```go
TokenBatchWriter {
    size:    1000    // Tokens per batch
    timeout: 5 * time.Second
}
```

**Issue**: During heavy load, 5s timeout may cause latency spikes

**Solution**: Adjust to balance throughput vs latency

**Option A - Throughput Focus:**
```go
size:    2000    // Double batch size
timeout: 10 * time.Second  // Longer timeout
```
- Pro: Higher throughput, lower DB load
- Con: Higher latency variance
- Impact: +15% throughput, +5ms latency

**Option B - Latency Focus:**
```go
size:    500     // Half batch size
timeout: 2 * time.Second  // Faster flush
```
- Pro: Lower latency, consistent response times
- Con: More frequent DB writes
- Impact: -10% throughput, -3ms latency

**Recommended for OTT**: Option A (throughput focus)

**Effort**: 5 minutes

---

### 3. Add Request Queueing/Throttling

**Issue**: At 100+ concurrent requests, latency increases exponentially

**Solution**: Implement graceful backpressure

**In auth/handlers.go:**
```go
var (
    tokenChan    = make(chan *TokenRequest, 1000)
    ottChan      = make(chan *OTTRequest, 1000)
)

func init() {
    // Token processor
    for i := 0; i < 10; i++ {
        go func() {
            for req := range tokenChan {
                // Process token request
            }
        }()
    }
}

func tokenHandler(c *gin.Context) {
    select {
    case tokenChan <- &req:
        // Queued
    case <-time.After(100 * time.Millisecond):
        c.JSON(429, gin.H{"error": "Queue full"})
    }
}
```

**Expected Impact**: 
- Smoother latency distribution under load
- Prevents conn drops
- Impact: Better P95/P99 metrics

**Effort**: 30 minutes

---

## Level 2: Medium-Effort Optimizations

### 4. Implement Redis Cache Layer

**Current**: In-memory cache (subject to GC pauses, no persistence)

**Proposed**: Redis for shared caching

**Benefits**:
- Persistent across restarts
- Shareable between service instances
- Better memory management
- Fast (~1ms latency)

**Implementation:**
```go
// auth/redis.go
package auth

import "github.com/redis/go-redis/v9"

type RedisCache struct {
    client *redis.Client
}

func (r *RedisCache) SetToken(ctx context.Context, id string, token *Token) error {
    return r.client.Set(ctx, "token:"+id, token, time.Hour).Err()
}
```

**Expected Impact:**
- Token generation: +30% (better cache hits)
- Validation: +40% (shared cache)
- Overall: +25% throughput

**Effort**: 2-3 hours

---

### 5. Add Database Connection Pooling Library

**Current**: database/sql only

**Proposed**: pgBouncer-like connection pooler (for Oracle)

**Options**:
- pgBouncer (PostgreSQL-only, won't help)
- Implement custom pooler (2+ hours)
- Upgrade connection pool limits (Level 1, done)

**Recommendation**: Continue with current approach for now

---

### 6. Optimize Database Queries

**Current Queries:**

```sql
-- Client lookup (cache hit usually)
SELECT client_id, client_secret, scopes 
FROM clients 
WHERE client_id = ?

-- Token info (cache hit usually)
SELECT token_type, scope, client_id, expires_at 
FROM tokens 
WHERE token_id = ? AND revoked = 0

-- Revoke token
UPDATE tokens SET revoked = 1, revoked_at = ? WHERE token_id = ?

-- Insert token (async batched)
INSERT INTO tokens (token_id, token_type, scope, client_id, ...) VALUES (?, ?, ?, ?, ...)
```

**Optimizations Done**: ✓ All indexes present, queries use bind variables

**Further Optimizations**:

Add covering index for token validation:
```sql
CREATE INDEX idx_token_validate ON tokens(token_id, revoked, expires_at) 
    INCLUDE (token_type, scope, client_id);
```

Impact: -5% query time

**Effort**: 10 minutes (database only, no code changes)

---

## Level 3: High-Effort, Architectural Changes

### 7. Implement Horizontal Scaling

**Current**: Single-instance auth service

**Proposed**: Multi-instance with load balancer

**Architecture:**
```
Load Balancer (nginx)
    ├─→ Auth Server 1
    ├─→ Auth Server 2
    ├─→ Auth Server 3
    └─→ Auth Server 4
            ↓
         Oracle DB
```

**Benefits**:
- Scales to unlimited throughput
- Fault tolerance
- Can reach 500k+ total r/s

**Changes Required**:
1. Shared Redis for cache (addresses Level 2 #4)
2. Nginx load balancer configuration
3. No code changes (auth service is stateless)

**Expected Impact**: 4x throughput (4 instances)

**Effort**: 4-6 hours setup, ongoing maintenance

---

### 8. Implement Async Token Revocation Pooling

**Current**: All OTT tokens revoked in background via goroutine

**Issue**: During heavy load, revocation goroutines queue up

**Solution**: Dedicated revocation worker pool

**In auth/cache.go:**
```go
type RevocationWorker struct {
    queue chan string
    done  chan struct{}
    pool  *sql.DB
}

func NewRevocationWorker(db *sql.DB, workers int) *RevocationWorker {
    rw := &RevocationWorker{
        queue: make(chan string, 10000),
        done:  make(chan struct{}),
        pool:  db,
    }
    for i := 0; i < workers; i++ {
        go rw.worker()
    }
    return rw
}

func (rw *RevocationWorker) worker() {
    for tokenID := range rw.queue {
        rw.pool.Exec(
            "UPDATE tokens SET revoked = 1, revoked_at = ? WHERE token_id = ?",
            time.Now(), tokenID)
    }
}
```

**Expected Impact**: 
- OTT: +150% (5k r/s)
- Revoke: +50% (31k r/s)

**Effort**: 1-2 hours

---

## Recommended Optimization Path

### Phase 1 (Week 1): Quick Wins
1. ✓ Increase connection pool (Level 1.1) - 5 min
2. ✓ Optimize batch sizes (Level 1.2) - 5 min
3. ✓ Impact: ~60% throughput improvement

### Phase 2 (Week 2): Operational Excellence
4. Implement request queueing (Level 1.3) - 30 min
5. Add Redis cache layer (Level 2.4) - 2-3 hours
6. Impact: ~50% additional improvement, better consistency

### Phase 3 (Week 3): Scale
7. Implement revocation pooling (Level 3.8) - 1-2 hours
8. Add horizontal scaling setup (Level 3.7) - 4-6 hours
9. Impact: Unlimited scaling, 4x-10x throughput

## Testing Each Optimization

After each change, re-run baseline:

```bash
cd load-tests
bash run_all_tests.sh
bash compare_performance.sh
```

Compare against [BASELINE_REPORT.md](BASELINE_REPORT.md)

## Monitoring Performance

### Real-time Monitoring

```bash
# Terminal 1: Start auth service
go run main.go

# Terminal 2: Monitor metrics
watch -n 1 'curl -s http://localhost:8080/metrics | grep -E "auth|request|latency"'

# Terminal 3: Run load tests
cd load-tests && bash run_all_tests.sh
```

### Prometheus Metrics to Watch

- `http_requests_total` - Request count by endpoint
- `http_request_duration_seconds` - Latency distribution
- `db_connections_open` - Active connections
- `db_connections_idle` - Idle connections
- `token_cache_hits` - Cache effectiveness
- `token_cache_size` - Memory usage

### Key Performance Indicators (KPIs)

Monitor daily:
- Health endpoint: >30k r/s
- Token endpoint: >8k r/s (target 15k)
- OTT endpoint: >1.2k r/s (target 5k)
- Validation endpoint: >18k r/s (target 30k)
- Revocation endpoint: >21k r/s (target 30k)

Alert if any metric drops >20% from baseline.

## Performance in Production

### Hardware Recommendations

**CPU**: 4+ cores (current load uses 2-3 cores)
- Each core can handle ~20k r/s peak

**Memory**: 2GB minimum (current footprint ~300MB)
- Token cache: ~100MB for 100k tokens
- DB connections: ~150MB (1000 connections)
- Buffer: +500MB for GC headroom

**Disk**: SSD 10GB minimum
- Oracle DB storage
- Log files (rotate daily)

**Network**: 1Gbps minimum
- Current bandwidth: 50-100 Mbps
- Headroom for 10x growth

### Deployment Recommendations

1. Use Linux (better than Windows for sustained load)
2. Set kernel parameters:
   ```bash
   # Increase connection limits
   ulimit -n 100000
   
   # TCP tuning
   sysctl -w net.ipv4.tcp_max_syn_backlog=100000
   sysctl -w net.core.somaxconn=100000
   ```

3. Use process monitoring (supervisord/systemd)
4. Set up automated scaling rules (Kubernetes/Docker)

## Next Steps

1. Pick optimization phase 1 to implement
2. Run load tests after each change
3. Document results in [BASELINE_REPORT.md](BASELINE_REPORT.md)
4. Share performance improvements with team

**Questions?** Refer to [README.md](README.md) for test execution details.
