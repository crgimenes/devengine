package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeAPIBody(t *testing.T) {
	tests := []struct {
		name string
		body string
		ok   bool
	}{
		{name: "object", body: `{"name":"Ana"}`, ok: true},
		{name: "null", body: `null`, ok: false},
		{name: "trailing document", body: `{} {}`, ok: false},
		{name: "oversized", body: `{"value":"` + strings.Repeat("x", maxAPIJSONBody) + `"}`, ok: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/test", strings.NewReader(tc.body))
			rec := httptest.NewRecorder()

			_, ok := decodeAPIBody(rec, req)
			if ok != tc.ok {
				t.Fatalf("ok = %v, want %v; status=%d body=%s", ok, tc.ok, rec.Code, rec.Body.String())
			}
			if !tc.ok && rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", rec.Code)
			}
		})
	}
}
