package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
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

// TestAPIAndFormInterleaved proves the REST API and the HTML runtime are
// two doors into the same pipeline: edits through one are visible through
// the other and optimistic locking spans both.
func TestAPIAndFormInterleaved(t *testing.T) {
	mux, s := newHTTPTestEnv(t)
	seedAPIForm(t, s)
	token := mintToken(t, s, "apiuser3")
	browser := plantUser(t, "weboper", false)

	// Create through the API.
	rr := apiReq(t, mux, http.MethodPost, "/api/v1/itens", token, `{"titulo": "caderno", "quantidade": 3}`)
	if rr.Code != http.StatusCreated {
		t.Fatalf("api create = %d: %.300s", rr.Code, rr.Body.String())
	}
	ref, _ := decodeJSON(t, rr)["reference_id"].(string)

	// The HTML edit form renders the API-created values.
	rr = doGet(t, mux, "/form/itens/r/"+ref, browser)
	assertRendered(t, rr, "form edit")
	if !strings.Contains(rr.Body.String(), "caderno") {
		t.Fatalf("HTML form missing API-created value: %.300s", rr.Body.String())
	}

	// Update through the HTML form (rev 1 came from creation).
	rec, err := db.Storage.GetEAVRecordByRefID(ref)
	if err != nil {
		t.Fatalf("GetEAVRecordByRefID: %v", err)
	}
	form := url.Values{
		"titulo":     {"caderno grande"},
		"quantidade": {"5"},
		"rev":        {strconv.Itoa(rec.Rev)},
	}
	rr = doPostForm(t, mux, "/form/itens/r/"+ref, form, browser)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("html update = %d: %.300s", rr.Code, rr.Body.String())
	}

	// The API sees the HTML edit.
	rr = apiReq(t, mux, http.MethodGet, "/api/v1/itens/"+ref, token, "")
	got := decodeJSON(t, rr)
	values, _ := got["values"].(map[string]any)
	if values["titulo"] != "caderno grande" || values["quantidade"] != float64(5) {
		t.Fatalf("api does not see html edit: %v", values)
	}
	apiRev := int(got["rev"].(float64))

	// A stale API update (rev from before the HTML edit) is a 409.
	rr = apiReq(t, mux, http.MethodPut, "/api/v1/itens/"+ref, token,
		fmt.Sprintf(`{"rev": %d, "titulo": "x", "quantidade": 1}`, rec.Rev))
	if rr.Code != http.StatusConflict {
		t.Fatalf("stale cross-door rev = %d, want 409", rr.Code)
	}

	// Fresh rev succeeds, and the HTML listing reflects it.
	rr = apiReq(t, mux, http.MethodPut, "/api/v1/itens/"+ref, token,
		fmt.Sprintf(`{"rev": %d, "titulo": "estojo", "quantidade": 2}`, apiRev))
	if rr.Code != http.StatusOK {
		t.Fatalf("fresh api update = %d: %.300s", rr.Code, rr.Body.String())
	}
	rr = doGet(t, mux, "/form/itens/list", browser)
	assertRendered(t, rr, "form list")
	if !strings.Contains(rr.Body.String(), "estojo") {
		t.Fatal("html listing missing api edit")
	}
}

// TestConcurrentRecordWrites hammers one record from parallel writers; the
// rev trigger must serialize them so every successful bump is exactly +1.
func TestConcurrentRecordWrites(t *testing.T) {
	_, s := newHTTPTestEnv(t)
	et, err := s.CreateEAVEntityType("Contador", "contador", "", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType: %v", err)
	}
	intAttr, err := s.CreateEAVAttribute(et.ID, "valor", "Valor", "", "INT",
		false, false, false, nil, false, "", nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateEAVAttribute: %v", err)
	}

	rec, err := s.CreateEAVRecord(et.ID)
	if err != nil {
		t.Fatalf("CreateEAVRecord: %v", err)
	}

	const writers = 8
	const attempts = 20
	var wg sync.WaitGroup
	var conflicts, successes atomic.Int64
	for w := range writers {
		wg.Add(1)
		go func(seed int64) {
			defer wg.Done()
			for range attempts {
				cur, err := s.GetEAVRecordByID(rec.ID)
				if err != nil {
					t.Errorf("GetEAVRecordByID: %v", err)
					return
				}
				v := seed
				_, err = s.UpsertEAVValueWithRev(rec.ID, intAttr.ID, cur.Rev,
					nil, &v, nil, nil, nil)
				if err == nil {
					successes.Add(1)
					continue
				}
				if errors.Is(err, db.ErrConflict) {
					conflicts.Add(1)
					continue
				}
				t.Errorf("unexpected write error: %v", err)
				return
			}
		}(int64(w))
	}
	wg.Wait()

	if successes.Load() == 0 {
		t.Fatal("no write ever succeeded")
	}
	final, err := s.GetEAVRecordByID(rec.ID)
	if err != nil {
		t.Fatalf("final read: %v", err)
	}
	// Every successful optimistic write bumps rev by exactly 1.
	if int64(final.Rev) != 1+successes.Load() {
		t.Fatalf("rev = %d, want 1+%d successes (lost or double bump)",
			final.Rev, successes.Load())
	}
	t.Logf("successes=%d conflicts=%d final rev=%d", successes.Load(), conflicts.Load(), final.Rev)
}

// A stale rev submitted through the HTML runtime re-renders with the
// conflict notice, not the generic "Could not save (ref ...)" text. The
// admin records screen and the JSON API already said "conflict"; the user
// runtime used to hide it behind a reference id.
func TestHTMLUpdateStaleRevShowsConflict(t *testing.T) {
	mux, s := newHTTPTestEnv(t)
	seedAPIForm(t, s)
	browser := plantUser(t, "webconf", false)

	form := url.Values{"titulo": {"agenda"}, "quantidade": {"1"}}
	rr := doPostForm(t, mux, "/form/itens", form, browser)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("create = %d: %.300s", rr.Code, rr.Body.String())
	}

	et, err := db.Storage.GetEAVEntityTypeByMachineName("item")
	if err != nil {
		t.Fatalf("entity: %v", err)
	}
	recs, _, err := db.Storage.ListEAVRecordsByEntityTypeID(et.ID, 10, 0)
	if err != nil || len(recs) == 0 {
		t.Fatalf("records: %v (%d)", err, len(recs))
	}
	ref := recs[0].ReferenceID

	// First update with the current rev succeeds and bumps it.
	form = url.Values{
		"titulo": {"agenda 2027"}, "quantidade": {"1"},
		"rev": {strconv.Itoa(recs[0].Rev)},
	}
	rr = doPostForm(t, mux, "/form/itens/r/"+ref, form, browser)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("fresh update = %d: %.300s", rr.Code, rr.Body.String())
	}

	// Replaying the same stale rev re-renders with the conflict notice.
	rr = doPostForm(t, mux, "/form/itens/r/"+ref, form, browser)
	if rr.Code != http.StatusOK {
		t.Fatalf("stale update = %d, want 200 re-render", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "modified by another user") {
		t.Fatalf("conflict notice missing from re-render: %.400s", body)
	}
	if strings.Contains(body, "(ref ") {
		t.Fatalf("stale rev fell through to the generic ref-id error: %.400s", body)
	}
}

// A partial JSON payload omits fields the HTML form would always post. The
// validate/computed env must fill the gaps with each kind's zero value —
// otherwise (>= field:quantidade 0) sees a string and the create 500s.
func TestAPICreatePartialPayloadRunsNumericValidate(t *testing.T) {
	mux, s := newHTTPTestEnv(t)
	seedAPIForm(t, s)
	token := mintToken(t, s, "apipartial")

	rr := apiReq(t, mux, http.MethodPost, "/api/v1/itens", token, `{"titulo": "so o titulo"}`)
	if rr.Code != http.StatusCreated {
		t.Fatalf("partial create = %d, want 201: %.300s", rr.Code, rr.Body.String())
	}
	ref, _ := decodeJSON(t, rr)["reference_id"].(string)

	rr = apiReq(t, mux, http.MethodGet, "/api/v1/itens/"+ref, token, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("get = %d: %.300s", rr.Code, rr.Body.String())
	}
	values, _ := decodeJSON(t, rr)["values"].(map[string]any)
	if values["quantidade"] != float64(0) {
		t.Fatalf("quantidade = %v, want 0 (INT zero fill, same as empty HTML input)", values["quantidade"])
	}
}
