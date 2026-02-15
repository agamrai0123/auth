package auth

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"golang.org/x/time/rate"
)

// SECURITY FIX: Rate limiting to prevent DDoS and brute force attacks

type RateLimiter struct {
	clients map[string]*rate.Limiter
	mu      sync.RWMutex
	ticker  *time.Ticker
	done    chan bool
}

// NewRateLimiter creates a new rate limiter with global and per-client limits
func NewRateLimiter() *RateLimiter {
	rl := &RateLimiter{
		clients: make(map[string]*rate.Limiter),
		done:    make(chan bool),
	}

	// Clean up old limiters every 10 minutes
	rl.ticker = time.NewTicker(10 * time.Minute)
	go rl.cleanupOldClients()

	return rl
}

// cleanupOldClients removes client limiters that haven't been used recently
func (rl *RateLimiter) cleanupOldClients() {
	for range rl.ticker.C {
		rl.mu.Lock()
		for clientID := range rl.clients {
			// Keep removing old entries to prevent unbounded memory growth
			if len(rl.clients) > 1000 {
				delete(rl.clients, clientID)
			}
		}
		rl.mu.Unlock()
	}
}

// Stop stops the rate limiter cleanup goroutine
func (rl *RateLimiter) Stop() {
	rl.ticker.Stop()
	close(rl.done)
}

// getClientLimiter gets or creates a rate limiter for a client (10 req/s per client)
func (rl *RateLimiter) getClientLimiter(clientID string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	limiter, exists := rl.clients[clientID]
	if !exists {
		// 10 requests per second per client, burst of 2
		limiter = rate.NewLimiter(rate.Limit(10), 2)
		rl.clients[clientID] = limiter
	}
	return limiter
}

// GlobalRateLimitMiddleware applies global rate limiting (100 req/s global)
func GlobalRateLimitMiddleware(globalLimiter *rate.Limiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !globalLimiter.Allow() {
			log.Warn().
				Str("client_ip", c.ClientIP()).
				Msg("Global rate limit exceeded")
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":             "rate_limit_exceeded",
				"error_description": "Too many requests. Please try again later.",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

// PerClientRateLimitMiddleware applies per-client rate limiting (10 req/s per client)
func PerClientRateLimitMiddleware(rl *RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract client ID from request body or use IP as fallback
		var clientID string

		// Try to get from JSON body (for token endpoint)
		type ClientRequest struct {
			ClientID string `json:"client_id"`
		}
		var req ClientRequest
		if err := c.ShouldBindJSON(&req); err == nil && req.ClientID != "" {
			clientID = req.ClientID
		} else {
			// Fallback to IP address
			clientID = c.ClientIP()
		}

		limiter := rl.getClientLimiter(clientID)
		if !limiter.Allow() {
			log.Warn().
				Str("client_id", clientID).
				Msg("Per-client rate limit exceeded")
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":             "rate_limit_exceeded",
				"error_description": "Too many requests from this client. Please try again later.",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
