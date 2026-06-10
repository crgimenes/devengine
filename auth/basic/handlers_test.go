package basic_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/crgimenes/devengine/auth/basic"
	"github.com/crgimenes/devengine/config"
	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/devengine/session"
	"github.com/crgimenes/devengine/utils"
)

// Each test runs serially: db.Storage and session state are process-global.
// We do not call t.Parallel().

func setupHandler(t *testing.T) (*basic.Handlers, func()) {
	t.Helper()

	s, err := db.NewWithPath(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("NewWithPath: %v", err)
	}
	err = db.RunMigrationOn(s)
	if err != nil {
		t.Fatalf("RunMigrationOn: %v", err)
	}

	prevStorage := db.Storage
	db.Storage = s

	cfg := &config.Config{
		BaseURL:         "http://localhost:3210",
		LoginURL:        "/login",
		SessionDuration: time.Hour,
	}
	prevCfg := config.Cfg
	config.Cfg = cfg

	session.EnableInsecureCookie()

	h := basic.New(cfg, stubTemplates)

	cleanup := func() {
		s.Close()
		db.Storage = prevStorage
		config.Cfg = prevCfg
	}
	return h, cleanup
}

func stubTemplates(w io.Writer, name string, data any) error {
	b, _ := json.Marshal(data)
	_, err := fmt.Fprintf(w, "TEMPLATE=%s DATA=%s", name, string(b))
	return err
}

func createSysop(t *testing.T, username, password string) *db.User {
	t.Helper()
	hash, err := basic.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	u, err := db.Storage.CreateUser(username, username+"@example.com", hash, true)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	return u
}

func plantSession(t *testing.T, u *db.User) string {
	t.Helper()
	sid := utils.NewOpaqueID()
	session.Put(sid, *u)
	return sid
}

func postForm(handler http.HandlerFunc, body url.Values, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(body.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rr := httptest.NewRecorder()
	handler(rr, req)
	return rr
}

func sessionCookie(sid string) *http.Cookie {
	return &http.Cookie{Name: "sid", Value: sid}
}

// ---------- LoginSubmit ----------

func TestLoginSubmit_EmptyForm(t *testing.T) {
	h, cleanup := setupHandler(t)
	defer cleanup()

	rr := postForm(h.LoginSubmit, url.Values{})
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (render error page)", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Informe usuário e senha") {
		t.Fatalf("body did not include expected error: %s", rr.Body.String())
	}
}

func TestLoginSubmit_UnknownUser(t *testing.T) {
	h, cleanup := setupHandler(t)
	defer cleanup()

	body := url.Values{"username": {"ghost"}, "password": {"x"}}
	rr := postForm(h.LoginSubmit, body)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Credenciais inválidas") {
		t.Fatalf("body: %s", rr.Body.String())
	}
}

func TestLoginSubmit_WrongPassword(t *testing.T) {
	h, cleanup := setupHandler(t)
	defer cleanup()
	createSysop(t, "alice", "correctpw")

	body := url.Values{"username": {"alice"}, "password": {"wrongpw"}}
	rr := postForm(h.LoginSubmit, body)
	if !strings.Contains(rr.Body.String(), "Credenciais inválidas") {
		t.Fatalf("body: %s", rr.Body.String())
	}
}

func TestLoginSubmit_Success(t *testing.T) {
	h, cleanup := setupHandler(t)
	defer cleanup()
	createSysop(t, "alice", "secret")

	body := url.Values{"username": {"alice"}, "password": {"secret"}}
	rr := postForm(h.LoginSubmit, body)
	if rr.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", rr.Code)
	}
	loc := rr.Header().Get("Location")
	if loc != "http://localhost:3210/" {
		t.Fatalf("Location = %q", loc)
	}
	cookies := rr.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("no Set-Cookie on successful login")
	}
	var sid string
	for _, c := range cookies {
		if c.Name == "sid" {
			sid = c.Value
		}
	}
	if sid == "" {
		t.Fatal("sid cookie not set")
	}
	_, ok := session.Get(sid)
	if !ok {
		t.Fatal("session not stored")
	}
}

// ---------- SignupSubmit ----------

func TestSignupSubmit_BadTokenFormat(t *testing.T) {
	h, cleanup := setupHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/signup/notvalid", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("token", "notvalid")
	rr := httptest.NewRecorder()
	h.SignupSubmit(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestSignupSubmit_TokenNotInDB(t *testing.T) {
	h, cleanup := setupHandler(t)
	defer cleanup()

	token := utils.NewOpaqueID()
	body := url.Values{"username": {"bob"}, "password": {"longenoughpw"}, "password_confirm": {"longenoughpw"}}
	req := httptest.NewRequest(http.MethodPost, "/signup/"+token, strings.NewReader(body.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("token", token)
	rr := httptest.NewRecorder()
	h.SignupSubmit(rr, req)

	if !strings.Contains(rr.Body.String(), "inválido ou expirado") {
		t.Fatalf("body: %s", rr.Body.String())
	}
}

func TestSignupSubmit_PasswordMismatch(t *testing.T) {
	h, cleanup := setupHandler(t)
	defer cleanup()

	token, err := basic.CreateInvite("invitee@example.com")
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}

	body := url.Values{"username": {"bob"}, "password": {"longpassword"}, "password_confirm": {"different"}}
	req := httptest.NewRequest(http.MethodPost, "/signup/"+token, strings.NewReader(body.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("token", token)
	rr := httptest.NewRecorder()
	h.SignupSubmit(rr, req)

	if !strings.Contains(rr.Body.String(), "não conferem") {
		t.Fatalf("body: %s", rr.Body.String())
	}

	// And the token must still be consumable since signup failed before consuming
	email, err := db.Storage.ConsumeToken(token, "invite")
	if err != nil || email == "" {
		t.Fatalf("token was consumed despite failed signup: email=%q err=%v", email, err)
	}
}

func TestSignupSubmit_InvalidUsername(t *testing.T) {
	h, cleanup := setupHandler(t)
	defer cleanup()

	token, _ := basic.CreateInvite("invitee@example.com")

	body := url.Values{"username": {"ab"}, "password": {"longpassword"}, "password_confirm": {"longpassword"}}
	req := httptest.NewRequest(http.MethodPost, "/signup/"+token, strings.NewReader(body.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("token", token)
	rr := httptest.NewRecorder()
	h.SignupSubmit(rr, req)

	if !strings.Contains(rr.Body.String(), "between 3 and 30") {
		t.Fatalf("body: %s", rr.Body.String())
	}
}

func TestSignupSubmit_Success(t *testing.T) {
	h, cleanup := setupHandler(t)
	defer cleanup()

	token, _ := basic.CreateInvite("newbie@example.com")

	body := url.Values{"username": {"newbie"}, "password": {"longpassword"}, "password_confirm": {"longpassword"}}
	req := httptest.NewRequest(http.MethodPost, "/signup/"+token, strings.NewReader(body.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("token", token)
	rr := httptest.NewRecorder()
	h.SignupSubmit(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302; body: %s", rr.Code, rr.Body.String())
	}

	u, err := db.Storage.GetUserByUsername("newbie")
	if err != nil {
		t.Fatalf("GetUserByUsername: %v", err)
	}
	if u == nil {
		t.Fatal("user not created")
	}
	if u.Email != "newbie@example.com" {
		t.Fatalf("email = %q", u.Email)
	}
	if u.Sysop {
		t.Fatal("Sysop = true, want false (regular invite signup)")
	}
	if !u.Enabled {
		t.Fatal("Enabled = false")
	}

	// Token must be gone (single-use)
	email, _ := db.Storage.ConsumeToken(token, "invite")
	if email != "" {
		t.Fatalf("token survived signup: %q", email)
	}
}

// ---------- InviteCreate ----------

func TestInviteCreate_UnauthenticatedRedirects(t *testing.T) {
	h, cleanup := setupHandler(t)
	defer cleanup()

	rr := postForm(h.InviteCreate, url.Values{"email": {"a@b.com"}})
	// Prelude redirects unauthenticated requests to /login.
	if rr.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302 (Prelude redirect)", rr.Code)
	}
	loc := rr.Header().Get("Location")
	if !strings.HasSuffix(loc, "/login") {
		t.Fatalf("Location = %q, want suffix /login", loc)
	}
}

func TestInviteCreate_NonSysopForbidden(t *testing.T) {
	h, cleanup := setupHandler(t)
	defer cleanup()

	// Plant a non-sysop session.
	hash, _ := basic.HashPassword("x")
	plain, _ := db.Storage.CreateUser("plainuser", "p@e.com", hash, false)
	sid := plantSession(t, plain)

	rr := postForm(h.InviteCreate, url.Values{"email": {"a@b.com"}}, sessionCookie(sid))
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rr.Code)
	}
}

func TestInviteCreate_MissingEmail(t *testing.T) {
	h, cleanup := setupHandler(t)
	defer cleanup()

	admin := createSysop(t, "admin", "x")
	sid := plantSession(t, admin)

	rr := postForm(h.InviteCreate, url.Values{}, sessionCookie(sid))
	if rr.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", rr.Code)
	}
	loc := rr.Header().Get("Location")
	if !strings.Contains(loc, "error=") {
		t.Fatalf("Location = %q, expected error qs", loc)
	}
}

func TestInviteCreate_Success(t *testing.T) {
	h, cleanup := setupHandler(t)
	defer cleanup()

	admin := createSysop(t, "admin", "x")
	sid := plantSession(t, admin)

	rr := postForm(h.InviteCreate, url.Values{"email": {"new@example.com"}}, sessionCookie(sid))
	if rr.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", rr.Code)
	}
	loc := rr.Header().Get("Location")
	parsed, err := url.Parse(loc)
	if err != nil {
		t.Fatalf("parse Location: %v", err)
	}
	created := parsed.Query().Get("created")
	if !strings.HasPrefix(created, "http://localhost:3210/signup/") {
		t.Fatalf("created = %q, want /signup/ URL", created)
	}
}
