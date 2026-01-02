package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
	"github.com/microcosm-cc/bluemonday"

	"github.com/crgimenes/devengine/auth"
	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/devengine/log"
)

// Default pagination values
const (
	defaultLimit = 20
	maxLimit     = 100
)

// parsePagination extracts offset and limit from query parameters
func parsePagination(r *http.Request) (offset, limit int) {
	offset = 0
	limit = defaultLimit

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if v, err := strconv.Atoi(offsetStr); err == nil && v >= 0 {
			offset = v
		}
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 {
			limit = v
		}
	}

	limit = min(limit, maxLimit)

	return offset, limit
}

// paginatedResponse wraps paginated data
type paginatedResponse struct {
	Data   any `json:"data"`
	Offset int `json:"offset"`
	Limit  int `json:"limit"`
	Total  int `json:"total"`
}

// MdToHTML converts user-provided Markdown to safe HTML.
func MdToHTML(md []byte) []byte {
	// Markdown parser with common extensions
	extensions := parser.CommonExtensions |
		parser.AutoHeadingIDs |
		parser.NoEmptyLineBeforeBlock

	p := parser.NewWithExtensions(extensions)
	doc := p.Parse(md)

	htmlFlags := html.CommonFlags | html.HrefTargetBlank
	opts := html.RendererOptions{Flags: htmlFlags}
	renderer := html.NewRenderer(opts)

	unsafeHTML := markdown.Render(doc, renderer)

	policy := bluemonday.UGCPolicy()

	policy = policy.AddTargetBlankToFullyQualifiedLinks(true)

	// Whitelist
	policy.AllowElements("audio", "video", "source")
	policy.AllowAttrs("controls").OnElements("audio", "video")
	policy.AllowAttrs("preload").OnElements("audio", "video")
	policy.AllowAttrs("poster").OnElements("video")
	policy.AllowAttrs("src").OnElements("audio", "video", "source")
	policy.AllowAttrs("type").OnElements("source")
	policy.AllowAttrs("loop", "muted").OnElements("audio", "video")

	safeHTML := policy.SanitizeBytes(unsafeHTML)

	return bytes.TrimSpace(safeHTML)
}

func MarkdownToHTMLHandler(w http.ResponseWriter, r *http.Request) {
	_, _, _, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		true,  // check auth
		false, // check ratelimit
		true,  // prevent cache
	)
	if err != nil {
		log.Printf("auth.Prelude error: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Limit body size to avoid abuse; adjust as needed
	const maxBody = 512 * 1024 // 512 KB
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)

	md, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	// Convert Markdown -> HTML using your existing function
	html := MdToHTML(md)

	// Respond
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(html)
}

// GetUserFilesHandler returns a JSON list of user's files with pagination
// Query params: offset (default 0), limit (default 20, max 100)
func GetUserFilesHandler(w http.ResponseWriter, r *http.Request) {
	u, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodGet},
		true,  // check auth
		false, // check ratelimit
		true,  // prevent cache
	)
	if err != nil {
		log.Printf("auth.Prelude error: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if !authed {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	offset, limit := parsePagination(r)

	// Use efficient database query with pagination
	files, err := db.Storage.ListFilesByUserID(u.ID, offset, limit)
	if err != nil {
		log.Printf("error listing files: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	total, err := db.Storage.CountFilesByUserID(u.ID)
	if err != nil {
		log.Printf("error counting files: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Build response
	type fileInfo struct {
		Filename    string `json:"filename"`
		DisplayName string `json:"display_name"`
		Description string `json:"description"`
		Filetype    string `json:"filetype"`
		Filesize    int64  `json:"filesize"`
		FileURL     string `json:"file_url"`
		CreatedAt   string `json:"created_at"`
	}

	result := make([]fileInfo, 0, len(files))
	for _, f := range files {
		fileURL := "/file/" + u.ReferenceID + "/" + f.Filename

		result = append(result, fileInfo{
			Filename:    f.Filename,
			DisplayName: f.OriginalFilename,
			Description: f.Filedescription,
			Filetype:    f.Filetype,
			Filesize:    f.Filesize,
			FileURL:     fileURL,
			CreatedAt:   f.CreatedAt,
		})
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store, no-cache, max-age=0, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	response := paginatedResponse{
		Data:   result,
		Offset: offset,
		Limit:  limit,
		Total:  total,
	}

	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		log.Printf("error encoding JSON response: %v", err)
	}
}

func Routes(mux *http.ServeMux) {
	mux.HandleFunc("/api/markdown", MarkdownToHTMLHandler)
	mux.HandleFunc("/api/files", GetUserFilesHandler)
}
