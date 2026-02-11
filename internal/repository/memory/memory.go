package memory

import (
	"context"
	"sync"

	"github.com/raakeshmj/apigatewayplane/internal/auth"
	"github.com/raakeshmj/apigatewayplane/internal/db"
	"github.com/raakeshmj/apigatewayplane/internal/repository"
)

type MemoryRepository struct {
	users   map[string]*db.User
	apiKeys map[string]*db.APIKey // Map keyHash -> APIKey
	mu      sync.RWMutex
}

func New() *MemoryRepository {
	return &MemoryRepository{
		users:   make(map[string]*db.User),
		apiKeys: make(map[string]*db.APIKey),
	}
}

// User Repo Implementation
func (r *MemoryRepository) Get(ctx context.Context, id string) (*db.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if u, ok := r.users[id]; ok {
		return u, nil
	}
	return nil, auth.ErrInvalidToken // Simplified
}

func (r *MemoryRepository) CreateUser(ctx context.Context, user *db.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.users[user.ID] = user
	return nil
}

// APIKey Repo Implementation
func (r *MemoryRepository) GetByHash(ctx context.Context, keyHash string) (*db.APIKey, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if k, ok := r.apiKeys[keyHash]; ok {
		return k, nil
	}
	return nil, auth.ErrInvalidToken
}

func (r *MemoryRepository) ListByUser(ctx context.Context, userID string) ([]*db.APIKey, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var list []*db.APIKey
	for _, k := range r.apiKeys {
		if k.UserID == userID {
			list = append(list, k)
		}
	}
	return list, nil
}

func (r *MemoryRepository) CreateAPIKey(ctx context.Context, apiKey *db.APIKey) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.apiKeys[apiKey.KeyHash] = apiKey
	return nil
}

func (r *MemoryRepository) InvalidateAll(ctx context.Context, userID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, k := range r.apiKeys {
		if k.UserID == userID {
			k.IsActive = false
		}
	}
	return nil
}

// Interface check
var _ repository.UserRepository = (*MemoryRepository)(nil)
var _ repository.APIKeyRepository = (*MemoryRepository)(nil)
