package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/raakeshmj/apigatewayplane/internal/audit"
	"github.com/raakeshmj/apigatewayplane/internal/auth"
	"github.com/raakeshmj/apigatewayplane/internal/cache"
	"github.com/raakeshmj/apigatewayplane/internal/circuitbreaker"
	"github.com/raakeshmj/apigatewayplane/internal/config"
	"github.com/raakeshmj/apigatewayplane/internal/limiter"
	"github.com/raakeshmj/apigatewayplane/internal/metrics"
	"github.com/raakeshmj/apigatewayplane/internal/middleware"
	"github.com/raakeshmj/apigatewayplane/internal/policy"
	"github.com/raakeshmj/apigatewayplane/internal/repository/memory"
	"github.com/raakeshmj/apigatewayplane/internal/service"
	"github.com/redis/go-redis/v9"
)

type Server struct {
	cfg            *config.Config
	router         *http.ServeMux
	authService    *service.AuthService
	rateLimiter    *limiter.TokenBucketLimiter
	circuitBreaker *circuitbreaker.CircuitBreaker
	metrics        *metrics.MetricsCollector
	auditLogger    audit.Logger
	configManager  *config.DynamicConfigManager
	policyEngine   *policy.Engine // Policy Engine
	redisClient    *redis.Client
	// Cache not exposed in struct? Or useful for stats?
	l1Cache *cache.MemoryCache
}

func New(cfg *config.Config) *Server {
	// Initialize Redis
	rdb := redis.NewClient(&redis.Options{
		Addr: cfg.RedisAddr,
	})

	// Initialize Dependencies
	repo := memory.New()
	jwtManager := auth.NewJWTManager(cfg.JWTSecret, time.Hour)

	// L1 Cache
	l1 := cache.NewMemoryCache()

	authSvc := service.NewAuthService(repo, repo, jwtManager, l1)

	limit := limiter.NewTokenBucketLimiter(rdb)

	cb := circuitbreaker.New(rdb, 3, 5, 10*time.Second)

	met := metrics.NewCollector(1000)

	auditLog := audit.NewJSONLogger(os.Stdout)

	// Dynamic Config
	cfgMgr := config.NewDynamicConfigManager()

	// Policy Engine
	eng := policy.NewEngine()
	// Load Initial Policies
	eng.LoadPolicies([]policy.Policy{
		{
			ID:      "admin-policy",
			Matcher: policy.Matcher{Path: "/api/admin"},
			Rules:   policy.Rules{AuthRequired: true, RateLimit: 10, Burst: 20},
		},
		{
			ID:      "public-policy",
			Matcher: policy.Matcher{Path: "/api/public"},
			Rules:   policy.Rules{AuthRequired: false, RateLimit: 5, Burst: 10},
		},
		{
			ID:      "health-policy",
			Matcher: policy.Matcher{Path: "/health"},
			Rules:   policy.Rules{AuthRequired: false, RateLimit: 100, Burst: 100},
		},
		{
			ID:      "ready-policy",
			Matcher: policy.Matcher{Path: "/ready"},
			Rules:   policy.Rules{AuthRequired: false, RateLimit: 100, Burst: 100},
		},
		{
			ID:      "test-policy",
			Matcher: policy.Matcher{Path: "/api/test"},
			Rules:   policy.Rules{AuthRequired: false, RateLimit: 10, Burst: 20},
		},
		// Default implies fallback logic in middleware if no match
	})

	return &Server{
		cfg:            cfg,
		router:         http.NewServeMux(),
		authService:    authSvc,
		rateLimiter:    limit,
		circuitBreaker: cb,
		metrics:        met,
		auditLogger:    auditLog,
		configManager:  cfgMgr,
		policyEngine:   eng,
		redisClient:    rdb,
		l1Cache:        l1,
	}
}

func (s *Server) Start() error {
	// Basic Health Check (Liveness)
	s.router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Readiness Check (Dependencies)
	s.router.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := s.redisClient.Ping(ctx).Err(); err != nil {
			http.Error(w, "Redis Unavailable", http.StatusServiceUnavailable)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Ready"))
	})

	// Public Endpoint Demo
	s.router.Handle("/api/public/hello", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello Public World"))
	}))

	// Admin Endpoints (Protected by /api/admin/* policy)
	s.router.HandleFunc("/api/admin/reload", s.ReloadPolicies)
	s.router.HandleFunc("/api/admin/keys/create", s.GenerateAPIKeyHandler)
	s.router.HandleFunc("/api/admin/keys/rotate", s.RevokeAPIKeyHandler)

	// Metrics Endpoint (Public for now, or protected?)
	s.router.HandleFunc("/api/metrics", func(w http.ResponseWriter, r *http.Request) {
		stats := s.metrics.GetStats()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)
	})

	// Setup Middleware Chain
	// Order: Metrics (Outer) -> Security -> Policy -> Auth -> RateLimit -> Handler

	securityMw := middleware.SecureHeaders(middleware.SecurityConfig{
		EnableReplayProtection: true,
		ReplayWindow:           60 * time.Second,
	})

	metricsMw := middleware.MetricsMiddleware(s.metrics)
	auditMw := middleware.AuditMiddleware(s.auditLogger)

	// Policy Enforcer
	policyMw := middleware.PolicyEnforcer(s.policyEngine)

	authMiddleware := middleware.NewAuth(s.authService.JWTManager(), s.authService)
	// Pass Config Manager
	rateLimitMiddleware := middleware.RateLimit(s.rateLimiter, s.configManager)
	cbMiddleware := middleware.CircuitBreakerMiddleware(s.circuitBreaker, "main-service")

	// Public Chain (Need middleware to apply Policy so RateLimit works!)
	// Wait, /api/public is also needing RateLimit?
	// Yes, Policy sets RateLimit.
	// So ALL endpoints under /api should use the chain?
	// Yes. Unified chain is better.

	// Unified Handler for everything under /api/
	// Use subrouter logic or apply middleware per handler?
	// Apply per handler for now.

	// Protected Endpoint (Now protected by Global Chain + Policy)
	s.router.HandleFunc("/api/whoami", func(w http.ResponseWriter, r *http.Request) {
		userID := r.Context().Value(middleware.UserContextKey).(string)
		w.Write([]byte(fmt.Sprintf("Hello, User %s!", userID)))
	})

	// Faulty Endpoint (Wrapped with Circuit Breaker + Metrics + Audit)
	// Metrics -> Audit -> CircuitBreaker -> Handler
	s.router.Handle("/api/unstable", cbMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate Failure
		if r.URL.Query().Get("fail") == "true" {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Internal Server Error"))
			return
		}
		w.Write([]byte("Stable"))
	})))

	// Helper endpoint to generate a key (FOR TESTING ONLY)
	s.router.HandleFunc("/api/test/generate-key", func(w http.ResponseWriter, r *http.Request) {
		userID := r.URL.Query().Get("user_id")
		if userID == "" {
			userID = "test-user"
		}
		rawKey, err := s.authService.CreateAPIKey(r.Context(), userID, "test-key", nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Write([]byte(rawKey))
	})

	// Global Chain
	globalChain := func(h http.Handler) http.Handler {
		// Metrics -> Audit -> Security -> Policy -> Auth -> RateLimit -> Handler
		return metricsMw(auditMw(securityMw(policyMw(authMiddleware.Handle(rateLimitMiddleware(h))))))
	}

	srv := &http.Server{
		Addr:    ":" + s.cfg.ServerPort,
		Handler: globalChain(s.router), // Wrap everything
	}

	// Channel to listen for errors coming from the listener.
	serverErrors := make(chan error, 1)

	go func() {
		log.Printf("Server starting on port %s", s.cfg.ServerPort)
		serverErrors <- srv.ListenAndServe()
	}()

	// Channel to listen for an interrupt or terminate signal from the OS.
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	// Block mainly waiting for a signal
	select {
	case err := <-serverErrors:
		return fmt.Errorf("server error: %w", err)

	case sig := <-shutdown:
		log.Printf("main: %v : Start shutdown", sig)

		// Give outstanding requests a deadline for completion.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			srv.Close()
			return fmt.Errorf("could not stop server gracefully: %w", err)
		}
	}

	return nil
}
