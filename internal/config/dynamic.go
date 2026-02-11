package config

import (
	"sync"
)

// PolicyConfig holds dynamic policy values
type PolicyConfig struct {
	DefaultRateLimit float64 `json:"default_rate"`
	DefaultBurst     int     `json:"default_burst"`
}

// DynamicConfigManager manages thread-safe config updates
type DynamicConfigManager struct {
	mu     sync.RWMutex
	policy PolicyConfig
}

func NewDynamicConfigManager() *DynamicConfigManager {
	return &DynamicConfigManager{
		policy: PolicyConfig{
			DefaultRateLimit: 1.0, // Default 1 req/sec
			DefaultBurst:     5,   // Default burst 5
		},
	}
}

func (m *DynamicConfigManager) GetPolicy() PolicyConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.policy
}

func (m *DynamicConfigManager) UpdatePolicy(newPolicy PolicyConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.policy = newPolicy
}
