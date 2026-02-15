package auth

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/prometheus/client_golang/prometheus"
)

type authServer struct {
	jwtSecret     []byte
	ctx           context.Context
	cancel        context.CancelFunc
	httpSrv       *http.Server
	db            *sql.DB
	clientCache   *clientCache
	endpointCache *endpointCache
	tokenCache    *tokenCache
	tokenBatcher  *TokenBatchWriter // Batch token writer for async writes

	// token metrics
	tokenRequestsCount      *prometheus.CounterVec
	tokenSuccessCount       *prometheus.CounterVec
	tokenErrorCount         *prometheus.CounterVec
	tokenGenerationDuration *prometheus.HistogramVec

	// validate token metrics
	validateTokenRequestsCount *prometheus.CounterVec
	validateTokenSuccessCount  *prometheus.CounterVec
	validateTokenErrorCount    *prometheus.CounterVec
	validateTokenLatency       *prometheus.HistogramVec

	// revoke token metrics
	revokeRequestsCount *prometheus.CounterVec
	revokeSuccessCount  *prometheus.CounterVec
	revokeErrorCount    *prometheus.CounterVec
	revokeTokenLatency  *prometheus.HistogramVec

	// cache metrics
	clientCacheHitRate   *prometheus.CounterVec
	endpointCacheHitRate *prometheus.CounterVec
	cacheSize            *prometheus.GaugeVec

	// database metrics
	dbStatus            *prometheus.GaugeVec
	dbConnectionsActive *prometheus.GaugeVec
	dbConnectionsIdle   *prometheus.GaugeVec
	dbQueryDuration     *prometheus.HistogramVec

	// error metrics
	errorCount *prometheus.CounterVec
}

type clientCache struct {
	mu    sync.RWMutex
	cache map[string]*Clients
}

type endpointCache struct {
	mu    sync.RWMutex
	cache map[string]*Endpoints
}

type tokenCacheEntry struct {
	token     *Token
	expiresAt time.Time
}

type tokenCache struct {
	mu    sync.RWMutex
	cache map[string]*tokenCacheEntry // token_id -> token with TTL
	ttl   time.Duration
}

type Clients struct {
	ClientID       string
	ClientSecret   string
	Name           string
	AccessTokenTTL int32
	AllowedScopes  []string
}

type Endpoints struct {
	ClientID    string `json:"client_id"`
	Scope       string `json:"scope"`
	Method      string `json:"method"`
	Url         string `json:"api_url"`
	Description string `json:"description"`
	Active      int    `json:"active"`
}

type Token struct {
	TokenID   string    `json:"token_id"`
	TokenType string    `json:"token_type"`
	JWT_token string    `json:"jwt"`
	ClientID  string    `json:"client_id"`
	IssuedAt  time.Time `json:"issued_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Revoked   bool      `json:"revoked"`
	RevokedAt time.Time
}

type RevokedToken struct {
	ClientID  string    `json:"client_id"`
	TokenID   string    `json:"token_id"`
	RevokedAt time.Time `json:"revoked_at"`
}

// JWT Claims
type Claims struct {
	ClientID  string   `json:"client_id"`
	TokenID   string   `json:"token_id"`
	TokenType string   `json:"token_type"`
	Scopes    []string `json:"scopes"`
	jwt.RegisteredClaims
}

type TokenRequest struct {
	GrantType    string `json:"grant_type"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	// Scope        string `json:"scope,omitempty"`
}

// SECURITY FIX: Validate input parameters to prevent injection attacks
func (tr *TokenRequest) Validate() error {
	if tr.ClientID == "" {
		return fmt.Errorf("client_id is required")
	}
	if len(tr.ClientID) > 255 {
		return fmt.Errorf("client_id exceeds maximum length (255 characters)")
	}
	if tr.ClientSecret == "" {
		return fmt.Errorf("client_secret is required")
	}
	if len(tr.ClientSecret) > 255 {
		return fmt.Errorf("client_secret exceeds maximum length (255 characters)")
	}
	if tr.GrantType == "" {
		return fmt.Errorf("grant_type is required")
	}
	if tr.GrantType != "client_credentials" {
		return fmt.Errorf("invalid grant_type: only 'client_credentials' is supported")
	}
	return nil
}

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
	// AuthCode string `json:"auth_code"`
	// Method       string `json:"method"`
	// Scope        string `json:"scope"`
	// Audience     string `json:"aud"`
	// RefreshToken string `json:"refresh_token,omitempty"`
}

type ErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

type TokenValidationResponse struct {
	Valid     bool      `json:"valid"`
	ClientID  string    `json:"client_id"`
	ExpiresAt time.Time `json:"expires_at"`
	Scopes    []string  `json:"scopes"`
	// TokenID   string    `json:"token_id"`
	// Role      string    `json:"role"`
}
