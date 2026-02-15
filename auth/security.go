package auth

import (
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

// SecurityHeadersMiddleware adds security headers to all responses
func SecurityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// HSTS - HTTP Strict-Transport-Security
		// Tells browsers to only use HTTPS for this domain for max-age seconds
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")

		// X-Content-Type-Options - Prevent MIME type sniffing
		c.Header("X-Content-Type-Options", "nosniff")

		// X-Frame-Options - Clickjacking protection (iframe embedding)
		c.Header("X-Frame-Options", "DENY")

		// X-XSS-Protection - Browser XSS filter
		c.Header("X-XSS-Protection", "1; mode=block")

		// Referrer-Policy - Control what information about the request is shared
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

		// Permissions-Policy (Feature-Policy) - Control browser features
		c.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=(), payment=()")

		// Content-Security-Policy - Mitigate XSS attacks
		c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'")

		// Remove server information header
		c.Header("Server", "SecureAuthServer/1.0")

		log.Debug().
			Str("method", c.Request.Method).
			Str("path", c.Request.URL.Path).
			Msg("Security headers applied")

		c.Next()
	}
}

// TLSRedirectMiddleware redirects HTTP requests to HTTPS on non-metrics routes
func TLSRedirectMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Don't redirect metrics endpoint (runs on separate port)
		if c.Request.Host == "localhost:7071" || c.Request.Host == "127.0.0.1:7071" {
			c.Next()
			return
		}

		// If not already HTTPS and HTTPS is enabled
		if !AppConfig.HTTPSEnabled {
			c.Next()
			return
		}

		if c.Request.Header.Get("X-Forwarded-Proto") != "https" && c.Request.Proto != "HTTP/2.0" {
			// Redirect to HTTPS
			redirectURL := "https://" + c.Request.Host + c.Request.URL.Path
			if c.Request.URL.RawQuery != "" {
				redirectURL += "?" + c.Request.URL.RawQuery
			}
			c.Redirect(301, redirectURL)
			return
		}

		c.Next()
	}
}
