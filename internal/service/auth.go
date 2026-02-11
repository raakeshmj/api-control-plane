package service

import (
	"context"
	"time"

	"github.com/raakeshmj/apigatewayplane/internal/auth"
	"github.com/raakeshmj/apigatewayplane/internal/cache"
	"github.com/raakeshmj/apigatewayplane/internal/db"
	"github.com/raakeshmj/apigatewayplane/internal/repository"
)

type AuthService struct {
	userRepo   repository.UserRepository
	apiKeyRepo repository.APIKeyRepository
	jwtManager *auth.JWTManager
	cache      *cache.MemoryCache
}

func NewAuthService(u repository.UserRepository, k repository.APIKeyRepository, j *auth.JWTManager, c *cache.MemoryCache) *AuthService {
	return &AuthService{
		userRepo:   u,
		apiKeyRepo: k,
		jwtManager: j,
		cache:      c,
	}
}

func (s *AuthService) JWTManager() *auth.JWTManager {
	return s.jwtManager
}

// VerifyAPIKey verifies the API key and returns the UserID
func (s *AuthService) VerifyAPIKey(ctx context.Context, rawKey string) (string, error) {
	hashed := auth.HashAPIKey(rawKey)

	// L1 Cache Check
	if val, found := s.cache.Get(hashed); found {
		if userID, ok := val.(string); ok {
			return userID, nil
		}
	}

	apiKey, err := s.apiKeyRepo.GetByHash(ctx, hashed)
	if err != nil {
		return "", err
	}

	if !apiKey.IsActive {
		return "", auth.ErrInvalidToken
	}

	// Set Cache
	s.cache.Set(hashed, apiKey.UserID, 1*time.Minute)

	return apiKey.UserID, nil
}

// CreateAPIKey generates a new key for the user
func (s *AuthService) CreateAPIKey(ctx context.Context, userID, name string, scopes []string) (string, error) {
	rawKey, keyHash, prefix, err := auth.GenerateAPIKey()
	if err != nil {
		return "", err
	}

	apiKey := &db.APIKey{
		UserID:    userID,
		KeyHash:   keyHash,
		Prefix:    prefix,
		Name:      name,
		Scopes:    scopes,
		CreatedAt: time.Now(),
		IsActive:  true,
	}

	if err := s.apiKeyRepo.CreateAPIKey(ctx, apiKey); err != nil {
		return "", err
	}

	return rawKey, nil
}

// RotateAPIKey invalidates old keys and creates a new one
func (s *AuthService) RotateAPIKey(ctx context.Context, userID string) (string, error) {
	// 1. Invalidate all existing keys for this user
	if err := s.apiKeyRepo.InvalidateAll(ctx, userID); err != nil {
		return "", err
	}

	// 2. Create new key
	return s.CreateAPIKey(ctx, userID, "rotated-key", nil)
}
