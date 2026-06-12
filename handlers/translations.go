package handlers

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/crgimenes/devengine/auth"
	"github.com/crgimenes/devengine/config"
	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/devengine/i18n"
	"github.com/crgimenes/devengine/log"
)

// TranslationRow is one message key as shown in the editor.
type TranslationRow struct {
	Key        string // the US English source text
	Effective  string // what the locale currently shows
	Overridden bool   // true when a user adjustment is active
}

// translationsPrelude runs the shared sysop prologue and resolves the target
// locale being edited (first non-default registered locale by default).
func (h *Handlers) translationsPrelude(w http.ResponseWriter, r *http.Request, methods []string) (*db.User, string, bool) {
	user, _, authed, err := auth.Prelude(w, r, methods, true, true)
	if err != nil {
		h.serverError(w, r, "ToolsTranslations", err)
		return nil, "", false
	}
	if !authed {
		return nil, "", false
	}
	if !user.Sysop {
		h.forbidden(w, r)
		return nil, "", false
	}

	target := strings.TrimSpace(r.FormValue("locale"))
	if target == "" {
		for _, loc := range i18n.Locales() {
			if loc != i18n.DefaultLocale {
				target = loc
				break
			}
		}
	}
	if target == "" {
		target = i18n.DefaultLocale
	}
	return user, target, true
}

// ToolsTranslations lists every known message key with its current
// translation in the chosen locale, filtered by ?q=.
func (h *Handlers) ToolsTranslations(w http.ResponseWriter, r *http.Request) {
	user, target, ok := h.translationsPrelude(w, r, []string{http.MethodGet})
	if !ok {
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	tab := r.URL.Query().Get("tab")
	if tab != "content" {
		tab = "system"
	}

	var cRows []ContentRow
	if tab == "content" {
		var err error
		cRows, err = contentRows(target, query)
		if err != nil {
			h.serverError(w, r, "contentRows", err)
			return
		}
	}

	var rows []TranslationRow
	for _, key := range i18n.Keys() {
		effective := i18n.TL(target, key)
		if query != "" &&
			!strings.Contains(strings.ToLower(key), strings.ToLower(query)) &&
			!strings.Contains(strings.ToLower(effective), strings.ToLower(query)) {
			continue
		}
		_, overridden := i18n.Override(target, key)
		rows = append(rows, TranslationRow{
			Key:        key,
			Effective:  effective,
			Overridden: overridden,
		})
	}

	data := struct {
		Authed      bool
		User        db.User
		Error       string
		Message     string
		Config      config.Config
		CurrentPage string
		Locale      string
		Target      string
		Locales     []string
		Query       string
		Tab         string
		Rows        []TranslationRow
		ContentRows []ContentRow
	}{
		Authed:      true,
		User:        *user,
		Config:      *h.cfg,
		CurrentPage: "translations",
		Locale:      auth.RequestLocale(r),
		Target:      target,
		Locales:     i18n.Locales(),
		Query:       query,
		Tab:         tab,
		Message:     r.URL.Query().Get("message"),
		Rows:        rows,
		ContentRows: cRows,
	}

	h.render(w, "tools_translations.go.tmpl", data)
}

// ToolsTranslationsSave upserts one adjustment (or deletes it when the text
// comes empty, restoring the built-in translation) and applies it live.
func (h *Handlers) ToolsTranslationsSave(w http.ResponseWriter, r *http.Request) {
	_, target, ok := h.translationsPrelude(w, r, []string{http.MethodPost})
	if !ok {
		return
	}

	key := r.FormValue("msg_key")
	translation := strings.TrimSpace(r.FormValue("translation"))
	back := "/tools/translations?locale=" + url.QueryEscape(target) +
		"&q=" + url.QueryEscape(r.FormValue("q"))

	if key == "" || target == i18n.DefaultLocale {
		http.Redirect(w, r, back+"&message="+url.QueryEscape(tr(r, "Bad request")), http.StatusSeeOther)
		return
	}

	if translation == "" {
		err := db.Storage.DeleteI18nOverride(target, key)
		if err != nil {
			ref := logRef("DeleteI18nOverride", err)
			http.Redirect(w, r, back+"&message="+url.QueryEscape(tr(r, "Could not save the translation (ref %s)", ref)), http.StatusSeeOther)
			return
		}
		i18n.DeleteOverride(target, key)
		http.Redirect(w, r, back+"&message="+url.QueryEscape(tr(r, "Built-in translation restored")), http.StatusSeeOther)
		return
	}

	err := db.Storage.UpsertI18nOverride(target, key, translation)
	if err != nil {
		ref := logRef("UpsertI18nOverride", err)
		http.Redirect(w, r, back+"&message="+url.QueryEscape(tr(r, "Could not save the translation (ref %s)", ref)), http.StatusSeeOther)
		return
	}
	i18n.SetOverride(target, key, translation)
	http.Redirect(w, r, back+"&message="+url.QueryEscape(tr(r, "Translation saved")), http.StatusSeeOther)
}

// ToolsTranslationsExport streams the chosen locale as CSV
// (key, translation, overridden) for external translators.
func (h *Handlers) ToolsTranslationsExport(w http.ResponseWriter, r *http.Request) {
	_, target, ok := h.translationsPrelude(w, r, []string{http.MethodGet})
	if !ok {
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="translations-`+target+`.csv"`)

	cw := csv.NewWriter(w)
	defer cw.Flush()
	err := cw.Write([]string{"key", "translation", "overridden"})
	if err != nil {
		log.Printf("translations csv header: %v", err)
		return
	}
	for _, key := range i18n.Keys() {
		_, overridden := i18n.Override(target, key)
		flag := ""
		if overridden {
			flag = "1"
		}
		err := cw.Write([]string{key, i18n.TL(target, key), flag})
		if err != nil {
			log.Printf("translations csv row: %v", err)
			return
		}
	}
}

// ContentRow is one translatable field of a user-created object.
type ContentRow struct {
	Kind       string // form, element, attribute, menu item
	Context    string // owning form / entity / menu
	Field      string // label or help_text
	RefID      string
	Original   string
	Effective  string
	Overridden bool
}

// contentRows inventories every translatable user-content field.
func contentRows(target, query string) ([]ContentRow, error) {
	var rows []ContentRow
	add := func(kind, context, field, refID, original string) {
		if original == "" {
			return
		}
		effective := i18n.ContentOr(target, refID, field, original)
		if query != "" &&
			!strings.Contains(strings.ToLower(original), strings.ToLower(query)) &&
			!strings.Contains(strings.ToLower(effective), strings.ToLower(query)) &&
			!strings.Contains(strings.ToLower(context), strings.ToLower(query)) {
			return
		}
		_, overridden := i18n.Content(target, refID, field)
		rows = append(rows, ContentRow{
			Kind:       kind,
			Context:    context,
			Field:      field,
			RefID:      refID,
			Original:   original,
			Effective:  effective,
			Overridden: overridden,
		})
	}

	forms, err := db.Storage.ListForms()
	if err != nil {
		return nil, err
	}
	for _, f := range forms {
		add("form", f.MachineName, "label", f.ReferenceID, f.Label)
		elements, err := db.Storage.ListFormElements(f.ID)
		if err != nil {
			return nil, err
		}
		for _, el := range elements {
			add("element", f.MachineName, "label", el.ReferenceID, el.Label)
			add("element", f.MachineName, "help_text", el.ReferenceID, el.HelpText)
		}
	}

	entityTypes, err := db.Storage.ListEAVEntityTypes()
	if err != nil {
		return nil, err
	}
	for _, et := range entityTypes {
		attributes, err := db.Storage.ListEAVAttributesByEntityTypeID(et.ID)
		if err != nil {
			return nil, err
		}
		for _, a := range attributes {
			add("attribute", et.MachineName, "label", a.ReferenceID, a.Label)
		}
	}

	menus, err := db.Storage.ListMenus()
	if err != nil {
		return nil, err
	}
	for _, m := range menus {
		items, err := db.Storage.ListMenuItems(m.ID)
		if err != nil {
			return nil, err
		}
		for _, it := range items {
			add("menu item", m.MachineName, "label", it.ReferenceID, it.Label)
		}
	}
	return rows, nil
}

// ToolsTranslationsContentSave upserts one content translation (empty text
// restores the original) and applies it live.
func (h *Handlers) ToolsTranslationsContentSave(w http.ResponseWriter, r *http.Request) {
	_, target, ok := h.translationsPrelude(w, r, []string{http.MethodPost})
	if !ok {
		return
	}

	refID := r.FormValue("ref_id")
	field := r.FormValue("field")
	text := strings.TrimSpace(r.FormValue("text"))
	back := "/tools/translations?tab=content&locale=" + url.QueryEscape(target) +
		"&q=" + url.QueryEscape(r.FormValue("q"))

	// Unlike system strings (where the key IS the English text), user
	// content has no canonical language: translating INTO en-US is valid.
	if refID == "" || field == "" {
		http.Redirect(w, r, back+"&message="+url.QueryEscape(tr(r, "Bad request")), http.StatusSeeOther)
		return
	}

	if text == "" {
		err := db.Storage.DeleteContentTranslation(target, refID, field)
		if err != nil {
			ref := logRef("DeleteContentTranslation", err)
			http.Redirect(w, r, back+"&message="+url.QueryEscape(tr(r, "Could not save the translation (ref %s)", ref)), http.StatusSeeOther)
			return
		}
		i18n.DeleteContent(target, refID, field)
		http.Redirect(w, r, back+"&message="+url.QueryEscape(tr(r, "Original text restored")), http.StatusSeeOther)
		return
	}

	err := db.Storage.UpsertContentTranslation(target, refID, field, text)
	if err != nil {
		ref := logRef("UpsertContentTranslation", err)
		http.Redirect(w, r, back+"&message="+url.QueryEscape(tr(r, "Could not save the translation (ref %s)", ref)), http.StatusSeeOther)
		return
	}
	i18n.SetContent(target, refID, field, text)
	http.Redirect(w, r, back+"&message="+url.QueryEscape(tr(r, "Translation saved")), http.StatusSeeOther)
}

// ToolsTranslationsExportGo emits the locale as a Go dictionary file in the
// shape of i18n/pt_br.go, ready to be compiled into an application or
// contributed upstream. mode=overrides limits it to the user adjustments.
func (h *Handlers) ToolsTranslationsExportGo(w http.ResponseWriter, r *http.Request) {
	_, target, ok := h.translationsPrelude(w, r, []string{http.MethodGet})
	if !ok {
		return
	}
	mode := r.URL.Query().Get("mode")

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+strings.ReplaceAll(strings.ToLower(target), "-", "_")+`.go"`)

	var b strings.Builder
	b.WriteString("package i18n\n\n")
	fmt.Fprintf(&b, "// %s dictionary exported from /tools/translations (mode: %s).\n", target, cmpOr(mode, "merged"))
	b.WriteString("func init() {\n")
	fmt.Fprintf(&b, "\tRegister(%q, map[string]string{\n", target)
	for _, key := range i18n.Keys() {
		var text string
		if mode == "overrides" {
			t, ok := i18n.Override(target, key)
			if !ok {
				continue
			}
			text = t
		} else {
			text = i18n.TL(target, key)
			if text == key {
				continue // untranslated: the key is the English text
			}
		}
		fmt.Fprintf(&b, "\t\t%q: %q,\n", key, text)
	}
	b.WriteString("\t})\n}\n")
	_, _ = w.Write([]byte(b.String()))
}

// cmpOr returns a when non-empty, else b.
func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// ToolsTranslationsImport ingests a CSV (key,translation) into the chosen
// locale as overrides, applied live.
func (h *Handlers) ToolsTranslationsImport(w http.ResponseWriter, r *http.Request) {
	_, target, ok := h.translationsPrelude(w, r, []string{http.MethodPost})
	if !ok {
		return
	}
	back := "/tools/translations?locale=" + url.QueryEscape(target)

	if target == i18n.DefaultLocale {
		http.Redirect(w, r, back+"&message="+url.QueryEscape(tr(r, "Bad request")), http.StatusSeeOther)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	err := r.ParseMultipartForm(1 << 20) // #nosec G120 -- bounded by MaxBytesReader above
	if err != nil {
		h.errorPage(w, r, http.StatusBadRequest, "bad request")
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Redirect(w, r, back+"&message="+url.QueryEscape(tr(r, "Select a CSV file")), http.StatusSeeOther)
		return
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	imported := 0
	for {
		record, err := reader.Read()
		if err != nil {
			break
		}
		if len(record) < 2 || record[0] == "" || record[0] == "key" || record[1] == "" {
			continue
		}
		key, text := record[0], record[1]
		err = db.Storage.UpsertI18nOverride(target, key, text)
		if err != nil {
			ref := logRef("import UpsertI18nOverride", err)
			http.Redirect(w, r, back+"&message="+url.QueryEscape(tr(r, "Could not save the translation (ref %s)", ref)), http.StatusSeeOther)
			return
		}
		i18n.SetOverride(target, key, text)
		imported++
	}
	http.Redirect(w, r, back+"&message="+url.QueryEscape(tr(r, "%d translations imported", imported)), http.StatusSeeOther)
}
