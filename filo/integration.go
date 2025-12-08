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
