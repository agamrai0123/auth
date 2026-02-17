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
	clients     map[string]*rate.Limiter
	mu          sync.RWMutex
	ticker      *time.Ticker
	done        chan bool
	clientRPS   int
	clientBurst int
}

// NewRateLimiter creates a new rate limiter with specified per-client limits
func NewRateLimiter(clientRPS int, clientBurst int) *RateLimiter {
	rl := &RateLimiter{
		clients:     make(map[string]*rate.Limiter),
		done:        make(chan bool),
		clientRPS:   clientRPS,
		clientBurst: clientBurst,
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

// getClientLimiter gets or creates a rate limiter for a client based on configured limits
func (rl *RateLimiter) getClientLimiter(clientID string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	limiter, exists := rl.clients[clientID]
	if !exists {
		// Create limiter with configured RPS and burst values
		limiter = rate.NewLimiter(rate.Limit(rl.clientRPS), rl.clientBurst)
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
		// Extract client ID from query parameters first (doesn't consume body)
		clientID := c.Query("client_id")

		// If not in query, try to extract from Authorization header (X-Client-ID)
		if clientID == "" {
			clientID = c.GetHeader("X-Client-ID")
		}

		// Fallback to IP address if no client_id found in request
		if clientID == "" {
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
