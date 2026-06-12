package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/crgimenes/devengine/auth"
	"github.com/crgimenes/devengine/config"
	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/filo"
)

// FiloGlobalView is one row in the REPL's globals table: the key, its
// formatted value, and the Filo kind label.
type FiloGlobalView struct {
	Key   string
	Value string
	Kind  string
}

// ToolsFilo renders the REPL page. The result panel starts empty; it gets
// populated by HTMX after a POST to ToolsFiloRun.
func (h *Handlers) ToolsFilo(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodGet},
		true, true,
	)
	if err != nil {
		h.serverError(w, r, "ToolsFilo", err)
		return
	}
	if !authed {
		return
	}
	if !user.Sysop {
		h.forbidden(w, r)
		return
	}

	data := struct {
		Authed      bool
		User        db.User
		Locale      string
		Config      config.Config
		CurrentPage string
	}{
		Authed:      true,
		Locale:      auth.RequestLocale(r),
		User:        *user,
		Config:      *h.cfg,
		CurrentPage: "filo",
	}

	h.render(w, "tools_filo.go.tmpl", data)
}

// ToolsFiloRun executes the submitted script and renders the result fragment
// used by HTMX. The handler swallows script errors and surfaces them inside
// the fragment; only system-level errors return a non-200 status.
func (h *Handlers) ToolsFiloRun(w http.ResponseWriter, r *http.Request) {
	user, _, authed, err := auth.Prelude(w, r,
		[]string{http.MethodPost},
		true, true,
	)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if !authed {
		return
	}
	if !user.Sysop {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	script := r.FormValue("script")
	globalsJSON := strings.TrimSpace(r.FormValue("globals"))

	result := h.runFiloScript(r, user, script, globalsJSON)

	h.render(w, "tools_filo_result.go.tmpl", result)
}

// filoResult is the shape passed to the result fragment template.
type filoResult struct {
	Locale     string
	Script     string
	Globals    []FiloGlobalView
	Result     string
	ResultKind string
	UserError  string
	SysError   string
	ElapsedMS  float64
}

func (h *Handlers) runFiloScript(r *http.Request, user *db.User, script, globalsJSON string) filoResult {
	if strings.TrimSpace(script) == "" {
		return filoResult{Locale: auth.RequestLocale(r), Script: script, UserError: tr(r, "Empty script.")}
	}

	globals := make(map[string]filo.Value)
	if globalsJSON != "" {
		raw := map[string]any{}
		err := json.Unmarshal([]byte(globalsJSON), &raw)
		if err != nil {
			return filoResult{Locale: auth.RequestLocale(r), Script: script, UserError: tr(r, "Invalid globals JSON: %s", err.Error())}
		}
		for k, v := range raw {
			globals[k] = goToFilo(v)
		}
	}

	eng := newFiloEngine(user, globals)

	ctx, cancel := context.WithTimeout(context.Background(), filoEvalConfig().Timeout)
	defer cancel()

	start := time.Now()
	value, newGlobals, execErr := eng.RunScript(ctx, script, globals, filoEvalConfig())
	elapsed := time.Since(start)

	out := filoResult{
		Locale:    auth.RequestLocale(r),
		Script:    script,
		ElapsedMS: float64(elapsed.Nanoseconds()) / 1e6,
	}

	if execErr != nil {
		if strings.Contains(execErr.Error(), "empty script") {
			out.UserError = tr(r, "Empty script.")
			return out
		}
		out.SysError = execErr.Error()
		out.Globals = viewGlobals(globals)
		return out
	}

	if e, ok := newGlobals["error"]; ok && e.Kind == filo.KString && e.Str != "" {
		out.UserError = e.Str
	}

	out.Result = formatFilo(value)
	out.ResultKind = filoKindName(value.Kind)
	out.Globals = viewGlobals(newGlobals)
	return out
}

func viewGlobals(g map[string]filo.Value) []FiloGlobalView {
	keys := make([]string, 0, len(g))
	for k := range g {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]FiloGlobalView, 0, len(keys))
	for _, k := range keys {
		v := g[k]
		out = append(out, FiloGlobalView{
			Key:   k,
			Value: formatFilo(v),
			Kind:  filoKindName(v.Kind),
		})
	}
	return out
}

func formatFilo(v filo.Value) string {
	switch v.Kind {
	case filo.KBool:
		if v.Bool {
			return "true"
		}
		return "false"
	case filo.KNumber:
		if v.Num == float64(int64(v.Num)) {
			return fmt.Sprintf("%d", int64(v.Num))
		}
		return fmt.Sprintf("%g", v.Num)
	case filo.KString:
		return v.Str
	case filo.KList:
		return v.String()
	}
	return v.String()
}

func filoKindName(k filo.Kind) string {
	switch k {
	case filo.KBool:
		return "bool"
	case filo.KNumber:
		return "number"
	case filo.KString:
		return "string"
	case filo.KList:
		return "list"
	case filo.KFunc:
		return "function"
	}
	return "unknown"
}
