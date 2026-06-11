package session

import (
	"fmt"
	"net/http"
	"time"

	"github.com/crgimenes/devengine/log"
)

// Routes registers the session routes on the mux.
func Routes(mux *http.ServeMux) {
	mux.HandleFunc("/events", sseHandler)
}

func sseHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// check session - session is required for SSE
	sid, ok := GetCookie(r)
	if !ok {
		log.Printf("SSE: No session cookie found")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	u, ok := Get(sid)
	if !ok {
		log.Printf("SSE: Session %s not found in store", sid[:min(8, len(sid))])
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// user must be enabled to use SSE
	if !u.Enabled {
		log.Printf("SSE: User %s is not enabled (enabled=%v)", u.Email, u.Enabled)
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	// Disable write deadline for SSE connections
	rc := http.NewResponseController(w)
	if rc != nil {
		_ = rc.SetWriteDeadline(time.Time{})
	}

	// Set headers for SSE - critical for client reconnection behavior
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Keep-Alive", "timeout=60")
	w.Header().Set("X-Accel-Buffering", "no") // disable nginx buffering

	// Ensure response writer supports flushing
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Send initial SSE directives/data to confirm connection on client
	_, _ = fmt.Fprintf(w, "retry: 10000\n") // suggest 10s retry if client reconnects
	_, _ = fmt.Fprintf(w, "data: ready\n\n")
	flusher.Flush()

	// Create channel for this session with a larger buffer to absorb short bursts
	ch := make(chan string, 64)
	RegisterSSEChannel(sid, ch)

	defer func() {
		UnregisterSSEChannel(sid, ch)
		close(ch)
	}()

	ctx := r.Context()
	// Many proxies use a 30s idle timeout; send heartbeat sooner to avoid drop
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	// periodic heartbeat to keep connection alive (prevents timeout by proxy/firewall)
	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			// Send heartbeat as data event so clients see activity and proxies keep stream
			_, err := fmt.Fprintf(w, "data: heartbeat\n\n")
			if err != nil {
				log.Printf("SSE heartbeat write error: %v", err)
				return
			}
			flusher.Flush()
		case msg, ok := <-ch:
			if !ok {
				// Channel closed, connection shutting down
				return
			}

			// Format message according to SSE spec
			_, err := fmt.Fprintf(w, "data: %s\n\n", msg)
			if err != nil {
				log.Printf("SSE write error: %v", err)
				return
			}
			flusher.Flush()
			log.Printf("SSE message sent to user %s: %q", u.Email, msg)
		}
	}
}
