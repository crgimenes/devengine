package filo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/crgimenes/devengine/log"
)

type Filo struct {
	eng     *Engine
	globals map[string]Value
}

var (
	ErrorFunctionNotFound = errors.New("function not found")
	ErrorNotAllowedType   = errors.New("not allowed return type")
)

// New creates a new Filo instance for configuration loading.
func New() *Filo {
	return &Filo{
		eng:     NewEngine(),
		globals: make(map[string]Value),
	}
}

// Close is a no-op for Filo but provided for API compatibility.
func (f *Filo) Close() {
	// No cleanup needed for Filo
}

// RegisterBuiltin registers a custom builtin function in the Filo engine.
func (f *Filo) RegisterBuiltin(name string, fn Builtin) {
	f.eng.RegisterBuiltin(name, fn)
}

// GetEngine returns the underlying Filo engine for advanced operations.
func (f *Filo) GetEngine() *Engine {
	return f.eng
}

// SetGlobal sets a global variable that will be available in the Filo script.
func (f *Filo) SetGlobal(name string, value any) {
	switch v := value.(type) {
	case string:
		f.globals[name] = VString(v)
	case int:
		f.globals[name] = VNum(float64(v))
	case int64:
		f.globals[name] = VNum(float64(v))
	case float32:
		f.globals[name] = VNum(float64(v))
	case float64:
		f.globals[name] = VNum(v)
	case bool:
		f.globals[name] = VBool(v)
	case []string:
		vals := make([]Value, len(v))
		for i, s := range v {
			vals[i] = VString(s)
		}
		f.globals[name] = VList(vals)
	case map[string]string:
		// Convert map to list of key-value tuples for Filo
		pairs := make([]Value, 0, len(v))
		for k, val := range v {
			pairs = append(pairs, VTuple([]Value{VString(k), VString(val)}))
		}
		f.globals[name] = VList(pairs)
	default:
		f.globals[name] = VString(fmt.Sprintf("%v", v))
	}
}

// DoString executes a Filo script and updates globals with any (set ...) statements.
func (f *Filo) DoString(filoScript string) error {
	ctx := context.Background()
	cfg := EvalConfig{
		StepLimit:      10000,
		RecursionLimit: 64,
		Timeout:        5 * time.Second,
	}

	_, updatedGlobals, err := f.eng.RunScript(ctx, filoScript, f.globals, cfg)
	if err != nil {
		return err
	}

	// Update globals with any values set during script execution
	f.globals = updatedGlobals
	return nil
}

// MustGetString retrieves a global variable as a string or fatals.
func (f *Filo) MustGetString(vGlobal string) string {
	v, ok := f.globals[vGlobal]
	if !ok {
		log.Fatalf("Global variable %q not found", vGlobal)
	}
	s, err := v.AsString()
	if err != nil {
		log.Fatalf("Error converting %q to string: %v", vGlobal, err)
	}
	return s
}

// MustGetInt retrieves a global variable as an int or fatals.
func (f *Filo) MustGetInt(vGlobal string) int {
	v, ok := f.globals[vGlobal]
	if !ok {
		log.Fatalf("Global variable %q not found", vGlobal)
	}
	n, err := v.AsNumber()
	if err != nil {
		log.Fatalf("Error converting %q to int: %v", vGlobal, err)
	}
	return int(n)
}

// MustGetBool retrieves a global variable as a bool or fatals.
func (f *Filo) MustGetBool(vGlobal string) bool {
	v, ok := f.globals[vGlobal]
	if !ok {
		log.Fatalf("Global variable %q not found", vGlobal)
	}
	b, err := v.AsBool()
	if err != nil {
		log.Fatalf("Error converting %q to bool: %v", vGlobal, err)
	}
	return b
}

// MustGetTable retrieves a global variable as a []string or fatals.
func (f *Filo) MustGetTable(vGlobal string) []string {
	v, ok := f.globals[vGlobal]
	if !ok {
		log.Fatalf("Global variable %q not found", vGlobal)
	}
	list, err := v.AsList()
	if err != nil {
		log.Fatalf("Error converting %q to list: %v", vGlobal, err)
	}
	ret := make([]string, len(list))
	for i, item := range list {
		s, convErr := item.AsString()
		if convErr != nil {
			log.Fatalf("Error converting list item %d to string: %v", i, convErr)
		}
		ret[i] = s
	}
	return ret
}

// MustGetMap retrieves a global variable as a map[string]string or fatals.
func (f *Filo) MustGetMap(vGlobal string) map[string]string {
	v, ok := f.globals[vGlobal]
	if !ok {
		log.Fatalf("Global variable %q not found", vGlobal)
	}
	list, err := v.AsList()
	if err != nil {
		log.Fatalf("Error converting %q to list: %v", vGlobal, err)
	}
	ret := make(map[string]string)
	for i, item := range list {
		tuple, tupleErr := item.AsTuple()
		if tupleErr != nil {
			log.Fatalf("Error converting map item %d to tuple: %v", i, tupleErr)
		}
		if len(tuple) != 2 {
			log.Fatalf("Map tuple item %d must have exactly 2 elements", i)
		}
		key, keyErr := tuple[0].AsString()
		if keyErr != nil {
			log.Fatalf("Error converting map key %d to string: %v", i, keyErr)
		}
		val, valErr := tuple[1].AsString()
		if valErr != nil {
			log.Fatalf("Error converting map value %d to string: %v", i, valErr)
		}
		ret[key] = val
	}
	return ret
}

// SetGlobalMapOfLists sets a global variable from a map[string][]string.
// The structure is stored as a list of (key, list-of-values) tuples.
// Example: {"a": ["x", "y"]} becomes (list (list "a" (list "x" "y")))
func (f *Filo) SetGlobalMapOfLists(name string, m map[string][]string) {
	pairs := make([]Value, 0, len(m))
	for k, vals := range m {
		valList := make([]Value, len(vals))
		for i, v := range vals {
			valList[i] = VString(v)
		}
		pairs = append(pairs, VList([]Value{VString(k), VList(valList)}))
	}
	f.globals[name] = VList(pairs)
}

// MustGetMapOfLists retrieves a global variable as a map[string][]string or fatals.
// Expects the structure: (list (list "key" (list "val1" "val2")) ...)
func (f *Filo) MustGetMapOfLists(vGlobal string) map[string][]string {
	v, ok := f.globals[vGlobal]
	if !ok {
		log.Fatalf("Global variable %q not found", vGlobal)
	}
	list, err := v.AsList()
	if err != nil {
		log.Fatalf("Error converting %q to list: %v", vGlobal, err)
	}
	ret := make(map[string][]string)
	for i, item := range list {
		tuple, tupleErr := item.AsList()
		if tupleErr != nil {
			log.Fatalf("Error converting map item %d to list: %v", i, tupleErr)
		}
		if len(tuple) != 2 {
			log.Fatalf("Map list item %d must have exactly 2 elements", i)
		}
		key, keyErr := tuple[0].AsString()
		if keyErr != nil {
			log.Fatalf("Error converting map key %d to string: %v", i, keyErr)
		}
		valList, valErr := tuple[1].AsList()
		if valErr != nil {
			log.Fatalf("Error converting map value %d to list: %v", i, valErr)
		}
		vals := make([]string, len(valList))
		for j, val := range valList {
			s, sErr := val.AsString()
			if sErr != nil {
				log.Fatalf("Error converting value list item %d.%d to string: %v", i, j, sErr)
			}
			vals[j] = s
		}
		ret[key] = vals
	}
	return ret
}

// HasFunction checks if a function with the given name is defined in globals.
func (f *Filo) HasFunction(name string) bool {
	v, ok := f.globals[name]
	if !ok {
		return false
	}
	return v.Kind == KFunc
}

// CallFunction invokes a Filo-defined function by name with the given arguments.
// Arguments are converted from Go types to Filo Values.
// Returns the result Value or an error if the function doesn't exist or fails.
func (f *Filo) CallFunction(name string, args ...any) (Value, error) {
	fnVal, ok := f.globals[name]
	if !ok {
		return Value{}, fmt.Errorf("%w: %s", ErrorFunctionNotFound, name)
	}
	if fnVal.Kind != KFunc {
		return Value{}, fmt.Errorf("%s is not a function", name)
	}

	// Convert Go args to Filo Values
	filoArgs := make([]Value, len(args))
	for i, arg := range args {
		switch v := arg.(type) {
		case string:
			filoArgs[i] = VString(v)
		case int:
			filoArgs[i] = VNum(float64(v))
		case int64:
			filoArgs[i] = VNum(float64(v))
		case float64:
			filoArgs[i] = VNum(v)
		case bool:
			filoArgs[i] = VBool(v)
		case []byte:
			filoArgs[i] = VString(string(v))
		case Value:
			filoArgs[i] = v
		default:
			filoArgs[i] = VString(fmt.Sprintf("%v", v))
		}
	}

	// Build call expression
	var callExpr string
	callExpr = "(" + name
	for i := range filoArgs {
		callExpr += fmt.Sprintf(" arg%d", i)
	}
	callExpr += ")"

	// Set up globals with args
	callGlobals := make(map[string]Value, len(f.globals)+len(filoArgs))
	for k, v := range f.globals {
		callGlobals[k] = v
	}
	for i, v := range filoArgs {
		callGlobals[fmt.Sprintf("arg%d", i)] = v
	}

	ctx := context.Background()
	cfg := EvalConfig{
		StepLimit:      10000,
		RecursionLimit: 64,
		Timeout:        5 * time.Second,
	}

	result, _, err := f.eng.RunScript(ctx, callExpr, callGlobals, cfg)
	if err != nil {
		return Value{}, fmt.Errorf("error calling %s: %w", name, err)
	}

	return result, nil
}

// CallFunctionString is a convenience wrapper that calls a function and converts
// the result to a string. If the function doesn't exist or returns a non-string,
// the fallback string is returned.
func (f *Filo) CallFunctionString(name string, fallback string, args ...any) string {
	if !f.HasFunction(name) {
		return fallback
	}
	result, err := f.CallFunction(name, args...)
	if err != nil {
		return fallback
	}
	s, err := result.AsString()
	if err != nil {
		return fallback
	}
	return s
}
