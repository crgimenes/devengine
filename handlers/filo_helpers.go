package handlers

import (
	"fmt"
	"time"

	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/devengine/filoeav"
	"github.com/crgimenes/devengine/filofile"
	"github.com/crgimenes/devengine/filolog"
	"github.com/crgimenes/devengine/filosession"
	"github.com/crgimenes/filo"
	"github.com/crgimenes/filo/filostrings"
)

// filoEvalConfig is the safe-default execution envelope for any Filo script
// run by the forms runtime.
func filoEvalConfig() filo.EvalConfig {
	return filo.EvalConfig{
		StepLimit:      10000,
		RecursionLimit: 100,
		Timeout:        5 * time.Second,
	}
}

// newFiloEngine builds a Filo engine wired with every builtin the forms
// runtime exposes: strings, log (with the supplied globals), session
// (current user), and the read-only EAV + filemanager helpers. globals must
// be the same map handed to RunScript so log-globals reflects script state.
func newFiloEngine(user *db.User, globals map[string]filo.Value) *filo.Engine {
	eng := filo.NewEngine()
	filostrings.RegisterBuiltins(eng)
	filoeav.RegisterEAVBuiltins(eng, filoeav.NewContext(db.Storage))
	filofile.RegisterFileBuiltins(eng, filofile.NewContext(db.Storage))
	filosession.RegisterSessionBuiltins(eng, filosession.NewContext(user))
	filolog.RegisterLogBuiltins(eng, filolog.NewContext(globals))
	return eng
}

// goToFilo converts a Go value (as parsed by a field plugin) into the Filo
// value used as a script global. Unknown types are stringified rather than
// rejected so scripts never see a nil global.
func goToFilo(v any) filo.Value {
	switch x := v.(type) {
	case nil:
		return filo.VString("")
	case bool:
		return filo.VBool(x)
	case int64:
		return filo.VNum(float64(x))
	case float64:
		return filo.VNum(x)
	case string:
		return filo.VString(x)
	}
	return filo.VString(fmt.Sprintf("%v", v))
}

// filoToGoTyped coerces a Filo value into the Go type that matches the EAV
// primitive_kind. When the Filo value's kind doesn't fit, the function falls
// back to the type-appropriate zero so the save path never rejects the row.
func filoToGoTyped(primitive string, v filo.Value) any {
	switch primitive {
	case "BOOL":
		if v.Kind == filo.KBool {
			return v.Bool
		}
		if v.Kind == filo.KNumber {
			return v.Num != 0
		}
		return false
	case "INT":
		if v.Kind == filo.KNumber {
			return int64(v.Num)
		}
		return int64(0)
	case "REAL":
		if v.Kind == filo.KNumber {
			return v.Num
		}
		return float64(0)
	case "TEXT", "DATETIME":
		if v.Kind == filo.KString {
			return v.Str
		}
		return ""
	}
	return ""
}
