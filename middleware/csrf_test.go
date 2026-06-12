package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/crgimenes/devengine/config"
)

func csrfEnv(t *testing.T) http.Handler {
	t.Helper()
	prev := config.Cfg
	config.Cfg = &config.Config{BaseURL: "http://app.example.com"}
	t.Cleanup(func() { config.Cfg = prev })

	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return CSRFProtection(ok)
}

func TestCSRFProtection(t *testing.T) {
	h := csrfEnv(t)

	do := func(method string, headers map[string]string) int {
		req := httptest.NewRequest(method, "/x", nil)
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		return rr.Code
	}

	// Cross-site POST from a browser: rejected.
	if code := do(http.MethodPost, map[string]string{"Sec-Fetch-Site": "cross-site"}); code != http.StatusForbidden {
		t.Fatalf("cross-site POST = %d, want 403", code)
	}

	// Same-origin POST: allowed.
	if code := do(http.MethodPost, map[string]string{"Sec-Fetch-Site": "same-origin"}); code != http.StatusOK {
		t.Fatalf("same-origin POST = %d, want 200", code)
	}

	// Non-browser POST (no Sec-Fetch-Site, no Origin — curl, API clients):
	// allowed; bearer-token API calls rely on this.
	if code := do(http.MethodPost, nil); code != http.StatusOK {
		t.Fatalf("non-browser POST = %d, want 200", code)
	}

	// Safe methods are never blocked, even cross-site.
	if code := do(http.MethodGet, map[string]string{"Sec-Fetch-Site": "cross-site"}); code != http.StatusOK {
		t.Fatalf("cross-site GET = %d, want 200", code)
	}

	// Origin matching the configured BaseURL (proxy scenario): allowed.
	if code := do(http.MethodPost, map[string]string{"Origin": "http://app.example.com"}); code != http.StatusOK {
		t.Fatalf("trusted origin POST = %d, want 200", code)
	}

	// Foreign Origin: rejected.
	if code := do(http.MethodPost, map[string]string{"Origin": "http://evil.example.com"}); code != http.StatusForbidden {
		t.Fatalf("foreign origin POST = %d, want 403", code)
	}
}
