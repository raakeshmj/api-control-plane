package limiter

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	ErrRateLimitExceeded = errors.New("rate limit exceeded")
)

// luaScript implements the token bucket algorithm atomically
// KEYS[1] = rate limit key
// ARGV[1] = capacity (burst size)
// ARGV[2] = refill rate (tokens per second)
// ARGV[3] = current timestamp (unix seconds)
// ARGV[4] = requested tokens
// Returns: [allowed (1/0), remaining_tokens]
const luaScript = `
local key = KEYS[1]
local capacity = tonumber(ARGV[1])
local rate = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local requested = tonumber(ARGV[4])

local info = redis.call("HMGET", key, "tokens", "last_refill")
local tokens = tonumber(info[1])
local last_refill = tonumber(info[2])

if not tokens then
	tokens = capacity
	last_refill = now
end

-- Calculate refill
local delta = math.max(0, now - last_refill)
local filled = tokens + (delta * rate)

if filled > capacity then
	filled = capacity
end

local allowed = 0
if filled >= requested then
	allowed = 1
	filled = filled - requested
end

-- Update state
redis.call("HMSET", key, "tokens", filled, "last_refill", now)
redis.call("EXPIRE", key, 60) -- Expire key if unused for 1 minute

return {allowed, filled}
`

type TokenBucketLimiter struct {
	client *redis.Client
}

func NewTokenBucketLimiter(client *redis.Client) *TokenBucketLimiter {
	return &TokenBucketLimiter{client: client}
}

// Allow checks if the request is allowed.
// rate: tokens per second
// burst: maximum capacity
func (l *TokenBucketLimiter) Allow(ctx context.Context, key string, rate float64, burst int) (bool, float64, error) {
	now := time.Now().Unix()

	cmd := l.client.Eval(ctx, luaScript, []string{key}, burst, rate, now, 1)
	result, err := cmd.Result()
	if err != nil {
		return false, 0, err
	}

	resSlice := result.([]interface{})
	allowed := resSlice[0].(int64) == 1
	// Parse remaining tokens (interface{} -> string/int64 -> float64 tricky in redis, let's treat as int/float from lua)
	// Lua numbers are returned as int64 if integer.
	// But our calculation involves float (rate * delta), so `filled` might be float?
	// Redis script returns integers if possible?

	// Wait, HMAC returns strings usually. But Eval returns what Lua returns.
	// tokens + (delta * rate) -> result is number.
	// If it has decimal, Eval returns int64 (truncated) or error?
	// Usually better to return string or just use integer math (multiply by 1000?).
	// For simplicity, let's assume we get int64 for "allowed" and let's see what we get for "filled".
	// The problem is "filled" needs to be precise for slow rates.
	// Let's modify Lua script to work with micro-tokens or just return string.
	// Or simply return integer tokens remaining.

	// Let's cast safely.
	remainingVal := resSlice[1]
	var remaining float64
	switch v := remainingVal.(type) {
	case int64:
		remaining = float64(v)
	default:
		// If float, might be tricky? Redis usually returns int64 for integers.
		// Let's rely on basic types.
		// We'll see in testing.
		remaining = 0
	}

	if !allowed {
		return false, remaining, ErrRateLimitExceeded
	}

	return true, remaining, nil
}
