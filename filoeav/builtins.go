package filoeav

import (
	"context"
	"fmt"
	"time"

	"github.com/crgimenes/devengine/filo"
)

// makeGetValueBuiltin creates the eav-get-value builtin.
// Usage: (eav-get-value "form-slug" record-id "field-machine-name")
// Returns the field value as appropriate Filo type, or #f if not found.
func makeGetValueBuiltin(store EAVStore, cfg Config) filo.Builtin {
	return func(ctx context.Context, args []filo.Value) (filo.Value, error) {
		if len(args) != 3 {
			return filo.Value{}, fmt.Errorf("eav-get-value expects 3 arguments: form-slug, record-id, field-machine-name")
		}

		formSlug, err := args[0].AsString()
		if err != nil {
			return filo.Value{}, fmt.Errorf("eav-get-value: form-slug must be string: %w", err)
		}

		recordIDFloat, err := args[1].AsNumber()
		if err != nil {
			return filo.Value{}, fmt.Errorf("eav-get-value: record-id must be number: %w", err)
		}
		recordID := int64(recordIDFloat)

		fieldName, err := args[2].AsString()
		if err != nil {
			return filo.Value{}, fmt.Errorf("eav-get-value: field-machine-name must be string: %w", err)
		}

		val, err := store.GetFieldValue(ctx, cfg.WorkspaceID, formSlug, fieldName, recordID)
		if err != nil {
			return filo.Value{}, fmt.Errorf("eav-get-value: %w", err)
		}

		return goToFiloValue(val), nil
	}
}

// makeFindOneBuiltin creates the eav-find-one builtin.
// Usage: (eav-find-one "form-slug" "field-machine-name" "=" value)
// Returns the record ID as number, or #f if not found.
func makeFindOneBuiltin(store EAVStore, cfg Config) filo.Builtin {
	return func(ctx context.Context, args []filo.Value) (filo.Value, error) {
		if len(args) != 4 {
			return filo.Value{}, fmt.Errorf("eav-find-one expects 4 arguments: form-slug, field-machine-name, operator, value")
		}

		formSlug, err := args[0].AsString()
		if err != nil {
			return filo.Value{}, fmt.Errorf("eav-find-one: form-slug must be string: %w", err)
		}

		fieldName, err := args[1].AsString()
		if err != nil {
			return filo.Value{}, fmt.Errorf("eav-find-one: field-machine-name must be string: %w", err)
		}

		operator, err := args[2].AsString()
		if err != nil {
			return filo.Value{}, fmt.Errorf("eav-find-one: operator must be string: %w", err)
		}

		if !isValidOperator(operator) {
			return filo.Value{}, fmt.Errorf("eav-find-one: invalid operator %q, must be one of: =, !=, <, <=, >, >=", operator)
		}

		goVal := filoToGoValue(args[3])

		recordID, err := store.FindRecordByField(ctx, cfg.WorkspaceID, formSlug, fieldName, operator, goVal)
		if err != nil {
			return filo.Value{}, fmt.Errorf("eav-find-one: %w", err)
		}

		if recordID == 0 {
			return filo.VBool(false), nil
		}
		return filo.VNum(float64(recordID)), nil
	}
}

// makeAggregateBuiltin creates an aggregation builtin (sum, count, min, max, avg).
// Usage: (eav-sum "form-slug" "field-machine-name" filters)
// filters is a list of (values field op value) tuples, or empty list for no filter.
func makeAggregateBuiltin(store EAVStore, cfg Config, aggFunc string) filo.Builtin {
	return func(ctx context.Context, args []filo.Value) (filo.Value, error) {
		// For COUNT, field-machine-name is optional (can pass empty string)
		minArgs := 2
		if len(args) < minArgs || len(args) > 3 {
			return filo.Value{}, fmt.Errorf("eav-%s expects 2-3 arguments: form-slug, field-machine-name, [filters]", aggFunc)
		}

		formSlug, err := args[0].AsString()
		if err != nil {
			return filo.Value{}, fmt.Errorf("eav-%s: form-slug must be string: %w", aggFunc, err)
		}

		fieldName, err := args[1].AsString()
		if err != nil {
			return filo.Value{}, fmt.Errorf("eav-%s: field-machine-name must be string: %w", aggFunc, err)
		}

		var filters []FieldFilter
		if len(args) == 3 {
			filters, err = parseFilters(args[2])
			if err != nil {
				return filo.Value{}, fmt.Errorf("eav-%s: %w", aggFunc, err)
			}
		}

		result, err := store.AggregateField(ctx, cfg.WorkspaceID, formSlug, fieldName, aggFunc, filters)
		if err != nil {
			return filo.Value{}, fmt.Errorf("eav-%s: %w", aggFunc, err)
		}

		return filo.VNum(result), nil
	}
}

// parseFilters converts a Filo list of filter tuples to Go FieldFilter slice.
// Each filter should be (values field-name operator value).
func parseFilters(arg filo.Value) ([]FieldFilter, error) {
	list, err := arg.AsList()
	if err != nil {
		return nil, fmt.Errorf("filters must be a list: %w", err)
	}

	filters := make([]FieldFilter, 0, len(list))
	for i, item := range list {
		tup, err := item.AsTuple()
		if err != nil {
			return nil, fmt.Errorf("filter[%d] must be a tuple (values field op value): %w", i, err)
		}

		if len(tup) != 3 {
			return nil, fmt.Errorf("filter[%d] must have 3 elements (field op value), got %d", i, len(tup))
		}

		fieldName, err := tup[0].AsString()
		if err != nil {
			return nil, fmt.Errorf("filter[%d] field name must be string: %w", i, err)
		}

		operator, err := tup[1].AsString()
		if err != nil {
			return nil, fmt.Errorf("filter[%d] operator must be string: %w", i, err)
		}

		if !isValidOperator(operator) {
			return nil, fmt.Errorf("filter[%d] invalid operator %q", i, operator)
		}

		filters = append(filters, FieldFilter{
			FieldMachineName: fieldName,
			Operator:         operator,
			Value:            filoToGoValue(tup[2]),
		})
	}

	return filters, nil
}

// isValidOperator checks if the operator is allowed.
func isValidOperator(op string) bool {
	switch op {
	case "=", "!=", "<", "<=", ">", ">=":
		return true
	default:
		return false
	}
}

// goToFiloValue converts a Go value to a Filo Value.
func goToFiloValue(v any) filo.Value {
	if v == nil {
		return filo.VBool(false)
	}

	switch val := v.(type) {
	case int64:
		return filo.VNum(float64(val))
	case float64:
		return filo.VNum(val)
	case string:
		return filo.VString(val)
	case bool:
		return filo.VBool(val)
	case time.Time:
		// Return datetime as RFC3339 string
		return filo.VString(val.Format(time.RFC3339))
	default:
		// Fallback: convert to string representation
		return filo.VString(fmt.Sprintf("%v", val))
	}
}

// filoToGoValue converts a Filo Value to a Go value for database queries.
func filoToGoValue(v filo.Value) any {
	switch v.Kind {
	case filo.KNumber:
		return v.Num
	case filo.KString:
		return v.Str
	case filo.KBool:
		return v.Bool
	default:
		return nil
	}
}
