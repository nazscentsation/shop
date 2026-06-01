package middleware

import (
	"net/http"
	"sync"
	"time"
)

type bucket struct {
	tokens   int
	lastSeen time.Time
}

type RateLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*bucket
	rate     int
	capacity int
	cleanup  time.Duration
}

func NewRateLimiter(requestsPerMinute, capacity int) *RateLimiter {
	rl := &RateLimiter{
		buckets:  make(map[string]*bucket),
		rate:     requestsPerMinute,
		capacity: capacity,
		cleanup:  5 * time.Minute,
	}
	go rl.runCleanup()
	return rl
}

func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	b, ok := rl.buckets[ip]
	if !ok {
		rl.buckets[ip] = &bucket{tokens: rl.capacity - 1, lastSeen: time.Now()}
		return true
	}

	elapsed := time.Since(b.lastSeen).Minutes()
	b.tokens += int(elapsed * float64(rl.rate))
	if b.tokens > rl.capacity {
		b.tokens = rl.capacity
	}
	b.lastSeen = time.Now()

	if b.tokens <= 0 {
		return false
	}
	b.tokens--
	return true
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			ip = xff
		}
		if !rl.Allow(ip) {
			http.Error(w, `{"error":"too many requests"}`, http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (rl *RateLimiter) runCleanup() {
	ticker := time.NewTicker(rl.cleanup)
	for range ticker.C {
		rl.mu.Lock()
		for ip, b := range rl.buckets {
			if time.Since(b.lastSeen) > rl.cleanup {
				delete(rl.buckets, ip)
			}
		}
		rl.mu.Unlock()
	}
}
