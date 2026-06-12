package session

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/crgimenes/devengine/db"
)

// resetStore empties the global session map between tests.
func resetStore(t *testing.T) {
	t.Helper()
	sessions.Lock()
	sessions.m = make(map[string]session)
	sessions.Unlock()
}

func TestPutGetDelCount(t *testing.T) {
	resetStore(t)

	u := db.User{ID: 7, Username: "ana"}
	Put("sid-1", u)

	got, ok := Get("sid-1")
	if !ok || got.Username != "ana" {
		t.Fatalf("Get = %+v, %v", got, ok)
	}
	if Count() != 1 {
		t.Fatalf("Count = %d, want 1", Count())
	}

	_, ok = Get("sid-unknown")
	if ok {
		t.Fatal("unknown sid must not resolve")
	}

	Del("sid-1")
	if _, ok := Get("sid-1"); ok {
		t.Fatal("deleted session still resolves")
	}
	if Count() != 0 {
		t.Fatalf("Count after delete = %d", Count())
	}
}

// An expired session must stop authenticating immediately, not only after
// the periodic Cleanup sweep runs.
func TestGetRejectsExpired(t *testing.T) {
	resetStore(t)

	old := MaxSessionAge
	MaxSessionAge = -1 // freshly put sessions are already expired
	t.Cleanup(func() { MaxSessionAge = old })

	Put("sid-old", db.User{ID: 1, Username: "ghost"})
	if _, ok := Get("sid-old"); ok {
		t.Fatal("expired session still authenticates")
	}

	// Cleanup also removes it from the store.
	Cleanup()
	if Count() != 0 {
		t.Fatalf("Cleanup left %d sessions", Count())
	}
}

func TestCleanupKeepsLiveSessions(t *testing.T) {
	resetStore(t)

	Put("sid-live", db.User{ID: 2, Username: "viva"})
	Cleanup()
	if _, ok := Get("sid-live"); !ok {
		t.Fatal("Cleanup removed a live session")
	}
}

func TestSyncSessionsPropagatesUser(t *testing.T) {
	resetStore(t)

	Put("sid-a", db.User{ID: 9, Username: "before"})
	Put("sid-b", db.User{ID: 9, Username: "before"})
	Put("sid-c", db.User{ID: 10, Username: "other"})

	Put("sid-a", db.User{ID: 9, Username: "after"})
	SyncSessions("sid-a")

	if u, _ := Get("sid-b"); u.Username != "after" {
		t.Fatalf("same-user session not synced: %+v", u)
	}
	if u, _ := Get("sid-c"); u.Username != "other" {
		t.Fatalf("other user's session touched: %+v", u)
	}
}

func TestGobRoundTrip(t *testing.T) {
	resetStore(t)

	Put("sid-1", db.User{ID: 1, Username: "ana"})
	Put("sid-2", db.User{ID: 2, Username: "bruno"})

	path := filepath.Join(t.TempDir(), "sessions.gob")
	err := SaveToGobFile(path)
	if err != nil {
		t.Fatalf("SaveToGobFile: %v", err)
	}

	resetStore(t)
	err = LoadFromGobFile(path)
	if err != nil {
		t.Fatalf("LoadFromGobFile: %v", err)
	}
	if Count() != 2 {
		t.Fatalf("restored %d sessions, want 2", Count())
	}
	if u, ok := Get("sid-1"); !ok || u.Username != "ana" {
		t.Fatalf("restored session wrong: %+v, %v", u, ok)
	}
}

func TestLoadFromGobFileMissingStartsFresh(t *testing.T) {
	resetStore(t)
	err := LoadFromGobFile(filepath.Join(t.TempDir(), "absent.gob"))
	if err != nil {
		t.Fatalf("missing file must not error: %v", err)
	}
}

func TestLoadFromGobFileCorrupted(t *testing.T) {
	resetStore(t)
	path := filepath.Join(t.TempDir(), "corrupt.gob")
	err := os.WriteFile(path, []byte("not a gob stream"), 0o600)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	err = LoadFromGobFile(path)
	if err == nil {
		t.Fatal("corrupted file must error")
	}
}

func TestSSERoundTrip(t *testing.T) {
	resetStore(t)
	Put("sid-sse", db.User{ID: 3})

	ch := make(chan string, 1)
	RegisterSSEChannel("sid-sse", ch)

	SendSSENotification("sid-sse", "ping")
	select {
	case msg := <-ch:
		if msg != "ping" {
			t.Fatalf("msg = %q", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("notification not delivered")
	}

	if n := BroadcastSSENotification("all"); n != 1 {
		t.Fatalf("broadcast reached %d channels, want 1", n)
	}
	<-ch

	UnregisterSSEChannel("sid-sse", ch)
	SendSSENotification("sid-sse", "lost")
	select {
	case msg := <-ch:
		t.Fatalf("unregistered channel received %q", msg)
	default:
	}
}

func TestCookieRoundTripInsecure(t *testing.T) {
	EnableInsecureCookie()

	rr := httptest.NewRecorder()
	SetCookie(rr, "sid-value", time.Hour)

	res := rr.Result()
	cookies := res.Cookies()
	if len(cookies) != 1 || cookies[0].Name != "sid" {
		t.Fatalf("cookies = %+v", cookies)
	}
	if !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteLaxMode {
		t.Fatalf("cookie flags wrong: %+v", cookies[0])
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(cookies[0])
	got, ok := GetCookie(req)
	if !ok || got != "sid-value" {
		t.Fatalf("GetCookie = %q, %v", got, ok)
	}

	// Clearing: negative max age.
	rr = httptest.NewRecorder()
	SetCookie(rr, "", -1)
	cleared := rr.Result().Cookies()
	if len(cleared) != 1 || cleared[0].MaxAge >= 0 && cleared[0].Value != "" {
		t.Fatalf("clear cookie = %+v", cleared)
	}
}

func TestCSRFDoubleSubmit(t *testing.T) {
	EnableInsecureCookie()

	// Mint the token (sets the cookie).
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	token := GenerateCSRFToken(rr, req)
	if token == "" {
		t.Fatal("empty CSRF token")
	}

	// Same cookie + matching form field: valid.
	form := strings.NewReader("csrf_token=" + token)
	post := httptest.NewRequest(http.MethodPost, "/", form)
	post.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	post.AddCookie(&http.Cookie{Name: "csrf", Value: token})
	if !ValidateCSRF(post) {
		t.Fatal("valid double-submit rejected")
	}

	// Mismatched form token: invalid.
	form = strings.NewReader("csrf_token=evil")
	post = httptest.NewRequest(http.MethodPost, "/", form)
	post.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	post.AddCookie(&http.Cookie{Name: "csrf", Value: token})
	if ValidateCSRF(post) {
		t.Fatal("mismatched token accepted")
	}

	// Missing cookie: invalid.
	form = strings.NewReader("csrf_token=" + token)
	post = httptest.NewRequest(http.MethodPost, "/", form)
	post.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if ValidateCSRF(post) {
		t.Fatal("missing cookie accepted")
	}

	// Existing cookie is reused, not regenerated.
	again := httptest.NewRequest(http.MethodGet, "/", nil)
	again.AddCookie(&http.Cookie{Name: "csrf", Value: token})
	if got := GenerateCSRFToken(httptest.NewRecorder(), again); got != token {
		t.Fatalf("token regenerated: %q != %q", got, token)
	}
}
