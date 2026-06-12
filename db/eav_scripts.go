package db

import (
	"context"
	"fmt"
	"maps"
	"strings"
	"time"

	"github.com/crgimenes/devengine/filolog"
	"github.com/crgimenes/filo"
	"github.com/crgimenes/filo/filostrings"
)

// EAVRecordValues holds typed values keyed by attribute machine_name.
// Values are typed as interface{} and must be one of: bool, int64, float64, string, nil.
type EAVRecordValues map[string]any

// ScriptEngineSetupFunc is a function that configures a Filo engine before execution.
// This allows callers to register additional builtins (like DB access) without
// creating circular dependencies.
type ScriptEngineSetupFunc func(*filo.Engine)

// DefaultScriptSetup is the default setup that only registers string builtins.
var DefaultScriptSetup ScriptEngineSetupFunc = func(eng *filo.Engine) {
	filostrings.RegisterBuiltins(eng)
}

// CurrentScriptSetup is the active setup function. Set this at application startup
// to register additional builtins like DB access.
var CurrentScriptSetup ScriptEngineSetupFunc = DefaultScriptSetup

// filoDefaultConfig returns safe default limits for Filo script execution.
func filoDefaultConfig() filo.EvalConfig {
	return filo.EvalConfig{
		StepLimit:      10000,
		RecursionLimit: 100,
		Timeout:        5 * time.Second,
	}
}

// ExecutePreSaveScript runs the entity type's pre_save Filo script.
// This is a core EAV function that must be called before any record is saved,
// regardless of the caller (sysop tools, forms, APIs, etc.).
//
// Parameters:
//   - entityType: The entity type containing the pre_save script
//   - values: Current field values keyed by attribute machine_name
//
// Returns:
//   - modifiedValues: Values after script execution (may be modified by script)
//   - userError: User-facing error message if script set "error" variable
//   - err: System error if script execution failed
func ExecutePreSaveScript(
	entityType *EAVEntityType,
	values EAVRecordValues,
) (modifiedValues EAVRecordValues, userError string, err error) {
	// If no script, return original values unchanged
	if entityType.PreSave == "" {
		return values, "", nil
	}

	return executeFiloScript(entityType.PreSave, values, "pre_save", CurrentScriptSetup)
}

// ExecutePreSaveScriptWithSetup runs the pre_save script with a custom engine setup.
// This allows the caller to inject custom builtins (like DB access with a transaction).
//
// Parameters:
//   - entityType: The entity type containing the pre_save script
//   - values: Current field values keyed by attribute machine_name
//   - setup: Custom function to configure the Filo engine (register builtins)
//
// Returns:
//   - modifiedValues: Values after script execution (may be modified by script)
//   - userError: User-facing error message if script set "error" variable
//   - err: System error if script execution failed
func ExecutePreSaveScriptWithSetup(
	entityType *EAVEntityType,
	values EAVRecordValues,
	setup ScriptEngineSetupFunc,
) (modifiedValues EAVRecordValues, userError string, err error) {
	// If no script, return original values unchanged
	if entityType.PreSave == "" {
		return values, "", nil
	}

	return executeFiloScript(entityType.PreSave, values, "pre_save", setup)
}

// ExecutePosLoadScript runs the entity type's pos_load Filo script.
// This is called after loading record data and applying defaults, but before display.
// It is the last transformation step before the user sees the data.
//
// Parameters:
//   - entityType: The entity type containing the pos_load script
//   - values: Current field values keyed by attribute machine_name (after defaults applied)
//
// Returns:
//   - modifiedValues: Values after script execution (may be modified by script)
//   - userError: User-facing error message if script set "error" variable
//   - err: System error if script execution failed
func ExecutePosLoadScript(
	entityType *EAVEntityType,
	values EAVRecordValues,
) (modifiedValues EAVRecordValues, userError string, err error) {
	// If no script, return original values unchanged
	if entityType.PosLoad == "" {
		return values, "", nil
	}

	return executeFiloScript(entityType.PosLoad, values, "pos_load", CurrentScriptSetup)
}

// executeFiloScript is the shared implementation for pre_save and pos_load scripts.
func executeFiloScript(script string, values EAVRecordValues, scriptName string, setup ScriptEngineSetupFunc) (EAVRecordValues, string, error) {
	// Create Filo engine and apply provided setup
	eng := filo.NewEngine()
	if setup != nil {
		setup(eng)
	}

	// Build globals map from values
	globals := make(map[string]filo.Value)
	for k, v := range values {
		globals["field:"+k] = goValueToFiloValue(v)
	}

	// Inject empty error variable
	globals["error"] = filo.VString("")

	// Register log builtins (always available for debugging)
	filolog.RegisterLogBuiltins(eng, filolog.NewContext(globals))

	// Execute script
	ctx := context.Background()
	_, newGlobals, execErr := eng.RunScript(ctx, script, globals, filoDefaultConfig())
	if execErr != nil {
		// Handle comment-only scripts gracefully - they result in "empty script" error
		if strings.Contains(execErr.Error(), "empty script") {
			return values, "", nil
		}
		return nil, "", fmt.Errorf("%s script execution failed: %w", scriptName, execErr)
	}

	// Check if error variable was set
	if errVal, ok := newGlobals["error"]; ok {
		if errVal.Kind == filo.KString && errVal.Str != "" {
			return nil, errVal.Str, nil
		}
	}

	// Extract modified values from globals
	modifiedValues := make(EAVRecordValues)

	// First, copy all original values
	maps.Copy(modifiedValues, values)

	// Then, apply any modifications from the script (including new variables)
	for kRaw, newVal := range newGlobals {
		// Skip the built-in "error" variable
		if kRaw == "error" {
			continue
		}

		// Handle field: prefix
		k := kRaw
		if after, ok := strings.CutPrefix(k, "field:"); ok {
			k = after
		} else {
			// For EAV scripts (pre_save/pos_load), we ONLY accept field: prefixed variables
			// to modify record values. This prevents accidental pollution.
			continue
		}

		modifiedValues[k] = filoValueToGoValue(newVal)
	}

	return modifiedValues, "", nil
}

// goValueToFiloValue converts a Go value to a Filo Value.
func goValueToFiloValue(v any) filo.Value {
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
func filoValueToGoValue(v filo.Value) any {
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
