package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/zerolog/log"
)

// Generate random string
func generateRandomString(length int) string {
	bytes := make([]byte, length)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// Generate JWT token
func (as *authServer) generateJWT(client *Clients, tokenType string) (string, *Token, error) {
	tokenID := generateRandomString(16)
	now := time.Now()
	var expiresAt time.Time

	// CRITICAL SECURITY FIX: Correct token expiration times
	// One-time tokens: 30 minutes
	// Normal tokens: 1 hour (production standard)
	if tokenType == "O" {
		expiresAt = now.Add(30 * time.Minute) // One-time tokens: 30 min
	} else {
		expiresAt = now.Add(1 * time.Hour) // Normal tokens: 1 hour
	}

	claims := Claims{
		ClientID:  client.ClientID,
		TokenID:   tokenID,
		TokenType: tokenType,
		Scopes:    client.AllowedScopes,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "auth-server",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(as.jwtSecret)
	if err != nil {
		log.Error().Err(err).Str("client_id", client.ClientID).Msg("Failed to sign JWT token")
		return "", nil, err
	}

	// Store token info
	tokenInfo := Token{
		TokenID:   tokenID,
		TokenType: tokenType,
		JWT_token: tokenString,
		ClientID:  client.ClientID,
		IssuedAt:  now,
		ExpiresAt: expiresAt,
		Revoked:   false,
	}

	// Add to cache immediately for fast lookup in validate/revoke
	log.Debug().Str("token_id", tokenID).Str("client_id", client.ClientID).Msg("[DEBUG] Adding token to cache in generateJWT")
	as.tokenCache.Set(tokenID, &tokenInfo)

	// Also queue for async batch write to database
	log.Debug().Str("token_id", tokenID).Msg("[DEBUG] Queuing token for async batch write")
	as.tokenBatcher.Add(tokenInfo)

	return tokenString, &tokenInfo, nil
}

// Validate JWT token
func (as *authServer) validateJWT(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return as.jwtSecret, nil
	})

	if err != nil {
		log.Warn().Err(err).Msg("JWT token parsing failed")
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		revoked, tokenType, err := as.getTokenInfo(claims.TokenID)
		if err != nil {
			return nil, fmt.Errorf("error fetching token info: %v", err)
		}

		if revoked {
			return nil, fmt.Errorf("token has been revoked")
		}

		// Set token type in claims for use in handlers
		claims.TokenType = tokenType

		// Handle OTT token auto-revocation asynchronously
		if tokenType == "O" {
			revokedToken := RevokedToken{
				ClientID:  claims.ClientID,
				TokenID:   claims.TokenID,
				RevokedAt: time.Now(),
			}
			// Queue for async processing instead of blocking
			go func() {
				if err := as.revokeToken(revokedToken); err != nil {
					// Silent OTT auto-revocation failure
				}
			}()
		}

		return claims, nil
	}
	log.Warn().Msg("JWT token validation failed - invalid token")
	return nil, fmt.Errorf("invalid token")
}
