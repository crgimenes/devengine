package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/crgimenes/devengine/auth"
	"github.com/crgimenes/devengine/config"
	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/devengine/i18n"
	"github.com/crgimenes/devengine/session"
)

func (h *Handlers) Profile(w http.ResponseWriter, r *http.Request) {
	u, sid, authed, err := auth.Prelude(w, r,
		[]string{
			http.MethodGet,
			http.MethodPost,
		},
		true, // check auth
		true, // prevent cache
	)
	if err != nil {
		h.serverError(w, r, "Profile", err)
		return
	}

	if !authed {
		return
	}

	if r.Method == http.MethodGet {
		data := struct {
			Authed  bool
			User    db.User
			Error   string
			Message string
			Config  config.Config
			Locale  string
			Locales []string
		}{
			Authed:  true,
			User:    *u,
			Config:  *h.cfg,
			Locale:  auth.RequestLocale(r),
			Locales: i18n.Locales(),
		}
		h.render(w, "me.go.tmpl", data)
		return
	}

	// Bound the request body so a hostile client cannot stream an unbounded
	// upload; 10 MB avatar plus multipart overhead slack.
	r.Body = http.MaxBytesReader(w, r.Body, 12<<20)
	err = r.ParseMultipartForm(10 << 20) // #nosec G120 -- bounded by MaxBytesReader above
	if err != nil {
		h.errorPage(w, r, http.StatusBadRequest, "bad request")
		return
	}

	username := r.FormValue("username")
	email := r.FormValue("email")
	avatarURL := u.AvatarURL

	file, fh, err := r.FormFile("avatar_file")
	if err != nil && err != http.ErrMissingFile {
		h.errorPage(w, r, http.StatusBadRequest, "bad request")
		return
	}
	defer func() {
		if file != nil {
			_ = file.Close()
		}
	}()

	if file != nil {
		if h.files.Validate == nil || h.files.DataPath == nil || h.files.SaveMetadata == nil || h.files.NewFilename == nil {
			h.serverError(w, r, "file utilities not configured", err)
			return
		}

		typeDetected, size, err := h.files.Validate(
			file,
			fh,
			[]string{"image/jpeg", "image/png", "image/gif", "image/webp"},
			[]string{"jpg", "jpeg", "png", "gif", "webp"},
			5<<20,
		)
		if err != nil {
			h.errorPage(w, r, http.StatusBadRequest, "invalid avatar file: "+err.Error())
			return
		}

		seeker, ok := file.(io.Seeker)
		if !ok {
			h.serverError(w, r, "Profile", err)
			return
		}
		_, err = seeker.Seek(0, io.SeekStart)
		if err != nil {
			h.serverError(w, r, "Profile", err)
			return
		}

		avatarData, err := io.ReadAll(file)
		if err != nil {
			h.serverError(w, r, "Profile", err)
			return
		}

		sum := sha256.Sum256(avatarData)
		fileHash := hex.EncodeToString(sum[:])

		freshUser, gerr := db.Storage.GetUserByID(u.ID)
		if gerr != nil {
			h.serverError(w, r, "Profile", err)
			return
		}
		if freshUser.ReferenceID == "" {
			h.serverError(w, r, "Profile", err)
			return
		}

		u = freshUser
		session.Put(sid, *u)
		session.SyncSessions(sid)

		uploadsDir, err := h.files.DataPath(u)
		if err != nil {
			h.serverError(w, r, "Profile", err)
			return
		}

		fileExt := strings.ToLower(filepath.Ext(fh.Filename))
		avatarPath := filepath.Join(uploadsDir, h.files.NewFilename()+fileExt)

		err = os.WriteFile(avatarPath, avatarData, 0600) // #nosec G703 G304 -- filename is a server-generated opaque ID; filepath.Ext cannot contain separators
		if err != nil {
			h.serverError(w, r, "Profile", err)
			return
		}

		fileMeta := &db.File{
			UserID:           u.ID,
			OriginalFilename: fh.Filename,
			Filename:         filepath.Base(avatarPath),
			Filesize:         size,
			Filetype:         typeDetected,
			Filehash:         fileHash,
			Filetag:          "avatar",
			Filedescription:  "User avatar image",
			Processed:        false,
			CreatedAt:        time.Now().UTC().Format(time.RFC3339),
			UpdatedAt:        time.Now().UTC().Format(time.RFC3339),
		}

		fileMeta, err = h.files.SaveMetadata(fileMeta)
		if err != nil {
			h.serverError(w, r, "Profile", err)
			return
		}

		avatarURL = "/file/" + u.ReferenceID + "/" + fileMeta.Filename
	}

	locale := r.FormValue("locale")
	if locale != "" && !i18n.Known(locale) {
		locale = ""
	}

	updatedUser, err := db.Storage.UpdateUserProfile(u.ID, username, avatarURL, email)
	if err != nil {
		data := struct {
			Authed  bool
			User    db.User
			Error   string
			Message string
			Config  config.Config
			Locale  string
			Locales []string
		}{
			Authed:  true,
			User:    *u,
			Error:   tr(r, "Could not update the profile (ref %s)", logRef("UpdateUserProfile", err)),
			Config:  *h.cfg,
			Locale:  auth.RequestLocale(r),
			Locales: i18n.Locales(),
		}
		h.render(w, "me.go.tmpl", data)
		return
	}

	err = db.Storage.UpdateUserLocale(u.ID, locale)
	if err != nil {
		h.serverError(w, r, "UpdateUserLocale", err)
		return
	}
	updatedUser.Locale = locale

	session.Put(sid, *updatedUser)
	session.SyncSessions(sid)

	http.Redirect(w, r, h.cfg.BaseURL+"/", http.StatusFound)
}
