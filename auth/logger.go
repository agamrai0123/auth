package auth

import (
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/rs/zerolog/pkgerrors"
	"gopkg.in/natefinch/lumberjack.v2"
)

var onceLog sync.Once

func GetLogger() zerolog.Logger {
	onceLog.Do(func() {
		zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
		zerolog.TimeFieldFormat = time.RFC3339Nano

		rotatingLog := &lumberjack.Logger{
			Filename:   AppConfig.Logging.Path,
			MaxSize:    AppConfig.Logging.MaxSizeMB,
			MaxBackups: 10,
			MaxAge:     14, //days
			Compress:   true,
		}

		logger := zerolog.New(rotatingLog).
			Level(zerolog.Level(AppConfig.Logging.Level)).
			With().
			Timestamp().
			Str("service", "auth_server").
			Logger()

		log.Logger = logger
		log.Info().
			Str("log_path", AppConfig.Logging.Path).
			Int("log_level", AppConfig.Logging.Level).
			Msg("Logger initialized for auth_server")
	})

	return log.Logger
}

func LoggingMiddleware() gin.HandlerFunc {
	hostname, _ := os.Hostname()
	processID := os.Getpid()

	return func(c *gin.Context) {
		start := time.Now()
		requestID := uuid.New().String()
		// c.Set("RequestID", requestID)

		logger := log.With().
			Str("request_id", requestID).
			Str("client_ip", c.ClientIP()).
			Str("host", hostname).
			Int("pid", processID).
			Str("user_agent", c.Request.UserAgent()).
			Logger()

		c.Set("logger", logger)
		c.Set("request_id", requestID)

		logger.Debug().
			Str("method", c.Request.Method).
			Str("path", c.Request.URL.Path).
			Msg("Incoming request")

		c.Next()

		duration := time.Since(start)
		statusCode := c.Writer.Status()

		logEvent := getLogEventLevel(statusCode)

		logEvent().
			Str("method", c.Request.Method).
			Str("path", c.Request.URL.Path).
			Int("status", statusCode).
			Float64("duration_ms", float64(duration.Microseconds())).
			Msg("Request completed")
	}
}

func getLogEventLevel(statusCode int) func() *zerolog.Event {
	switch {
	case statusCode >= 500:
		return log.Error
	case statusCode >= 400:
		return log.Warn
	case statusCode >= 300:
		return log.Debug
	default:
		return log.Info
	}
}

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// SECURITY FIX: Use origin whitelist instead of wildcard (*)
		// Prevents CSRF attacks
		origin := c.Request.Header.Get("Origin")

		// Whitelist of allowed origins - configure in production
		allowedOrigins := map[string]bool{
			"http://localhost:3000":      true, // Development
			"http://localhost:8080":      true, // Development
			"https://trusted-domain.com": true, // Production example
		}

		// Only set CORS headers if origin is allowed (not wildcard)
		if allowedOrigins[origin] {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
			c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
			c.Writer.Header().Set("Access-Control-Max-Age", "86400")
		}

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

func RecoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				logger, ok := c.Get("logger")
				if !ok {
					logger = log.Logger
				}

				requestLogger := logger.(zerolog.Logger)
				requestLogger.Error().
					Interface("panic", err).
					Str("path", c.Request.URL.Path).
					Str("method", c.Request.Method).
					Msg("Request panic recovered")

				c.JSON(500, gin.H{
					"error": "Internal server error",
				})
			}
		}()
		c.Next()
	}
}

// GetRequestLogger retrieves the request-specific logger from context
func GetRequestLogger(c *gin.Context) zerolog.Logger {
	logger, ok := c.Get("logger")
	if !ok {
		return log.Logger
	}
	return logger.(zerolog.Logger)
}

func GetRequestID(c *gin.Context) string {
	requestID, ok := c.Get("request_id")
	if !ok {
		return ""
	}
	return requestID.(string)
}

// SECURITY FIX: Sanitize sensitive headers before logging
// Prevents credentials, API keys, and tokens from being exposed in logs
func sanitizeHeaders(h map[string][]string) map[string]string {
	safe := make(map[string]string)
	sensitiveHeaders := map[string]bool{
		"authorization": true,
		"x-api-key":     true,
		"cookie":        true,
		"x-auth-token":  true,
		"client-secret": true,
	}

	for key, values := range h {
		keyLower := strings.ToLower(key)
		if sensitiveHeaders[keyLower] {
			safe[key] = "***REDACTED***"
		} else if len(values) > 0 {
			safe[key] = values[0]
		}
	}
	return safe
}
