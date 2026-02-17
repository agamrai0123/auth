# Implementation Roadmap & Fixes

**Date:** February 15, 2026  
**Status:** Ready for Implementation  
**Estimated Total Time:** 20-30 hours

---

## 📋 PRIORITY MATRIX

```
        IMPACT
         ↑
         │   CRITICAL FIXES    │    QUALITY
         │   (Do First)        │    (Nice to Have)
    HIGH │───────────────────┼──────────────────
         │   1,2,3,4,5       │    11,14,15
         │                   │
    MED  │───────────────────┼──────────────────
         │   6,7,8,9,10      │    12,13
         │                   │
         └─────────────────────────────────────→ EFFORT
           LOW           MEDIUM          HIGH
```

---

## 🔴 PHASE 1: CRITICAL SECURITY FIXES (Must Do - 2-3 hours)

### Fix #1: Remove Hardcoded JWT Secret
**Status:** NOT STARTED  
**Priority:** 🔴 CRITICAL  
**Effort:** 15 minutes  
**Risk:** HIGH if not fixed  
**Complexity:** EASY

**Files to Modify:**
- [auth/service.go](auth/service.go) - Remove hardcoded secret
- [auth/config.go](auth/config.go) - Add env var loading

**Changes:**

```go
// BEFORE (auth/service.go)
var JWTsecret = []byte("67d81e2c5717548a4ee1bd1e81395746")

// AFTER (auth/config.go)
func getJWTSecret() []byte {
    secret := os.Getenv("JWT_SECRET")
    if secret == "" {
        log.Fatal().Msg("JWT_SECRET environment variable not set")
    }
    if len(secret) < 32 {
        log.Error().Msg("JWT_SECRET too short (minimum 32 bytes)")
    }
    return []byte(secret)
}

// auth/service.go
func init() {
    JWTsecret = getJWTSecret()
}
```

**Testing:**
```bash
# Should fail
./auth-service
# Expected: FATAL JWT_SECRET environment variable not set

# Should work
export JWT_SECRET="12345678901234567890123456789012"
./auth-service
```

**Documentation:** Update README.md with JWT_SECRET requirement

---

### Fix #2: Correct Token Expiration Times
**Status:** NOT STARTED  
**Priority:** 🔴 CRITICAL  
**Effort:** 10 minutes  
**Risk:** MEDIUM - affects user experience  
**Complexity:** EASY

**Files to Modify:**
- [auth/handlers.go](auth/handlers.go) - Line 129
- [auth/tokens.go](auth/tokens.go) - Lines 25-31
- [config/config.json](config/config.json)

**Changes:**

```go
// BEFORE (auth/handlers.go line 129)
ExpiresIn: 2 * 60,  // 2 min for testing

// AFTER
ExpiresIn: int((time.Hour).Seconds()),  // 3600 (1 hour)

// Or from config:
ExpiresIn: AppConfig.Token.ExpiresIn,  // Default: 3600
```

```go
// BEFORE (auth/tokens.go lines 25-31)
if tokenType == "O" {
    expiresAt = now.Add(time.Hour * 2)      // 2 hours
} else {
    expiresAt = now.Add(time.Minute * 2)    // 2 minutes (WRONG!)
}

// AFTER
if tokenType == "O" {
    expiresAt = now.Add(30 * time.Minute)   // 30 minutes
} else {
    expiresAt = now.Add(time.Hour)          // 1 hour
}
```

```json
// config/config.json
{
  "token": {
    "expires_in": 3600,
    "ott_expires_in": 1800
  }
}
```

**Testing:**
```bash
# Get token and decode
TOKEN=$(curl -s -X POST ... | jq -r '.access_token')
echo $TOKEN | jq -R 'split(".")[1] | @base64d | fromjson'
# Check "exp" claim - should be ~1 hour from "iat"
```

---

### Fix #3: Restrict CORS Origins
**Status:** NOT STARTED  
**Priority:** 🔴 CRITICAL  
**Effort:** 30 minutes  
**Risk:** MEDIUM - breaks cross-domain clients  
**Complexity:** MEDIUM

**Files to Modify:**
- [auth/logger.go](auth/logger.go) - CORS middleware
- [config/config.json](config/config.json)
- [auth/models.go](auth/models.go)

**Changes:**

```go
// BEFORE (auth/logger.go)
w.Header().Set("access-control-allow-origin", "*")

// AFTER (auth/logger.go)
func CORSMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        origin := c.Request.Header.Get("Origin")
        
        // Load from config
        allowedOrigins := AppConfig.CORS.AllowedOrigins
        
        // Check if origin is allowed
        for _, allowed := range allowedOrigins {
            if origin == allowed {
                c.Header("Access-Control-Allow-Origin", origin)
                break
            }
        }
        
        c.Header("Access-Control-Allow-Methods", 
            strings.Join(AppConfig.CORS.AllowedMethods, ", "))
        c.Header("Access-Control-Allow-Headers", 
            strings.Join(AppConfig.CORS.AllowedHeaders, ", "))
        c.Header("Access-Control-Max-Age", 
            fmt.Sprintf("%d", AppConfig.CORS.MaxAge))
        
        if c.Request.Method == http.MethodOptions {
            c.AbortWithStatus(204)
            return
        }
        c.Next()
    }
}

// auth/models.go
type CORSConfig struct {
    AllowedOrigins []string `json:"allowed_origins"`
    AllowedMethods []string `json:"allowed_methods"`
    AllowedHeaders []string `json:"allowed_headers"`
    MaxAge         int      `json:"max_age"`
}
```

```json
// config/config.json
{
  "cors": {
    "allowed_origins": [
      "https://trusted-domain.com",
      "https://app.domain.com"
    ],
    "allowed_methods": ["GET", "POST", "OPTIONS"],
    "allowed_headers": ["Authorization", "Content-Type"],
    "max_age": 86400
  }
}
```

**Testing:**
```bash
# Should work
curl -H "Origin: https://trusted-domain.com" \
  -H "Access-Control-Request-Method: POST" \
  https://localhost:8443/token -v

# Should be blocked (no Allow header)
curl -H "Origin: https://untrusted.com" \
  -H "Access-Control-Request-Method: POST" \
  https://localhost:8443/token -v
```

---

### Fix #4: Input Validation
**Status:** NOT STARTED  
**Priority:** 🔴 CRITICAL  
**Effort:** 45 minutes  
**Risk:** MEDIUM - may reject valid requests  
**Complexity:** MEDIUM

**Files to Modify:**
- [auth/models.go](auth/models.go)
- [auth/handlers.go](auth/handlers.go)

**Changes:**

```go
// auth/models.go
type TokenRequest struct {
    ClientID     string `json:"client_id" validate:"required,max=100"`
    ClientSecret string `json:"client_secret" validate:"required,max=200"`
    GrantType    string `json:"grant_type" validate:"required,oneof=client_credentials"`
    Scope        string `json:"scope" validate:"max=500"`
    RequestID    string `json:"request_id" validate:"max=100"`
}

func (tr *TokenRequest) Validate() error {
    // Length checks
    if len(tr.ClientID) > 100 {
        return fmt.Errorf("client_id too long (max 100)")
    }
    if len(tr.ClientSecret) > 200 {
        return fmt.Errorf("client_secret too long")
    }
    
    // Required fields
    if tr.ClientID == "" {
        return fmt.Errorf("client_id is required")
    }
    if tr.ClientSecret == "" {
        return fmt.Errorf("client_secret is required")
    }
    
    // Validate grant type
    validGrants := []string{"client_credentials"}
    valid := false
    for _, g := range validGrants {
        if tr.GrantType == g {
            valid = true
            break
        }
    }
    if !valid {
        return fmt.Errorf("invalid grant_type")
    }
    
    // Validate scopes (if provided)
    if tr.Scope != "" {
        for _, scope := range strings.Fields(tr.Scope) {
            if !isValidScope(scope) {
                return fmt.Errorf("invalid scope: %s", scope)
            }
        }
    }
    
    return nil
}

// auth/handlers.go
func PostToken(c *gin.Context) {
    var tokenReq TokenRequest
    if err := c.ShouldBindJSON(&tokenReq); err != nil {
        return error("invalid_request", err.Error())
    }
    
    // ADD THIS
    if err := tokenReq.Validate(); err != nil {
        return error("invalid_request", err.Error())
    }
    
    // ... rest of function
}
```

**Testing:**
```bash
# Should fail - empty client_id
curl -X POST ... -d '{"client_secret":"x"}'

# Should fail - client_id too long
curl -X POST ... -d '{"client_id":"'$(python3 -c 'print("x"*101)')'","client_secret":"x"}'

# Should work
curl -X POST ... -d '{"client_id":"app","client_secret":"secret","grant_type":"client_credentials"}'
```

---

### Fix #5: Database Password from Environment
**Status:** NOT STARTED  
**Priority:** 🔴 CRITICAL  
**Effort:** 20 minutes  
**Risk:** LOW - env vars safer  
**Complexity:** EASY

**Files to Modify:**
- [config/config.json](config/config.json)
- [auth/config.go](auth/config.go)

**Changes:**

```go
// auth/config.go - in LoadConfig()
// BEFORE
c.Database.Password = configData.Database.Password

// AFTER
c.Database.Password = os.Getenv("DB_PASSWORD")
if c.Database.Password == "" {
    // Fallback to config file for backward compatibility
    c.Database.Password = configData.Database.Password
    if c.Database.Password == "" {
        log.Warn().Msg("DB_PASSWORD not set in environment, using config file")
    }
}

// Or require it:
c.Database.Password = os.Getenv("DB_PASSWORD")
if c.Database.Password == "" {
    log.Fatal().Msg("DB_PASSWORD environment variable not set")
}
```

```json
// config/config.json - REMOVE PASSWORD
{
  "database": {
    "user": "system",
    // "password": "REMOVE THIS LINE"
  }
}
```

**Testing:**
```bash
# Should fail
./auth-service
# Expected: FATAL DB_PASSWORD environment variable not set

# Should work
export DB_PASSWORD="your_password"
./auth-service
```

---

## 🟠 PHASE 2: HIGH PRIORITY FIXES (Important - 8-10 hours)

### Fix #6: Rate Limiting
**Status:** NOT STARTED  
**Priority:** 🟠 HIGH  
**Effort:** 2 hours  
**Risk:** LOW  
**Complexity:** MEDIUM

**Implementation:**
1. Choose rate limiter library: `github.com/gin-contrib/ratelimit`
2. Add per-client rate limiting
3. Add global rate limiting
4. Add configuration options
5. Add metrics for rate limit hits

**Files:** Create [auth/ratelimit.go](auth/ratelimit.go)

**Code:**
```go
package auth

import "github.com/gin-contrib/ratelimit"

func RateLimitMiddleware() gin.HandlerFunc {
    // 100 requests per second, burst of 10
    limiter := rate.NewLimiter(100, 10)
    
    return func(c *gin.Context) {
        if !limiter.Allow() {
            c.JSON(429, gin.H{
                "error": "rate_limit_exceeded",
                "message": "Too many requests",
            })
            c.Abort()
            return
        }
        c.Next()
    }
}

// Per-client rate limiting
func PerClientRateLimit() gin.HandlerFunc {
    limiters := make(map[string]*rate.Limiter)
    var mu sync.Mutex
    
    return func(c *gin.Context) {
        clientID := c.PostForm("client_id")
        
        mu.Lock()
        limiter, exists := limiters[clientID]
        if !exists {
            limiter = rate.NewLimiter(10, 2)  // 10 req/s per client
            limiters[clientID] = limiter
        }
        mu.Unlock()
        
        if !limiter.Allow() {
            c.JSON(429, gin.H{"error": "rate_limit_exceeded"})
            c.Abort()
            return
        }
        c.Next()
    }
}
```

---

### Fix #7: Add Rate Limiting Middleware to Routes
**Status:** NOT STARTED  
**Priority:** 🟠 HIGH  
**Effort:** 30 minutes  
**Risk:** LOW  
**Complexity:** EASY

**Files:** [auth/routes.go](auth/routes.go)

```go
// Add to SetupRoutes()
router.Use(RateLimitMiddleware())
router.POST("/token", PerClientRateLimit(), PostToken)
```

---

### Fix #8: Sanitize Sensitive Data in Logs
**Status:** NOT STARTED  
**Priority:** 🟠 HIGH  
**Effort:** 1 hour  
**Risk:** MEDIUM - may break debug logging  
**Complexity:** MEDIUM

**Files:** [auth/logger.go](auth/logger.go)

```go
// Create sanitizer
func SanitizeHeaders(h http.Header) map[string]string {
    safe := make(map[string]string)
    sensitiveHeaders := map[string]bool{
        "Authorization": true,
        "X-API-Key":    true,
        "Cookie":       true,
        "Set-Cookie":   true,
    }
    
    for key, values := range h {
        if sensitiveHeaders[key] {
            safe[key] = "***REDACTED***"
        } else {
            safe[key] = values[0]
        }
    }
    return safe
}

// Use in handlers
log.Debug().
    Str("method", c.Request.Method).
    Any("headers", SanitizeHeaders(c.Request.Header)).
    Msg("Request received")
```

---

### Fix #9: Add Comprehensive Error Context
**Status:** NOT STARTED  
**Priority:** 🟠 HIGH  
**Effort:** 2 hours  
**Risk:** LOW  
**Complexity:** EASY

**Pattern to apply everywhere:**

```go
// BEFORE
if err != nil {
    log.Error().Err(err).Msg("Failed to generate token")
    return nil, err
}

// AFTER
if err != nil {
    log.Error().
        Err(err).
        Str("client_id", client.ClientID).
        Str("grant_type", req.GrantType).
        Int("timestamp", int(time.Now().Unix())).
        Msg("Failed to generate token")
    
    return nil, fmt.Errorf("token generation failed for client %s: %w", 
        client.ClientID, err)
}
```

---

### Fix #10: Add Security Tests
**Status:** NOT STARTED  
**Priority:** 🟠 HIGH  
**Effort:** 3-4 hours  
**Risk:** LOW  
**Complexity:** MEDIUM

**Files:** Create [auth/security_test.go](auth/security_test.go)

```go
package auth

import "testing"

func TestSQLInjection(t *testing.T) {
    payloads := []string{
        "' OR '1'='1",
        "admin' --",
        "' UNION SELECT * --",
    }
    
    for _, payload := range payloads {
        // Should be sanitized or return error
        _, err := getClient(payload)
        if err == nil {
            t.Errorf("SQL injection not prevented: %s", payload)
        }
    }
}

func TestXSSPayloads(t *testing.T) {
    payloads := []string{
        "<script>alert('xss')</script>",
        "javascript:alert('xss')",
    }
    
    for _, payload := range payloads {
        // Should be escaped in response
        resp := validateToken(payload)
        if strings.Contains(resp, "<script>") {
            t.Errorf("XSS payload not escaped: %s", payload)
        }
    }
}

func TestCSRFProtection(t *testing.T) {
    // Test CORS header validation
    req := httptest.NewRequest("POST", "/token", nil)
    req.Header.Set("Origin", "https://evil.com")
    
    w := httptest.NewRecorder()
    // Handle request
    
    allowOrigin := w.Header().Get("Access-Control-Allow-Origin")
    if allowOrigin == "*" || allowOrigin == "https://evil.com" {
        t.Error("CSRF protection failed")
    }
}

func TestAuthenticationBypass(t *testing.T) {
    // Test missing credentials
    _, err := PostToken(nil)
    if err == nil {
        t.Error("Should require authentication")
    }
}

func TestRateLimiting(t *testing.T) {
    // Send 150 requests (limit is 100/s)
    for i := 0; i < 150; i++ {
        resp := SendTokenRequest()
        if i < 100 {
            if resp.StatusCode != 200 {
                t.Error("Rate limit triggered too early")
            }
        } else {
            if resp.StatusCode != 429 {
                t.Error("Rate limiting not working")
            }
        }
    }
}
```

---

## 🟡 PHASE 3: MEDIUM PRIORITY (Quality - 8-12 hours)

### Fix #11: Replace Magic Numbers with Constants
**Status:** NOT STARTED  
**Priority:** 🟡 MEDIUM  
**Effort:** 1 hour  
**Risk:** LOW  
**Complexity:** EASY

**Create:** [auth/constants.go](auth/constants.go)

```go
package auth

import "time"

const (
    // Token TTLs
    NormalTokenTTL      = 1 * time.Hour
    OneTimeTokenTTL     = 30 * time.Minute
    TokenIDLength       = 16
    
    // Database
    DBQueryTimeout      = 5 * time.Second
    MaxDBConnections    = 100
    MaxIdleConnections  = 20
    MaxConnLifetime     = 5 * time.Minute
    
    // Cache
    CacheTTL            = 5 * time.Minute
    MaxCacheEntries     = 10000
    
    // Batch operations
    TokenBatchSize      = 1000
    BatchFlushInterval  = 5 * time.Second
    
    // Request
    MaxRequestSize      = 1 * 1024 * 1024  // 1MB
    RequestTimeout      = 30 * time.Second
    
    // Rate limiting
    RateLimitPerSecond  = 100
    RateLimitBurst      = 10
)
```

---

### Fix #12: Add Comprehensive Documentation
**Status:** ✅ COMPLETED  
**Priority:** 🟡 MEDIUM  
**Effort:** 3 hours  
**Risk:** LOW  
**Complexity:** LOW

**Created:**
- [PROJECT_DOCUMENTATION.md](PROJECT_DOCUMENTATION.md)
- [SECURITY_AUDIT_REPORT.md](SECURITY_AUDIT_REPORT.md)
- [QUICK_REFERENCE.md](QUICK_REFERENCE.md)

---

### Fix #13: Improve Cache Invalidation
**Status:** NOT STARTED  
**Priority:** 🟡 MEDIUM  
**Effort:** 1-2 hours  
**Risk:** MEDIUM  
**Complexity:** MEDIUM

**Files:** [auth/cache.go](auth/cache.go)

```go
// Current implementation is already reasonable, add:
// 1. Atomic operations for concurrent access
// 2. Cleanup goroutine for expired entries
// 3. Cache statistics
// 4. Cache invalidation events (logging)

type TokenCache struct {
    data        map[string]*CacheEntry
    mu          sync.RWMutex
    ttl         time.Duration
    maxEntries  int
    
    // Metrics
    hits        int64
    misses      int64
    evictions   int64
}

func (tc *TokenCache) InvalidateIfExists(id string) bool {
    tc.mu.Lock()
    defer tc.mu.Unlock()
    
    if _, exists := tc.data[id]; exists {
        delete(tc.data, id)
        atomic.AddInt64(&tc.evictions, 1)
        log.Debug().Str("token_id", id).Msg("Token cache invalidated")
        return true
    }
    return false
}

// Start cleanup goroutine in init
func (tc *TokenCache) startCleanup() {
    ticker := time.NewTicker(1 * time.Minute)
    go func() {
        for range ticker.C {
            tc.cleanup()
        }
    }()
}

func (tc *TokenCache) cleanup() {
    tc.mu.Lock()
    defer tc.mu.Unlock()
    
    now := time.Now()
    for id, entry := range tc.data {
        if entry.ExpiresAt.Before(now) {
            delete(tc.data, id)
        }
    }
}
```

---

### Fix #14: Add Connection Pool Metrics
**Status:** NOT STARTED  
**Priority:** 🟡 MEDIUM  
**Effort:** 2 hours  
**Risk:** LOW  
**Complexity:** MEDIUM

**Files:** [auth/metrics.go](auth/metrics.go)

```go
// Add new metrics
var (
    dbPoolConnections = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "db_pool_connections",
            Help: "Database connection pool status",
        },
        []string{"state"},  // open, idle, in_use, max
    )
    
    dbPoolWait = prometheus.NewHistorySummary(...)
    
    cacheStats = prometheus.NewGaugeVec(...)
)

// Monitor connection pool
func monitorConnectionPool() {
    ticker := time.NewTicker(30 * time.Second)
    go func() {
        for range ticker.C {
            if db != nil {
                stats := db.Stats()
                dbPoolConnections.WithLabelValues("open").Set(float64(stats.OpenConnections))
                dbPoolConnections.WithLabelValues("in_use").Set(float64(stats.InUse))
                dbPoolConnections.WithLabelValues("idle").Set(float64(stats.Idle))
                dbPoolConnections.WithLabelValues("max").Set(float64(100))  // configured max
            }
        }
    }()
}
```

---

### Fix #15: Add .env.example File
**Status:** NOT STARTED  
**Priority:** 🟡 MEDIUM  
**Effort:** 30 minutes  
**Risk:** LOW  
**Complexity:** EASY

**Create:** [.env.example](.env.example)

```bash
# Server Configuration
SERVER_PORT=8080
HTTPS_SERVER_PORT=8443
HTTPS_ENABLED=true

# Security
JWT_SECRET=your_secret_key_min_32_chars
DB_PASSWORD=your_database_password

# Database
DB_HOST=localhost
DB_PORT=1521
DB_SERVICE=XE
DB_USER=system

# TLS/HTTPS
CERT_FILE=./config/server.crt
KEY_FILE=./config/server.key

# Token Configuration
TOKEN_EXPIRES_IN=3600
OTT_EXPIRES_IN=1800

# Logging
LOG_LEVEL=-1
LOG_PATH=./log/auth-server.log
LOG_MAX_SIZE_MB=1024

# Metrics
METRICS_ENABLED=true
METRICS_PORT=9090

# Cache
CACHE_ENABLED=true
CACHE_TTL=300
CACHE_MAX_ENTRIES=10000
```

---

## ✅ COMPLETED ITEMS

- ✅ Security Audit Report (SECURITY_AUDIT_REPORT.md)
- ✅ Project Documentation (PROJECT_DOCUMENTATION.md)
- ✅ Quick Reference Guide (QUICK_REFERENCE.md)
- ✅ Implementation Roadmap (This file)

---

## 📅 TIMELINE RECOMMENDATION

### Week 1 - Critical Fixes
- Day 1: Fixes #1-3 (JWT secret, TTL, CORS)
- Day 2: Fixes #4-5 (Input validation, DB password)
- Deploy to staging, test thoroughly

### Week 2 - High Priority
- Day 1-2: Fix #6-7 (Rate limiting)
- Day 2: Fix #8-9 (Log sanitization, error context)
- Day 3: Fix #10 (Security tests)
- Deploy to staging, run security audit

### Week 3 - Medium Priority
- Day 1: Fix #11-12 (Constants, documentation - mostly done)
- Day 2-3: Fix #13-15 (Cache, metrics, .env.example)
- Enhancement deployment

---

## 🔍 VALIDATION CHECKLIST

After each fix, verify:

- [ ] Code compiles without errors
- [ ] Unit tests pass
- [ ] Integration tests pass
- [ ] No new warnings/lint issues
- [ ] Logs look normal (no sensitive data)
- [ ] Performance metrics acceptable
- [ ] Documentation updated

---

## 📊 METRICS TO MONITOR

Track these before/after each phase:

| Metric | Before | After | Goal |
|--------|--------|-------|------|
| Build time | ? | ? | <2s |
| Test coverage | 75% | >80% | >85% |
| Security issues | 8 | ? | 0 |
| Code quality | 7.5/10 | ? | 9/10 |
| Performance latency P95 | <100ms | ? | <50ms |

---

## 🎯 SUCCESS CRITERIA

**Phase 1 Complete When:**
- All 5 critical fixes implemented and tested
- No security warnings from static analysis
- Service passes security audit

**Phase 2 Complete When:**
- All 5 high-priority fixes implemented and tested
- Test coverage >80%
- Performance latency P95 <100ms

**Phase 3 Complete When:**
- All 5 medium-priority fixes implemented
- Documentation complete
- Code quality score 9+/10

---

## 📞 SUPPORT & QUESTIONS

Refer to:
1. SECURITY_AUDIT_REPORT.md for security details
2. PROJECT_DOCUMENTATION.md for usage and API
3. QUICK_REFERENCE.md for common tasks
4. Code comments for implementation details

---

**Ready to implement! Start with Phase 1 Critical Fixes for immediate security improvement.**
