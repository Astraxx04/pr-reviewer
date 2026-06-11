package middleware

import (
	"net/http"
	"sync"
	"time"
)

type tokenBucket struct {
	tokens   float64
	lastSeen time.Time
}

// RateLimiter is an in-memory token-bucket rate limiter keyed by string.
type RateLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*tokenBucket
	capacity float64 // max burst
	rate     float64 // tokens added per second
}

func NewRateLimiter(reqPerHour int) *RateLimiter {
	capacity := float64(reqPerHour) / 10 // allow burst of ~6 minutes worth
	if capacity < 10 {
		capacity = 10
	}
	rl := &RateLimiter{
		buckets:  make(map[string]*tokenBucket),
		capacity: capacity,
		rate:     float64(reqPerHour) / 3600.0,
	}
	go rl.cleanup()
	return rl
}

func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, ok := rl.buckets[key]
	if !ok {
		b = &tokenBucket{tokens: rl.capacity, lastSeen: now}
		rl.buckets[key] = b
	}

	elapsed := now.Sub(b.lastSeen).Seconds()
	b.tokens += elapsed * rl.rate
	if b.tokens > rl.capacity {
		b.tokens = rl.capacity
	}
	b.lastSeen = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// Middleware returns an http.Handler middleware that rate-limits by the given key function.
func (rl *RateLimiter) Middleware(keyFn func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := keyFn(r)
			if key == "" {
				next.ServeHTTP(w, r)
				return
			}
			if !rl.Allow(key) {
				http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// cleanup removes buckets that haven't been used in the last hour.
func (rl *RateLimiter) cleanup() {
	t := time.NewTicker(10 * time.Minute)
	for range t.C {
		cutoff := time.Now().Add(-1 * time.Hour)
		rl.mu.Lock()
		for k, b := range rl.buckets {
			if b.lastSeen.Before(cutoff) {
				delete(rl.buckets, k)
			}
		}
		rl.mu.Unlock()
	}
}
