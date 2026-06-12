package filemanager

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// doFM performs a request against the filemanager mux with the user cookie.
func doFM(t *testing.T, mux *http.ServeMux, method, path string, form url.Values, c *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if form == nil {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if c != nil {
		req.AddCookie(c)
		if form != nil {
			// Mutations validate the CSRF double-submit pair.
			req.AddCookie(&http.Cookie{Name: "csrf", Value: "test-csrf"})
		}
	}
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	return rr
}

// TestFileLifecycle drives upload → list → serve (ETag/304) → edit →
// soft delete through the real handlers.
func TestFileLifecycle(t *testing.T) {
	mux, _, u, cookie := newQuotaTestEnv(t)

	// Upload.
	body, ctype := pngUpload(t)
	req := httptest.NewRequest(http.MethodPost, "/files/upload", body)
	req.Header.Set("Content-Type", ctype)
	req.AddCookie(cookie)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code >= 400 {
		t.Fatalf("upload = %d: %.300s", rr.Code, rr.Body.String())
	}

	files, err := ListFilesByUserID(u.ID, 0, 10)
	if err != nil || len(files) != 1 {
		t.Fatalf("ListFilesByUserID = %d, %v", len(files), err)
	}
	stored := files[0]
	if stored.OriginalFilename != "tiny.png" || stored.Filehash == "" {
		t.Fatalf("stored metadata wrong: %+v", stored)
	}

	// Listing page shows the file.
	rr = doFM(t, mux, http.MethodGet, "/files/list", nil, cookie)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "tiny.png") {
		t.Fatalf("list = %d, file missing: %.300s", rr.Code, rr.Body.String())
	}

	// Serving: 200 with ETag, then 304 on If-None-Match.
	servePath := "/file/" + u.ReferenceID + "/" + stored.Filename
	rr = doFM(t, mux, http.MethodGet, servePath, nil, cookie)
	if rr.Code != http.StatusOK {
		t.Fatalf("serve = %d: %.300s", rr.Code, rr.Body.String())
	}
	etag := rr.Header().Get("ETag")
	if !strings.Contains(etag, stored.Filehash) {
		t.Fatalf("ETag = %q, want hash %q", etag, stored.Filehash)
	}
	req = httptest.NewRequest(http.MethodGet, servePath, nil)
	req.Header.Set("If-None-Match", etag)
	req.AddCookie(cookie)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotModified {
		t.Fatalf("If-None-Match = %d, want 304", rr.Code)
	}

	// Edit metadata.
	rr = doFM(t, mux, http.MethodPost, "/files/edit", url.Values{
		"file_id":     {stored.Filename},
		"description": {"relatorio anual"},
		"filetag":     {"document"},
		"csrf_token":  {"test-csrf"},
	}, cookie)
	if rr.Code >= 400 {
		t.Fatalf("edit = %d: %.300s", rr.Code, rr.Body.String())
	}
	updated, err := GetFileByUserIDAndFilename(u.ID, stored.Filename)
	if err != nil || updated == nil {
		t.Fatalf("GetFileByUserIDAndFilename: %v", err)
	}
	if updated.Filedescription != "relatorio anual" || updated.Filetag != "document" {
		t.Fatalf("metadata not updated: %+v", updated)
	}

	// Full-text search reaches the fresh description.
	found, err := SearchFilesByUserIDFTS(u.ID, "relatorio", "", 0, 10)
	if err != nil || len(found) != 1 || found[0].Filename != stored.Filename {
		t.Fatalf("FTS after edit = %d, %v", len(found), err)
	}
	found, err = SearchFilesByUserIDFTS(u.ID, "inexistente", "", 0, 10)
	if err != nil || len(found) != 0 {
		t.Fatalf("FTS bogus term = %d, %v", len(found), err)
	}

	// Soft delete: file leaves the listing and stops being served.
	rr = doFM(t, mux, http.MethodPost, "/files/delete", url.Values{
		"file_id":    {stored.Filename},
		"confirm":    {"yes"},
		"csrf_token": {"test-csrf"},
	}, cookie)
	if rr.Code >= 400 {
		t.Fatalf("delete = %d: %.300s", rr.Code, rr.Body.String())
	}
	files, _ = ListFilesByUserID(u.ID, 0, 10)
	if len(files) != 0 {
		t.Fatalf("deleted file still listed: %d", len(files))
	}
	rr = doFM(t, mux, http.MethodGet, servePath, nil, cookie)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("serve after delete = %d, want 404", rr.Code)
	}

	// The FTS index forgets soft-deleted files.
	found, _ = SearchFilesByUserIDFTS(u.ID, "relatorio", "", 0, 10)
	if len(found) != 0 {
		t.Fatalf("FTS still finds deleted file: %d", len(found))
	}
}

func TestServeFileRejectsBadPaths(t *testing.T) {
	mux, _, u, cookie := newQuotaTestEnv(t)

	// Path traversal in the filename segment.
	rr := doFM(t, mux, http.MethodGet, "/file/"+u.ReferenceID+"/..%2Fsecret", nil, cookie)
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusNotFound {
		t.Fatalf("traversal = %d, want 400/404", rr.Code)
	}

	// Missing filename segment.
	rr = doFM(t, mux, http.MethodGet, "/file/"+u.ReferenceID, nil, cookie)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("missing filename = %d, want 400", rr.Code)
	}

	// Unknown file.
	rr = doFM(t, mux, http.MethodGet, "/file/"+u.ReferenceID+"/nope.png", nil, cookie)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("unknown file = %d, want 404", rr.Code)
	}

	// Unauthenticated: redirect to login, never the bytes.
	rr = doFM(t, mux, http.MethodGet, "/file/"+u.ReferenceID+"/whatever.png", nil, nil)
	if rr.Code != http.StatusFound {
		t.Fatalf("unauthenticated = %d, want 302", rr.Code)
	}
}

func TestIndexAndQuotaPages(t *testing.T) {
	mux, s, u, cookie := newQuotaTestEnv(t)
	seedUsage(t, s, u.ID, 512<<10) // half of the 1 MB quota

	rr := doFM(t, mux, http.MethodGet, "/files/", nil, cookie)
	if rr.Code != http.StatusOK {
		t.Fatalf("index = %d", rr.Code)
	}

	rr = doFM(t, mux, http.MethodGet, "/files/quota", nil, cookie)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "KB") {
		t.Fatalf("quota fragment = %d: %.300s", rr.Code, rr.Body.String())
	}
}
