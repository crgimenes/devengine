// Package filolog provides Filo builtins for debug logging and script introspection.
// These builtins wrap the devengine/log package to print debug information to the
// server terminal and inspect global variables during script execution.
//
// Log functions:
//   - log-print: Print all arguments (uses log.Print)
//   - log-printf: Print with format string (uses log.Printf)
//
// Introspection functions:
//   - log-globals: Print all global variable names and values
package filolog

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/crgimenes/devengine/log"
	"github.com/crgimenes/filo"
)

// FiloLogContext holds the globals map for introspection.
type FiloLogContext struct {
	globals map[string]filo.Value
}

// NewContext creates a new FiloLogContext with access to globals.
func NewContext(globals map[string]filo.Value) *FiloLogContext {
	return &FiloLogContext{
		globals: globals,
	}
}

// RegisterLogBuiltins adds log and introspection functions to the engine.
func RegisterLogBuiltins(eng *filo.Engine, ctx *FiloLogContext) {
	eng.MustRegisterBuiltin("log-print", builtinLogPrint)
	eng.MustRegisterBuiltin("log-printf", builtinLogPrintf)
	eng.MustRegisterBuiltin("log-globals", ctx.builtinLogGlobals)
	// Alias for convenience
	eng.MustRegisterBuiltin("print", builtinLogPrint)
}

// builtinLogPrint prints all arguments concatenated.
// Usage: (log-print "Value is:" x)
func builtinLogPrint(ctx context.Context, args []filo.Value) (filo.Value, error) {
	if len(args) == 0 {
		return filo.Value{}, fmt.Errorf("log-print expects at least 1 argument")
	}

	parts := make([]any, len(args))
	for i, arg := range args {
		parts[i] = valueToGo(arg)
	}
	log.Print(parts...)

	return filo.VList(nil), nil
}

// builtinLogPrintf prints with format string.
// Supports %T to show the Filo type of a value.
// Usage: (log-printf "User %s has %d points" name points)
// Usage: (log-printf "Type of x is %T, value is %v" x x)
func builtinLogPrintf(ctx context.Context, args []filo.Value) (filo.Value, error) {
	if len(args) < 1 {
		return filo.Value{}, fmt.Errorf("log-printf expects at least 1 argument (format string)")
	}

	format, err := args[0].AsString()
	if err != nil {
		return filo.Value{}, fmt.Errorf("log-printf: first argument must be string: %w", err)
	}

	// Process format string to handle %T specially
	result := formatWithFiloTypes(format, args[1:])
	log.Print(result)

	return filo.VList(nil), nil
}

// formatWithFiloTypes processes a format string, handling %T for Filo types.
func formatWithFiloTypes(format string, args []filo.Value) string {
	var result strings.Builder
	argIndex := 0

	for i := 0; i < len(format); i++ {
		if format[i] != '%' {
			result.WriteByte(format[i])
			continue
		}

		// Check for %%
		if i+1 < len(format) && format[i+1] == '%' {
			result.WriteString("%%")
			i++
			continue
		}

		// Look for format specifier
		if i+1 < len(format) {
			spec := format[i+1]
			if spec == 'T' {
				// Custom %T: show Filo type
				if argIndex < len(args) {
					result.WriteString(filoTypeName(args[argIndex]))
					argIndex++
				} else {
					result.WriteString("%!T(MISSING)")
				}
				i++
				continue
			}

			// Standard format specifier - find end of specifier
			j := i + 1
			for j < len(format) && !isFormatVerb(format[j]) {
				j++
			}
			if j < len(format) {
				specFull := format[i : j+1]
				if argIndex < len(args) {
					result.WriteString(fmt.Sprintf(specFull, valueToGo(args[argIndex])))
					argIndex++
				} else {
					result.WriteString(specFull)
					result.WriteString("(MISSING)")
				}
				i = j
				continue
			}
		}

		result.WriteByte(format[i])
	}

	return result.String()
}

// builtinLogGlobals prints all global variables and their values.
// Usage: (log-globals)
func (c *FiloLogContext) builtinLogGlobals(ctx context.Context, args []filo.Value) (filo.Value, error) {
	if len(args) != 0 {
		return filo.Value{}, fmt.Errorf("log-globals expects 0 arguments")
	}

	if len(c.globals) == 0 {
		log.Print("[GLOBALS] (empty)")
		return filo.VList(nil), nil
	}

	// Sort keys for deterministic output
	keys := make([]string, 0, len(c.globals))
	for k := range c.globals {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	sb.WriteString("[GLOBALS] ")
	for i, k := range keys {
		if i > 0 {
			sb.WriteString(", ")
		}
		v := c.globals[k]
		sb.WriteString(k)
		sb.WriteString("=")
		sb.WriteString(valueToString(v))
	}
	log.Print(sb.String())

	return filo.VList(nil), nil
}

// valueToGo converts a Filo value to a Go value for log formatting.
func valueToGo(v filo.Value) any {
	switch v.Kind {
	case filo.KBool:
		return v.Bool
	case filo.KNumber:
		if v.Num == float64(int64(v.Num)) {
			return int64(v.Num)
		}
		return v.Num
	case filo.KString:
		return v.Str
	default:
		return v.String()
	}
}

// valueToString converts a Filo value to a readable string.
func valueToString(v filo.Value) string {
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
	default:
		return v.String()
	}
}

// filoTypeName returns the Filo type name for a value.
func filoTypeName(v filo.Value) string {
	switch v.Kind {
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
	default:
		return "unknown"
	}
}

// isFormatVerb returns true if c is a valid format verb character.
func isFormatVerb(c byte) bool {
	switch c {
	case 'v', 's', 'd', 'b', 'o', 'O', 'x', 'X', 'c', 'q', 'U',
		'e', 'E', 'f', 'F', 'g', 'G', 'p', 't':
		return true
	default:
		return false
	}
}
