package repository

import (
	"context"

	"github.com/raakeshmj/apigatewayplane/internal/db"
)

type UserRepository interface {
	Get(ctx context.Context, id string) (*db.User, error)
	CreateUser(ctx context.Context, user *db.User) error
}

type APIKeyRepository interface {
	GetByHash(ctx context.Context, keyHash string) (*db.APIKey, error)
	ListByUser(ctx context.Context, userID string) ([]*db.APIKey, error)
	CreateAPIKey(ctx context.Context, apiKey *db.APIKey) error
	InvalidateAll(ctx context.Context, userID string) error
}
