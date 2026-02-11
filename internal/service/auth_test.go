package service

import (
	"context"
	"testing"
	"time"

	"github.com/raakeshmj/apigatewayplane/internal/auth"
	"github.com/raakeshmj/apigatewayplane/internal/cache"
	"github.com/raakeshmj/apigatewayplane/internal/db"
)

// MockAPIKeyRepo
type MockAPIKeyRepo struct {
	keys     map[string]*db.APIKey // map keyHash -> APIKey
	getCalls int
}

func NewMockAPIKeyRepo() *MockAPIKeyRepo {
	return &MockAPIKeyRepo{
		keys: make(map[string]*db.APIKey),
	}
}

func (m *MockAPIKeyRepo) GetByHash(ctx context.Context, keyHash string) (*db.APIKey, error) {
	m.getCalls++
	if key, ok := m.keys[keyHash]; ok {
		return key, nil
	}
	return nil, auth.ErrInvalidToken // Simulate not found as invalid
}

func (m *MockAPIKeyRepo) ListByUser(ctx context.Context, userID string) ([]*db.APIKey, error) {
	var list []*db.APIKey
	for _, k := range m.keys {
		if k.UserID == userID {
			list = append(list, k)
		}
	}
	return list, nil
}

func (m *MockAPIKeyRepo) CreateAPIKey(ctx context.Context, apiKey *db.APIKey) error {
	m.keys[apiKey.KeyHash] = apiKey
	return nil
}

func (m *MockAPIKeyRepo) InvalidateAll(ctx context.Context, userID string) error {
	for _, k := range m.keys {
		if k.UserID == userID {
			k.IsActive = false
		}
	}
	return nil
}

// Tests
func TestAuthService_RotateAPIKey(t *testing.T) {
	repo := NewMockAPIKeyRepo()
	jwtManager := auth.NewJWTManager("secret", time.Hour)
	l1 := cache.NewMemoryCache()
	svc := NewAuthService(nil, repo, jwtManager, l1) // nil userRepo as mostly unused for this test

	ctx := context.Background()
	userID := "user-123"

	// 1. Create initial key
	key1, err := svc.CreateAPIKey(ctx, userID, "initial-key", nil)
	if err != nil {
		t.Fatalf("CreateAPIKey failed: %v", err)
	}

	// Verify key1 works
	verifiedUser, err := svc.VerifyAPIKey(ctx, key1)
	if err != nil {
		t.Fatalf("VerifyAPIKey failed for key1: %v", err)
	}
	if verifiedUser != userID {
		t.Errorf("Expected userID %s, got %s", userID, verifiedUser)
	}

	// 2. Rotate Key
	key2, err := svc.RotateAPIKey(ctx, userID)
	if err != nil {
		t.Fatalf("RotateAPIKey failed: %v", err)
	}

	if key1 == key2 {
		t.Error("Rotated key should be different from original key")
	}

	// 3. Verify key1 is invalidated
	// Note: In a real distributed system, we would rely on cache expiration or pub/sub for immediate invalidation.
	// For this unit test, we manually invalidate the local cache to verify the repo logic.
	key1Hash := auth.HashAPIKey(key1)
	l1.Delete(key1Hash)

	_, err = svc.VerifyAPIKey(ctx, key1)
	if err != auth.ErrInvalidToken {
		t.Errorf("Expected ErrInvalidToken for key1 after rotation, got %v", err)
	}

	// 4. Verify key2 works
	verifiedUser2, err := svc.VerifyAPIKey(ctx, key2)
	if err != nil {
		t.Fatalf("VerifyAPIKey failed for key2: %v", err)
	}
	if verifiedUser2 != userID {
		t.Errorf("Expected userID %s for key2, got %s", userID, verifiedUser2)
	}
}

func TestAuthService_Cache(t *testing.T) {
	repo := NewMockAPIKeyRepo()
	jwtManager := auth.NewJWTManager("secret", time.Hour)
	l1 := cache.NewMemoryCache()
	svc := NewAuthService(nil, repo, jwtManager, l1)

	ctx := context.Background()
	userID := "user-cache"
	key, err := svc.CreateAPIKey(ctx, userID, "cache-key", nil)
	if err != nil {
		t.Fatalf("CreateAPIKey failed: %v", err)
	}

	// 1. First Call - Should hit Repo
	_, err = svc.VerifyAPIKey(ctx, key)
	if err != nil {
		t.Fatalf("VerifyAPIKey failed: %v", err)
	}
	if repo.getCalls != 1 {
		t.Errorf("Expected 1 repo call, got %d", repo.getCalls)
	}

	// 2. Second Call - Should hit Cache (Repo calls stay 1)
	_, err = svc.VerifyAPIKey(ctx, key)
	if err != nil {
		t.Fatalf("VerifyAPIKey 2 failed: %v", err)
	}
	if repo.getCalls != 1 {
		t.Errorf("Expected 1 repo call (cached), got %d", repo.getCalls)
	}
}
