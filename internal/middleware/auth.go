package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/raakeshmj/apigatewayplane/internal/auth"
)

type ContextKey string

const (
	UserContextKey ContextKey = "user"
)

type AuthProvider interface {
	VerifyAPIKey(ctx context.Context, key string) (string, error) // Returns UserID
}

type AuthMiddleware struct {
	jwtManager *auth.JWTManager
	provider   AuthProvider
}

func NewAuth(jwtManager *auth.JWTManager, provider AuthProvider) *AuthMiddleware {
	return &AuthMiddleware{
		jwtManager: jwtManager,
		provider:   provider,
	}
}

func (m *AuthMiddleware) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Check Policy
		var authRequired = true
		if p := GetPolicy(r.Context()); p != nil {
			authRequired = p.Rules.AuthRequired
		}

		// 2. Extract Token
		var tokenStr string
		var apiKeyUserID string // To store userID if authenticated via API Key

		// Check Authorization Header for Bearer token
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
		} else {
			// Check API Key Header
			apiKey := r.Header.Get("X-API-Key")
			if apiKey != "" {
				// Validate API Key using the middleware's provider
				var err error
				apiKeyUserID, err = m.provider.VerifyAPIKey(r.Context(), apiKey)
				if err != nil {
					// Simulating a delay to prevent timing attacks (basic)
					time.Sleep(100 * time.Millisecond)
					http.Error(w, "Unauthorized: invalid API key", http.StatusUnauthorized)
					return
				}
				// If API Key is valid, inject UserID and proceed immediately
				ctx := context.WithValue(r.Context(), UserContextKey, apiKeyUserID)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}

		// 3. Handle Missing Token (if not already handled by API Key)
		if tokenStr == "" {
			if authRequired {
				http.Error(w, "Unauthorized: missing credentials", http.StatusUnauthorized)
				return
			}
			// Public access: if auth is not required and no token is present, proceed
			next.ServeHTTP(w, r)
			return
		}

		// 4. Verify JWT (if Bearer token found)
		claims, err := m.jwtManager.Verify(tokenStr)
		if err != nil {
			http.Error(w, "Unauthorized: invalid token", http.StatusUnauthorized)
			return
		}

		// Inject user into context and proceed
		ctx := context.WithValue(r.Context(), UserContextKey, claims.UserID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
