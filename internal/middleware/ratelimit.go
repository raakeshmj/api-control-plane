package middleware

import (
	"context"
	"fmt"
	"net/http"

	"github.com/raakeshmj/apigatewayplane/internal/config"
	"github.com/raakeshmj/apigatewayplane/internal/limiter"
	"github.com/raakeshmj/apigatewayplane/internal/reliability"
)

type RateLimiter interface {
	Allow(ctx context.Context, key string, rate float64, burst int) (bool, float64, error)
}

// In-Memory mock or Redis interface
// We need to support Context and errors.
// Let's use the concrete struct for now or better an interface.
// Since internal/limiter has the struct, let's inject it.

// Config defines rate limits per tier
type RateLimitConfig struct {
	Rate  float64
	Burst int
}

func RateLimit(l *limiter.TokenBucketLimiter, cfgMgr *config.DynamicConfigManager) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 1. Get Policy from Context
			// Note: We need to import `middleware.GetPolicy` but we are IN `middleware` package.
			// So we can call `GetPolicy` directly if it's exported, or just use context key if private.
			// `PolicyContextKey` is unexported? No, it's exported `PolicyContextKey`.
			// But `GetPolicy` is in `policy.go` (same package).

			p := GetPolicy(r.Context())

			// Configuration from Policy
			rate := 1.0
			burst := 5
			if p != nil {
				rate = p.Rules.RateLimit
				burst = p.Rules.Burst
			}

			// 2. Identify client
			// We MUST use UserID if auth middleware ran before.
			userID, ok := r.Context().Value(UserContextKey).(string)
			key := ""
			if ok {
				key = "ratelimit:user:" + userID
			} else {
				// Fallback to IP
				key = "ratelimit:ip:" + r.RemoteAddr
			}

			allowed, remaining, err := l.Allow(r.Context(), key, rate, burst)

			// Set Headers
			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", burst))
			w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", int(remaining)))
			w.Header().Set("X-RateLimit-Reset", "1") // Simplified

			if err != nil {
				// If Redis fails, check strategy
				if err == limiter.ErrRateLimitExceeded {
					http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
					return
				}

				// System error (Redis down)
				// Strategy: Fail Open
				if reliability.ShouldAllow(reliability.FailOpen, err) {
					// Log error but allow
					fmt.Printf("Rate limiter error (Fail Open): %v\n", err)
					next.ServeHTTP(w, r)
					return
				}

				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}

			if !allowed {
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
