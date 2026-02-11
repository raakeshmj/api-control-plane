package middleware

import (
	"context"
	"net/http"

	"github.com/raakeshmj/apigatewayplane/internal/policy"
)

type contextKey string

const PolicyContextKey contextKey = "policy"

// PolicyEnforcer evaluates the request and attaches the policy to context
func PolicyEnforcer(engine *policy.Engine) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p := engine.Evaluate(r)

			// If no policy, what is default?
			// Secure default: Deny all? Or Allow generic?
			// Let's assume a "Default Policy" exists or we create one on fly.
			if p == nil {
				// Fallback Default
				p = &policy.Policy{
					ID: "default",
					Rules: policy.Rules{
						AuthRequired: true, // Default Secure
						RateLimit:    1.0,
						Burst:        5,
					},
				}
			}

			ctx := context.WithValue(r.Context(), PolicyContextKey, p)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Helper to get policy from context
func GetPolicy(ctx context.Context) *policy.Policy {
	if p, ok := ctx.Value(PolicyContextKey).(*policy.Policy); ok {
		return p
	}
	return nil
}
