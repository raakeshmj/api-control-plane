package db

import (
	"encoding/json"
	"time"
)

type User struct {
	ID           string    `json:"id" db:"id"`
	Username     string    `json:"username" db:"username"`
	PasswordHash string    `json:"-" db:"password_hash"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

type APIKey struct {
	ID        string    `json:"id" db:"id"`
	UserID    string    `json:"user_id" db:"user_id"`
	KeyHash   string    `json:"-" db:"key_hash"`    // SHA256 hash of the raw key
	Prefix    string    `json:"prefix" db:"prefix"` // First few chars clear for identification
	Name      string    `json:"name" db:"name"`
	Scopes    []string  `json:"scopes" db:"scopes"` // e.g., "read", "write"
	ExpiresAt time.Time `json:"expires_at" db:"expires_at"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	IsActive  bool      `json:"is_active" db:"is_active"`
}

type Policy struct {
	ID         string          `json:"id" db:"id"`
	Name       string          `json:"name" db:"name"`
	Type       string          `json:"type" db:"type"`             // e.g., "rate_limit", "access_control"
	Definition json.RawMessage `json:"definition" db:"definition"` // Flexible JSON config
	CreatedAt  time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at" db:"updated_at"`
}
