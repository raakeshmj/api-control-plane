package middleware

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"
)

// SecurityConfig options
type SecurityConfig struct {
	EnableReplayProtection bool
	ReplayWindow           time.Duration
}

func SecureHeaders(cfg SecurityConfig) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 1. Set Secure Headers
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("X-XSS-Protection", "1; mode=block")
			// HSTS (enable only if HTTPS, but good practice to include in prod)
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")

			// 2. Replay Protection (Optional)
			if cfg.EnableReplayProtection {
				ts := r.Header.Get("X-Timestamp")
				if ts == "" {
					http.Error(w, "Missing X-Timestamp header", http.StatusBadRequest)
					return
				}

				reqTime, err := strconv.ParseInt(ts, 10, 64)
				if err != nil {
					http.Error(w, "Invalid X-Timestamp header", http.StatusBadRequest)
					return
				}

				now := time.Now().Unix()
				diff := float64(now - reqTime)

				// Allow clock skew window (e.g. +/- 60s)
				window := cfg.ReplayWindow.Seconds()
				if math.Abs(diff) > window {
					http.Error(w, fmt.Sprintf("Request timestamp skewed (server: %d, req: %d)", now, reqTime), http.StatusForbidden)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}
