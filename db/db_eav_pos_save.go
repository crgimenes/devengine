package db

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/crgimenes/filo"
)

// EAVRecordValues holds typed values keyed by attribute machine_name.
// Values are typed as interface{} and must be one of: bool, int64, float64, string, nil.
type EAVRecordValues map[string]interface{}

// filoDefaultConfig returns safe default limits for Filo script execution.
func filoDefaultConfig() filo.EvalConfig {
	return filo.EvalConfig{
		StepLimit:      10000,
		RecursionLimit: 100,
		Timeout:        5 * time.Second,
	}
}

// ExecutePosSaveScript runs the entity type's pos_save Filo script.
// This is a core EAV function that must be called before any record is saved,
// regardless of the caller (sysop tools, forms, APIs, etc.).
//
// Parameters:
//   - entityType: The entity type containing the pos_save script
//   - values: Current field values keyed by attribute machine_name
//
// Returns:
//   - modifiedValues: Values after script execution (may be modified by script)
//   - userError: User-facing error message if script set "error" variable
//   - err: System error if script execution failed
func ExecutePosSaveScript(
	entityType *EAVEntityType,
	values EAVRecordValues,
) (modifiedValues EAVRecordValues, userError string, err error) {
	// If no script, return original values unchanged
	if entityType.PosSave == "" {
		return values, "", nil
	}

	// Create Filo engine and register string builtins
	eng := filo.NewEngine()
	filo.RegisterStringBuiltins(eng)

	// Build globals map from values
	globals := make(map[string]filo.Value)
	for k, v := range values {
		globals[k] = goValueToFiloValue(v)
	}

	// Inject empty error variable
	globals["error"] = filo.VString("")

	// Execute script
	ctx := context.Background()
	_, newGlobals, execErr := eng.RunScript(ctx, entityType.PosSave, globals, filoDefaultConfig())
	if execErr != nil {
		// Handle comment-only scripts gracefully - they result in "empty script" error
		if strings.Contains(execErr.Error(), "empty script") {
			return values, "", nil
		}
		return nil, "", fmt.Errorf("pos_save script execution failed: %w", execErr)
	}

	// Check if error variable was set
	if errVal, ok := newGlobals["error"]; ok {
		if errVal.Kind == filo.KString && errVal.Str != "" {
			return nil, errVal.Str, nil
		}
	}

	// Extract modified values from globals
	modifiedValues = make(EAVRecordValues)
	for k := range values {
		if newVal, ok := newGlobals[k]; ok {
			modifiedValues[k] = filoValueToGoValue(newVal)
		} else {
			// Keep original value if not in returned globals
			modifiedValues[k] = values[k]
		}
	}

	return modifiedValues, "", nil
}

// goValueToFiloValue converts a Go value to a Filo Value.
func goValueToFiloValue(v interface{}) filo.Value {
	if v == nil {
		// Filo doesn't have a nil type, use empty string
		return filo.VString("")
	}
	switch val := v.(type) {
	case bool:
		return filo.VBool(val)
	case int64:
		return filo.VNum(float64(val))
	case float64:
		return filo.VNum(val)
	case string:
		return filo.VString(val)
	default:
		// Try to convert to string representation
		return filo.VString(fmt.Sprintf("%v", val))
	}
}

// filoValueToGoValue converts a Filo Value back to a Go value.
func filoValueToGoValue(v filo.Value) interface{} {
	switch v.Kind {
	case filo.KBool:
		return v.Bool
	case filo.KNumber:
		// Check if it's an integer
		if v.Num == float64(int64(v.Num)) {
			return int64(v.Num)
		}
		return v.Num
	case filo.KString:
		return v.Str
	default:
		// For lists and other types, convert to string
		return v.String()
	}
}
