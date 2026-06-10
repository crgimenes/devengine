package handlers

import (
	"context"
	"fmt"
	"maps"
	"strings"

	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/filo"
)

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
		v, err := runComputedExpr(ctx, user, attr, out)
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
	values db.EAVRecordValues,
) (any, error) {
	globals := make(map[string]filo.Value, len(values))
	for k, v := range values {
		globals["field:"+k] = goToFilo(v)
	}

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
