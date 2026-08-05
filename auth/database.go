package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	_ "github.com/sijms/go-ora/v2"
)

func newDbClient(url string) (*sql.DB, error) {
	// Connection string format for go-ora: oracle://user:password@host:port/service
	// The url parameter from NewAuthServer already has the godror format (user/password@//host:port/service)
	// We need to convert it to go-ora format
	db, err := sql.Open("oracle", url)
	if err != nil {
		log.Error().Err(err).Msg("failed to open database connection")
		return nil, err
	}

	// connection pool configuration from config
	log.Info().Msg("started connection pool...")
	db.SetMaxOpenConns(AppConfig.Database.ConnectionPool.MaxOpenConns)
	db.SetMaxIdleConns(AppConfig.Database.ConnectionPool.MaxIdleConns)
	db.SetConnMaxLifetime(time.Duration(AppConfig.Database.ConnectionPool.MaxLifetime) * time.Second)
	db.SetConnMaxIdleTime(time.Duration(AppConfig.Database.ConnectionPool.MaxIdleLifetime) * time.Minute)

	err = db.Ping()
	if err != nil {
		log.Error().Err(err).Msg("database ping failed - connection validation error")
		return nil, err
	}

	log.Info().Msg("database connected successfully")
	return db, nil
}

// prepareStatements prepares and caches the statements used on every request's
// hot path once, at startup, instead of preparing (and closing) a fresh
// statement on every call.
func (as *authServer) prepareStatements() error {
	var err error

	if as.stmtClientByID, err = as.db.Prepare(
		"SELECT client_id, client_secret, access_token_ttl, allowed_scopes FROM clients WHERE client_id = :1"); err != nil {
		return fmt.Errorf("failed to prepare clientByID statement: %w", err)
	}

	if as.stmtGetTokenInfo, err = as.db.Prepare(
		"SELECT revoked, token_type FROM tokens WHERE token_id = :1"); err != nil {
		return fmt.Errorf("failed to prepare getTokenInfo statement: %w", err)
	}

	if as.stmtGetScopeForEndpt, err = as.db.Prepare(
		"SELECT scope from endpoints where endpoint_url=:1 AND active=1"); err != nil {
		return fmt.Errorf("failed to prepare getScopeForEndpoint statement: %w", err)
	}

	if as.stmtRevokeToken, err = as.db.Prepare(
		"UPDATE tokens SET revoked = 1, revoked_at = :1 WHERE token_id = :2"); err != nil {
		return fmt.Errorf("failed to prepare revokeToken statement: %w", err)
	}

	log.Info().Msg("prepared statements initialized")
	return nil
}

// revokeToken is on the security-critical path: it must complete synchronously
// and durably before returning, since callers use it to kill a token they
// believe is compromised or misused. It's a single statement, so no explicit
// transaction is needed - that would only add BEGIN/COMMIT round trips.
func (as *authServer) revokeToken(revokedToken RevokedToken) error {
	log.Trace().Msg("in revokeToken function")
	ctx, cancel := context.WithTimeout(as.ctx, 5*time.Second)
	defer cancel()

	if _, err := as.stmtRevokeToken.ExecContext(ctx, revokedToken.RevokedAt, revokedToken.TokenID); err != nil {
		log.Error().Err(err).Str("token_id", revokedToken.TokenID).Msg("Failed to revoke token")
		return err
	}

	// Invalidate token from cache since it's now revoked
	as.tokenCache.Invalidate(revokedToken.TokenID)

	log.Info().Str("token_id", revokedToken.TokenID).Msg("token revoked successfully")
	return nil
}

func (as *authServer) getTokenInfo(tokenID string) (revoked bool, tokenType string, err error) {
	// Check token cache first (fast path)
	cachedToken, found := as.tokenCache.Get(tokenID)
	if found && cachedToken != nil {
		return cachedToken.Revoked, cachedToken.TokenType, nil
	}

	var revokedInt int
	ctx, cancel := context.WithTimeout(as.ctx, 3*time.Second)
	defer cancel()

	if err := as.stmtGetTokenInfo.QueryRowContext(ctx, tokenID).Scan(&revokedInt, &tokenType); err != nil {
		if err == sql.ErrNoRows {
			return false, "", fmt.Errorf("token %s: not found", tokenID)
		}
		log.Error().Err(err).Str("token_id", tokenID).Msg("Failed to fetch token info")
		return false, "", fmt.Errorf("failed to fetch token info: %w", err)
	}

	revoked = revokedInt == 1

	// Cache the token (for both revoked and non-revoked to avoid repeated lookups)
	tokenToCache := Token{
		TokenID:   tokenID,
		TokenType: tokenType,
		Revoked:   revoked,
	}
	as.tokenCache.Set(tokenID, &tokenToCache)

	return revoked, tokenType, nil
}

func (as *authServer) insertToken(token Token) error {
	log.Trace().Str("token_id", token.TokenID).Msg("Queuing token for batch insertion via tokenBatcher")
	// Use the tokenBatcher for async batch insertion instead of single inserts
	// This is more efficient and reduces database round trips
	as.tokenBatcher.Add(token)
	return nil
}

func (as *authServer) getScopeForEndpoint(endpoint_url string) (string, error) {
	log.Trace().Msg("in getScopeForEndpoint")
	var scope string
	ctx, cancel := context.WithTimeout(as.ctx, 5*time.Second)
	defer cancel()

	if err := as.stmtGetScopeForEndpt.QueryRowContext(ctx, endpoint_url).Scan(&scope); err != nil {
		if err == sql.ErrNoRows {
			return scope, fmt.Errorf("clientByID %s: no such client", endpoint_url)
		}
		return scope, fmt.Errorf("clientByID %s: %v", endpoint_url, err)
	}

	return scope, nil
}

func (as *authServer) clientByID(clientID string) (*Clients, error) {
	log.Trace().Str("client_id", clientID).Msg("Looking up client in database")
	ctx, cancel := context.WithTimeout(as.ctx, 5*time.Second)
	defer cancel()

	var client Clients
	var scope string
	var err error

	if err := as.stmtClientByID.QueryRowContext(ctx, clientID).Scan(&client.ClientID, &client.ClientSecret, &client.AccessTokenTTL, &scope); err != nil {
		if err == sql.ErrNoRows {
			log.Warn().Str("client_id", clientID).Msg("Client not found in database")
			return nil, fmt.Errorf("clientByID %s: no such client", clientID)
		}
		log.Error().Err(err).Str("client_id", clientID).Msg("Database query failed")
		return nil, fmt.Errorf("clientByID %s: %v", clientID, err)
	}

	client.AllowedScopes, err = parseStringArray(scope)
	if err != nil {
		log.Error().Err(err).Str("client_id", clientID).Msg("Failed to parse allowed scopes")
		return nil, err
	}

	log.Debug().Str("client_id", clientID).Strs("allowed_scopes", client.AllowedScopes).Msg("Client found and scopes parsed")
	return &client, nil
}

func parseStringArray(s string) ([]string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}

	var out []string
	if strings.HasPrefix(s, "[") {
		if err := json.Unmarshal([]byte(s), &out); err == nil {
			return out, nil
		}

		s2 := strings.ReplaceAll(s, `'`, `"`)
		if err := json.Unmarshal([]byte(s2), &out); err == nil {
			return out, nil
		}
	}

	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	parts := strings.Split(s, ",")
	out = make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		p = strings.Trim(p, `"'`)
		if p != "" {
			out = append(out, p)
		}
	}
	return out, nil
}

// insertTokenBatch performs batch insertion of multiple tokens in a single transaction
// This is much more efficient than inserting one at a time
func (as *authServer) insertTokenBatch(tokens []Token) error {
	if len(tokens) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(as.ctx, 10*time.Second)
	defer cancel()

	// Begin transaction for atomic batch insert
	tx, err := as.db.BeginTx(ctx, nil)
	if err != nil {
		log.Error().
			Err(err).
			Int("batch_size", len(tokens)).
			Msg("Failed to begin transaction for batch insert")
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Prepare statement for batch insert (reused for all tokens in batch)
	stmt, err := tx.PrepareContext(ctx, "INSERT INTO tokens(token_id, token_type, jwt_token, client_id, issued_at, expires_at) VALUES (:1, :2, :3, :4, :5, :6)")
	if err != nil {
		log.Error().
			Err(err).
			Int("batch_size", len(tokens)).
			Msg("Failed to prepare batch insert statement")
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	// Execute insert for each token in batch
	inserted := 0
	for i, token := range tokens {
		_, err := stmt.ExecContext(ctx, token.TokenID, token.TokenType, token.JWT_token, token.ClientID, token.IssuedAt, token.ExpiresAt)
		if err != nil {
			log.Error().
				Err(err).
				Str("token_id", token.TokenID).
				Str("client_id", token.ClientID).
				Int("position", i).
				Int("batch_size", len(tokens)).
				Msg("Failed to insert token in batch")
			return fmt.Errorf("failed to insert token at position %d: %w", i, err)
		}
		inserted++
	}

	// Commit transaction (atomicity ensures all or nothing)
	if err := tx.Commit(); err != nil {
		log.Error().
			Err(err).
			Int("inserted", inserted).
			Int("batch_size", len(tokens)).
			Msg("Failed to commit batch insert transaction")
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Debug().
		Int("count", len(tokens)).
		Msg("Token batch inserted successfully")
	return nil
}
