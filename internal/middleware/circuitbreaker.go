package middleware

import (
	"net/http"

	"github.com/raakeshmj/apigatewayplane/internal/circuitbreaker"
)

func CircuitBreakerMiddleware(cb *circuitbreaker.CircuitBreaker, serviceName string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			// Wrapper function for the "action"
			// But wait, middleware wraps the HANDLER.
			// The handler calls downstream.
			// The middleware doesn't know if the handler "failed" unless the handler returns an error?
			// `ServeHTTP` doesn't return error. It writes to ResponseWriter.

			// How to capture error?
			// 1. Check HTTP Status Code (ResponseRecorder).
			// 2. Panic?

			// Capturing 5xx status codes is the standard way.

			rw := &responseWriterInterceptor{
				ResponseWriter: w,
				statusCode:     http.StatusOK, // Default
			}

			err := cb.Execute(r.Context(), serviceName, func() error {
				next.ServeHTTP(rw, r)

				// Interpret status code
				if rw.statusCode >= 500 {
					return http.ErrHandlerTimeout // Just a generic error marker
				}
				return nil
			})

			if err == circuitbreaker.ErrCircuitOpen {
				http.Error(w, "Service Unavailable (Circuit Open)", http.StatusServiceUnavailable)
				return
			}

			// If Execute returns error (500 from downstream), it's already written to RW?
			// Yes, `next.ServeHTTP` wrote to `rw`.
			// But `rw` buffers? No, it writes to `w`.
			// Wait, `Execute` calls the function. The function calls `next.ServeHTTP`.
			// If `next.ServeHTTP` writes 500, `cb.Execute` sees error, trips breaker.
			// The client receives the 500.
			// Next request -> `cb.Execute` sees Open -> returns ErrCircuitOpen -> We write 503.

			// Perfect.
		})
	}
}

// responseWriterInterceptor captures the status code
type responseWriterInterceptor struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriterInterceptor) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
