package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/crgimenes/devengine/db"
)

// The session provider can be swapped by applications; Prelude must honor
// it. Internal test so the default provider can be restored afterwards.
func TestSetSessionProvider(t *testing.T) {
	t.Cleanup(func() { sessions = sessionStore{} })
	SetSessionProvider(stubProvider{})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	user, sid, authed, err := Prelude(rr, req, []string{http.MethodGet}, true, false)
	if err != nil || !authed || user == nil || user.Username != "provided" || sid != "stub-sid" {
		t.Fatalf("custom provider not honored: %+v, %q, %v, %v", user, sid, authed, err)
	}
}

type stubProvider struct{}

func (stubProvider) GetCookie(*http.Request) (string, bool)               { return "stub-sid", true }
func (stubProvider) SetCookie(http.ResponseWriter, string, time.Duration) {}
func (stubProvider) Get(string) (SessionUser, bool)                       { return stubUser{}, true }
func (stubProvider) Del(string)                                           {}

type stubUser struct{}

func (stubUser) ToDBUser() db.User {
	return db.User{ID: 99, Username: "provided", Enabled: true}
}
