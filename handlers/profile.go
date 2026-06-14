package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/crgimenes/devengine/auth"
	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/devengine/i18n"
	"github.com/crgimenes/devengine/session"
)

func (h *Handlers) Profile(w http.ResponseWriter, r *http.Request) {
	u, _, authed, err := auth.Prelude(w, r,
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
		h.renderMe(w, r, u, meData{
			Message: r.URL.Query().Get("message"),
			Error:   r.URL.Query().Get("error"),
		})
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
			h.serverError(w, r, "Profile", errors.New("file utilities not configured"))
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
			h.serverError(w, r, "Profile", errors.New("avatar upload is not seekable"))
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
			h.serverError(w, r, "Profile", gerr)
			return
		}
		if freshUser == nil || freshUser.ReferenceID == "" {
			h.serverError(w, r, "Profile", errors.New("refreshed user has no reference ID"))
			return
		}

		u = freshUser

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
			_ = os.Remove(avatarPath) // #nosec G703 -- avatarPath uses a server-generated filename inside the configured upload directory
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
		h.renderMe(w, r, u, meData{
			Error: tr(r, "Could not update the profile (ref %s)", logRef("UpdateUserProfile", err)),
		})
		return
	}

	err = db.Storage.UpdateUserLocale(u.ID, locale)
	if err != nil {
		h.serverError(w, r, "UpdateUserLocale", err)
		return
	}
	updatedUser.Locale = locale

	session.UpdateUser(*updatedUser)

	http.Redirect(w, r, h.cfg.BaseURL+"/", http.StatusFound)
}
