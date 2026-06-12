package session

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/crgimenes/devengine/db"
)

func TestSSEHandlerGates(t *testing.T) {
	resetStore(t)
	EnableInsecureCookie()

	// Wrong method.
	rr := httptest.NewRecorder()
	sseHandler(rr, httptest.NewRequest(http.MethodPost, "/events", nil))
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST = %d, want 405", rr.Code)
	}

	// No cookie.
	rr = httptest.NewRecorder()
	sseHandler(rr, httptest.NewRequest(http.MethodGet, "/events", nil))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("no cookie = %d, want 401", rr.Code)
	}

	// Unknown session.
	rr = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/events", nil)
	req.AddCookie(&http.Cookie{Name: "sid", Value: "ghost"})
	sseHandler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("unknown session = %d, want 401", rr.Code)
	}

	// Disabled user.
	Put("sid-disabled", db.User{ID: 1, Enabled: false})
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/events", nil)
	req.AddCookie(&http.Cookie{Name: "sid", Value: "sid-disabled"})
	sseHandler(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("disabled user = %d, want 403", rr.Code)
	}
}

func TestSSEHandlerStreams(t *testing.T) {
	resetStore(t)
	EnableInsecureCookie()
	Put("sid-sse", db.User{ID: 2, Enabled: true, Email: "sse@example.com"})

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/events", nil).WithContext(ctx)
	req.AddCookie(&http.Cookie{Name: "sid", Value: "sid-sse"})
	rr := httptest.NewRecorder()

	var wg sync.WaitGroup
	wg.Go(func() {
		sseHandler(rr, req)
	})

	// Wait for the channel registration, push a message, then disconnect.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		sessions.RLock()
		n := len(sessions.m["sid-sse"].channels)
		sessions.RUnlock()
		if n > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	SendSSENotification("sid-sse", "hello")
	time.Sleep(50 * time.Millisecond) // let the handler flush the message
	cancel()
	wg.Wait()

	body := rr.Body.String()
	if !strings.Contains(body, "data: ready") {
		t.Fatalf("missing ready event: %q", body)
	}
	if !strings.Contains(body, "data: hello") {
		t.Fatalf("missing pushed message: %q", body)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("Content-Type = %q", ct)
	}

	// Channel unregistered after disconnect.
	sessions.RLock()
	left := len(sessions.m["sid-sse"].channels)
	sessions.RUnlock()
	if left != 0 {
		t.Fatalf("%d channels left after disconnect", left)
	}
}
