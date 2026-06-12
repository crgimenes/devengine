package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/devengine/db/sqlite"
	"github.com/crgimenes/devengine/session"
	"github.com/crgimenes/devengine/utils"
)

// newAPITestEnv wires the api routes over a fresh database and returns the
// mux plus an authenticated user and its cookie.
func newAPITestEnv(t *testing.T) (*http.ServeMux, *db.User, *http.Cookie) {
	t.Helper()

	s, err := sqlite.NewWithPath(filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatalf("NewWithPath: %v", err)
	}
	t.Cleanup(s.Close)
	err = sqlite.RunMigrationOn(s)
	if err != nil {
		t.Fatalf("RunMigrationOn: %v", err)
	}
	prev := db.Storage
	db.Storage = s
	t.Cleanup(func() { db.Storage = prev })

	session.EnableInsecureCookie()
	u, err := s.CreateUser("ana", "ana@example.com", "x", false)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	sid := utils.NewOpaqueID()
	session.Put(sid, *u)
	t.Cleanup(func() { session.Del(sid) })

	mux := http.NewServeMux()
	Routes(mux)
	return mux, u, &http.Cookie{Name: "sid", Value: sid}
}

func TestMarkdownToHTMLHandler(t *testing.T) {
	mux, _, cookie := newAPITestEnv(t)

	req := httptest.NewRequest(http.MethodPost, "/api/markdown", strings.NewReader("# Title\n\n*bold-ish*"))
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("markdown = %d: %.300s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, "<h1") || !strings.Contains(body, "<em>") {
		t.Fatalf("markdown not converted: %.300s", body)
	}

	// Unauthenticated: redirected to login, no conversion.
	req = httptest.NewRequest(http.MethodPost, "/api/markdown", strings.NewReader("# x"))
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusFound {
		t.Fatalf("unauthenticated = %d, want 302", rr.Code)
	}

	// Wrong method.
	req = httptest.NewRequest(http.MethodGet, "/api/markdown", nil)
	req.AddCookie(cookie)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET = %d, want 405", rr.Code)
	}
}

func TestGetUserFilesHandler(t *testing.T) {
	mux, u, cookie := newAPITestEnv(t)

	for _, name := range []string{"f1.png", "f2.png", "f3.png"} {
		_, err := db.Storage.SaveFileMetadata(&db.File{
			UserID:           u.ID,
			OriginalFilename: name,
			Filename:         "opaque-" + name,
			Filesize:         10,
			Filetype:         "image/png",
			Filehash:         "h-" + name,
		})
		if err != nil {
			t.Fatalf("SaveFileMetadata: %v", err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/files?page=1&limit=2", nil)
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("files = %d: %.300s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	if err != nil {
		t.Fatalf("invalid JSON: %v: %.300s", err, rr.Body.String())
	}
	files, _ := resp["data"].([]any)
	if len(files) != 2 {
		t.Fatalf("page of %d files, want 2 (resp: %v)", len(files), resp)
	}
	if total, _ := resp["total"].(float64); total != 3 {
		t.Fatalf("total = %v, want 3", resp["total"])
	}

	// Unauthenticated: redirect.
	req = httptest.NewRequest(http.MethodGet, "/api/files", nil)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusFound {
		t.Fatalf("unauthenticated = %d, want 302", rr.Code)
	}
}
