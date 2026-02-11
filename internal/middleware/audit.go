package middleware

import (
	"net/http"
	"time"

	"github.com/raakeshmj/apigatewayplane/internal/audit"
)

func AuditMiddleware(logger audit.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Capture Response Status
			rw := &responseWriterInterceptor{
				ResponseWriter: w,
				statusCode:     http.StatusOK, // Default
			}

			next.ServeHTTP(rw, r)

			// Extract Actor (from Auth context)
			actorID := "anonymous"
			if u, ok := r.Context().Value(UserContextKey).(string); ok {
				actorID = u
			}

			// Tenant? (Mock: "default" or from context)
			tenantID := "default"

			entry := audit.LogEntry{
				Timestamp: start,
				TenantID:  tenantID,
				ActorID:   actorID,
				Action:    r.Method + " " + r.URL.Path,
				Resource:  r.URL.Path,
				Status:    rw.statusCode,
				Metadata: map[string]interface{}{
					"remote_addr": r.RemoteAddr,
					"duration_ms": time.Since(start).Milliseconds(),
					// Sensitive check: headers?
				},
			}

			logger.Log(entry)
		})
	}
}
