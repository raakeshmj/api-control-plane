package circuitbreaker

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	ErrCircuitOpen = errors.New("circuit breaker is open")
)

type State int

const (
	StateClosed   State = 0
	StateOpen     State = 1
	StateHalfOpen State = 2
)

// Redis Keys:
// cb:{service}:state -> 0 (Closed), 1 (Open), 2 (Half-Open)
// cb:{service}:failures -> count
// cb:{service}:successes -> count (for Half-Open)

type CircuitBreaker struct {
	client           *redis.Client
	failureThreshold int64
	successThreshold int64
	timeout          time.Duration
}

func New(client *redis.Client, failureThreshold, successThreshold int64, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		client:           client,
		failureThreshold: failureThreshold,
		successThreshold: successThreshold,
		timeout:          timeout,
	}
}

// Execute wrapper
func (cb *CircuitBreaker) Execute(ctx context.Context, serviceName string, action func() error) error {
	stateKey := "cb:" + serviceName + ":state"
	failureKey := "cb:" + serviceName + ":failures"

	// 1. Check State
	stateVal, err := cb.client.Get(ctx, stateKey).Int()
	if err != nil && err != redis.Nil {
		return err // Redis error
	}
	// Default Closed if nil

	state := State(stateVal)

	if state == StateOpen {
		return ErrCircuitOpen
	}

	// Double Check Half-Open?
	// If Open, we rely on TTL of the state key to transition to Half-Open?
	// Or we check if time passed?
	// Redis EXPIRE is great for this. If StateOpen key expires, it's gone (or we check existence).
	// Let's say:
	// - StateOpen set with TTL.
	// - If key doesn't exist, we assume Closed (or we need explicit Half-Open logic).
	// Better:
	// - Open -> Set "state" to Open with TTL.
	// - When TTL expires, key is gone -> Closed? No, should be Half-Open.
	// Logic:
	// - Check "open_lock" key. If exists, Circuit is Open. Return Error.
	// - If not exists, proceed (Closed or Half-Open implicitly).

	// Let's refine:
	// Use a Lua script to check state and increment counters atomically?
	// Or simple:
	// Key: cb:{service}:failures

	// If failures > threshold -> Set "cb:{service}:open" = 1 with TTL.

	open, _ := cb.client.Exists(ctx, "cb:"+serviceName+":open").Result()
	if open > 0 {
		return ErrCircuitOpen
	}

	// Execute Action
	opErr := action()

	if opErr != nil {
		// Failure
		// Increment failures
		failures, _ := cb.client.Incr(ctx, failureKey).Result()
		if failures >= cb.failureThreshold {
			// Trip Breaker
			cb.client.Set(ctx, "cb:"+serviceName+":open", "1", cb.timeout)
			// Reset failures? Or keep them?
			cb.client.Del(ctx, failureKey)
		}
		return opErr
	}

	// Success
	// Should simplify: simpler is better.
	// We are in Closed (implied).
	// If success, maybe reset failures? (Sliding window or Reset on success?)
	// Common: Reset failures on success (Consecutive failures).
	cb.client.Del(ctx, failureKey)

	return nil
}
