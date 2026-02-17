package auth

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// Clients cache
func newClientCache() *clientCache {
	cc := &clientCache{
		cache: make(map[string]*Clients),
	}

	log.Info().Msg("Client cache initialized")
	return cc
}

// Get retrieves a client from cache if it exists and hasn't expired
// Returns the cached client and true if found and not expired, nil and false otherwise
func (cc *clientCache) Get(clientID string) (*Clients, bool) {
	cc.mu.RLock()

	cached, exists := cc.cache[clientID]
	cc.mu.RUnlock()
	if !exists || cached == nil {
		return nil, false
	}
	return cached, true
}

// Set stores a client in cache, evicting oldest entry if cache is full
func (cc *clientCache) Set(clientID string, client *Clients) {
	if client == nil {
		log.Warn().Str("client_id", clientID).Msg("Attempted to cache nil client, skipping")
		return
	}

	cc.mu.Lock()
	defer cc.mu.Unlock()

	cc.cache[clientID] = client
}

// Invalidate removes a specific client from cache (useful for forced updates)
func (cc *clientCache) Invalidate(clientID string) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	if _, exists := cc.cache[clientID]; exists {
		delete(cc.cache, clientID)
		log.Debug().Str("client_id", clientID).Msg("Client cache entry invalidated")
	}
}

// Clear removes all clients from cache (e.g., during shutdown or restart)
func (cc *clientCache) Clear() {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	cacheSize := len(cc.cache)
	cc.cache = make(map[string]*Clients)
	log.Info().Int("cleared_entries", cacheSize).Msg("Client cache cleared")
}

// GetSize returns current number of entries in cache
func (cc *clientCache) GetSize() int {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	return len(cc.cache)
}

func (s *authServer) populateClientCache() {
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Minute)
	defer cancel()

	query := `SELECT client_id, client_secret, access_token_ttl, allowed_scopes FROM clients`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		log.Error().Err(err).Msgf("failed to populate client cache")
		return
	}
	defer rows.Close()

	if s.clientCache == nil {
		s.clientCache = newClientCache()
	}

	for rows.Next() {
		client := &Clients{}
		var scope string
		if err = rows.Scan(&client.ClientID, &client.ClientSecret, &client.AccessTokenTTL, &scope); err != nil {
			log.Error().Msgf("failed to retrieve row while populating client cache: %s", err)
			continue
		}
		client.AllowedScopes, err = parseStringArray(scope)
		if err != nil {
			log.Error().Err(err).Str("client_id", client.ClientID).Msg("Failed to parse allowed scopes")
		}
		s.clientCache.Set(client.ClientID, client)
	}

	if err = rows.Err(); err != nil {
		log.Error().Err(err).Msg("rows iteration error in populating client cache")
	}
}

func newEndpointsCache() *endpointCache {
	return &endpointCache{
		cache: make(map[string]*Endpoints),
	}
}

func (ec *endpointCache) Get(endpoint_url string) (*Endpoints, bool) {
	ec.mu.RLock()

	cached, exists := ec.cache[endpoint_url]
	ec.mu.RUnlock()
	if !exists || cached == nil {
		return nil, false
	}
	return cached, true
}

// Set stores a client in cache, evicting oldest entry if cache is full
func (ec *endpointCache) Set(endpoint_url string, endpoint *Endpoints) {
	if endpoint == nil {
		log.Warn().Str("endpoint_url", endpoint_url).Msg("Attempted to cache nil endpoint, skipping")
		return
	}

	ec.mu.Lock()
	defer ec.mu.Unlock()

	ec.cache[endpoint_url] = endpoint
}

// Invalidate removes a specific client from cache (useful for forced updates)
func (ec *endpointCache) Invalidate(endpoint_url string) {
	ec.mu.Lock()
	defer ec.mu.Unlock()

	delete(ec.cache, endpoint_url)
}

// Clear removes all clients from cache (e.g., during shutdown or restart)
func (ec *endpointCache) Clear() {
	ec.mu.Lock()
	defer ec.mu.Unlock()

	ec.cache = make(map[string]*Endpoints)
}

// GetSize returns current number of entries in cache
func (ec *endpointCache) GetSize() int {
	ec.mu.RLock()
	defer ec.mu.RUnlock()
	return len(ec.cache)
}

func (s *authServer) populateEndpointsCache() {
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Minute)
	defer cancel()

	query := `SELECT client_id, scope, method, endpoint_url, description, active FROM endpoints`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		log.Error().Err(err).Msgf("failed to populate endpoint cache")
		return
	}
	defer rows.Close()

	if s.endpointCache == nil {
		s.endpointCache = newEndpointsCache()
	}

	for rows.Next() {
		endpoint := &Endpoints{}
		if err = rows.Scan(&endpoint.ClientID, &endpoint.Scope, &endpoint.Method, &endpoint.Url, &endpoint.Description, &endpoint.Active); err != nil {
			log.Error().Msgf("failed to retrieve row while populating endpoint cache: %s", err)
			continue
		}
		s.endpointCache.Set(endpoint.Url, endpoint)
	}

	if err = rows.Err(); err != nil {
		log.Error().Err(err).Msg("rows iteration error in populating endpoint cache")
	}
}

// TokenBatchWriter handles asynchronous batch insertion of tokens to reduce DB load
type TokenBatchWriter struct {
	mu         sync.Mutex
	tokens     []Token
	maxBatch   int
	flushTick  *time.Ticker
	done       chan struct{}
	authServer *authServer
}

// NewTokenBatchWriter creates a new token batch writer with specified parameters
// Parameters: authServer - server instance for DB access, maxBatch - size before auto-flush, flushInterval - max time before flush
func NewTokenBatchWriter(as *authServer, maxBatch int, flushInterval time.Duration) *TokenBatchWriter {
	if maxBatch <= 0 {
		log.Warn().Int("max_batch", maxBatch).Msg("Invalid maxBatch, using default 1000")
		maxBatch = 1000
	}
	if flushInterval <= 0 {
		log.Warn().Dur("flush_interval", flushInterval).Msg("Invalid flushInterval, using default 5 seconds")
		flushInterval = 5 * time.Second
	}

	tbw := &TokenBatchWriter{
		tokens:     make([]Token, 0, maxBatch),
		maxBatch:   maxBatch,
		done:       make(chan struct{}),
		authServer: as,
		flushTick:  time.NewTicker(flushInterval),
	}

	// Start background flush goroutine
	go tbw.backgroundFlush()

	log.Info().
		Int("max_batch", maxBatch).
		Str("flush_interval", flushInterval.String()).
		Msg("Token batch writer initialized")

	return tbw
}

// Add queues a token for batch insertion (non-blocking)
func (tbw *TokenBatchWriter) Add(token Token) {
	if token.TokenID == "" || token.ClientID == "" {
		log.Error().Msg("Attempted to add invalid token (missing TokenID or ClientID)")
		return
	}

	tbw.mu.Lock()
	defer tbw.mu.Unlock()

	tbw.tokens = append(tbw.tokens, token)

	// Flush immediately if batch is full
	if len(tbw.tokens) >= tbw.maxBatch {
		tbw.flushLockedAsync()
	}
}

// Flush immediately writes pending tokens to database (blocking)
func (tbw *TokenBatchWriter) Flush() {
	tbw.mu.Lock()
	defer tbw.mu.Unlock()

	if len(tbw.tokens) > 0 {
		tbw.flushLockedAsync()
	}
}

// flushLockedAsync flushes tokens asynchronously without acquiring lock (assumes lock is held)
func (tbw *TokenBatchWriter) flushLockedAsync() {
	if len(tbw.tokens) == 0 {
		return
	}

	// Copy tokens and reset buffer (prevents holding lock during DB operation)
	batch := make([]Token, len(tbw.tokens))
	copy(batch, tbw.tokens)
	tbw.tokens = tbw.tokens[:0]

	// Write to database asynchronously in separate goroutine
	go func() {
		if err := tbw.authServer.insertTokenBatch(batch); err != nil {
			log.Error().
				Err(err).
				Int("batch_size", len(batch)).
				Msg("Failed to insert token batch")
		} else {
			log.Debug().
				Int("batch_size", len(batch)).
				Msg("Token batch inserted successfully")
		}
	}()
}

// backgroundFlush flushes tokens periodically or on shutdown (runs in background goroutine)
func (tbw *TokenBatchWriter) backgroundFlush() {
	for {
		select {
		case <-tbw.done:
			tbw.flushTick.Stop()
			// Final flush before shutdown
			tbw.Flush()
			log.Debug().Msg("Token batch writer background flush stopped")
			return
		case <-tbw.flushTick.C:
			tbw.Flush()
		}
	}
}

// Stop gracefully stops the batch writer and flushes any pending tokens
func (tbw *TokenBatchWriter) Stop() {
	close(tbw.done)
	log.Info().Msg("Token batch writer stopped")
}

// GetPendingCount returns number of tokens currently waiting for flush
func (tbw *TokenBatchWriter) GetPendingCount() int {
	tbw.mu.Lock()
	defer tbw.mu.Unlock()
	return len(tbw.tokens)
}

// Token Cache with TTL

func newTokenCache(ttl time.Duration) *tokenCache {
	tc := &tokenCache{
		cache: make(map[string]*tokenCacheEntry),
		ttl:   ttl,
	}
	log.Info().Str("ttl", ttl.String()).Msg("Token cache initialized")
	return tc
}

// Get retrieves a token from cache if it exists and hasn't expired
func (tc *tokenCache) Get(tokenID string) (*Token, bool) {
	tc.mu.RLock()
	entry, exists := tc.cache[tokenID]
	tc.mu.RUnlock()

	if !exists || entry == nil {
		return nil, false
	}

	// Check if entry has expired
	if time.Now().After(entry.expiresAt) {
		log.Debug().Str("token_id", tokenID).Msg("Token cache entry expired, removing")
		tc.Invalidate(tokenID)
		return nil, false
	}

	log.Debug().Str("token_id", tokenID).Msg("Token found in cache (hit)")
	return entry.token, true
}

// Set stores a token in cache with TTL
func (tc *tokenCache) Set(tokenID string, token *Token) {
	if tokenID == "" || token == nil {
		log.Warn().Str("token_id", tokenID).Msg("Attempted to cache invalid token")
		return
	}

	tc.mu.Lock()
	defer tc.mu.Unlock()

	tc.cache[tokenID] = &tokenCacheEntry{
		token:     token,
		expiresAt: time.Now().Add(tc.ttl),
	}
	log.Debug().Str("token_id", tokenID).Msg("Token cached successfully")
}

// Invalidate removes a specific token from cache
func (tc *tokenCache) Invalidate(tokenID string) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	if _, exists := tc.cache[tokenID]; exists {
		delete(tc.cache, tokenID)
		log.Debug().Str("token_id", tokenID).Msg("Token cache entry invalidated")
	}
}

// Clear removes all tokens from cache
func (tc *tokenCache) Clear() {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	cacheSize := len(tc.cache)
	tc.cache = make(map[string]*tokenCacheEntry)
	log.Info().Int("cleared_entries", cacheSize).Msg("Token cache cleared")
}

// GetSize returns current number of entries in cache
func (tc *tokenCache) GetSize() int {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return len(tc.cache)
}

// CleanExpired removes all expired entries from cache
func (tc *tokenCache) CleanExpired() int {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	removed := 0
	now := time.Now()

	for tokenID, entry := range tc.cache {
		if now.After(entry.expiresAt) {
			delete(tc.cache, tokenID)
			removed++
		}
	}

	if removed > 0 {
		log.Debug().Int("removed", removed).Msg("Cleaned expired entries from token cache")
	}

	return removed
}
