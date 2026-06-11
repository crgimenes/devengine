// Package ratelimit implements a per-key token bucket used to slow down
// abuse of public endpoints (login, signup). Keys are normally client IPs.
// The limiter holds no configuration: rate and burst arrive on each Allow
// call, so limits can come straight from the application config.
package ratelimit

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	sweepInterval = 5 * time.Minute
	bucketIdleTTL = 10 * time.Minute
)

// Default is the process-wide limiter shared by the engine's auth endpoints.
var Default = New()

type bucket struct {
	tokens float64
	last   time.Time
}

// Limiter is a token bucket map keyed by client. The zero limit (perMin <= 0)
// disables it, so applications opt in via configuration.
type Limiter struct {
	mu        sync.Mutex
	buckets   map[string]*bucket
	lastSweep time.Time
	now       func() time.Time // test hook
}

func New() *Limiter {
	return &Limiter{
		buckets: make(map[string]*bucket),
		now:     time.Now,
	}
}

// Allow reports whether key may proceed under a budget of perMin sustained
// requests per minute with bursts of up to burst requests. perMin <= 0
// disables the limit; burst <= 0 falls back to perMin.
func (l *Limiter) Allow(key string, perMin, burst int) bool {
	if perMin <= 0 {
		return true
	}
	if burst <= 0 {
		burst = perMin
	}
	rate := float64(perMin) / 60.0
	capacity := float64(burst)

	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	l.sweep(now)

	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: capacity, last: now}
		l.buckets[key] = b
	}

	b.tokens = min(capacity, b.tokens+rate*now.Sub(b.last).Seconds())
	b.last = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// Reset drops every bucket. Test helper.
func (l *Limiter) Reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.buckets = make(map[string]*bucket)
}

// sweep drops buckets idle long enough to be full again, bounding the map.
// Caller holds the lock.
func (l *Limiter) sweep(now time.Time) {
	if now.Sub(l.lastSweep) < sweepInterval {
		return
	}
	l.lastSweep = now
	for key, b := range l.buckets {
		if now.Sub(b.last) > bucketIdleTTL {
			delete(l.buckets, key)
		}
	}
}

// ClientIP extracts the client address used as the limiter key. With
// trustProxy it honors the first X-Forwarded-For hop, for deployments behind
// a reverse proxy (never enable it with the port exposed directly: the
// header is client-controlled).
func ClientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		fwd := r.Header.Get("X-Forwarded-For")
		if fwd != "" {
			first, _, _ := strings.Cut(fwd, ",")
			first = strings.TrimSpace(first)
			if first != "" {
				return first
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
