package auth

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
	"golang.org/x/time/rate"
)

// getJWTSecret loads JWT secret from environment variable (CRITICAL SECURITY FIX)
func getJWTSecret() []byte {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		log.Fatal().Msg("SECURITY ERROR: JWT_SECRET environment variable not set")
	}
	if len(secret) < 32 {
		log.Fatal().Msg("SECURITY ERROR: JWT_SECRET must be at least 32 characters")
	}
	return []byte(secret)
}

var JWTsecret = getJWTSecret()

func (s *authServer) Start() {
	var err error
	// token
	s.tokenRequestsCount, err = registerCounterVecMetric("token_requests_count",
		"total number of token requests",
		"",
		[]string{"token"})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create prometheus counter vector metric for token_requests_count")
	}

	s.tokenSuccessCount, err = registerCounterVecMetric("token_success_count",
		"total number of issued token success",
		"",
		[]string{"token"})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create prometheus counter vector metric for token_success_count")
	}

	s.tokenErrorCount, err = registerCounterVecMetric("token_error_count",
		"total number of token generation errors",
		"",
		[]string{"token", "error_type"})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create prometheus counter vector metric for token_error_count")
	}

	s.tokenGenerationDuration, err = registerHistogramVecMetric("token_generation_duration_seconds",
		"duration of each token",
		"",
		[]float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		[]string{"token"})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create prometheus histogram vector metric token_generation_duration_seconds")
	}

	// validate
	s.validateTokenRequestsCount, err = registerCounterVecMetric("validate_token_requests_count",
		"total number of validate token requests",
		"",
		[]string{"token"})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create prometheus counter vector metric for validate_token_request_count")
	}

	s.validateTokenSuccessCount, err = registerCounterVecMetric("validate_token_success_count",
		"total number of validate token success",
		"",
		[]string{"token"})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create prometheus counter vector metric for validate_token_success_count")
	}

	s.validateTokenErrorCount, err = registerCounterVecMetric("validate_token_error_count",
		"total number of validate token errors",
		"",
		[]string{"token", "error_type"})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create prometheus counter vector metric for validate_token_error_count")
	}

	s.validateTokenLatency, err = registerHistogramVecMetric("validate_token_latency_seconds",
		"validated token latency",
		"",
		[]float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		[]string{"token"})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create prometheus histogram vector metric validate_token_latency_seconds")
	}

	// revoke
	s.revokeRequestsCount, err = registerCounterVecMetric("revoke_token_requests_count",
		"total number of revoke token requests",
		"",
		[]string{"token"})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create prometheus counter vector metric for revoke_token_requests_count")
	}

	s.revokeSuccessCount, err = registerCounterVecMetric("revoke_token_success_count",
		"total number of revoke token success",
		"",
		[]string{"token"})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create prometheus counter vector metric for revoke_token_success_count")
	}

	s.revokeErrorCount, err = registerCounterVecMetric("revoke_token_error_count",
		"total number of revoke token errors",
		"",
		[]string{"token", "error_type"})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create prometheus counter vector metric for revoke_token_error_count")
	}

	s.revokeTokenLatency, err = registerHistogramVecMetric("revoke_token_latency_seconds",
		"revoked token latency",
		"",
		[]float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		[]string{"token"})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create prometheus histogram vector metric revoke_token_latency_seconds")
	}

	// database
	s.dbStatus, err = registerGaugeVecMetric("db_status",
		"oracle database status (1=healthy, 0=unhealthy)",
		"",
		[]string{"db"})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create prometheus gauge vector metric for db_status")
	}

	s.dbConnectionsActive, err = registerGaugeVecMetric("db_connections_active",
		"number of active database connections",
		"",
		[]string{"db"})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create prometheus gauge vector metric for db_connections_active")
	}

	s.dbConnectionsIdle, err = registerGaugeVecMetric("db_connections_idle",
		"number of idle database connections",
		"",
		[]string{"db"})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create prometheus gauge vector metric for db_connections_idle")
	}

	s.dbQueryDuration, err = registerHistogramVecMetric("db_query_duration_seconds",
		"duration of database queries",
		"",
		[]float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		[]string{"operation"})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create prometheus histogram vector metric for db_query_duration_seconds")
	}

	// cache metrics
	s.clientCacheHitRate, err = registerCounterVecMetric("client_cache_hits_total",
		"total number of client cache hits",
		"",
		[]string{"cache_type"})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create prometheus counter vector metric for client_cache_hits")
	}

	s.endpointCacheHitRate, err = registerCounterVecMetric("endpoint_cache_hits_total",
		"total number of endpoint cache hits",
		"",
		[]string{"cache_type"})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create prometheus counter vector metric for endpoint_cache_hits")
	}

	s.cacheSize, err = registerGaugeVecMetric("cache_size",
		"current size of cache entries",
		"",
		[]string{"cache_type"})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create prometheus gauge vector metric for cache_size")
	}

	// error metrics
	s.errorCount, err = registerCounterVecMetric("api_errors_total",
		"total number of API errors by type",
		"",
		[]string{"error_code", "error_type"})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create prometheus counter vector metric for api_errors_total")
	}

	// metrics
	reg := getMetricRegistry()
	log.Info().Msg("starting metrics for auth server")
	metricReport := mux.NewRouter()
	metricReport.Handle("/auth-server/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{Registry: reg}))

	go func() {
		err := http.ListenAndServe(":"+strconv.Itoa(AppConfig.MetricPort), metricReport)
		log.Info().Msgf("listening on %d", AppConfig.MetricPort)
		if err != nil {
			log.Error().Msgf("error listening on port %d", AppConfig.MetricPort)
		}
	}()

	router := gin.New()

	// SECURITY FIX: Initialize rate limiting
	// Global limit: 100 requests per second
	// Per-client limit: 10 requests per second
	globalLimiter := rate.NewLimiter(100, 10)
	clientRateLimiter := NewRateLimiter()
	defer clientRateLimiter.Stop()

	router.Use(
		GlobalRateLimitMiddleware(globalLimiter),        // Apply global rate limiting
		LoggingMiddleware(),                             // Log all requests
		CORSMiddleware(),                                // Handle CORS (with origin whitelist)
		PerClientRateLimitMiddleware(clientRateLimiter), // Apply per-client rate limiting
		SecurityHeadersMiddleware(),                     // Add security headers (HSTS, CSP, etc)
		RecoveryMiddleware(),                            // Handle panics
	)
	routes(router, s)

	s.populateClientCache()
	s.populateEndpointsCache()

	// --- HTTPS server (primary) ---
	if AppConfig.HTTPSEnabled && AppConfig.HTTPSServerPort != "" && AppConfig.CertFile != "" && AppConfig.KeyFile != "" {
		httpsPort := AppConfig.HTTPSServerPort
		if httpsPort == "" {
			httpsPort = "8443"
		}
		httpsAddr := ":" + httpsPort

		s.httpSrv = &http.Server{
			Addr:    httpsAddr,
			Handler: router,
		}
		go func() {
			log.Info().
				Str("address", httpsAddr).
				Msg("Starting HTTPS server")

			err := s.httpSrv.ListenAndServeTLS(AppConfig.CertFile, AppConfig.KeyFile)
			if err != nil && err != http.ErrServerClosed {
				log.Error().Err(err).Msg("HTTPS server failed")
			}
		}()

		// Redirect HTTP to HTTPS
		redirectRouter := gin.New()
		redirectRouter.Use(
			LoggingMiddleware(),
			RecoveryMiddleware(),
		)
		redirectRouter.NoRoute(func(c *gin.Context) {
			redirectURL := "https://" + c.Request.Host + c.Request.URL.Path
			if c.Request.URL.RawQuery != "" {
				redirectURL += "?" + c.Request.URL.RawQuery
			}
			c.Redirect(http.StatusMovedPermanently, redirectURL)
		})

		httpAddr := ":" + AppConfig.ServerPort
		go func() {
			log.Info().
				Str("address", httpAddr).
				Msg("Starting HTTP to HTTPS redirect server")

			err := http.ListenAndServe(httpAddr, redirectRouter)
			if err != nil && err != http.ErrServerClosed {
				log.Error().Err(err).Msg("HTTP redirect server failed")
			}
		}()
	} else {
		// Fallback to plain HTTP (not recommended for production)
		log.Warn().Msg("HTTPS not fully configured, falling back to HTTP")
		httpPort := AppConfig.ServerPort
		if httpPort == "" {
			httpPort = "8080"
		}
		httpAddr := ":" + httpPort

		s.httpSrv = &http.Server{
			Addr:    httpAddr,
			Handler: router,
		}
		go func() {
			log.Info().
				Str("address", httpAddr).
				Msg("Starting HTTP server (insecure)")

			err := s.httpSrv.ListenAndServe()
			if err != nil && err != http.ErrServerClosed {
				log.Error().Err(err).Msg("HTTP server failed")
			}
		}()
	}
}

func NewAuthServer() *authServer {
	ctx, cancel := context.WithCancel(context.Background())

	// Build Oracle connection string for go-ora driver: oracle://user:password@host:port/service
	connectionString := fmt.Sprintf("oracle://%s:%s@%s:%d/%s",
		AppConfig.Database.User,
		AppConfig.Database.Password,
		AppConfig.Database.Host,
		AppConfig.Database.Port,
		AppConfig.Database.Service)

	db, err := newDbClient(connectionString)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to initialize Oracle database connection - cannot proceed")
	}

	clientCache := newClientCache()
	endpointCache := newEndpointsCache()
	tokenCache := newTokenCache(1 * time.Hour) // 1-hour TTL for tokens

	authServer := &authServer{
		jwtSecret:     JWTsecret,
		ctx:           ctx,
		cancel:        cancel,
		db:            db,
		clientCache:   clientCache,
		endpointCache: endpointCache,
		tokenCache:    tokenCache,
	}

	authServer.tokenBatcher = NewTokenBatchWriter(authServer, 1000, 5*time.Second)

	// Start periodic cleanup of expired token cache entries
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			tokenCache.CleanExpired()
		}
	}()

	log.Info().Msg("Auth server initialized successfully")
	return authServer
}

func (s *authServer) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if s.tokenBatcher != nil {
		log.Info().Msg("Stopping token batch writer...")
		s.tokenBatcher.Stop()
	}

	if s.clientCache != nil {
		log.Info().Msg("Clearing client cache...")
		s.clientCache.Clear()
	}

	if s.endpointCache != nil {
		log.Info().Msg("Clearing endpoint cache...")
		s.endpointCache.Clear()
	}

	if s.tokenCache != nil {
		log.Info().Msg("Clearing token cache...")
		s.tokenCache.Clear()
	}

	// Close database connection
	if s.db != nil {
		if err := s.db.Close(); err != nil {
			log.Warn().Err(err).Msg("error closing database connection")
		}
	}
	// Cancel context
	if s.cancel != nil {
		s.cancel()
	}

	if s.httpSrv != nil {
		log.Info().Msg("Shutting down HTTP server...")
		if err := s.httpSrv.Shutdown(ctx); err != nil {
			log.Error().Err(err).Msg("HTTP server shutdown error")
			return fmt.Errorf("HTTP server shutdown error: %w", err)
		}
		log.Info().Msg("HTTP server shutdown complete")
	}
	return nil
}
