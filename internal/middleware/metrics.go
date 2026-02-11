package middleware

import (
	"net/http"
	"time"

	"github.com/raakeshmj/apigatewayplane/internal/metrics"
)

func MetricsMiddleware(collector *metrics.MetricsCollector) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Capture Status Code
			rw := &responseWriterInterceptor{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			next.ServeHTTP(rw, r)

			duration := time.Since(start)
			collector.Record(duration, rw.statusCode)
		})
	}
}
