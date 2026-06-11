package filemanager

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/crgimenes/devengine/config"
	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/devengine/session"
	"github.com/crgimenes/devengine/utils"
)

// newQuotaTestEnv wires the filemanager routes over a fresh database with a
// 1 MB quota and returns the mux plus an authenticated user cookie.
func newQuotaTestEnv(t *testing.T) (*http.ServeMux, *db.SQLite, *db.User, *http.Cookie) {
	t.Helper()

	s, err := db.NewWithPath(filepath.Join(t.TempDir(), "fm.db"))
	if err != nil {
		t.Fatalf("NewWithPath: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	err = db.RunMigrationOn(s)
	if err != nil {
		t.Fatalf("RunMigrationOn: %v", err)
	}
	prevStorage := db.Storage
	db.Storage = s
	t.Cleanup(func() { db.Storage = prevStorage })

	dir := t.TempDir()
	cfg := &config.Config{
		BaseURL:         "http://localhost:3210",
		LoginURL:        "/login",
		SessionDuration: time.Hour,
		SiteTitle:       "test",
		GitTag:          "test",
		DataPath:        dir,
		UploadPath:      dir,
		FileQuotaMB:     1,
	}
	prevCfg := config.Cfg
	config.Cfg = cfg
	t.Cleanup(func() { config.Cfg = prevCfg })

	session.EnableInsecureCookie()

	u, err := s.CreateUser("alice", "alice@example.com", "x", false)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	sid := utils.NewOpaqueID()
	session.Put(sid, *u)

	mux := http.NewServeMux()
	Routes(mux)
	return mux, s, u, &http.Cookie{Name: "sid", Value: sid}
}

// seedUsage records metadata as if the user already stored size bytes.
func seedUsage(t *testing.T, s *db.SQLite, userID, size int64) {
	t.Helper()
	_, err := s.SaveFileMetadata(&db.File{
		UserID:           userID,
		OriginalFilename: "big.png",
		Filename:         "big.png",
		Filesize:         size,
		Filetype:         "image/png",
		Filehash:         "big",
	})
	if err != nil {
		t.Fatalf("SaveFileMetadata: %v", err)
	}
}

// pngUpload builds a multipart body carrying a tiny valid PNG.
func pngUpload(t *testing.T) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile("file", "tiny.png")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	// PNG magic followed by filler so DetectContentType sees image/png.
	_, err = fw.Write(append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 64)...))
	if err != nil {
		t.Fatalf("write png: %v", err)
	}
	err = mw.Close()
	if err != nil {
		t.Fatalf("close multipart: %v", err)
	}
	return &body, mw.FormDataContentType()
}

func TestUploadBlockedOverQuota(t *testing.T) {
	mux, s, u, cookie := newQuotaTestEnv(t)
	seedUsage(t, s, u.ID, 1<<20) // exactly at the 1 MB quota

	body, ctype := pngUpload(t)
	req := httptest.NewRequest(http.MethodPost, "/files/upload", body)
	req.Header.Set("Content-Type", ctype)
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if !strings.Contains(rr.Body.String(), "exceeds your storage quota") {
		t.Fatalf("upload over quota not blocked: %d %.300s", rr.Code, rr.Body.String())
	}

	files, err := ListFilesByUserID(u.ID, 0, 10)
	if err != nil {
		t.Fatalf("ListFilesByUserID: %v", err)
	}
	if len(files) != 1 { // only the seeded row
		t.Fatalf("files = %d, want 1 (upload must not persist)", len(files))
	}
}

func TestUploadAllowedUnderQuota(t *testing.T) {
	mux, s, u, cookie := newQuotaTestEnv(t)
	seedUsage(t, s, u.ID, 1024) // far below the quota

	body, ctype := pngUpload(t)
	req := httptest.NewRequest(http.MethodPost, "/files/upload", body)
	req.Header.Set("Content-Type", ctype)
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if strings.Contains(rr.Body.String(), "exceeds your storage quota") {
		t.Fatalf("upload under quota was blocked: %.300s", rr.Body.String())
	}
}

func TestQuotaFragmentShowsUsage(t *testing.T) {
	mux, s, u, cookie := newQuotaTestEnv(t)
	seedUsage(t, s, u.ID, 512<<10) // 0.5 MB

	req := httptest.NewRequest(http.MethodGet, "/files/quota", nil)
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("GET /files/quota = %d", rr.Code)
	}
	bodyStr := rr.Body.String()
	if !strings.Contains(bodyStr, `id="quota"`) {
		t.Fatalf("fragment missing #quota: %q", bodyStr)
	}
	if !strings.Contains(bodyStr, "512.0 KB") || !strings.Contains(bodyStr, "1.0 MB") {
		t.Fatalf("fragment missing usage numbers: %q", bodyStr)
	}
}
