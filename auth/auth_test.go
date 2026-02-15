package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func setupTestAuthServer(t *testing.T) (*authServer, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error initializing sqlmock: %v", err)
	}
	// defer db.Close()

	as := &authServer{
		db:        db,
		ctx:       context.Background(),
		jwtSecret: JWTsecret,
		clientCache: &clientCache{
			cache: make(map[string]*Clients),
		},
	}

	// token
	as.tokenRequestsCount, err = registerCounterVecMetric("token_requests_count",
		"total number of token requests",
		"",
		[]string{"token"})
	if err != nil {
		t.Fatal("failed to create prometheus counter vector metric for token_requests_count")
	}

	as.tokenSuccessCount, err = registerCounterVecMetric("token_success_count",
		"total number of issued token success",
		"",
		[]string{"token"})
	if err != nil {
		t.Fatal("failed to create prometheus counter vector metric for token_success_count")
	}

	as.tokenGenerationDuration, err = registerHistogramVecMetric("token_generation_duration_seconds",
		"duration of each token",
		"",
		[]float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		[]string{"token"})
	if err != nil {
		t.Fatal("failed to create prometheus histogram vector metric token_generation_duration_seconds")
	}

	// validate
	as.validateTokenRequestsCount, err = registerCounterVecMetric("validate_token_requests_count",
		"total number of validate token requests",
		"",
		[]string{"token"})
	if err != nil {
		t.Fatal("failed to create prometheus counter vector metric for validate_token_request_count")
	}

	as.validateTokenSuccessCount, err = registerCounterVecMetric("validate_token_success_count",
		"total number of validate token success",
		"",
		[]string{"token"})
	if err != nil {
		t.Fatal("failed to create prometheus counter vector metric for validate_token_success_count")
	}

	as.validateTokenLatency, err = registerHistogramVecMetric("validate_token_latency_seconds",
		"validated token latency",
		"",
		[]float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		[]string{"token"})
	if err != nil {
		t.Fatal("failed to create prometheus histogram vector metric validate_token_latency_seconds")
	}

	// revoke
	as.revokeRequestsCount, err = registerCounterVecMetric("revoke_token_requests_count",
		"total number of revoke token requests",
		"",
		[]string{"token"})
	if err != nil {
		t.Fatal("failed to create prometheus counter vector metric for revoke_token_requests_count")
	}

	as.revokeSuccessCount, err = registerCounterVecMetric("revoke_token_success_count",
		"total number of revoke token success",
		"",
		[]string{"token"})
	if err != nil {
		t.Fatal("failed to create prometheus counter vector metric for revoke_token_success_count")
	}

	as.revokeTokenLatency, err = registerHistogramVecMetric("revoke_token_latency_seconds",
		"revoked token latency",
		"",
		[]float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		[]string{"token"})
	if err != nil {
		t.Fatal("failed to create prometheus histogram vector metric revoke_token_latency_seconds")
	}

	// Initialize token cache and batcher for tests
	as.tokenCache = newTokenCache(1 * time.Hour)
	as.tokenBatcher = NewTokenBatchWriter(as, 1000, 5*time.Second)

	return as, mock
}

// test clientByID : success
func TestClientByID_Success(t *testing.T) {
	as, mock := setupTestAuthServer(t)

	rows := sqlmock.NewRows([]string{"client_id", "client_secret", "access_token_ttl", "allowed_scopes"}).
		AddRow("test-client-1", "test-secret-1", 3600, `["read:ltp", "read:quote"]`)

	mock.ExpectPrepare(
		"SELECT client_id, client_secret, access_token_ttl, allowed_scopes FROM clients WHERE client_id = :1",
	).ExpectQuery().WithArgs("test-client-1").WillReturnRows(rows)

	client, err := as.clientByID("test-client-1")

	if err != nil || client == nil {
		t.Fatal("expected valid client")
	}
}

// test clientByID : DB error
func TestClientByID_DBError(t *testing.T) {
	as, mock := setupTestAuthServer(t)

	mock.ExpectPrepare(
		"SELECT client_id, client_secret, access_token_ttl, allowed_scopes FROM clients WHERE client_id = :1",
	).ExpectQuery().WithArgs("test-client-1").WillReturnError(fmt.Errorf("db error"))

	client, err := as.clientByID("test-client-1")

	if err == nil {
		t.Fatal("expected DB error")
	}

	if client != nil {
		t.Fatal("expected nil client on DB error")
	}
}

// test InsertToken - now queues to tokenBatcher
func TestInsertToken(t *testing.T) {
	as, _ := setupTestAuthServer(t)

	// insertToken now queues to batcher instead of direct DB insert
	err := as.insertToken(Token{
		TokenID:   "tkn123",
		TokenType: "N",
		ClientID:  "test-client-1",
		IssuedAt:  time.Now(),
		ExpiresAt: time.Now().Add(time.Minute),
	})

	if err != nil {
		t.Fatalf("insertToken failed: %v", err)
	}

	// Verify token is queued in batcher
	if as.tokenBatcher.GetPendingCount() != 1 {
		t.Fatalf("expected 1 pending token, got %d", as.tokenBatcher.GetPendingCount())
	}
}

// test getScopeForEndpoint
func TestGetScopeForEndpoint(t *testing.T) {
	as, mock := setupTestAuthServer(t)

	scopeRows := sqlmock.NewRows([]string{
		"scope",
	}).AddRow(
		"read:ltp",
	)

	mock.ExpectPrepare(regexp.QuoteMeta(
		"SELECT scope from endpoints where endpoint_url=:1",
	)).ExpectQuery().WithArgs("http://localhost:8080/ltp").WillReturnRows(scopeRows)

	requestedScope, err := as.getScopeForEndpoint("http://localhost:8080/ltp")
	if err != nil {
		t.Fatalf("scope does not match with endpoint: %v", err)
	}

	if requestedScope != "read:ltp" {
		t.Fatalf("unexpected scope: %s", requestedScope)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations not met: %v", err)
	}
}

func TestGetTokenType(t *testing.T) {
	as, mock := setupTestAuthServer(t)

	mock.ExpectPrepare(regexp.QuoteMeta(
		"SELECT revoked, token_type FROM tokens WHERE token_id = :1",
	)).ExpectQuery().WithArgs("tkn123").WillReturnRows(sqlmock.NewRows([]string{"revoked", "token_type"}).AddRow(0, "N"))

	revoked, tokenType, err := as.getTokenInfo("tkn123")
	if err != nil {
		t.Fatalf("getTokenInfo failed: %v", err)
	}

	if revoked {
		t.Fatalf("unexpected revoked status")
	}

	if tokenType != "N" {
		t.Fatalf("unexpected token type")
	}
}

// test revokeToken
func TestRevokeToken(t *testing.T) {
	as, mock := setupTestAuthServer(t)

	mock.ExpectBegin()
	mock.ExpectPrepare(regexp.QuoteMeta(
		"Update tokens set revoked=true, revoked_at=:1 where token_id=:2",
	)).ExpectExec().WithArgs(
		sqlmock.AnyArg(), // reoked_at
		"tkn123",         // token_id
	).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := as.revokeToken(RevokedToken{
		TokenID:   "tkn123",
		RevokedAt: time.Now(),
	})

	if err != nil {
		t.Fatalf("revokeToken failed: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations not met: %v", err)

	}
}

// test validateClient : success
func TestValidateClient_MissingCredentials(t *testing.T) {
	as, _ := setupTestAuthServer(t)

	client, err := as.validateClient("", "")

	if err == nil || client != nil {
		t.Fatal("expected error for missing credentials")
	}
}

// test validateClient : invalid secret
func TestValidateClient_InvalidSecret(t *testing.T) {
	as, mock := setupTestAuthServer(t)

	rows := sqlmock.NewRows([]string{"client_id", "client_secret", "access_token_ttl", "allowed_scopes"}).
		AddRow("test-client-1", "correct", 3600, `["read:ltp", "read:quote"]`)

	mock.ExpectPrepare(
		"SELECT client_id, client_secret, access_token_ttl, allowed_scopes FROM clients WHERE client_id = :1",
	).ExpectQuery().WithArgs("test-client-1").WillReturnRows(rows)

	client, err := as.validateClient("test-client-1", "wrong-secret")

	if err == nil || client != nil {
		t.Fatal("expected invalid secret error")
	}
}

// test validateClient : cache interaction
func TestValidateClient_CacheHit(t *testing.T) {
	as, _ := setupTestAuthServer(t)

	as.clientCache.Set("test-client-1", &Clients{
		ClientID:     "test-client-1",
		ClientSecret: "test-secret-1",
	})

	client, err := as.validateClient("test-client-1", "test-secret-1")

	if err != nil || client == nil {
		t.Fatal("expected cached client")
	}
}

// test validateGrantType : success
func TestValidateGrantType_Success(t *testing.T) {
	as, _ := setupTestAuthServer(t)

	err := as.validateGrantType("client_credentials")
	if err != nil {
		t.Fatalf("unspported grant type: %v", err)
	}
}

// test validateGrantType : invalid
func TestValidateGrantType_Invalid(t *testing.T) {
	as, _ := setupTestAuthServer(t)

	err := as.validateGrantType("dummy_type")

	if err == nil {
		t.Fatal("expected unsupported grant type error")
	}
}

// test getTokenInfo : N
func TestGetTokenTypeN(t *testing.T) {
	as, mock := setupTestAuthServer(t)

	mock.ExpectPrepare(regexp.QuoteMeta(
		"SELECT revoked, token_type FROM tokens WHERE token_id = :1",
	)).ExpectQuery().WithArgs("tkn123").WillReturnRows(sqlmock.NewRows([]string{"revoked", "token_type"}).AddRow(0, "N"))

	revoked, tokenType, err := as.getTokenInfo("tkn123")
	if err != nil {
		t.Fatalf("getTokenInfo failed: %v", err)
	}

	if revoked {
		t.Fatalf("unexpected revoked status")
	}

	if tokenType != "N" {
		t.Fatalf("unexpected token type")
	}
}

// test getTokenInfo : O
func TestGetTokenTypeO(t *testing.T) {
	as, mock := setupTestAuthServer(t)

	mock.ExpectPrepare(regexp.QuoteMeta(
		"SELECT revoked, token_type FROM tokens WHERE token_id = :1",
	)).ExpectQuery().WithArgs("tkn123").WillReturnRows(sqlmock.NewRows([]string{"revoked", "token_type"}).AddRow(0, "O"))

	revoked, tokenType, err := as.getTokenInfo("tkn123")
	if err != nil {
		t.Fatalf("getTokenInfo failed: %v", err)
	}

	if revoked {
		t.Fatalf("unexpected revoked status")
	}

	if tokenType != "O" {
		t.Fatalf("unexpected token type")
	}
}

// test generateJWT : success
func TestGenerateJWT_Success(t *testing.T) {
	as, mock := setupTestAuthServer(t)

	// insertToken
	mock.ExpectBegin()
	mock.ExpectPrepare(regexp.QuoteMeta(
		"INSERT INTO tokens(token_id, token_type, jwt_token, client_id, issued_at, expires_at) VALUES (:1, :2, :3, :4, :5, :6)",
	)).ExpectExec().WithArgs(
		sqlmock.AnyArg(),
		"N", // token_type (normal)
		sqlmock.AnyArg(),
		"test-client-1",
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
	).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	client := &Clients{
		ClientID:      "test-client-1",
		AllowedScopes: []string{"read:ltp", "read:quote"},
	}

	// token, tokenInfo, err := as.generateJWT("test-client-1", "N")
	token, tokenInfo, err := as.generateJWT(client, "N")
	if err != nil {
		t.Fatalf("generateJWT failed: %v", err)
	}

	if token == "" {
		t.Fatal("token or tokenID empty")
	}

	if tokenInfo == nil {
		t.Fatal("expiresaAt is zero")
	}

	if tokenInfo.ClientID != "test-client-1" {
		t.Fatal("invalid client id", err)
	}

	if tokenInfo.TokenType != "N" {
		t.Fatal("invalid token type", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations not met: %v", err)
	}
}

// test validateJWT : success
func TestValidateJWT_Success(t *testing.T) {
	as, mock := setupTestAuthServer(t)

	// JWT token
	now := time.Now()
	claims := Claims{
		ClientID: "test-client-1",
		TokenID:  "tkn123",
		Scopes:   []string{"read:ltp", "write:quote"},
		RegisteredClaims: jwt.RegisteredClaims{
			// ExpiresAt: jwt.NewNumericDate(expiresAt),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute * 5)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "auth-server",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(as.jwtSecret)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	// isTokenRevoked
	mock.ExpectPrepare(regexp.QuoteMeta(
		"SELECT revoked FROM tokens WHERE token_id = :1",
	)).ExpectQuery().WithArgs("tkn123").WillReturnRows(sqlmock.NewRows([]string{"revoked"}).AddRow(false))

	// getTokenType
	mock.ExpectPrepare(regexp.QuoteMeta(
		"SELECT token_type from tokens where token_id=:1",
	)).ExpectQuery().WithArgs("tkn123").WillReturnRows(sqlmock.NewRows([]string{"token_type"}).AddRow("N"))

	// call validateJWT
	tokenClaims, err := as.validateJWT(tokenString)
	if err != nil {
		t.Fatalf("validateJWT failed: %v", err)
	}

	if tokenClaims.ClientID != "test-client-1" {
		t.Fatalf("unexpected clientID: %s", tokenClaims.ClientID)
	}

	if tokenClaims.TokenID != "tkn123" {
		t.Fatalf("unexpected tokenID: %s", tokenClaims.TokenID)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations not met: %v", err)
	}
}

// test validateJWT : invalid signature
func TestValidateJWT_InvalidSignature(t *testing.T) {
	as, _ := setupTestAuthServer(t)

	dummyJWTSecret := []byte("dummy-jwt-secret")

	// JWT token
	now := time.Now()
	claims := Claims{
		ClientID: "test-client-1",
		TokenID:  "tkn123",
		Scopes:   []string{"read:ltp", "write:quote"},
		RegisteredClaims: jwt.RegisteredClaims{
			// ExpiresAt: jwt.NewNumericDate(expiresAt),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute * 5)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "auth-server",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(dummyJWTSecret)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	// call validateJWT
	_, err = as.validateJWT(tokenString)
	if err == nil {
		t.Fatalf("validateJWT failed: %v", err)
	}
}

// test validateJWT : token revoked
func TestValidateJWT_TokenRevoked(t *testing.T) {
	as, mock := setupTestAuthServer(t)

	// JWT token
	now := time.Now()
	claims := Claims{
		ClientID: "test-client-1",
		TokenID:  "tkn123",
		Scopes:   []string{"read:ltp", "write:quote"},
		RegisteredClaims: jwt.RegisteredClaims{
			// ExpiresAt: jwt.NewNumericDate(expiresAt),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute * 5)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "auth-server",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(as.jwtSecret)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	// isTokenRevoked
	mock.ExpectPrepare(regexp.QuoteMeta(
		"SELECT revoked FROM tokens WHERE token_id = :1",
	)).ExpectQuery().WithArgs("tkn123").WillReturnRows(sqlmock.NewRows([]string{"revoked"}).AddRow(true)) //true -> revoked

	// getTokenType
	mock.ExpectPrepare(regexp.QuoteMeta(
		"SELECT token_type from tokens where token_id=:1",
	)).ExpectQuery().WithArgs("tkn123").WillReturnRows(sqlmock.NewRows([]string{"token_type"}).AddRow("N"))

	// call validateJWT
	_, err = as.validateJWT(tokenString)
	if err == nil {
		t.Fatal("expected reoked token error")
	}
}

// test tokenHandler : success
func TestTokenHandler_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	as, mock := setupTestAuthServer(t)

	// clientByID
	rows := sqlmock.NewRows([]string{
		"client_id",
		"client_secret",
		"access_token_ttl",
		"allowed_scopes",
	}).AddRow(
		"test-client-1",
		"test-secret-1",
		3600,
		`["read:ltp","read:quote"]`,
	)

	mock.ExpectPrepare(regexp.QuoteMeta(
		"SELECT client_id, client_secret, access_token_ttl, allowed_scopes FROM clients WHERE client_id = :1",
	)).ExpectQuery().WithArgs("test-client-1").WillReturnRows(rows)

	// insert token
	mock.ExpectBegin()
	mock.ExpectPrepare(regexp.QuoteMeta(
		"INSERT INTO tokens(token_id, token_type, jwt_token, client_id, issued_at, expires_at) VALUES (:1, :2, :3, :4, :5, :6)",
	)).ExpectExec().WithArgs(
		sqlmock.AnyArg(),
		sqlmock.AnyArg(), //  token_type
		sqlmock.AnyArg(),
		"test-client-1",
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
	).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	// HTTP request
	body := `{
              "grant_type": "client_credentials",
              "client_id": "test-client-1",
  			  "client_secret": "test-secret-1"
             }`

	req := httptest.NewRequest(
		http.MethodPost,
		"/auth-server/v1/oauth/token",
		strings.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()

	r := gin.New()
	r.POST("/auth-server/v1/oauth/token", as.tokenHandler)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}

	var resp TokenResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON response: %v, body=%s", err, w.Body.String())
	}

	if resp.AccessToken == "" {
		t.Fatal("access_token is empty")
	}

	if resp.TokenType != "Bearer" {
		t.Fatalf("unexpected token_type: %s", resp.TokenType)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations not met: %v", err)
	}
}

// test tokenHandler : invalid JSON
func TestTokenHandler_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	as, _ := setupTestAuthServer(t)

	body := `{ "grant_type": "client_credentials", `

	req := httptest.NewRequest(
		http.MethodPost,
		"/auth-server/v1/oauth/token",
		strings.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r := gin.New()
	r.POST("/auth-server/v1/oauth/token", as.tokenHandler)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d, body=%s", w.Code, w.Body.String())
	}
}

// test tokenHandler : missing clientID
func TestTokenHandler_MissingClientID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	as, _ := setupTestAuthServer(t)

	body := `{
		"grant_type": "client_credentials",
		"client_secret": "test-secret-1"
	}`

	req := httptest.NewRequest(http.MethodPost,
		"/auth-server/v1/oauth/token",
		strings.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r := gin.New()
	r.POST("/auth-server/v1/oauth/token", as.tokenHandler)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d, body=%s", w.Code, w.Body.String())
	}
}

// test tokenHandler : invalid grant
func TestTokenHandler_InvalidGrantType(t *testing.T) {
	gin.SetMode(gin.TestMode)

	as, mock := setupTestAuthServer(t)

	rows := sqlmock.NewRows([]string{
		"client_id", "client_secret", "access_token_ttl", "allowed_scopes",
	}).AddRow("test-client-1", "test-secret-1", 3600, `["read:ltp", "read:quote"]`)

	mock.ExpectPrepare(
		"SELECT client_id, client_secret, access_token_ttl, allowed_scopes FROM clients WHERE client_id = :1",
	).ExpectQuery().WithArgs("test-client-1").WillReturnRows(rows)

	body := `{
	"grant_type": "dummy",
	"client_id": "test-client-1",
	"client_secret": "test-secret-1"
	}`

	req := httptest.NewRequest(http.MethodPost,
		"/auth-server/v1/oauth/token",
		strings.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r := gin.New()
	r.POST("/auth-server/v1/oauth/token", as.tokenHandler)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d, body=%s", w.Code, w.Body.String())
	}
}

// test ottHandler : success
func TestOttHandler_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	as, mock := setupTestAuthServer(t)

	// clientByID
	rows := sqlmock.NewRows([]string{
		"client_id",
		"client_secret",
		"access_token_ttl",
		"allowed_scopes",
	}).AddRow(
		"test-client-1",
		"test-secret-1",
		3600,
		`["read:ltp","read:quote"]`,
	)

	mock.ExpectPrepare(regexp.QuoteMeta(
		"SELECT client_id, client_secret, access_token_ttl, allowed_scopes FROM clients WHERE client_id = :1",
	)).ExpectQuery().WithArgs("test-client-1").WillReturnRows(rows)

	// insert token
	mock.ExpectBegin()
	mock.ExpectPrepare(regexp.QuoteMeta(
		"INSERT INTO tokens(token_id, token_type, jwt_token, client_id, issued_at, expires_at) VALUES (:1, :2, :3, :4, :5, :6)",
	)).ExpectExec().WithArgs(
		sqlmock.AnyArg(),
		sqlmock.AnyArg(), //  token_type
		sqlmock.AnyArg(),
		"test-client-1",
		sqlmock.AnyArg(),
		sqlmock.AnyArg(),
	).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	// HTTP request
	body := `{
		"grant_type": "client_credentials",
		"client_id": "test-client-1",
		"client_secret": "test-secret-1"
	   }`

	req := httptest.NewRequest(
		http.MethodPost,
		"/auth-server/v1/oauth/ott",
		strings.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()

	r := gin.New()
	r.POST("/auth-server/v1/oauth/ott", as.ottHandler)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}

	var resp TokenResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON response: %v, body=%s", err, w.Body.String())
	}

	if resp.AccessToken == "" {
		t.Fatal("access_token is empty")
	}

	if resp.TokenType != "Bearer" {
		t.Fatalf("unexpected token_type: %s", resp.TokenType)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations not met: %v", err)
	}
}

// test OttHandler : invalid JSON
func TestOttHandler_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	as, _ := setupTestAuthServer(t)

	body := `{ "grant_type": "client_credentials", `

	req := httptest.NewRequest(
		http.MethodPost,
		"/auth-server/v1/oauth/token",
		strings.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r := gin.New()
	r.POST("/auth-server/v1/oauth/token", as.ottHandler)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d, body=%s", w.Code, w.Body.String())
	}
}

// test OttHandler : missing clientID
func TestOttHandler_MissingClientID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	as, _ := setupTestAuthServer(t)

	body := `{
		"grant_type": "client_credentials",
		"client_secret": "test-secret-1"
	}`

	req := httptest.NewRequest(http.MethodPost,
		"/auth-server/v1/oauth/token",
		strings.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r := gin.New()
	r.POST("/auth-server/v1/oauth/token", as.ottHandler)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d, body=%s", w.Code, w.Body.String())
	}
}

// test ottHandler : invalid grant
func TestOttHandler_InvalidGrantType(t *testing.T) {
	gin.SetMode(gin.TestMode)

	as, mock := setupTestAuthServer(t)

	rows := sqlmock.NewRows([]string{
		"client_id", "client_secret", "access_token_ttl", "allowed_scopes",
	}).AddRow("test-client-1", "test-secret-1", 3600, `["read:ltp", "read:quote"]`)

	mock.ExpectPrepare(
		"SELECT client_id, client_secret, access_token_ttl, allowed_scopes FROM clients WHERE client_id = :1",
	).ExpectQuery().WithArgs("test-client-1").WillReturnRows(rows)

	body := `{
	"grant_type": "dummy",
	"client_id": "test-client-1",
	"client_secret": "test-secret-1"
	}`

	req := httptest.NewRequest(http.MethodPost,
		"/auth-server/v1/oauth/token",
		strings.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r := gin.New()
	r.POST("/auth-server/v1/oauth/token", as.ottHandler)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d, body=%s", w.Code, w.Body.String())
	}
}

// test validateHandler : success
func TestValidateHandler_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	as, mock := setupTestAuthServer(t)

	// JWT token
	now := time.Now()
	claims := Claims{
		ClientID: "test-client-1",
		TokenID:  "tkn123",
		Scopes:   []string{"read:ltp", "read:quote"},
		RegisteredClaims: jwt.RegisteredClaims{
			// ExpiresAt: jwt.NewNumericDate(expiresAt),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute * 5)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "auth-server",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(as.jwtSecret)
	if err != nil {
		t.Fatalf("unexpected signing method: %v", err)
	}

	// getScopeForEndpoint
	scopeRows := sqlmock.NewRows([]string{
		"scope",
	}).AddRow(
		"read:ltp",
	)

	mock.ExpectPrepare(regexp.QuoteMeta(
		"SELECT scope from endpoints where endpoint_url=:1",
	)).ExpectQuery().WithArgs("http://localhost:8080/ltp").WillReturnRows(scopeRows)

	// isTokenRevoked
	mock.ExpectPrepare(regexp.QuoteMeta(
		"SELECT revoked FROM tokens WHERE token_id = :1",
	)).ExpectQuery().WithArgs("tkn123").WillReturnRows(sqlmock.NewRows([]string{"revoked"}).AddRow(false))

	// getTokenType
	mock.ExpectPrepare(regexp.QuoteMeta(
		"SELECT token_type from tokens where token_id=:1",
	)).ExpectQuery().WithArgs(sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"token_type"}).AddRow("N"))

	// HTTP request
	req := httptest.NewRequest(
		http.MethodPost,
		"/auth-server/v1/oauth/validate",
		nil, //
	)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokenString)
	req.Header.Set("X-Forwarded-For", "http://localhost:8080/ltp")

	w := httptest.NewRecorder()

	r := gin.New()
	r.POST("/auth-server/v1/oauth/validate", as.validateHandler)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", w.Code, w.Body.String())
	}

	var resp TokenValidationResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	fmt.Printf("tokenValidationResponse: %v", resp) //

	if !resp.Valid {
		t.Fatal("expected token to be valid")
	}

	if resp.ClientID != "test-client-1" {
		t.Fatalf("unexpected client_id: %s", resp.ClientID)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations not met: %v", err)
	}
}

// test validateHandler : missing Authorization header
func TestValidateHandler_MissingAuthHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	as, mock := setupTestAuthServer(t)

	// JWT token
	now := time.Now()
	claims := Claims{
		ClientID: "test-client-1",
		TokenID:  "tkn123",
		Scopes:   []string{"read:ltp", "read:quote"},
		RegisteredClaims: jwt.RegisteredClaims{
			// ExpiresAt: jwt.NewNumericDate(expiresAt),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute * 5)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "auth-server",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	_, err := token.SignedString(as.jwtSecret)
	if err != nil {
		t.Fatalf("unexpected signing method: %v", err)
	}

	// getScopeForEndpoint
	scopeRows := sqlmock.NewRows([]string{
		"scope",
	}).AddRow(
		"read:ltp",
	)

	mock.ExpectPrepare(regexp.QuoteMeta(
		"SELECT scope from endpoints where endpoint_url=:1",
	)).ExpectQuery().WithArgs("http://localhost:8080/ltp").WillReturnRows(scopeRows)

	// isTokenRevoked
	mock.ExpectPrepare(regexp.QuoteMeta(
		"SELECT revoked FROM tokens WHERE token_id = :1",
	)).ExpectQuery().WithArgs("tkn123").WillReturnRows(sqlmock.NewRows([]string{"revoked"}).AddRow(false))

	// getTokenType
	mock.ExpectPrepare(regexp.QuoteMeta(
		"SELECT token_type from tokens where token_id=:1",
	)).ExpectQuery().WithArgs(sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"token_type"}).AddRow("N"))

	// HTTP request
	req := httptest.NewRequest(
		http.MethodPost,
		"/auth-server/v1/oauth/validate",
		nil, //
	)
	req.Header.Set("Content-Type", "application/json")
	// req.Header.Set("Authorization", "Bearer "+tokenString)
	req.Header.Set("X-Forwarded-For", "http://localhost:8080/ltp")

	w := httptest.NewRecorder()

	r := gin.New()
	r.POST("/auth-server/v1/oauth/validate", as.validateHandler)
	r.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Fatalf("expected 401, got %d, body=%s", w.Code, w.Body.String())
	}
}

// test validateHandler : missing X-Forwarded-For
func TestValidateHandler_MissingXForwardedFor(t *testing.T) {
	gin.SetMode(gin.TestMode)

	as, _ := setupTestAuthServer(t)

	// Create a valid JWT
	now := time.Now()
	claims := Claims{
		ClientID: "test-client-1",
		TokenID:  "tkn123",
		Scopes:   []string{"read:ltp"},
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute * 5)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "auth-server",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString(as.jwtSecret)

	req := httptest.NewRequest(
		http.MethodPost,
		"/auth-server/v1/oauth/validate",
		nil,
	)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	req.Header.Set("Content-Type", "application/json")
	// X-Forwarded-For missing

	w := httptest.NewRecorder()
	r := gin.New()
	r.POST("/auth-server/v1/oauth/validate", as.validateHandler)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d, body=%s", w.Code, w.Body.String())
	}
}

// test validateHandler : scope mismatch
func TestValidateHandler_ScopeMismatch(t *testing.T) {
	gin.SetMode(gin.TestMode)

	as, mock := setupTestAuthServer(t)

	// JWT token with WRONG scope
	now := time.Now()
	claims := Claims{
		ClientID: "test-client-1",
		TokenID:  "tkn123",
		Scopes:   []string{"read:dummy"},
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute * 5)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "auth-server",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString(as.jwtSecret)

	// getScopeForEndpoint
	scopeRows := sqlmock.NewRows([]string{"scope"}).AddRow("read:ltp")

	mock.ExpectPrepare(regexp.QuoteMeta(
		"SELECT scope from endpoints where endpoint_url=:1",
	)).ExpectQuery().WithArgs("http://localhost:8082/ltp").WillReturnRows(scopeRows)

	// isTokenRevoked
	mock.ExpectPrepare(regexp.QuoteMeta(
		"SELECT revoked FROM tokens WHERE token_id = :1",
	)).ExpectQuery().WithArgs("tkn123").
		WillReturnRows(sqlmock.NewRows([]string{"revoked"}).AddRow(false))

	// getTokenType
	mock.ExpectPrepare(regexp.QuoteMeta(
		"SELECT token_type from tokens where token_id=:1",
	)).ExpectQuery().WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"token_type"}).AddRow("N"))

	req := httptest.NewRequest(
		http.MethodPost,
		"/auth-server/v1/oauth/validate",
		nil,
	)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	req.Header.Set("X-Forwarded-For", "http://localhost:8082/ltp")
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r := gin.New()
	r.POST("/auth-server/v1/oauth/validate", as.validateHandler)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d, body=%s", w.Code, w.Body.String())
	}
}

func TestValidateHandler_InvalidBearer(t *testing.T) {
	gin.SetMode(gin.TestMode)

	as, mock := setupTestAuthServer(t)

	mock.ExpectPrepare(regexp.QuoteMeta(
		"SELECT scope from endpoints where endpoint_url=:1",
	)).ExpectQuery().
		WithArgs("http://localhost:8080/ltp").
		WillReturnRows(sqlmock.NewRows([]string{"scope"}).AddRow("read:ltp"))

	req := httptest.NewRequest(
		http.MethodPost,
		"/auth-server/v1/oauth/validate",
		nil,
	)

	req.Header.Set("Authorization", "InvalidTokenFormat123")
	req.Header.Set("X-Forwarded-For", "http://localhost:8080/ltp")
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()

	r := gin.New()
	r.POST("/auth-server/v1/oauth/validate", as.validateHandler)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d, body=%s", w.Code, w.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations not met: %v", err)
	}
}

// test revokeHandler
func TestRevokeHandler_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	as, mock := setupTestAuthServer(t)

	// JWT token
	now := time.Now()
	claims := Claims{
		ClientID: "test-client-1",
		TokenID:  "tkn123",
		Scopes:   []string{"read:ltp"},
		RegisteredClaims: jwt.RegisteredClaims{
			// ExpiresAt: jwt.NewNumericDate(expiresAt),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute * 5)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "auth-server",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(as.jwtSecret)
	if err != nil {
		t.Fatalf("unexpected signing method: %v", err)
	}

	// isTokenRevoked
	mock.ExpectPrepare(regexp.QuoteMeta(
		"SELECT revoked FROM tokens WHERE token_id = :1",
	)).ExpectQuery().WithArgs("tkn123").WillReturnRows(sqlmock.NewRows([]string{"revoked"}).AddRow(false))

	// getTokenType
	mock.ExpectPrepare(regexp.QuoteMeta(
		"SELECT token_type from tokens where token_id=:1",
	)).ExpectQuery().WithArgs(sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"token_type"}).AddRow("N"))

	// revokeToken
	mock.ExpectBegin()
	mock.ExpectPrepare(regexp.QuoteMeta(
		"Update tokens set revoked=true, revoked_at=:1 where token_id=:2",
	)).ExpectExec().WithArgs(
		sqlmock.AnyArg(), // reoked_at
		"tkn123",         // token_id
	).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	// HTTP request
	req := httptest.NewRequest(
		http.MethodPost,
		"/auth-server/v1/oauth/revoke",
		nil, //
	)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokenString)

	w := httptest.NewRecorder()

	r := gin.New()
	r.POST("/auth-server/v1/oauth/revoke", as.revokeHandler)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body=%s", w.Code, w.Body.String())
	}

	// var reso RevokedToken
	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}

	if resp["message"] != "Token revoked successfully" {
		t.Fatalf("error revoking token: %v", resp)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations not met: %v", err)
	}
}

// test revokeHandler : missing token
func TestRevokeHandler_MissingToken(t *testing.T) {
	as, _ := setupTestAuthServer(t)

	r := gin.New()
	r.POST("/revoke", as.revokeHandler)

	req := httptest.NewRequest("POST", "/revoke", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatal("expected 401 for missing token")
	}
}

func TestRevokeHandler_AlreadyRevoked(t *testing.T) {
	gin.SetMode(gin.TestMode)

	as, mock := setupTestAuthServer(t)

	// JWT token
	claims := Claims{
		ClientID: "test-client-1",
		TokenID:  "tkn123",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString(as.jwtSecret)

	// Token is already revoked
	mock.ExpectPrepare(regexp.QuoteMeta(
		"SELECT revoked FROM tokens WHERE token_id = :1",
	)).ExpectQuery().WithArgs("tkn123").
		WillReturnRows(sqlmock.NewRows([]string{"revoked"}).AddRow(true))

	// Token type
	mock.ExpectPrepare(regexp.QuoteMeta(
		"SELECT token_type from tokens where token_id=:1",
	)).ExpectQuery().WithArgs("tkn123").
		WillReturnRows(sqlmock.NewRows([]string{"token_type"}).AddRow("N"))

	req := httptest.NewRequest(
		http.MethodPost,
		"/auth-server/v1/oauth/revoke",
		nil,
	)
	req.Header.Set("Authorization", "Bearer "+tokenString)

	w := httptest.NewRecorder()

	r := gin.New()
	r.POST("/auth-server/v1/oauth/revoke", as.revokeHandler)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d, body=%s", w.Code, w.Body.String())
	}
}

// cache
// test newClientCache
func TestNewClientCache(t *testing.T) {
	cc := newClientCache()

	if cc == nil {
		t.Fatal("cache should not be nil")
	}

	if cc.GetSize() != 0 {
		t.Fatal("new cache should be empty")
	}
}

// test newClientCache : Set nil
func TestClientCache_SetNil(t *testing.T) {
	cc := newClientCache()
	cc.Set("x", nil)

	if cc.GetSize() != 0 {
		t.Fatal("nil client should not be cached")
	}
}

// test newClientCache : Set & Get method
func TestClientCache_SetAndGet(t *testing.T) {
	cc := newClientCache()

	client := &Clients{
		ClientID:     "test-client-1",
		ClientSecret: "test-secret-1",
	}

	cc.Set("test-client-1", client)
	cached, found := cc.Get("test-client-1")

	if !found {
		t.Fatal("client should be found in cache")
	}

	if cached.ClientID != "test-client-1" {
		t.Fatal("wrong client returned")
	}
}

// test newClientCache : Invalidate
func TestClientCache_Invalidate(t *testing.T) {
	cc := newClientCache()

	cc.Set("test-client-1", &Clients{ClientID: "test-client-1"})
	cc.Invalidate("test-client-1")

	_, found := cc.Get("test-client-1")

	if found {
		t.Fatal("client should be removed from cache")
	}
}

// test newClientCache : Clear
func TestClientCache_Clear(t *testing.T) {
	cc := newClientCache()

	cc.Set("c1", &Clients{ClientID: "c1"})
	cc.Set("c2", &Clients{ClientID: "c2"})

	cc.Clear()

	if cc.GetSize() != 0 {
		t.Fatal("cache should be empty after clear")
	}
}

// benchmark generateJWT
func BenchmarkGenerateJWT(b *testing.B) {
	as, mock := setupTestAuthServer(nil)

	client := &Clients{
		ClientID:      "test-client-1",
		AllowedScopes: []string{"read:ltp", "read:quote"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// mock insertToken
		mock.ExpectBegin()
		mock.ExpectPrepare(regexp.QuoteMeta(
			"INSERT INTO tokens(token_id, token_type, jwt_token, client_id, issued_at, expires_at) VALUES (:1, :2, :3, :4, :5, :6)",
		)).ExpectExec().WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		_, _, err := as.generateJWT(client, "N")
		if err != nil {
			b.Fatal("failed to generate token")
		}
	}
	b.StopTimer()

	if err := mock.ExpectationsWereMet(); err != nil {
		b.Errorf("sql expectations were not met: %v", err)
	}
}

// benchmark validateJWT
func BenchmarkValidateJWT(b *testing.B) {
	as, mock := setupTestAuthServer(nil)

	// JWT token
	now := time.Now()
	claims := Claims{
		ClientID: "test-client-1",
		TokenID:  "tkn123",
		Scopes:   []string{"read:ltp", "write:quote"},
		RegisteredClaims: jwt.RegisteredClaims{
			// ExpiresAt: jwt.NewNumericDate(expiresAt),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute * 5)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "auth-server",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(as.jwtSecret)
	if err != nil {
		b.Fatalf("failed to sign token: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// isTokenRevoked
		mock.ExpectPrepare(regexp.QuoteMeta(
			"SELECT revoked FROM tokens WHERE token_id = :1",
		)).ExpectQuery().WithArgs("tkn123").WillReturnRows(sqlmock.NewRows([]string{"revoked"}).AddRow(false))

		// getTokenType
		mock.ExpectPrepare(regexp.QuoteMeta(
			"SELECT token_type from tokens where token_id=:1",
		)).ExpectQuery().WithArgs(sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"token_type"}).AddRow("N"))

		_, err := as.validateJWT(tokenString)
		if err != nil {
			b.Fatal("failed to validate token", err)
		}
	}
	b.StopTimer()

	if err := mock.ExpectationsWereMet(); err != nil {
		b.Errorf("sql expectations were not met: %v", err)
	}
}

// benchmark tokenHandler
func BenchmarkTokenHandler(b *testing.B) {
	as, mock := setupTestAuthServer(nil)

	// clientByID
	rows := sqlmock.NewRows([]string{
		"client_id",
		"client_secret",
		"access_token_ttl",
		"allowed_scopes",
	}).AddRow(
		"test-client-1",
		"test-secret-1",
		3600,
		`["read:ltp","read:quote"]`,
	)

	mock.ExpectPrepare(regexp.QuoteMeta(
		"SELECT client_id, client_secret, access_token_ttl, allowed_scopes FROM clients WHERE client_id = :1",
	)).ExpectQuery().WithArgs("test-client-1").WillReturnRows(rows)

	// HTTP request
	body := `{
		"grant_type": "client_credentials",
		"client_id": "test-client-1",
		  "client_secret": "test-secret-1"
	   }`

	r := gin.New()
	r.POST("/auth-server/v1/oauth/token", as.tokenHandler)

	for i := 0; i < b.N; i++ {
		// mock insertToken
		mock.ExpectBegin()
		mock.ExpectPrepare(regexp.QuoteMeta(
			"INSERT INTO tokens(token_id, token_type, jwt_token, client_id, issued_at, expires_at) VALUES (:1, :2, :3, :4, :5, :6)",
		)).ExpectExec().WithArgs(
			sqlmock.AnyArg(),
			"N", // token_type (normal)
			sqlmock.AnyArg(),
			"test-client-1",
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
		).WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		req := httptest.NewRequest(
			http.MethodPost,
			"/auth-server/v1/oauth/token",
			strings.NewReader(body),
		)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

// benchmark validateHandler
func BenchmarkValidateHandler(b *testing.B) {
	as, mock := setupTestAuthServer(nil)

	// JWT token
	now := time.Now()
	claims := Claims{
		ClientID: "test-client-1",
		TokenID:  "tkn123",
		Scopes:   []string{"read:ltp", "read:quote"},
		RegisteredClaims: jwt.RegisteredClaims{
			// ExpiresAt: jwt.NewNumericDate(expiresAt),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute * 5)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "auth-server",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(as.jwtSecret)
	if err != nil {
		b.Fatalf("unexpected signing method: %v", err)
	}

	router := gin.New()
	router.POST("/auth-server/v1/oauth/validate", as.validateHandler)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// getScopeForEndpoint
		scopeRows := sqlmock.NewRows([]string{
			"scope",
		}).AddRow(
			"read:ltp",
		)

		mock.ExpectPrepare(regexp.QuoteMeta(
			"SELECT scope from endpoints where endpoint_url=:1",
		)).ExpectQuery().WithArgs("http://localhost:8080/ltp").WillReturnRows(scopeRows)

		// isTokenRevoked
		mock.ExpectPrepare(regexp.QuoteMeta(
			"SELECT revoked FROM tokens WHERE token_id = :1",
		)).ExpectQuery().WithArgs("tkn123").WillReturnRows(sqlmock.NewRows([]string{"revoked"}).AddRow(false))

		// getTokenType
		mock.ExpectPrepare(regexp.QuoteMeta(
			"SELECT token_type from tokens where token_id=:1",
		)).ExpectQuery().WithArgs(sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"token_type"}).AddRow("N"))

		req := httptest.NewRequest(
			http.MethodPost,
			"/auth-server/v1/oauth/validate",
			nil,
		)
		req.Header.Set("Authorization", "Bearer "+tokenString)
		req.Header.Set("X-Forwarded-For", "http://localhost:8080/ltp")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}

// test Logging middleware
func TestLoggingMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(LoggingMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	// Create request
	req, _ := http.NewRequest("GET", "/test", nil)
	recorder := httptest.NewRecorder()

	// Execute
	router.ServeHTTP(recorder, req)

	// Verify
	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", recorder.Code)
	}
}

// test CORS middleware
func TestCORSMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(CORSMiddleware())
	router.OPTIONS("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	// Test OPTIONS request
	req, _ := http.NewRequest("OPTIONS", "/test", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != 204 {
		t.Errorf("Expected status 204 for OPTIONS, got %d", recorder.Code)
	}

	// Check CORS headers
	corsOrigin := recorder.Header().Get("Access-Control-Allow-Origin")
	if corsOrigin != "*" {
		t.Errorf("Expected CORS origin *, got %s", corsOrigin)
	}

	corsMethods := recorder.Header().Get("Access-Control-Allow-Methods")
	if corsMethods == "" {
		t.Errorf("Expected CORS methods header to be set")
	}
}

// test Recovery middleware
func TestRecoveryMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(RecoveryMiddleware())
	router.GET("/panic", func(c *gin.Context) {
		panic("test panic")
	})

	req, _ := http.NewRequest("GET", "/panic", nil)
	recorder := httptest.NewRecorder()

	// Should not panic
	router.ServeHTTP(recorder, req)

	if recorder.Code != 500 {
		t.Errorf("Expected status 500 for panic recovery, got %d", recorder.Code)
	}
}

// test GetLogger
func TestGetLogger(t *testing.T) {
	// Reset the once to test fresh logger
	onceLog = sync.Once{}

	logger := GetLogger()

	logEvent := logger.Info()
	if logEvent == nil {
		t.Errorf("Expected logger to be initialized")
	}
}

// benchmark revokeHandler
func BenchmarkRevokeHandler(b *testing.B) {
	as, mock := setupTestAuthServer(nil)

	// JWT token
	now := time.Now()
	claims := Claims{
		ClientID: "test-client-1",
		TokenID:  "tkn123",
		Scopes:   []string{"read:ltp", "read:quote"},
		RegisteredClaims: jwt.RegisteredClaims{
			// ExpiresAt: jwt.NewNumericDate(expiresAt),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute * 5)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "auth-server",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(as.jwtSecret)
	if err != nil {
		b.Fatalf("unexpected signing method: %v", err)
	}

	// HTTP request
	router := gin.New()
	router.POST("/auth-server/v1/oauth/revoke", as.revokeHandler)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// isTokenRevoked
		mock.ExpectPrepare(regexp.QuoteMeta(
			"SELECT revoked FROM tokens WHERE token_id = :1",
		)).ExpectQuery().WithArgs("tkn123").WillReturnRows(sqlmock.NewRows([]string{"revoked"}).AddRow(false))

		// getTokenType
		mock.ExpectPrepare(regexp.QuoteMeta(
			"SELECT token_type from tokens where token_id=:1",
		)).ExpectQuery().WithArgs(sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"token_type"}).AddRow("N"))

		//revokeToken
		mock.ExpectBegin()
		mock.ExpectPrepare(regexp.QuoteMeta(
			"Update tokens set revoked=true, revoked_at=:1 where token_id=:2",
		)).ExpectExec().WithArgs(
			sqlmock.AnyArg(), // reoked_at
			"tkn123",         // token_id
		).WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		req := httptest.NewRequest(
			http.MethodPost,
			"/auth-server/v1/oauth/revoke",
			nil,
		)
		req.Header.Set("Authorization", "Bearer "+tokenString)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}
