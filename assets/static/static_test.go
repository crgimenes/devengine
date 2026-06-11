package static

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/crgimenes/devengine/assets"
)

// initIndex builds the index over the real embedded assets, optionally with
// an app FS layered on top, and restores the package state afterwards.
func initIndex(t testing.TB, app http.FileSystem) {
	t.Helper()
	prevIndex, prevApp := index, appFS
	appFS = app
	err := Init()
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(func() { index, appFS = prevIndex, prevApp })
}

func get(t testing.TB, path string, header map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for k, v := range header {
		req.Header.Set(k, v)
	}
	rr := httptest.NewRecorder()
	Handler(rr, req)
	return rr
}

func TestHandlerServesAssetWithETag(t *testing.T) {
	initIndex(t, nil)

	rr := get(t, "/assets/style.css", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET style.css = %d", rr.Code)
	}
	if rr.Header().Get("ETag") == "" {
		t.Fatal("missing ETag header")
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/css") {
		t.Fatalf("Content-Type = %q, want text/css", ct)
	}
	if rr.Body.Len() == 0 {
		t.Fatal("empty body")
	}
	// Unversioned URL must force revalidation, not long-lived caching.
	if cc := rr.Header().Get("Cache-Control"); !strings.Contains(cc, "must-revalidate") {
		t.Fatalf("Cache-Control = %q, want must-revalidate", cc)
	}
}

func TestHandlerVersionedURLIsImmutable(t *testing.T) {
	initIndex(t, nil)

	rr := get(t, "/assets/style.css?v=abc123", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET versioned = %d", rr.Code)
	}
	cc := rr.Header().Get("Cache-Control")
	if !strings.Contains(cc, "immutable") {
		t.Fatalf("Cache-Control = %q, want immutable", cc)
	}
}

func TestHandlerIfNoneMatchAnswers304(t *testing.T) {
	initIndex(t, nil)

	etag, ok := ETag("style.css")
	if !ok {
		t.Fatal("style.css not indexed")
	}

	for _, inm := range []string{etag, "W/" + etag, `"other", ` + etag, "*"} {
		rr := get(t, "/assets/style.css", map[string]string{"If-None-Match": inm})
		if rr.Code != http.StatusNotModified {
			t.Fatalf("If-None-Match %q = %d, want 304", inm, rr.Code)
		}
		if rr.Body.Len() != 0 {
			t.Fatalf("304 carried a body for %q", inm)
		}
	}

	rr := get(t, "/assets/style.css", map[string]string{"If-None-Match": `"stale"`})
	if rr.Code != http.StatusOK {
		t.Fatalf("stale If-None-Match = %d, want 200", rr.Code)
	}
}

func TestHandlerHeadHasNoBody(t *testing.T) {
	initIndex(t, nil)

	req := httptest.NewRequest(http.MethodHead, "/assets/style.css", nil)
	rr := httptest.NewRecorder()
	Handler(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("HEAD = %d", rr.Code)
	}
	if rr.Body.Len() != 0 {
		t.Fatal("HEAD carried a body")
	}
	if rr.Header().Get("ETag") == "" {
		t.Fatal("HEAD missing ETag")
	}
}

func TestHandlerRejectsTraversalAndUnknown(t *testing.T) {
	initIndex(t, nil)

	rr := get(t, "/assets/../config/config.go", nil)
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusNotFound {
		t.Fatalf("traversal = %d, want 400/404", rr.Code)
	}

	rr = get(t, "/assets/nope.css", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("unknown asset = %d, want 404", rr.Code)
	}

	req := httptest.NewRequest(http.MethodPost, "/assets/style.css", nil)
	rec := httptest.NewRecorder()
	Handler(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST = %d, want 405", rec.Code)
	}
}

func TestAppFSOverridesEngineAsset(t *testing.T) {
	app := http.FS(fstest.MapFS{
		"style.css": {Data: []byte("/* app override */")},
		"extra.js":  {Data: []byte("console.log('app')")},
	})
	initIndex(t, app)

	rr := get(t, "/assets/style.css", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET overridden = %d", rr.Code)
	}
	body, _ := io.ReadAll(rr.Body)
	if string(body) != "/* app override */" {
		t.Fatalf("override not served: %.60q", body)
	}

	rr = get(t, "/assets/extra.js", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET app-only asset = %d", rr.Code)
	}
}

// The in-memory index must beat a plain http.FileServer over the embedded FS
// to justify existing; the benchmarks below document the margin.

func BenchmarkHandlerHit(b *testing.B) {
	initIndex(b, nil)
	b.ReportAllocs()
	for b.Loop() {
		req := httptest.NewRequest(http.MethodGet, "/assets/style.css", nil)
		rr := httptest.NewRecorder()
		Handler(rr, req)
		if rr.Code != http.StatusOK {
			b.Fatalf("status %d", rr.Code)
		}
	}
}

func BenchmarkHandler304(b *testing.B) {
	initIndex(b, nil)
	etag, _ := ETag("style.css")
	b.ReportAllocs()
	for b.Loop() {
		req := httptest.NewRequest(http.MethodGet, "/assets/style.css", nil)
		req.Header.Set("If-None-Match", etag)
		rr := httptest.NewRecorder()
		Handler(rr, req)
		if rr.Code != http.StatusNotModified {
			b.Fatalf("status %d", rr.Code)
		}
	}
}

func BenchmarkFileServerBaseline(b *testing.B) {
	fs := http.FileServer(assets.FS)
	b.ReportAllocs()
	for b.Loop() {
		req := httptest.NewRequest(http.MethodGet, "/style.css", nil)
		rr := httptest.NewRecorder()
		fs.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			b.Fatalf("status %d", rr.Code)
		}
	}
}
