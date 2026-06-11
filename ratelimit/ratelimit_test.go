package ratelimit

import (
	"net/http/httptest"
	"testing"
	"time"
)

// newTestLimiter returns a limiter with a controllable clock.
func newTestLimiter(start time.Time) (*Limiter, *time.Time) {
	clock := start
	l := New()
	l.now = func() time.Time { return clock }
	return l, &clock
}

func TestAllowBurstThenBlocks(t *testing.T) {
	l, _ := newTestLimiter(time.Unix(0, 0))

	for i := range 5 {
		if !l.Allow("ip", 60, 5) {
			t.Fatalf("request %d inside burst blocked", i)
		}
	}
	if l.Allow("ip", 60, 5) {
		t.Fatal("request beyond burst allowed")
	}
}

func TestAllowRefillsOverTime(t *testing.T) {
	l, clock := newTestLimiter(time.Unix(0, 0))

	for range 5 {
		l.Allow("ip", 60, 5)
	}
	if l.Allow("ip", 60, 5) {
		t.Fatal("bucket should be empty")
	}

	// 60/min refills one token per second.
	*clock = clock.Add(time.Second)
	if !l.Allow("ip", 60, 5) {
		t.Fatal("token not refilled after 1s at 60/min")
	}
	if l.Allow("ip", 60, 5) {
		t.Fatal("second request after single refill allowed")
	}
}

func TestAllowDisabledWhenPerMinZero(t *testing.T) {
	l, _ := newTestLimiter(time.Unix(0, 0))

	for i := range 1000 {
		if !l.Allow("ip", 0, 0) {
			t.Fatalf("disabled limiter blocked request %d", i)
		}
	}
}

func TestAllowKeysAreIndependent(t *testing.T) {
	l, _ := newTestLimiter(time.Unix(0, 0))

	for range 3 {
		l.Allow("a", 60, 3)
	}
	if l.Allow("a", 60, 3) {
		t.Fatal("key a should be exhausted")
	}
	if !l.Allow("b", 60, 3) {
		t.Fatal("key b should be untouched")
	}
}

func TestBurstFallsBackToPerMin(t *testing.T) {
	l, _ := newTestLimiter(time.Unix(0, 0))

	for i := range 10 {
		if !l.Allow("ip", 10, 0) {
			t.Fatalf("request %d inside implicit burst blocked", i)
		}
	}
	if l.Allow("ip", 10, 0) {
		t.Fatal("request beyond implicit burst allowed")
	}
}

func TestSweepDropsIdleBuckets(t *testing.T) {
	l, clock := newTestLimiter(time.Unix(0, 0))

	l.Allow("idle", 60, 5)
	*clock = clock.Add(bucketIdleTTL + sweepInterval + time.Second)
	l.Allow("active", 60, 5)

	l.mu.Lock()
	_, ok := l.buckets["idle"]
	l.mu.Unlock()
	if ok {
		t.Fatal("idle bucket survived the sweep")
	}
}

func TestClientIP(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.1.2.3:4567"
	r.Header.Set("X-Forwarded-For", "203.0.113.9, 10.0.0.1")

	got := ClientIP(r, false)
	if got != "10.1.2.3" {
		t.Fatalf("ClientIP without proxy trust = %q, want 10.1.2.3", got)
	}

	got = ClientIP(r, true)
	if got != "203.0.113.9" {
		t.Fatalf("ClientIP with proxy trust = %q, want 203.0.113.9", got)
	}

	r.Header.Del("X-Forwarded-For")
	got = ClientIP(r, true)
	if got != "10.1.2.3" {
		t.Fatalf("ClientIP with proxy trust but no header = %q, want 10.1.2.3", got)
	}
}
