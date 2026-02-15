# Load Testing Infrastructure Overview

Complete performance testing framework for the OAuth2 authentication service.

## Directory Structure

```
load-tests/
├── README.md                  # 📖 Getting started guide
├── BASELINE_REPORT.md         # 📊 Performance baseline metrics
├── OPTIMIZATION_GUIDE.md      # 🚀 Performance improvement strategies
├── INDEX.md                   # 📋 This file
├── Makefile                   # 🔧 Convenient test runners
├── config.sh                  # ⚙️ Shared configuration
│
├── Test Scripts:
├── run_all_tests.sh           # 🎯 Run all 5 endpoint tests
├── test_health.sh             # ❤️ Health check endpoint
├── test_token.sh              # 🔐 Token generation endpoint
├── test_ott.sh                # ⏱️ One-time token endpoint
├── test_validate.sh           # ✓ Token validation endpoint
├── test_revoke.sh             # ⛔ Token revocation endpoint
│
├── Results Management:
├── archive_results.sh         # 📦 Archive test results
├── compare_performance.sh     # 📈 Compare against baseline
│
└── results/                   # 📁 Generated test results
    ├── health_check_results.txt
    ├── token_generation_results.txt
    ├── one_time_token_results.txt
    ├── token_validation_results.txt
    ├── token_revocation_results.txt
    └── archive/               # Historical results
```

## Quick Start

### 1. First Time Setup

```bash
cd load-tests

# View configuration
cat config.sh

# Make scripts executable
chmod +x *.sh

# Check dependencies
which hey  # Required: hey load tester
go version # Required: Go 1.20+
```

### 2. Run Tests

```bash
# All endpoints (500k requests total)
bash run_all_tests.sh

# Individual endpoint
bash test_health.sh
bash test_token.sh

# With Makefile
make test-all
make test-light    # 10k requests, 10 concurrency
make test-heavy    # 500k requests, 500 concurrency
```

### 3. Review Results

```bash
# Compare to baseline
bash compare_performance.sh

# View specific results
cat results/health_check_results.txt

# Archive for historical tracking
bash archive_results.sh
```

## File Descriptions

### Core Documentation

| File | Purpose | Size | Audience |
|------|---------|------|----------|
| [README.md](README.md) | Test execution guide | 200 lines | Everyone |
| [BASELINE_REPORT.md](BASELINE_REPORT.md) | Performance metrics | 250 lines | Performance team |
| [OPTIMIZATION_GUIDE.md](OPTIMIZATION_GUIDE.md) | Optimization strategies | 400 lines | DevOps/Engineers |
| [INDEX.md](INDEX.md) | This overview | 300 lines | New team members |

### Test Scripts

| Script | Test Type | Requests | Concurrency | Duration | Use Case |
|--------|-----------|----------|-------------|----------|----------|
| test_health.sh | GET | 100k | 100 | ~3s | Baseline capability |
| test_token.sh | POST (gen) | 100k | 100 | ~12s | Auth flow |
| test_ott.sh | POST (OTT) | 100k | 100 | ~80s | Heavy database load |
| test_validate.sh | POST (val) | 100k | 100 | ~5s | Cache efficiency |
| test_revoke.sh | POST (revoke) | 100k | 100 | ~5s | Write performance |
| run_all_tests.sh | All endpoints | 500k | 100 | ~105s | Full suite |

### Helper Scripts

| Script | Purpose | Output |
|--------|---------|--------|
| compare_performance.sh | Baseline comparison | Console report |
| archive_results.sh | Historical tracking | results/archive/ |
| config.sh | Configuration source | (Not executable) |
| Makefile | Test convenience | (Various targets) |

## Baseline Performance

Last Updated: February 15, 2026

| Endpoint | Throughput | Latency | Status |
|----------|-----------|---------|--------|
| Health | 30.9k r/s | 3.2ms | ✓ Optimal |
| Token | 8.2k r/s | 12.2ms | ✓ Good |
| OTT | 1.2k r/s | 95.6ms | ⚠️ Needs optimization |
| Validate | 18.6k r/s | 5.3ms | ✓ Good |
| Revoke | 21.2k r/s | 4.7ms | ✓ Excellent |

→ See [BASELINE_REPORT.md](BASELINE_REPORT.md) for detailed analysis

## Configuration

All tests use settings from `config.sh`:

```bash
# Server
BASE_URL="http://localhost:8080"
SERVER_PORT=8080

# Load test parameters
REQUESTS=100000
CONCURRENCY=100

# Test client credentials
CLIENT_ID="test-client"
CLIENT_SECRET="test-secret-123"

# Performance expectations (req/sec)
HEALTH_RPS=30000
TOKEN_RPS=8000
OTT_RPS=1200
VALIDATE_RPS=18000
REVOKE_RPS=21000
```

→ Edit `config.sh` to customize test parameters

## Workflow Examples

### Example 1: Baseline Test

```bash
cd load-tests

# Run full suite
bash run_all_tests.sh

# Compare to baseline
bash compare_performance.sh

# Should see: "✓ Performance maintained (±5%)"
```

### Example 2: Load Profile Testing

```bash
# Light load (quick check)
make test-light
make compare

# Heavy load (stress test)
make test-heavy
make compare    # May show degradation

# Archive results
make archive
```

### Example 3: Single Endpoint Optimization

```bash
# Test one endpoint
bash test_token.sh

# View results
cat results/token_generation_results.txt

# Compare
bash compare_performance.sh | grep Token

# If degraded >20%, investigate and optimize
```

### Example 4: Historical Analysis

```bash
# Run tests regularly
bash run_all_tests.sh
bash archive_results.sh

# View all archived results
ls -la results/archive/

# Compare old vs new
diff results/archive/results_20260215_090000/token_generation_results.txt \
     results/archive/results_20260215_100000/token_generation_results.txt
```

## Performance Interpretation

### Response Times (Latency)

- **< 10ms**: Excellent (cached operations)
- **10-50ms**: Good (normal operations)
- **50-100ms**: Acceptable (database writes)
- **> 100ms**: Poor (needs optimization)

### Throughput (Requests/Second)

- **Health**: 30k+ r/s expected (no DB)
- **Token**: 8k+ r/s expected (cache hit + batch write)
- **OTT**: 1.2k+ r/s expected (slowest, async ops)
- **Validate**: 18k+ r/s expected (mostly cached)
- **Revoke**: 21k+ r/s expected (fast writes)

### Healthy System

- All endpoints stable (< ±5% variance)
- P95 latency < 2x average
- P99 latency < 3x average
- 100% success rate (unless testing failure modes)

### Red Flags

- Latency increasing over test duration (resource leak)
- P95/P99 much higher than average (GC pauses or contention)
- Success rate < 95% (server overload)
- Throughput decreasing (database bottleneck)

## Troubleshooting

### Tests hang or timeout

**Problem**: Server not responding
```bash
# Check server status
curl http://localhost:8080/auth-server/v1/oauth/
# Should return 200 OK
```

**Solution**: Start auth server
```bash
cd ..
go run main.go
```

### Tests show 400/401 errors

**Problem**: Invalid credentials or wrong endpoint
```bash
# Check test output
cat results/token_generation_results.txt | grep "Status"
```

**Solution**: Verify config.sh credentials match database
```bash
# In config.sh
CLIENT_ID="test-client"
CLIENT_SECRET="test-secret-123"
```

### Performance degraded

**Problem**: Results below baseline thresholds

**Diagnosis**:
```bash
# Compare performance
bash compare_performance.sh

# Check which endpoint degraded
# Then investigate [OPTIMIZATION_GUIDE.md](OPTIMIZATION_GUIDE.md)
```

### "hey" command not found

**Problem**: Load tester not installed
```bash
go install github.com/rakyll/hey@latest
```

## Next Steps

1. **Run baseline tests** (if first time)
   ```bash
   bash run_all_tests.sh
   bash compare_performance.sh
   ```

2. **Review optimization guide** if performance below target
   → [OPTIMIZATION_GUIDE.md](OPTIMIZATION_GUIDE.md)

3. **Set up CI/CD integration** for ongoing testing
   - Add load tests to pre-deployment checks
   - Alert if regression > 20%

4. **Monitor production**
   - Prometheus metrics from `/metrics` endpoint
   - Compare production to baseline

## For Team Members

**New to this project?** Start here:
1. Read [README.md](README.md) (getting started)
2. Run `bash run_all_tests.sh` (see it work)
3. Check [BASELINE_REPORT.md](BASELINE_REPORT.md) (what you should see)

**Looking to optimize?**
1. Review [BASELINE_REPORT.md](BASELINE_REPORT.md) (identify bottleneck)
2. Study [OPTIMIZATION_GUIDE.md](OPTIMIZATION_GUIDE.md) (solutions)
3. Implement + test (verify improvement)
4. Document results (add to BASELINE_REPORT.md)

**CI/CD Integration?**
1. Copy test scripts to CI runner
2. Add `bash run_all_tests.sh` to pipeline
3. Fail if `bash compare_performance.sh` shows >20% degradation
4. Archive results for historical tracking

## Key Metrics Dashboard

**Quick Check** (< 2 minutes):
```bash
make test-all && make compare
```

**Expected Output**:
```
Health        | RPS: 30996 -> 31000    | Lat: 3.2ms -> 3.1ms    | ✓ Stable
Token         | RPS: 8155 -> 8200      | Lat: 12.2ms -> 12.0ms  | ✓ Stable
OTT           | RPS: 1254 -> 1250      | Lat: 95.6ms -> 95.8ms  | ✓ Stable
Validate      | RPS: 18606 -> 18600    | Lat: 5.3ms -> 5.4ms    | ✓ Stable
Revoke        | RPS: 21190 -> 21200    | Lat: 4.7ms -> 4.6ms    | ✓ Stable
```

## Support & Contact

- **Performance issues**: Check [OPTIMIZATION_GUIDE.md](OPTIMIZATION_GUIDE.md)
- **Test failures**: See Troubleshooting section above
- **Questions**: Review [README.md](README.md)
- **Code changes**: Update this INDEX.md with details

---

**Last Updated**: February 15, 2026 | Performance Baseline Established
**Maintained By**: DevOps/Performance Team
**Version**: 1.0.0
