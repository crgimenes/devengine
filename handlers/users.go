package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/crgimenes/devengine/auth"
	"github.com/crgimenes/devengine/auth/basic"
	"github.com/crgimenes/devengine/config"
	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/devengine/session"
)

const usersPageSize = 50

// ToolsUsers renders a paginated list of users. sysop-only.
func (h *Handlers) ToolsUsers(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodGet},
		true, true,
	)
	if err != nil {
		h.serverError(w, r, "ToolsUsers", err)
		return
	}
	if !authed {
		return
	}
	if !user.Sysop {
		h.forbidden(w, r)
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * usersPageSize

	users, total, err := db.Storage.ListUsers(usersPageSize, offset)
	if err != nil {
		h.serverError(w, r, "ToolsUsers", err)
		return
	}

	pages := max((total+usersPageSize-1)/usersPageSize, 1)

	data := struct {
		Authed       bool
		User         db.User
		Locale       string
		Config       config.Config
		CurrentPage  string
		Message      string
		Error        string
		Users        []db.User
		Total        int
		Page         int
		Pages        int
		PrevPage     int
		NextPage     int
		HasPrev      bool
		HasNext      bool
		CurrentRefID string
	}{
		Authed:       true,
		Locale:       auth.RequestLocale(r),
		User:         *user,
		Config:       *h.cfg,
		CurrentPage:  "users",
		Message:      r.URL.Query().Get("message"),
		Error:        r.URL.Query().Get("error"),
		Users:        users,
		Total:        total,
		Page:         page,
		Pages:        pages,
		PrevPage:     page - 1,
		NextPage:     page + 1,
		HasPrev:      page > 1,
		HasNext:      page < pages,
		CurrentRefID: user.ReferenceID,
	}

	h.render(w, "tools_users.go.tmpl", data)
}

// ToolsUsersEdit renders the edit form for a single user. sysop-only.
func (h *Handlers) ToolsUsersEdit(w http.ResponseWriter, r *http.Request) {
	current, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodGet},
		true, true,
	)
	if err != nil {
		h.serverError(w, r, "ToolsUsersEdit", err)
		return
	}
	if !authed {
		return
	}
	if !current.Sysop {
		h.forbidden(w, r)
		return
	}

	target, err := h.loadTargetUser(r, w)
	if err != nil || target == nil {
		return
	}

	data := struct {
		Authed       bool
		User         db.User
		Locale       string
		Config       config.Config
		CurrentPage  string
		Message      string
		Error        string
		Target       db.User
		IsSelf       bool
		NewPassword  string
		CurrentRefID string
	}{
		Authed:       true,
		Locale:       auth.RequestLocale(r),
		User:         *current,
		Config:       *h.cfg,
		CurrentPage:  "users",
		Message:      r.URL.Query().Get("message"),
		Error:        r.URL.Query().Get("error"),
		Target:       *target,
		IsSelf:       target.ID == current.ID,
		NewPassword:  r.URL.Query().Get("new_password"),
		CurrentRefID: current.ReferenceID,
	}

	h.render(w, "tools_users_edit.go.tmpl", data)
}

// ToolsUsersUpdate persists username, email, sysop and enabled flags.
func (h *Handlers) ToolsUsersUpdate(w http.ResponseWriter, r *http.Request) {
	current, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		true, true,
	)
	if err != nil {
		h.serverError(w, r, "ToolsUsersUpdate", err)
		return
	}
	if !authed {
		return
	}
	if !current.Sysop {
		h.forbidden(w, r)
		return
	}

	target, err := h.loadTargetUser(r, w)
	if err != nil || target == nil {
		return
	}

	username := strings.TrimSpace(r.FormValue("username"))
	email := strings.TrimSpace(r.FormValue("email"))
	sysop := r.FormValue("sysop") == "1"
	enabled := r.FormValue("enabled") == "1"

	_, err = db.Storage.UpdateUserProfile(target.ID, username, target.AvatarURL, email)
	if err != nil {
		ref := logRef("UpdateUserProfile", err)
		usersEditRedirect(w, r, target.ReferenceID, "error", tr(r, "Could not update the profile (ref %s)", ref))
		return
	}

	err = db.Storage.UpdateUserSysop(target.ID, sysop, current.ID)
	if err != nil {
		ref := logRef("UpdateUserSysop", err)
		usersEditRedirect(w, r, target.ReferenceID, "error", tr(r, "Could not update the sysop flag (ref %s)", ref))
		return
	}

	err = db.Storage.UpdateUserEnabled(target.ID, enabled, current.ID)
	if err != nil {
		ref := logRef("UpdateUserEnabled", err)
		usersEditRedirect(w, r, target.ReferenceID, "error", tr(r, "Could not update the enabled flag (ref %s)", ref))
		return
	}

	updated, err := db.Storage.GetUserByID(target.ID)
	if err != nil || updated == nil {
		if err == nil {
			err = errors.New("updated user not found")
		}
		h.serverError(w, r, "GetUserByID", err)
		return
	}
	if !updated.Enabled {
		session.DeleteUserSessions(updated.ID)
	} else {
		session.UpdateUser(*updated)
	}

	usersEditRedirect(w, r, target.ReferenceID, "message", tr(r, "User updated."))
}

// ToolsUsersResetPassword generates a fresh random password for the target
// user, persists its hash and surfaces the plaintext to the admin once.
func (h *Handlers) ToolsUsersResetPassword(w http.ResponseWriter, r *http.Request) {
	current, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		true, true,
	)
	if err != nil {
		h.serverError(w, r, "ToolsUsersResetPassword", err)
		return
	}
	if !authed {
		return
	}
	if !current.Sysop {
		h.forbidden(w, r)
		return
	}

	target, err := h.loadTargetUser(r, w)
	if err != nil || target == nil {
		return
	}

	newPassword, err := randomPassword()
	if err != nil {
		h.serverError(w, r, "ToolsUsersResetPassword", err)
		return
	}

	hash, err := basic.HashPassword(newPassword)
	if err != nil {
		h.serverError(w, r, "ToolsUsersResetPassword", err)
		return
	}

	err = db.Storage.SetPasswordHash(target.ID, hash)
	if err != nil {
		h.serverError(w, r, "ToolsUsersResetPassword", err)
		return
	}
	session.DeleteUserSessions(target.ID)

	http.Redirect(w, r,
		h.cfg.BaseURL+"/tools/users/"+target.ReferenceID+"/edit?new_password="+url.QueryEscape(newPassword),
		http.StatusSeeOther)
}

// loadTargetUser parses the reference_id from the URL and fetches the target
// user. Writes the appropriate HTTP error and returns nil when the user
// cannot be loaded; the caller must check (nil, _) and return.
func (h *Handlers) loadTargetUser(r *http.Request, w http.ResponseWriter) (*db.User, error) {
	ref := r.PathValue("ref")
	if ref == "" {
		h.errorPage(w, r, http.StatusBadRequest, "missing user ref")
		return nil, nil
	}
	u, err := db.Storage.GetUserByRefID(ref)
	if err != nil {
		h.serverError(w, r, "GetUserByRefID", err)
		return nil, err
	}
	if u == nil {
		h.notFound(w, r)
		return nil, nil
	}
	return u, nil
}

func usersEditRedirect(w http.ResponseWriter, r *http.Request, ref, key, msg string) {
	http.Redirect(w, r,
		"/tools/users/"+ref+"/edit?"+key+"="+url.QueryEscape(msg),
		http.StatusSeeOther)
}

// randomPassword returns a hex-encoded 12-byte secret (24 chars). Caller
// stores the hash and shows the plaintext to the admin once.
func randomPassword() (string, error) {
	b := make([]byte, 12)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
