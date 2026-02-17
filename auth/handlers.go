package auth

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

func (as *authServer) validateClient(clientID, clientSecret string) (*Clients, error) {
	if clientID == "" || clientSecret == "" {
		log.Error().Msg("Missing client credentials")
		return nil, ErrUnauthorizedError("Missing client credentials")
	}

	if cachedClient, found := as.clientCache.Get(clientID); found {
		if cachedClient.ClientSecret != clientSecret {
			log.Error().Msg("Invalid client credentials")
			return nil, ErrUnauthorizedError("Invalid client credentials")
		}
		return cachedClient, nil
	}

	client, err := as.clientByID(clientID)
	if err != nil {
		log.Error().Err(err).Str("client_id", clientID).Msg("Database error while fetching client")
		return nil, ErrInternalServerError("Failed to lookup client").WithOriginalError(err)
	}

	if client == nil || client.ClientSecret != clientSecret {
		log.Error().Str("client_id", clientID).Msg("Invalid client credentials")
		return nil, ErrUnauthorizedError("Invalid client credentials")
	}

	as.clientCache.Set(clientID, client)
	return client, nil
}

func (as *authServer) validateGrantType(grantType string) error {
	if grantType != "client_credentials" {
		log.Error().Msg("unsupported grant_type")
		return ErrBadRequest("Unsupported grant type")
	}
	return nil
}

func (as *authServer) tokenHandler(c *gin.Context) {
	logger := GetRequestLogger(c)
	requestID := GetRequestID(c)
	tokenType := "N" //normal token
	if c.Request.Method != http.MethodPost {
		logger.Warn().Str("request_id", requestID).Str("method", c.Request.Method).Msg("Invalid HTTP method for token endpoint")
		as.errorCount.WithLabelValues(string(ErrInvalidRequest), "invalid_method").Inc()
		RespondWithError(c, ErrBadRequest("Only POST method is allowed"))
		return
	}

	start := time.Now()
	as.tokenRequestsCount.WithLabelValues(tokenType).Inc()

	var tokenReq TokenRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&tokenReq); err != nil {
		logger.Error().Str("request_id", requestID).Err(err).Msg("Failed to decode token request JSON")
		as.errorCount.WithLabelValues(string(ErrInvalidRequest), "decode_error").Inc()
		RespondWithError(c, ErrBadRequest("Invalid JSON format").WithOriginalError(err))
		return
	}

	if err := tokenReq.Validate(); err != nil {
		logger.Warn().Str("request_id", requestID).Err(err).Msg("Token request validation failed")
		as.errorCount.WithLabelValues(string(ErrInvalidRequest), "validation_error").Inc()
		RespondWithError(c, ErrBadRequest(err.Error()))
		return
	}

	// validate client
	client, err := as.validateClient(tokenReq.ClientID, tokenReq.ClientSecret)
	if err != nil {
		logger.Warn().Str("request_id", requestID).Str("client_id", tokenReq.ClientID).Msg("Client validation failed")
		as.errorCount.WithLabelValues(string(ErrUnauthorized), "invalid_credentials").Inc()
		RespondWithError(c, ErrUnauthorizedError("Invalid client credentials"))
		return
	}

	// validate grant type
	if err := as.validateGrantType(tokenReq.GrantType); err != nil {
		logger.Warn().Str("request_id", requestID).Str("grant_type", tokenReq.GrantType).Msg("Invalid grant type")
		as.errorCount.WithLabelValues(string(ErrInvalidRequest), "invalid_grant_type").Inc()
		RespondWithError(c, ErrBadRequest("Unsupported grant type"))
		return
	}

	token, tokenID, err := as.generateJWT(client, tokenType)
	if err != nil {
		logger.Error().Str("request_id", requestID).Str("client_id", tokenReq.ClientID).Err(err).Msg("Failed to generate JWT token")
		RespondWithError(c, ErrInternalServerError("Failed to generate token").WithOriginalError(err))
		return
	}
	log.Info().Str("client_id", tokenReq.ClientID).Str("token_id", tokenID.TokenID).Msg("JWT token generated successfully")

	as.tokenSuccessCount.WithLabelValues(tokenType).Inc()

	as.tokenGenerationDuration.WithLabelValues(tokenType).Observe(float64(time.Since(start).Seconds()))

	c.Header("Content-Type", "application/json")
	encoder := json.NewEncoder(c.Writer)
	if err := encoder.Encode(TokenResponse{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresIn:   3600,
	}); err != nil {
		logger.Error().Str("request_id", requestID).Err(err).Msg("Failed to encode token response")
		c.AbortWithError(http.StatusInternalServerError, err)
	}
}

func (as *authServer) ottHandler(c *gin.Context) {
	logger := GetRequestLogger(c)
	requestID := GetRequestID(c)
	tokenType := "O"
	if c.Request.Method != http.MethodPost {
		logger.Warn().Str("request_id", requestID).Str("method", c.Request.Method).Msg("Invalid HTTP method for token endpoint")
		RespondWithError(c, ErrBadRequest("Only POST method is allowed"))
		return
	}

	start := time.Now()
	as.tokenRequestsCount.WithLabelValues(tokenType).Inc()

	var tokenReq TokenRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&tokenReq); err != nil {
		logger.Error().Str("request_id", requestID).Err(err).Msg("Failed to decode token request JSON")
		RespondWithError(c, ErrBadRequest("Invalid JSON format").WithOriginalError(err))
		return
	}

	client, err := as.validateClient(tokenReq.ClientID, tokenReq.ClientSecret)
	if err != nil {
		logger.Error().Str("request_id", requestID).Str("client_id", tokenReq.ClientID).Msg("Invalid client credentials")
		RespondWithError(c, ErrUnauthorizedError("Invalid client credentials"))
		return
	}

	if err := as.validateGrantType(tokenReq.GrantType); err != nil {
		logger.Warn().Str("request_id", requestID).Str("grant_type", tokenReq.GrantType).Msg("Unsupported grant type")
		RespondWithError(c, ErrBadRequest("Unsupported grant type"))
		return
	}

	// generate token
	token, _, err := as.generateJWT(client, tokenType)
	if err != nil {
		logger.Error().Str("request_id", requestID).Str("client_id", tokenReq.ClientID).Err(err).Msg("Failed to generate JWT token")
		RespondWithError(c, ErrInternalServerError("Failed to generate token").WithOriginalError(err))
		return
	}

	as.tokenSuccessCount.WithLabelValues(tokenType).Inc()

	as.tokenGenerationDuration.WithLabelValues(tokenType).Observe(float64(time.Since(start).Seconds()))

	c.Header("Content-Type", "application/json")
	encoder := json.NewEncoder(c.Writer)
	if err := encoder.Encode(TokenResponse{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresIn:   2 * 60,
	}); err != nil {
		logger.Error().Str("request_id", requestID).Err(err).Msg("Failed to encode token response")
		c.AbortWithError(http.StatusInternalServerError, err)
	}
}

// Validate token handler
func (as *authServer) validateHandler(c *gin.Context) {
	requestURL := c.Request.Header.Get("X-Forwarded-For")
	if requestURL == "" {
		RespondWithError(c, ErrBadRequest("Missing X-Forwarded-For header (resource endpoint)"))
		return
	}

	var requestedScope string
	var err error
	if cachedEndpoint, found := as.endpointCache.Get(requestURL); found {
		log.Info().Str("endpoint_url", requestURL).Msg("[CACHE HIT] Endpoint found in cache")
		requestedScope = cachedEndpoint.Scope
	} else {
		log.Warn().Str("endpoint_url", requestURL).Msg("[CACHE MISS] Endpoint not in cache, querying DB")
		requestedScope, err = as.getScopeForEndpoint(requestURL)
		if err != nil {
			log.Error().Str("endpoint_url", requestURL).Err(err).Msg("Failed to get scope for endpoint")
			RespondWithError(c, ErrUnauthorizedError("Unauthorized scope for endpoint"))
			return
		}
		log.Info().Str("endpoint_url", requestURL).Str("scope", requestedScope).Msg("[DB QUERY] Retrieved scope from database")
	}

	authHeader := c.Request.Header.Get("Authorization")
	if authHeader == "" {
		RespondWithError(c, ErrUnauthorizedError("Missing Authorization header"))
		return
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenString == authHeader {
		RespondWithError(c, ErrUnauthorizedError("Bearer token required"))
		return
	}

	// Validate token
	claims, err := as.validateJWT(tokenString)
	if err != nil {
		RespondWithError(c, ErrUnauthorizedError("Invalid or expired token").WithOriginalError(err))
		return
	}

	// Token type is now available in claims
	tokenType := claims.TokenType

	log.Info().Str("requested_scope", requestedScope).Strs("token_scopes", claims.Scopes).Msg("[VALIDATION] Checking if requested scope in token scopes")

	if !slices.Contains(claims.Scopes, requestedScope) {
		RespondWithError(c, ErrForbiddenError("Resource not in token scopes"))
		return
	}

	// Success - increment metrics
	as.validateTokenSuccessCount.WithLabelValues(tokenType).Inc()

	c.Header("Content-Type", "application/json")
	encoder := json.NewEncoder(c.Writer)
	if err := encoder.Encode(TokenValidationResponse{
		Valid:     true,
		ClientID:  claims.ClientID,
		ExpiresAt: claims.ExpiresAt.Time,
		Scopes:    claims.Scopes,
	}); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
	}
}

// Revoke token handler
func (as *authServer) revokeHandler(c *gin.Context) {
	logger := GetRequestLogger(c)
	requestID := GetRequestID(c)
	if c.Request.Method != http.MethodPost {
		logger.Warn().Str("request_id", requestID).Str("method", c.Request.Method).Msg("Invalid HTTP method for revoke endpoint")
		c.String(http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	start := time.Now()
	as.revokeRequestsCount.WithLabelValues("revoke").Inc()

	authHeader := c.Request.Header.Get("Authorization")
	if authHeader == "" {
		logger.Error().Str("request_id", requestID).Msg("Missing Authorization header for token revocation")
		RespondWithError(c, ErrUnauthorizedError("Authorization header required"))
		return
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenString == authHeader {
		logger.Error().Str("request_id", requestID).Msg("Invalid Bearer token format for revocation")
		RespondWithError(c, ErrUnauthorizedError("Bearer token required"))
		return
	}

	// Validate token first
	claims, err := as.validateJWT(tokenString)
	if err != nil {
		logger.Error().Str("request_id", requestID).Err(err).Msg("JWT token validation failed during revocation")
		RespondWithError(c, ErrUnauthorizedError("Invalid or expired token").WithOriginalError(err))
		return
	}

	// Add to revoked tokens
	revokedToken := RevokedToken{
		ClientID:  claims.ClientID,
		TokenID:   claims.TokenID,
		RevokedAt: time.Now(),
	}

	if err := as.revokeToken(revokedToken); err != nil {
		logger.Error().Str("request_id", requestID).Str("client_id", claims.ClientID).Str("token_id", claims.TokenID).Err(err).Msg("Failed to revoke token")
		RespondWithError(c, ErrInternalServerError("Failed to revoke token").WithOriginalError(err))
		return
	}

	as.revokeSuccessCount.WithLabelValues("revoked").Inc()

	as.revokeTokenLatency.WithLabelValues("revoked").Observe(float64(time.Since(start).Seconds()))

	c.Header("Content-Type", "application/json")
	encoder := json.NewEncoder(c.Writer)
	if err := encoder.Encode(map[string]string{
		"message": "Token revoked successfully",
	}); err != nil {
		logger.Error().Str("request_id", requestID).Err(err).Msg("Failed to encode revocation response")
		c.AbortWithError(http.StatusBadRequest, err)
	}
}
