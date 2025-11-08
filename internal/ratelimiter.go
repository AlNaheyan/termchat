package internal

import (
	"sync"
	"time"
)

type RateLimiter struct {
	mu     sync.Mutex
	hits   map[string][]time.Time
	limit  int
	window time.Duration
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		hits:   make(map[string][]time.Time),
		limit:  limit,
		window: window,
	}
}

func (r *RateLimiter) Allow(key string) bool {
	now := time.Now()
	r.mu.Lock()
	defer r.mu.Unlock()
	windowStart := now.Add(-r.window)
	slice := r.hits[key]
	idx := 0
	for _, ts := range slice {
		if ts.After(windowStart) {
			slice[idx] = ts
			idx++
		}
	}
	slice = slice[:idx]
	if len(slice) >= r.limit {
		r.hits[key] = slice
		return false
	}
	slice = append(slice, now)
	r.hits[key] = slice
	return true
}
