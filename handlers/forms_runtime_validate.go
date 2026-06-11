package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/filo"
)

// evaluateValidateExprs runs each element's validate_expr against the parsed
// record values. The first failing expression wins: it returns its message as
// userError joined by "; ". All elements are evaluated even if one fails so
// the user sees every issue in a single round trip. UI-only elements (no EAV
// binding) are skipped.
//
// Semantics of a validate_expr's final value:
//   - empty string or true => field is valid
//   - non-empty string     => use that as the user-facing error message
//   - false                => generic "validation failed"
//
// In addition the script may set the `error` global to a non-empty string,
// matching the convention used by pre_save / pos_load.
func evaluateValidateExprs(
	ctx context.Context,
	user *db.User,
	elements []db.FormElement,
	attributes []db.EAVAttribute,
	values db.EAVRecordValues,
) (map[string]string, error) {
	attrByID := make(map[int64]*db.EAVAttribute, len(attributes))
	for i := range attributes {
		attrByID[attributes[i].ID] = &attributes[i]
	}

	fieldErrors := make(map[string]string)
	for _, el := range elements {
		if el.ValidateExpr == "" || el.EAVAttributeID == nil {
			continue
		}
		attr := attrByID[*el.EAVAttributeID]
		if attr == nil {
			continue
		}

		msg, err := runValidateExpr(ctx, user, el, attr, values)
		if err != nil {
			return nil, err
		}
		if msg == "" {
			continue
		}
		fieldErrors[el.MachineName] = msg
	}
	return fieldErrors, nil
}

func runValidateExpr(
	ctx context.Context,
	user *db.User,
	el db.FormElement,
	attr *db.EAVAttribute,
	values db.EAVRecordValues,
) (string, error) {
	globals := make(map[string]filo.Value, len(values)+1)
	for k, v := range values {
		globals["field:"+k] = goToFilo(v)
	}
	globals["error"] = filo.VString("")

	eng := newFiloEngine(user, globals)

	result, newGlobals, execErr := eng.RunScript(ctx, el.ValidateExpr, globals, filoEvalConfig())
	if execErr != nil {
		if strings.Contains(execErr.Error(), "empty script") {
			return "", nil
		}
		return "", fmt.Errorf("validate_expr (%s): %w", attr.MachineName, execErr)
	}

	if e, ok := newGlobals["error"]; ok && e.Kind == filo.KString && e.Str != "" {
		return e.Str, nil
	}

	switch result.Kind {
	case filo.KString:
		return result.Str, nil
	case filo.KBool:
		if result.Bool {
			return "", nil
		}
		return "validation failed", nil
	}
	return "", nil
}
