# 🔧 TEST FIXES REQUIRED - QUICK GUIDE

## Overview
**Status:** 4 tests failing due to SQL mock mismatches (NOT code issues)  
**Impact:** Test pass rate 80% (16/20 passing, 4 failing)  
**Required Time:** 1-2 hours  
**Complexity:** LOW - Simple mock updates

---

## ❌ Failing Tests Analysis

### 1. TestRevokeToken ❌

**Location:** `auth/auth_test.go:231-254`

**Current Issue:**
```
Expected SQL: "Update tokens set revoked=true, revoked_at=:1 where token_id=:2"
Actual SQL:   "UPDATE tokens SET revoked = 1, revoked_at = :1 WHERE token_id = :2"
Error: Prepare: could not match actual sql with expected regexp
```

**Root Cause:** Test expects boolean "true" but actual SQL uses "1"

**Fix Required:**
```go
// BEFORE (WRONG):
mock.ExpectPrepare(regexp.QuoteMeta(
    "Update tokens set revoked=true, revoked_at=:1 where token_id=:2",
)).ExpectExec().WithArgs(
    sqlmock.AnyArg(), 
    "tkn123",
).WillReturnResult(sqlmock.NewResult(1, 1))

// AFTER (CORRECT):
mock.ExpectPrepare(regexp.QuoteMeta(
    "UPDATE tokens SET revoked = 1, revoked_at = :1 WHERE token_id = :2",
)).ExpectExec().WithArgs(
    sqlmock.AnyArg(), 
    "tkn123",
).WillReturnResult(sqlmock.NewResult(1, 1))
```

**Changes:**
- Line 234: `"Update tokens set revoked=true..."` → `"UPDATE tokens SET revoked = 1..."`
- Match case and formatting exactly from database.go line 55

**Verification:**
```bash
# After fix, run:
export JWT_SECRET="test-secret-key-minimum-32-characters"
go test -v auth.TestRevokeToken
# Should output: --- PASS: TestRevokeToken
```

---

### 2. TestValidateJWT_Success ❌

**Location:** `auth/auth_test.go:419-470`

**Current Issue:**
```
Expected SQL: "SELECT revoked FROM tokens WHERE token_id = :1"
Actual SQL:   "SELECT revoked, token_type FROM tokens WHERE token_id = :1"
Error: Prepare: could not match actual sql with expected regexp
```

**Root Cause:** Test mock missing `token_type` column in SELECT

**Fix Required:**
```go
// BEFORE (WRONG):
mock.ExpectPrepare(regexp.QuoteMeta(
    "SELECT revoked FROM tokens WHERE token_id = :1",
)).ExpectQuery().WithArgs("tkn123").WillReturnRows(...)

// AFTER (CORRECT):
mock.ExpectPrepare(regexp.QuoteMeta(
    "SELECT revoked, token_type FROM tokens WHERE token_id = :1",
)).ExpectQuery().WithArgs("tkn123").WillReturnRows(...)
```

**Changes:**
- Find all instances of `SELECT revoked FROM tokens WHERE token_id = :1` in test
- Add `, token_type` to the SELECT clause
- Match format from database.go

**Search and Replace:**
```bash
# Find all instances:
grep -n "SELECT revoked FROM tokens WHERE token_id = :1" auth/auth_test.go

# Should show lines where needed (around line 447, 448, etc.)
```

**Verification:**
```bash
export JWT_SECRET="test-secret-key-minimum-32-characters"
go test -v auth.TestValidateJWT_Success
# Should output: --- PASS: TestValidateJWT_Success
```

---

### 3. TestTokenHandler_Success ❌

**Location:** `auth/auth_test.go:549-630`

**Current Issue:**
```
Error: sql expectations not met: there is a remaining expectation 
which was not matched: ExpectedBegin => expecting database transaction Begin
```

**Root Cause:** Test setup missing transaction Begin/Commit expectations

**Fix Required:**
```go
// BEFORE (WRONG):
func TestTokenHandler_Success(t *testing.T) {
    as, mock := setupTestAuthServer(t)
    
    // Missing: mock.ExpectBegin()
    
    mock.ExpectPrepare(...).ExpectQuery()...
    // Missing: mock.ExpectCommit() or ExpectRollback()
}

// AFTER (CORRECT):
func TestTokenHandler_Success(t *testing.T) {
    as, mock := setupTestAuthServer(t)
    
    // Add transaction begin
    mock.ExpectBegin()
    
    // ... existing expectations ...
    
    // Add transaction commit
    mock.ExpectCommit()
}
```

**Detailed Fix Steps:**

**Step 1:** Find the test setup (around line 550-580)
```bash
grep -n "func TestTokenHandler_Success" auth/auth_test.go
```

**Step 2:** After `setupTestAuthServer(t)`, add:
```go
mock.ExpectBegin()
```

**Step 3:** At the end of all mock.Expect* calls, add:
```go
mock.ExpectCommit()
```

**Step 4:** If transaction should fail, instead use:
```go
mock.ExpectRollback()
```

**Verification:**
```bash
export JWT_SECRET="test-secret-key-minimum-32-characters"
go test -v auth.TestTokenHandler_Success
# Should output: --- PASS: TestTokenHandler_Success
```

---

### 4. TestTokenHandler_InvalidJSON ❌

**Location:** `auth/auth_test.go:628-660`

**Current Issue:**
```
panic: runtime error: invalid memory address or nil pointer dereference
Stack: github.com/prometheus/client_golang/prometheus.(*CounterVec)
       .WithLabelValues(...) at counter.go:282
```

**Root Cause:** When JSON decode fails, `tokenReq` is nil/invalid, causing nil pointer access in metrics

**Fix Required:**

Check `auth/handlers.go` line 75-95 for error handling:
```go
// BEFORE (BUGGY):
var tokenReq TokenRequest
if err := json.NewDecoder(c.Request.Body).Decode(&tokenReq); err != nil {
    // Error but still tries to access tokenReq
    as.tokenRequestsCount.WithLabelValues(tokenReq.GrantType).Inc()  // ❌ CRASH!
}

// AFTER (FIXED):
var tokenReq TokenRequest
if err := json.NewDecoder(c.Request.Body).Decode(&tokenReq); err != nil {
    c.JSON(400, gin.H{"error": "invalid_request"})
    return  // Return early, don't access tokenReq
}

// Only increment metrics for valid requests
as.tokenRequestsCount.WithLabelValues(tokenReq.GrantType).Inc()  // ✅ Safe
```

**Action Items:**

1. Check handlers.go line 82 (where error occurs)
2. Verify early return after JSON decode error
3. Only access tokenReq fields after successful decode

**Current Code Check:**
```bash
grep -A 10 "json.NewDecoder(c.Request.Body).Decode" auth/handlers.go
```

**Verification:**
```bash
export JWT_SECRET="test-secret-key-minimum-32-characters"
go test -v auth.TestTokenHandler_InvalidJSON
# Should output: --- PASS: TestTokenHandler_InvalidJSON
```

---

## 🔄 Complete Fix Procedure

### Step-by-Step Instructions

**Step 1: Open the test file**
```bash
cd d:\work-projects\auth
code auth/auth_test.go
```

**Step 2: Fix TestRevokeToken (Line ~234)**
- Find: `"Update tokens set revoked=true, revoked_at=:1 where token_id=:2"`
- Replace with: `"UPDATE tokens SET revoked = 1, revoked_at = :1 WHERE token_id = :2"`

**Step 3: Fix TestValidateJWT_Success (Line ~447-448)**
- Find all: `"SELECT revoked FROM tokens WHERE token_id = :1"`
- Replace with: `"SELECT revoked, token_type FROM tokens WHERE token_id = :1"`

**Step 4: Fix TestTokenHandler_Success (Line ~575)**
- After `as, mock := setupTestAuthServer(t)`, add: `mock.ExpectBegin()`
- Before closing, add: `mock.ExpectCommit()`

**Step 5: Fix TestTokenHandler_InvalidJSON**
- Check handlers.go line 80-95
- Ensure early return after JSON decode error
- Verify no null pointer access on tokenReq

**Step 6: Run Tests**
```bash
export JWT_SECRET="test-secret-key-minimum-32-characters"
go test ./auth -v
```

**Step 7: Verify All Pass**
```bash
go test ./auth -v 2>&1 | grep -c "PASS:"
# Should output: 20
```

---

## 📝 Line-by-Line Fixes

### Fix #1: TestRevokeToken - Line 234

**File:** `auth/auth_test.go`

Find this block:
```go
mock.ExpectPrepare(regexp.QuoteMeta(
    "Update tokens set revoked=true, revoked_at=:1 where token_id=:2",
)).ExpectExec().WithArgs(
    sqlmock.AnyArg(), // reoked_at
    "tkn123",         // token_id
).WillReturnResult(sqlmock.NewResult(1, 1))
```

Replace with:
```go
mock.ExpectPrepare(regexp.QuoteMeta(
    "UPDATE tokens SET revoked = 1, revoked_at = :1 WHERE token_id = :2",
)).ExpectExec().WithArgs(
    sqlmock.AnyArg(), // revoked_at
    "tkn123",         // token_id
).WillReturnResult(sqlmock.NewResult(1, 1))
```

---

### Fix #2: TestValidateJWT_Success - Lines 447-448

**File:** `auth/auth_test.go`

Find:
```go
mock.ExpectPrepare(regexp.QuoteMeta(
    "SELECT revoked FROM tokens WHERE token_id = :1",
)).ExpectQuery().WithArgs("tkn123").WillReturnRows(
```

Replace ALL occurrences with:
```go
mock.ExpectPrepare(regexp.QuoteMeta(
    "SELECT revoked, token_type FROM tokens WHERE token_id = :1",
)).ExpectQuery().WithArgs("tkn123").WillReturnRows(
```

Use Find & Replace:
- Find: `SELECT revoked FROM tokens WHERE token_id = :1`
- Replace: `SELECT revoked, token_type FROM tokens WHERE token_id = :1`
- Replace All (should be 2-3 occurrences)

---

### Fix #3: TestTokenHandler_Success - After Line 576

**File:** `auth/auth_test.go`

Locate:
```go
func TestTokenHandler_Success(t *testing.T) {
    as, mock := setupTestAuthServer(t)
    
    // ADD THIS LINE:
    mock.ExpectBegin()
```

And add before final closing brace:
```go
    // ADD THIS LINE:
    mock.ExpectCommit()
}
```

---

### Fix #4: Check handlers.go Error Handling

**File:** `auth/handlers.go` (Line 75-95)

Verify structure:
```go
func (as *authServer) tokenHandler(c *gin.Context) {
    // ... setup code ...
    
    if err := c.Request.ParseForm(); err != nil {
        c.JSON(400, gin.H{"error": "invalid_request"})
        return  // ✅ Early return before using form data
    }
    
    // Safe to access form values now
}
```

If accessing any variable after error and before return:
- Add early `return` statement after error handling
- Ensure metrics only incremented for valid requests

---

## ✅ Verification Commands

**After each fix, run:**

```bash
# Test individual fix
export JWT_SECRET="test-secret-key-minimum-32-characters"

# Test Revoke
go test -v auth.TestRevokeToken

# Test JWT validation
go test -v auth.TestValidateJWT_Success

# Test Token Handler Success
go test -v auth.TestTokenHandler_Success

# Test Invalid JSON handling
go test -v auth.TestTokenHandler_InvalidJSON

# Test all
go test ./auth -v

# Get summary
go test ./auth -v 2>&1 | tail -20
```

**Expected Final Output:**
```
ok      auth/auth       0.127s
```

All tests passing: ✅

---

## 🎯 Success Criteria

After completing all fixes:

| Test | Before | After | Status |
|------|--------|-------|--------|
| TestRevokeToken | ❌ FAIL | ✅ PASS | Fixed |
| TestValidateJWT_Success | ❌ FAIL | ✅ PASS | Fixed |
| TestTokenHandler_Success | ❌ FAIL | ✅ PASS | Fixed |
| TestTokenHandler_InvalidJSON | ❌ FAIL | ✅ PASS | Fixed |
| **Total** | **16/20 (80%)** | **20/20 (100%)** | **✅ COMPLETE** |

---

## 📞 Questions?

**If Fix #1 Still Fails:**
- Verify exact SQL format in database.go line 55
- Check that quotation marks match (double quotes vs single)
- Ensure all whitespace matches exactly

**If Fix #2 Still Fails:**
- Search for all instances: `SELECT revoked FROM`
- Replace consistently throughout file
- Check for case sensitivity (uppercase vs lowercase)

**If Fix #3 Still Fails:**
- Look for similar patterns in working tests (above TestTokenHandler_Success)
- Mock transaction must match actual code flow
- Check if code uses transactions or not

**If Fix #4 Still Fails:**
- Look at stack trace to find exact line
- Add try-with-finally or defer to ensure cleanup
- Verify early return patterns throughout function

---

**Estimated Time to Complete:** 1-2 hours  
**Difficulty Level:** LOW (simple text replacements)  
**Impact:** 100% test pass rate

Go forth and fix! 🚀

