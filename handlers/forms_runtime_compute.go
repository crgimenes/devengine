package handlers

import (
	"context"
	"fmt"
	"maps"
	"strings"

	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/filo"
)

// fieldGlobals builds the field:<name> Filo globals for a script run. Every
// attribute is present: parsed values verbatim, absent ones as the zero of
// their primitive kind. Without the zero fill a partial payload (the JSON API
// allows omitting fields; HTML forms always post them) leaves the symbol as a
// string and numeric scripts like (< field:nota 0) abort with a type error.
func fieldGlobals(attributes []db.EAVAttribute, values db.EAVRecordValues) map[string]filo.Value {
	globals := make(map[string]filo.Value, len(attributes)+1)
	for i := range attributes {
		attr := &attributes[i]
		v, ok := values[attr.MachineName]
		if !ok || v == nil {
			switch attr.PrimitiveKind {
			case "BOOL":
				v = false
			case "INT":
				v = int64(0)
			case "REAL":
				v = float64(0)
			default: // TEXT, DATETIME
				v = ""
			}
		}
		globals["field:"+attr.MachineName] = goToFilo(v)
	}
	return globals
}

// applyComputedExprs replaces each is_computed attribute's value with the
// result of evaluating its computed_expr. Earlier computed fields are visible
// to later ones — order follows the attributes slice given by the caller. The
// returned map is a fresh copy; the input is left untouched.
func applyComputedExprs(
	ctx context.Context,
	user *db.User,
	attributes []db.EAVAttribute,
	values db.EAVRecordValues,
) (db.EAVRecordValues, error) {
	out := make(db.EAVRecordValues, len(values))
	maps.Copy(out, values)

	for i := range attributes {
		attr := &attributes[i]
		if !attr.IsComputed || strings.TrimSpace(attr.ComputedExpr) == "" {
			continue
		}
		v, err := runComputedExpr(ctx, user, attr, attributes, out)
		if err != nil {
			return nil, err
		}
		out[attr.MachineName] = v
	}
	return out, nil
}

func runComputedExpr(
	ctx context.Context,
	user *db.User,
	attr *db.EAVAttribute,
	attributes []db.EAVAttribute,
	values db.EAVRecordValues,
) (any, error) {
	globals := fieldGlobals(attributes, values)

	eng := newFiloEngine(user, globals)

	result, _, execErr := eng.RunScript(ctx, attr.ComputedExpr, globals, filoEvalConfig())
	if execErr != nil {
		if strings.Contains(execErr.Error(), "empty script") {
			return values[attr.MachineName], nil
		}
		return nil, fmt.Errorf("computed_expr (%s): %w", attr.MachineName, execErr)
	}

	return filoToGoTyped(attr.PrimitiveKind, result), nil
}
