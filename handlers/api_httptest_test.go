package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/devengine/utils"
)

// seedAPIForm creates an entity (titulo TEXT required + quantidade INT with
// a validate_expr), a form exposing both fields with expose_api on, and a
// pre_save script that blocks a magic word.
func seedAPIForm(t *testing.T, s db.Store) *db.Form {
	t.Helper()
	et, err := s.CreateEAVEntityType("Item", "item",
		"", `(if (= field:titulo "bloqueado") (set error "este titulo nao pode"))`, "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType: %v", err)
	}
	maxLen := 256
	titulo, err := s.CreateEAVAttribute(et.ID, "titulo", "Titulo", "", "TEXT",
		true, false, false, &maxLen, false, "", nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateEAVAttribute titulo: %v", err)
	}
	qtd, err := s.CreateEAVAttribute(et.ID, "quantidade", "Quantidade", "", "INT",
		false, false, false, nil, false, "", nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateEAVAttribute quantidade: %v", err)
	}

	form, err := s.CreateForm("itens", "Itens", "", &et.ID)
	if err != nil {
		t.Fatalf("CreateForm: %v", err)
	}
	_, err = s.CreateFormElement(form.ID, nil, "titulo", "field", "Titulo", "",
		1, 12, "text", "{}", &titulo.ID, false, false)
	if err != nil {
		t.Fatalf("CreateFormElement titulo: %v", err)
	}
	el2, err := s.CreateFormElement(form.ID, nil, "quantidade", "field", "Quantidade", "",
		2, 12, "int", "{}", &qtd.ID, false, false)
	if err != nil {
		t.Fatalf("CreateFormElement quantidade: %v", err)
	}
	err = s.UpdateFormElement(el2.ID, nil, "quantidade", "field", "Quantidade", "",
		2, 12, "left", "int", "{}", &qtd.ID,
		false, false, false, false, "(>= field:quantidade 0)", "", false, "", "", "")
	if err != nil {
		t.Fatalf("UpdateFormElement validate_expr: %v", err)
	}

	err = s.UpdateForm(form.ID, "itens", "Itens", "", &et.ID,
		false, false, false, false, nil, false, true) // expose_api
	if err != nil {
		t.Fatalf("UpdateForm expose_api: %v", err)
	}
	form, err = s.GetFormByMachineName("itens")
	if err != nil {
		t.Fatalf("reload form: %v", err)
	}
	return form
}

// mintToken creates a user + API token and returns the plaintext.
func mintToken(t *testing.T, s db.Store, username string) string {
	t.Helper()
	u, err := s.CreateUser(username, username+"@example.com", "x", false)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	token := utils.NewOpaqueID()
	_, err = s.CreateAPIToken(u.ID, hashAPIToken(token), "test")
	if err != nil {
		t.Fatalf("CreateAPIToken: %v", err)
	}
	return token
}

func apiReq(t *testing.T, mux *http.ServeMux, method, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	var rd *strings.Reader
	if body == "" {
		rd = strings.NewReader("")
	} else {
		rd = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rd)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	return rr
}

func decodeJSON(t *testing.T, rr *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	err := json.Unmarshal(rr.Body.Bytes(), &out)
	if err != nil {
		t.Fatalf("invalid JSON response (%d): %.300s", rr.Code, rr.Body.String())
	}
	return out
}

func TestAPIRecordsCRUD(t *testing.T) {
	mux, s := newHTTPTestEnv(t)
	form := seedAPIForm(t, s)
	_ = form
	token := mintToken(t, s, "apiuser")

	// Auth gates.
	rr := apiReq(t, mux, http.MethodGet, "/api/v1/itens", "", "")
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("no token = %d, want 401", rr.Code)
	}
	rr = apiReq(t, mux, http.MethodGet, "/api/v1/itens", "wrong-token", "")
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("bad token = %d, want 401", rr.Code)
	}

	// Create: validation failures first.
	rr = apiReq(t, mux, http.MethodPost, "/api/v1/itens", token, `{"quantidade": 5}`)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("missing required = %d: %.300s", rr.Code, rr.Body.String())
	}
	body := decodeJSON(t, rr)
	if fields, _ := body["fields"].(map[string]any); fields["titulo"] == nil {
		t.Fatalf("missing per-field error: %v", body)
	}

	rr = apiReq(t, mux, http.MethodPost, "/api/v1/itens", token, `{"titulo": "caneta", "quantidade": -1}`)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("validate_expr violation = %d: %.300s", rr.Code, rr.Body.String())
	}

	// pre_save block: the authored message passes verbatim.
	rr = apiReq(t, mux, http.MethodPost, "/api/v1/itens", token, `{"titulo": "bloqueado", "quantidade": 1}`)
	if rr.Code != http.StatusUnprocessableEntity || !strings.Contains(rr.Body.String(), "este titulo nao pode") {
		t.Fatalf("pre_save block = %d: %.300s", rr.Code, rr.Body.String())
	}

	// Create for real.
	rr = apiReq(t, mux, http.MethodPost, "/api/v1/itens", token, `{"titulo": "caneta", "quantidade": 5}`)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create = %d: %.300s", rr.Code, rr.Body.String())
	}
	ref, _ := decodeJSON(t, rr)["reference_id"].(string)
	if ref == "" {
		t.Fatal("create response missing reference_id")
	}

	// Read it back.
	rr = apiReq(t, mux, http.MethodGet, "/api/v1/itens/"+ref, token, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("get = %d: %.300s", rr.Code, rr.Body.String())
	}
	got := decodeJSON(t, rr)
	values, _ := got["values"].(map[string]any)
	if values["titulo"] != "caneta" || values["quantidade"] != float64(5) {
		t.Fatalf("values = %v", values)
	}
	rev, _ := got["rev"].(float64)
	if rev < 1 {
		t.Fatalf("rev = %v", got["rev"])
	}

	// List with text filter.
	rr = apiReq(t, mux, http.MethodGet, "/api/v1/itens?q=caneta", token, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("list = %d", rr.Code)
	}
	records, _ := decodeJSON(t, rr)["records"].([]any)
	if len(records) != 1 {
		t.Fatalf("list = %d records, want 1", len(records))
	}

	// Update: stale rev is a 409, fresh rev succeeds.
	rr = apiReq(t, mux, http.MethodPut, "/api/v1/itens/"+ref, token,
		fmt.Sprintf(`{"rev": %d, "titulo": "lapis", "quantidade": 2}`, int(rev)+10))
	if rr.Code != http.StatusConflict {
		t.Fatalf("stale rev = %d: %.300s", rr.Code, rr.Body.String())
	}
	rr = apiReq(t, mux, http.MethodPut, "/api/v1/itens/"+ref, token,
		fmt.Sprintf(`{"rev": %d, "titulo": "lapis", "quantidade": 2}`, int(rev)))
	if rr.Code != http.StatusOK {
		t.Fatalf("update = %d: %.300s", rr.Code, rr.Body.String())
	}
	rr = apiReq(t, mux, http.MethodGet, "/api/v1/itens/"+ref, token, "")
	values, _ = decodeJSON(t, rr)["values"].(map[string]any)
	if values["titulo"] != "lapis" {
		t.Fatalf("update not persisted: %v", values)
	}

	// Delete (soft) and confirm it vanished from the API.
	rr = apiReq(t, mux, http.MethodDelete, "/api/v1/itens/"+ref, token, "")
	if rr.Code != http.StatusNoContent {
		t.Fatalf("delete = %d: %.300s", rr.Code, rr.Body.String())
	}
	rr = apiReq(t, mux, http.MethodGet, "/api/v1/itens/"+ref, token, "")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("get after delete = %d", rr.Code)
	}
}

// Forms not flagged expose_api must not exist as far as the API can tell.
func TestAPIRequiresExposeFlag(t *testing.T) {
	mux, s := newHTTPTestEnv(t)
	form := seedAPIForm(t, s)
	token := mintToken(t, s, "apiuser2")

	err := s.UpdateForm(form.ID, form.MachineName, form.Label, "", form.EAVEntityTypeID,
		false, false, false, false, nil, false, false) // expose_api off
	if err != nil {
		t.Fatalf("UpdateForm: %v", err)
	}

	rr := apiReq(t, mux, http.MethodGet, "/api/v1/itens", token, "")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("unexposed form = %d, want 404", rr.Code)
	}
}

// The /me page mints and revokes tokens; the plaintext appears exactly once.
func TestAPITokenLifecycleOnProfile(t *testing.T) {
	mux, s := newHTTPTestEnv(t)
	user := plantUser(t, "tokenuser", false)

	form := strings.NewReader("label=ci+script")
	req := httptest.NewRequest(http.MethodPost, "/me/api-tokens", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(user)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("token create = %d: %.300s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, "new-api-token") || !strings.Contains(body, "ci script") {
		t.Fatalf("minted token not shown once: %.400s", body)
	}

	// The plaintext authenticates against the API once a form is exposed.
	seedAPIForm(t, s)
	start := strings.Index(body, `id="new-api-token"`)
	tokenStart := strings.Index(body[start:], ">") + start + 1
	tokenEnd := strings.Index(body[tokenStart:], "<") + tokenStart
	plaintext := strings.TrimSpace(body[tokenStart:tokenEnd])
	if plaintext == "" {
		t.Fatal("could not extract plaintext token from page")
	}
	api := apiReq(t, mux, http.MethodGet, "/api/v1/itens", plaintext, "")
	if api.Code != http.StatusOK {
		t.Fatalf("minted token rejected by API: %d", api.Code)
	}

	// The list page shows the token; revoking removes it.
	saved, err := db.Storage.GetUserByUsername("tokenuser")
	if err != nil || saved == nil {
		t.Fatalf("GetUserByUsername: %v", err)
	}
	tokens, err := db.Storage.ListAPITokensByUserID(saved.ID)
	if err != nil || len(tokens) != 1 {
		t.Fatalf("ListAPITokensByUserID = %d, %v", len(tokens), err)
	}

	del := httptest.NewRequest(http.MethodPost,
		fmt.Sprintf("/me/api-tokens/%d/delete", tokens[0].ID), nil)
	del.AddCookie(user)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, del)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("token delete = %d", rr.Code)
	}
	tokens, _ = db.Storage.ListAPITokensByUserID(saved.ID)
	if len(tokens) != 0 {
		t.Fatalf("token not revoked: %d left", len(tokens))
	}
	api = apiReq(t, mux, http.MethodGet, "/api/v1/itens", plaintext, "")
	if api.Code != http.StatusUnauthorized {
		t.Fatalf("revoked token still works: %d", api.Code)
	}
}
