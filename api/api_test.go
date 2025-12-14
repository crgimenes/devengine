package api

import (
	"net/http"
	"net/url"
	"testing"
)

func TestParsePagination(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		query          string
		expectedOffset int
		expectedLimit  int
	}{
		{
			name:           "default values",
			query:          "",
			expectedOffset: 0,
			expectedLimit:  defaultLimit,
		},
		{
			name:           "custom offset",
			query:          "offset=10",
			expectedOffset: 10,
			expectedLimit:  defaultLimit,
		},
		{
			name:           "custom limit",
			query:          "limit=50",
			expectedOffset: 0,
			expectedLimit:  50,
		},
		{
			name:           "both offset and limit",
			query:          "offset=20&limit=30",
			expectedOffset: 20,
			expectedLimit:  30,
		},
		{
			name:           "limit exceeds max",
			query:          "limit=500",
			expectedOffset: 0,
			expectedLimit:  maxLimit,
		},
		{
			name:           "negative offset ignored",
			query:          "offset=-5",
			expectedOffset: 0,
			expectedLimit:  defaultLimit,
		},
		{
			name:           "negative limit ignored",
			query:          "limit=-10",
			expectedOffset: 0,
			expectedLimit:  defaultLimit,
		},
		{
			name:           "invalid offset string",
			query:          "offset=abc",
			expectedOffset: 0,
			expectedLimit:  defaultLimit,
		},
		{
			name:           "invalid limit string",
			query:          "limit=xyz",
			expectedOffset: 0,
			expectedLimit:  defaultLimit,
		},
		{
			name:           "zero limit uses default",
			query:          "limit=0",
			expectedOffset: 0,
			expectedLimit:  defaultLimit,
		},
		{
			name:           "large offset",
			query:          "offset=99999",
			expectedOffset: 99999,
			expectedLimit:  defaultLimit,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, _ := url.Parse("/api/test?" + tt.query)
			r := &http.Request{URL: u}

			offset, limit := parsePagination(r)

			if offset != tt.expectedOffset {
				t.Errorf("offset: expected %d, got %d", tt.expectedOffset, offset)
			}
			if limit != tt.expectedLimit {
				t.Errorf("limit: expected %d, got %d", tt.expectedLimit, limit)
			}
		})
	}
}

func TestMdToHTML(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		contains string
	}{
		{
			name:     "simple paragraph",
			input:    "Hello world",
			contains: "Hello world",
		},
		{
			name:     "heading",
			input:    "# Title",
			contains: "<h1",
		},
		{
			name:     "bold text",
			input:    "**bold**",
			contains: "<strong>bold</strong>",
		},
		{
			name:     "italic text",
			input:    "*italic*",
			contains: "<em>italic</em>",
		},
		{
			name:     "link",
			input:    "[link](https://example.com)",
			contains: `href="https://example.com"`,
		},
		{
			name:     "code block",
			input:    "```\ncode\n```",
			contains: "<code>",
		},
		{
			name:     "inline code",
			input:    "`code`",
			contains: "<code>code</code>",
		},
		{
			name:     "list",
			input:    "- item1\n- item2",
			contains: "<li>",
		},
		{
			name:     "image",
			input:    "![alt](https://example.com/img.png)",
			contains: "<img",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MdToHTML([]byte(tt.input))
			if !contains(string(result), tt.contains) {
				t.Errorf("expected output to contain %q, got %q", tt.contains, result)
			}
		})
	}
}

func TestMdToHTMLSanitization(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		input      string
		shouldHave string
		shouldNot  string
	}{
		{
			name:       "script tag removed",
			input:      "<script>alert('xss')</script>",
			shouldHave: "",
			shouldNot:  "<script>",
		},
		{
			name:       "onclick removed",
			input:      `<a onclick="evil()" href="x">link</a>`,
			shouldHave: "link",
			shouldNot:  "onclick",
		},
		{
			name:       "audio allowed",
			input:      `<audio controls src="file.mp3"></audio>`,
			shouldHave: "<audio",
			shouldNot:  "",
		},
		{
			name:       "video allowed",
			input:      `<video controls src="file.mp4"></video>`,
			shouldHave: "<video",
			shouldNot:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := string(MdToHTML([]byte(tt.input)))

			if tt.shouldHave != "" && !contains(result, tt.shouldHave) {
				t.Errorf("expected output to contain %q, got %q", tt.shouldHave, result)
			}
			if tt.shouldNot != "" && contains(result, tt.shouldNot) {
				t.Errorf("expected output NOT to contain %q, got %q", tt.shouldNot, result)
			}
		})
	}
}

func TestPaginatedResponseStructure(t *testing.T) {
	t.Parallel()

	resp := paginatedResponse{
		Data:   []string{"a", "b", "c"},
		Offset: 0,
		Limit:  20,
		Total:  100,
	}

	if resp.Offset != 0 {
		t.Errorf("expected offset 0, got %d", resp.Offset)
	}
	if resp.Limit != 20 {
		t.Errorf("expected limit 20, got %d", resp.Limit)
	}
	if resp.Total != 100 {
		t.Errorf("expected total 100, got %d", resp.Total)
	}
}

// contains checks if substr is in s
func contains(s, substr string) bool {
	return len(substr) == 0 || (len(s) >= len(substr) && searchString(s, substr))
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
