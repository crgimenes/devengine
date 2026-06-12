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
func addButton(t *testing.T, s db.Store, form *db.Form, machine, filoCode string, runSave bool) {
	t.Helper()
	el, err := s.CreateFormElement(form.ID, nil, machine, "button", machine, "", 10, 12, "", "", nil, true, false)
	if err != nil {
		t.Fatalf("CreateFormElement(button): %v", err)
	}
	err = s.UpdateFormElement(el.ID, nil, machine, "button", machine, "", 10, 12, "left",
		"", "", nil, true, false, false, false,
		"",
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

// seedStockScenario builds produto (estoque INT) with one record at stock 10,
// plus a pedido2 form with quantidade/produto fields bound to its own entity.
func seedStockScenario(t *testing.T, s db.Store) (form *db.Form, produtoRef string) {
	t.Helper()
	prodET, err := s.CreateEAVEntityType("Produto", "produto", "", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType(produto): %v", err)
	}
	estoque, err := s.CreateEAVAttribute(prodET.ID, "estoque", "Estoque", "", "INT",
		false, false, false, nil, false, "", nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateEAVAttribute(estoque): %v", err)
	}
	prod, err := s.CreateEAVRecord(prodET.ID)
	if err != nil {
		t.Fatalf("CreateEAVRecord(produto): %v", err)
	}
	ten := int64(10)
	err = s.UpsertEAVValue(prod.ID, estoque.ID, nil, &ten, nil, nil, nil)
	if err != nil {
		t.Fatalf("UpsertEAVValue(estoque): %v", err)
	}

	pedET, err := s.CreateEAVEntityType("Pedido", "pedido", "", "", "")
	if err != nil {
		t.Fatalf("CreateEAVEntityType(pedido): %v", err)
	}
	mk := func(machine, kind string) db.EAVAttribute {
		a, err := s.CreateEAVAttribute(pedET.ID, machine, machine, "", kind,
			false, false, false, nil, false, "", nil, nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("CreateEAVAttribute(%s): %v", machine, err)
		}
		return *a
	}
	quantidade := mk("quantidade", "INT")
	produto := mk("produto", "TEXT")

	form, err = s.CreateForm("pedido", "Pedido", "", &pedET.ID)
	if err != nil {
		t.Fatalf("CreateForm: %v", err)
	}
	for i, a := range []db.EAVAttribute{quantidade, produto} {
		_, err = s.CreateFormElement(form.ID, nil, a.MachineName, "field", a.MachineName,
			"", i, 12, "", "", &a.ID, false, false)
		if err != nil {
			t.Fatalf("CreateFormElement(%s): %v", a.MachineName, err)
		}
	}
	return form, prod.ReferenceID
}

func stockOf(t *testing.T, s db.Store, produtoRef string) int64 {
	t.Helper()
	rec, err := s.GetEAVRecordByRefID(produtoRef)
	if err != nil {
		t.Fatalf("GetEAVRecordByRefID: %v", err)
	}
	vals, err := s.GetEAVValuesByRecordID(rec.ID)
	if err != nil {
		t.Fatalf("GetEAVValuesByRecordID: %v", err)
	}
	for _, v := range vals {
		if v.VInt != nil {
			return *v.VInt
		}
	}
	t.Fatal("estoque value missing")
	return 0
}

const stockFilo = `(if (< (eav-get-value "produto" field:produto "estoque") field:quantidade)
    (set error "estoque insuficiente")
    (eav-set-value "produto" field:produto "estoque"
        (- (eav-get-value "produto" field:produto "estoque") field:quantidade)))`

// A confirm button must decrement the product stock in the SAME transaction
// that saves the pedido.
func TestButtonEAVStockDecrement(t *testing.T) {
	mux, s := newHTTPTestEnv(t)
	form, produtoRef := seedStockScenario(t, s)
	addButton(t, s, form, "confirmar", stockFilo, true)
	user := plantUser(t, "user", false)

	resp := postButton(t, mux, form, "confirmar",
		url.Values{"quantidade": {"4"}, "produto": {produtoRef}}, user)
	if e, _ := resp["error"].(string); e != "" {
		t.Fatalf("unexpected error: %v", e)
	}
	if got := stockOf(t, s, produtoRef); got != 6 {
		t.Fatalf("estoque = %d, want 6", got)
	}
	records, err := s.ListEAVRecordsCursor(*form.EAVEntityTypeID, 0, 10, "")
	if err != nil || len(records) != 1 {
		t.Fatalf("pedido records = %d (%v), want 1", len(records), err)
	}
}

// Insufficient stock must block the save and leave the stock untouched.
func TestButtonEAVStockInsufficient(t *testing.T) {
	mux, s := newHTTPTestEnv(t)
	form, produtoRef := seedStockScenario(t, s)
	addButton(t, s, form, "confirmar", stockFilo, true)
	user := plantUser(t, "user", false)

	rr := doPostForm(t, mux, "/form/"+form.MachineName+"/action/confirmar",
		url.Values{"quantidade": {"99"}, "produto": {produtoRef}}, user)
	if !strings.Contains(rr.Body.String(), "estoque insuficiente") {
		t.Fatalf("want insufficiency error, body: %.200s", rr.Body.String())
	}
	if got := stockOf(t, s, produtoRef); got != 10 {
		t.Fatalf("estoque = %d, want 10 (untouched)", got)
	}
	records, err := s.ListEAVRecordsCursor(*form.EAVEntityTypeID, 0, 10, "")
	if err != nil || len(records) != 0 {
		t.Fatalf("pedido records = %d (%v), want 0", len(records), err)
	}
}

// A script write followed by a script error must roll back together with the
// save: eav-set-value joins the button transaction.
func TestButtonEAVWriteRollsBackOnError(t *testing.T) {
	mux, s := newHTTPTestEnv(t)
	form, produtoRef := seedStockScenario(t, s)
	addButton(t, s, form, "confirmar",
		`(eav-set-value "produto" field:produto "estoque" 0)
		 (set error "abort after write")`, true)
	user := plantUser(t, "user", false)

	rr := doPostForm(t, mux, "/form/"+form.MachineName+"/action/confirmar",
		url.Values{"quantidade": {"1"}, "produto": {produtoRef}}, user)
	if !strings.Contains(rr.Body.String(), "abort after write") {
		t.Fatalf("want script error, body: %.200s", rr.Body.String())
	}
	if got := stockOf(t, s, produtoRef); got != 10 {
		t.Fatalf("estoque = %d, want 10 (write rolled back)", got)
	}
}

// Button run-save must persist every posted field and apply computed
// attributes, matching the regular create path.
func TestButtonRunSavePersistsAllFieldsAndComputed(t *testing.T) {
	mux, s := newHTTPTestEnv(t)
	form, produtoRef := seedStockScenario(t, s)

	// Add a REAL field and a computed total to the pedido entity.
	valor, err := s.CreateEAVAttribute(*form.EAVEntityTypeID, "valor", "valor", "", "REAL",
		false, false, false, nil, false, "", nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateEAVAttribute(valor): %v", err)
	}
	_, err = s.CreateFormElement(form.ID, nil, "valor", "field", "valor",
		"", 5, 12, "", "", &valor.ID, false, false)
	if err != nil {
		t.Fatalf("CreateFormElement(valor): %v", err)
	}
	_, err = s.CreateEAVAttribute(*form.EAVEntityTypeID, "total", "total", "", "REAL",
		false, false, false, nil, true, `(* field:valor field:quantidade)`, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateEAVAttribute(total): %v", err)
	}

	addButton(t, s, form, "confirmar", stockFilo, true)
	user := plantUser(t, "user", false)

	resp := postButton(t, mux, form, "confirmar",
		url.Values{"quantidade": {"3"}, "valor": {"15"}, "produto": {produtoRef}}, user)
	if e, _ := resp["error"].(string); e != "" {
		t.Fatalf("unexpected error: %v", e)
	}

	records, err := s.ListEAVRecordsCursor(*form.EAVEntityTypeID, 0, 10, "")
	if err != nil || len(records) != 1 {
		t.Fatalf("records = %d (%v)", len(records), err)
	}
	vals, err := s.GetEAVValuesByRecordID(records[0].ID)
	if err != nil {
		t.Fatalf("GetEAVValuesByRecordID: %v", err)
	}
	byAttr := map[int64]db.EAVValue{}
	for _, v := range vals {
		byAttr[v.AttributeID] = v
	}
	if v := byAttr[valor.ID]; v.VReal == nil || *v.VReal != 15 {
		t.Fatalf("valor not persisted: %+v", v)
	}
	totalAttr, err := s.GetEAVAttributeByRefID(func() string {
		attrs, _ := s.ListEAVAttributesByEntityTypeID(*form.EAVEntityTypeID)
		for _, a := range attrs {
			if a.MachineName == "total" {
				return a.ReferenceID
			}
		}
		return ""
	}())
	if err != nil {
		t.Fatalf("total attr: %v", err)
	}
	if v := byAttr[totalAttr.ID]; v.VReal == nil || *v.VReal != 45 {
		t.Fatalf("computed total not applied on button save: %+v", v)
	}
}
