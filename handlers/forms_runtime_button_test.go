package handlers

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/crgimenes/devengine/db"
)

// addButton attaches a button element with the given Filo script to the form.
func addButton(t *testing.T, s *db.SQLite, form *db.Form, machine, filoCode string, runSave bool) {
	t.Helper()
	el, err := s.CreateFormElement(form.ID, nil, machine, "button", machine, "", 10, 12, "", "", nil, true, false)
	if err != nil {
		t.Fatalf("CreateFormElement(button): %v", err)
	}
	err = s.UpdateFormElement(el.ID, nil, machine, "button", machine, "", 10, 12, "left",
		"", "", nil, true, false, false, false,
		filoCode, runSave, "", "", "")
	if err != nil {
		t.Fatalf("UpdateFormElement(button): %v", err)
	}
}

func postButton(t *testing.T, mux *http.ServeMux, form *db.Form, button string, body url.Values, c *http.Cookie) map[string]any {
	t.Helper()
	rr := doPostForm(t, mux, "/form/"+form.MachineName+"/action/"+button, body, c)
	if rr.Code != http.StatusOK {
		t.Fatalf("button action = %d, body: %.300s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	if err != nil {
		t.Fatalf("unmarshal response: %v (%.200s)", err, rr.Body.String())
	}
	return resp
}

// A pure Filo button must run the script and surface the message control var.
func TestFormsRuntimeButtonFiloMessage(t *testing.T) {
	mux, s := newHTTPTestEnv(t)
	form := seedTaskForm(t, s, "Comprar leite")
	addButton(t, s, form, "avisar", `(set message "hello from filo")`, false)
	user := plantUser(t, "user", false)

	resp := postButton(t, mux, form, "avisar", url.Values{"titulo": {"x"}}, user)
	if resp["message"] != "hello from filo" {
		t.Fatalf("message = %v", resp["message"])
	}
}

// A run-save button must persist the record with values the script modified.
func TestFormsRuntimeButtonRunSaveCreates(t *testing.T) {
	mux, s := newHTTPTestEnv(t)
	form := seedTaskForm(t, s, "Comprar leite")
	addButton(t, s, form, "salvar", `(set field:titulo (str-concat field:titulo " [ok]"))`, true)
	user := plantUser(t, "user", false)

	resp := postButton(t, mux, form, "salvar", url.Values{"titulo": {"Nova"}}, user)
	redirect, _ := resp["redirect_to"].(string)
	if !strings.Contains(redirect, "/form/"+form.MachineName+"/r/") {
		t.Fatalf("redirect_to = %v", resp["redirect_to"])
	}

	records, err := s.ListEAVRecordsCursor(*form.EAVEntityTypeID, 0, 10, "")
	if err != nil {
		t.Fatalf("ListEAVRecordsCursor: %v", err)
	}
	if len(records) != 2 { // seed record + the one the button created
		t.Fatalf("records = %d, want 2", len(records))
	}
	vals, err := s.GetEAVValuesByRecordID(records[0].ID)
	if err != nil {
		t.Fatalf("GetEAVValuesByRecordID: %v", err)
	}
	if len(vals) != 1 || vals[0].VText == nil || *vals[0].VText != "Nova [ok]" {
		t.Fatalf("saved value = %+v", vals)
	}
}

// A Filo error must roll the action back without creating a record.
func TestFormsRuntimeButtonFiloErrorBlocks(t *testing.T) {
	mux, s := newHTTPTestEnv(t)
	form := seedTaskForm(t, s, "Comprar leite")
	addButton(t, s, form, "negar", `(set error "not allowed")`, true)
	user := plantUser(t, "user", false)

	rr := doPostForm(t, mux, "/form/"+form.MachineName+"/action/negar",
		url.Values{"titulo": {"Nova"}}, user)
	var resp map[string]any
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	if err != nil {
		t.Fatalf("unmarshal: %v (%.200s)", err, rr.Body.String())
	}
	if resp["error"] != "not allowed" {
		t.Fatalf("error = %v (status %d)", resp["error"], rr.Code)
	}

	records, err := s.ListEAVRecordsCursor(*form.EAVEntityTypeID, 0, 10, "")
	if err != nil {
		t.Fatalf("ListEAVRecordsCursor: %v", err)
	}
	if len(records) != 1 { // only the seed record
		t.Fatalf("records = %d, want 1 (no record from blocked action)", len(records))
	}
}
