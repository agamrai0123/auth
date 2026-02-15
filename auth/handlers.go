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

	// cache
	if cachedClient, found := as.clientCache.Get(clientID); found {
		log.Debug().Str("client_id", clientID).Msg("Client found in cache")
		if cachedClient.ClientSecret != clientSecret {
			log.Error().Msg("Invalid client credentials")
			return nil, ErrUnauthorizedError("Invalid client credentials")
		}
		log.Info().Str("client_id", clientID).Msg("Client validated successfully")
		return cachedClient, nil
	}

	// Cache miss - query database with timeout
	// log.Debug().Msg("Client not found in cache")
	log.Info().Str("client_id", clientID).Msg("Client not in cache, querying database")

	client, err := as.clientByID(clientID)
	if err != nil {
		log.Error().Err(err).Str("client_id", clientID).Msg("Database error while fetching client")
		return nil, ErrInternalServerError("Failed to lookup client").WithOriginalError(err)
	}

	if client == nil || client.ClientSecret != clientSecret {
		log.Error().Str("client_id", clientID).Msg("Invalid client credentials")
		return nil, ErrUnauthorizedError("Invalid client credentials")
	}

	// Store in cache for future requests (only cache valid clients)
	// if client != nil {
	as.clientCache.Set(clientID, client)
	// }

	log.Info().Str("client_id", clientID).Msg("Client validated successfully(DB)")
	return client, nil
}

func (as *authServer) validateGrantType(grantType string) error {
	if grantType != "client_credentials" {
		log.Error().Msg("unsupported grant_type")
		return ErrBadRequest("Unsupported grant type")
	}
	log.Info().Str("grant_type", grantType).Msg("Grant type validated successfully")
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
	logger.Debug().Str("request_id", requestID).Msg("Processing token request")
	as.tokenRequestsCount.WithLabelValues(tokenType).Inc()

	var tokenReq TokenRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&tokenReq); err != nil {
		logger.Error().Str("request_id", requestID).Err(err).Msg("Failed to decode token request JSON")
		as.errorCount.WithLabelValues(string(ErrInvalidRequest), "decode_error").Inc()
		RespondWithError(c, ErrBadRequest("Invalid JSON format").WithOriginalError(err))
		return
	}

	logger.Debug().Str("request_id", requestID).Str("client_id", tokenReq.ClientID).Msg("Decoded token request")

	// SECURITY FIX: Validate input parameters
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

	log.Debug().Str("client_id", tokenReq.ClientID).Msg("Client credentials validated")

	// validate grant type
	if err := as.validateGrantType(tokenReq.GrantType); err != nil {
		logger.Warn().Str("request_id", requestID).Str("grant_type", tokenReq.GrantType).Msg("Invalid grant type")
		as.errorCount.WithLabelValues(string(ErrInvalidRequest), "invalid_grant_type").Inc()
		RespondWithError(c, ErrBadRequest("Unsupported grant type"))
		return
	}

	log.Debug().Str("client_id", tokenReq.ClientID).Str("grant_type", tokenReq.GrantType).Msg("processing token request")

	// generate token
	token, tokenID, err := as.generateJWT(client, tokenType)
	if err != nil {
		log.Error().Err(err).Str("client_id", tokenReq.ClientID).Msg("Failed to generate JWT token")
		RespondWithError(c, ErrInternalServerError("Failed to generate token").WithOriginalError(err))
		return
	}
	log.Info().Str("client_id", tokenReq.ClientID).Str("token_id", tokenID.TokenID).Msg("JWT token generated successfully")

	as.tokenSuccessCount.WithLabelValues(tokenType).Inc()

	as.tokenGenerationDuration.WithLabelValues(tokenType).Observe(float64(time.Since(start).Seconds()))

	c.Header("Content-Type", "application/json")
	encoder := json.NewEncoder(c.Writer)
	// CRITICAL SECURITY FIX: Use 1 hour (3600 seconds) for token expiration
	// Previously was 2 minutes (2*60) which broke user experience
	if err := encoder.Encode(TokenResponse{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresIn:   3600, // 1 hour - standard OAuth2 duration
	}); err != nil {
		log.Error().Err(err).Msg("Failed to encode token response")
		c.AbortWithError(http.StatusInternalServerError, err)
	}
}

func (as *authServer) ottHandler(c *gin.Context) {
	tokenType := "O" // ott token
	if c.Request.Method != http.MethodPost {
		log.Warn().Str("method", c.Request.Method).Msg("Invalid HTTP method for token endpoint")
		RespondWithError(c, ErrBadRequest("Only POST method is allowed"))
		return
	}

	start := time.Now()
	as.tokenRequestsCount.WithLabelValues(tokenType).Inc()

	var tokenReq TokenRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&tokenReq); err != nil {
		log.Error().Err(err).Msg("Failed to decode token request JSON")
		RespondWithError(c, ErrBadRequest("Invalid JSON format").WithOriginalError(err))
		return
	}

	// log.Debug().Str("client_id", tokenReq.ClientID).Str("grant_type", tokenReq.GrantType).Msg("processing token request")
	log.Debug().Str("client_id", tokenReq.ClientID).Msg("Client credentials validated")

	// validate client
	client, err := as.validateClient(tokenReq.ClientID, tokenReq.ClientSecret)
	if err != nil {
		log.Error().Msg("Invalid client credentials")
		RespondWithError(c, ErrUnauthorizedError("Invalid client credentials"))
		return
	}

	log.Debug().Str("client_id", tokenReq.ClientID).Msg("Client credentials validated")

	// validate grant type
	if err := as.validateGrantType(tokenReq.GrantType); err != nil {
		log.Error().Str("grant_type", tokenReq.GrantType).Msg("Unsupported grant type")
		RespondWithError(c, ErrBadRequest("Unsupported grant type"))
		return
	}

	log.Debug().Str("client_id", tokenReq.ClientID).Str("grant_type", tokenReq.GrantType).Msg("processing token request")

	// generate token
	token, tokenID, err := as.generateJWT(client, tokenType)
	if err != nil {
		log.Error().Err(err).Str("client_id", tokenReq.ClientID).Msg("Failed to generate JWT token")
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
		ExpiresIn:   2 * 60, // 2 min for testing, use 3600 (1 hour) for production
	}); err != nil {
		log.Error().Err(err).Msg("Failed to encode token response")
		c.AbortWithError(http.StatusInternalServerError, err)
	}
}

// Validate token handler
func (as *authServer) validateHandler(c *gin.Context) {
	log.Debug().Msg("Processing validate request")
	if c.Request.Method != http.MethodPost {
		log.Warn().Str("method", c.Request.Method).Msg("Invalid HTTP method for token endpoint")
		RespondWithError(c, ErrBadRequest("Only POST method is allowed"))
		return
	}

	start := time.Now()
	as.validateTokenRequestsCount.WithLabelValues("validate").Inc()

	requestURL := c.Request.Header.Get("X-Forwarded-For")
	if requestURL == "" {
		log.Error().Msg("Missing X-Forwarded-For header (resource endpoint)")
		RespondWithError(c, ErrBadRequest("Missing X-Forwarded-For header (resource endpoint)"))
		return
	}
	var requestedScope string
	var err error
	if cachedEndpoint, found := as.endpointCache.Get(requestURL); found {
		log.Debug().Str("endpoint_url", requestURL).Msg("Endpoint found in cache")
		requestedScope = cachedEndpoint.Scope
	} else {
		requestedScope, err = as.getScopeForEndpoint(requestURL)
		if err != nil {
			log.Error().Err(err).Str("resource", requestURL).Msg("error in requested scope for endpoint")
			RespondWithError(c, ErrUnauthorizedError("Unauthorized scope for endpoint"))
			return
		}
	}

	authHeader := c.Request.Header.Get("Authorization")
	if authHeader == "" {
		log.Error().Str("resource", requestURL).Msg("Missing Authorization header")
		RespondWithError(c, ErrUnauthorizedError("Missing Authorization header"))
		return
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenString == authHeader {
		log.Error().Str("resource", requestURL).Msg("Invalid Bearer token format")
		RespondWithError(c, ErrUnauthorizedError("Bearer token required"))
		return
	}

	// Validate token
	claims, err := as.validateJWT(tokenString)
	if err != nil {
		log.Error().Err(err).Str("resource", requestURL).Msg("JWT token validation failed")
		RespondWithError(c, ErrUnauthorizedError("Invalid or expired token").WithOriginalError(err))
		return
	}

	// Token type is now available in claims
	tokenType := claims.TokenType

	as.validateTokenRequestsCount.WithLabelValues(tokenType).Inc()

	found := slices.Contains(claims.Scopes, requestedScope)
	if !found {
		log.Error().
			Str("client_id", claims.ClientID).
			Str("resource", requestURL).
			Strs("allowed_scopes", claims.Scopes).
			Msg("Resource not in token scopes - access denied")
		RespondWithError(c, ErrForbiddenError("Resource not in token scopes"))
		return
	}

	log.Info().
		Str("client_id", claims.ClientID).
		Str("resource", requestURL).
		Time("expires_at", claims.ExpiresAt.Time).
		Msg("Token validated for resource - access granted")

	as.validateTokenSuccessCount.WithLabelValues(tokenType).Inc()

	as.validateTokenLatency.WithLabelValues(tokenType).Observe(float64(time.Since(start).Seconds()))

	c.Header("Content-Type", "application/json")
	encoder := json.NewEncoder(c.Writer)
	if err := encoder.Encode(TokenValidationResponse{
		Valid:     true,
		ClientID:  claims.ClientID,
		ExpiresAt: claims.ExpiresAt.Time,
		Scopes:    claims.Scopes,
	}); err != nil {
		log.Error().Err(err).Msg("Failed to encode validation response")
		c.AbortWithError(http.StatusBadRequest, err)
	}
}

// Revoke token handler
func (as *authServer) revokeHandler(c *gin.Context) {
	if c.Request.Method != http.MethodPost {
		log.Warn().Str("method", c.Request.Method).Msg("Invalid HTTP method for revoke endpoint")
		c.String(http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	start := time.Now()
	as.revokeRequestsCount.WithLabelValues("revoke").Inc()

	authHeader := c.Request.Header.Get("Authorization")
	if authHeader == "" {
		log.Error().Msg("Missing Authorization header for token revocation")
		RespondWithError(c, ErrUnauthorizedError("Authorization header required"))
		return
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenString == authHeader {
		log.Error().Msg("Invalid Bearer token format for revocation")
		RespondWithError(c, ErrUnauthorizedError("Bearer token required"))
		return
	}

	// Validate token first
	claims, err := as.validateJWT(tokenString)
	if err != nil {
		log.Error().Err(err).Msg("JWT token validation failed during revocation")
		RespondWithError(c, ErrUnauthorizedError("Invalid or expired token").WithOriginalError(err))
		return
	}

	log.Debug().Str("client_id", claims.ClientID).Str("token_id", claims.TokenID).Msg("Revoking token")

	// Add to revoked tokens
	revokedToken := RevokedToken{
		ClientID:  claims.ClientID,
		TokenID:   claims.TokenID,
		RevokedAt: time.Now(),
	}

	if err := as.revokeToken(revokedToken); err != nil {
		log.Error().Err(err).Str("client_id", claims.ClientID).Str("token_id", claims.TokenID).Msg("Failed to revoke token")
		RespondWithError(c, ErrInternalServerError("Failed to revoke token").WithOriginalError(err))
		return
	}

	log.Info().Str("client_id", claims.ClientID).Str("token_id", claims.TokenID).Msg("Token revoked successfully")

	as.revokeSuccessCount.WithLabelValues("revoked").Inc()

	as.revokeTokenLatency.WithLabelValues("revoked").Observe(float64(time.Since(start).Seconds()))

	c.Header("Content-Type", "application/json")
	encoder := json.NewEncoder(c.Writer)
	if err := encoder.Encode(map[string]string{
		"message": "Token revoked successfully",
	}); err != nil {
		log.Error().Err(err).Msg("Failed to encode revocation response")
		c.AbortWithError(http.StatusBadRequest, err)
	}
}
