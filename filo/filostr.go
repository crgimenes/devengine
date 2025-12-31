package filo

import (
	"context"
	"fmt"
	"strings"
)

// RegisterStringBuiltins adds string manipulation functions to the engine.
// These builtins are pure, deterministic functions that do not access
// external resources.
func RegisterStringBuiltins(eng *Engine) {
	eng.RegisterBuiltin("str-join", builtinStrJoin)
	eng.RegisterBuiltin("str-split", builtinStrSplit)
	eng.RegisterBuiltin("str-find", builtinStrFind)
	eng.RegisterBuiltin("str-trim", builtinStrTrim)
	eng.RegisterBuiltin("str-replace", builtinStrReplace)
	eng.RegisterBuiltin("str-upper", builtinStrUpper)
	eng.RegisterBuiltin("str-lower", builtinStrLower)
	eng.RegisterBuiltin("str-concat", builtinStrConcat)
	eng.RegisterBuiltin("str-len", builtinStrLen)
	eng.RegisterBuiltin("str-sub", builtinStrSub)
}

// builtinStrJoin joins a list of strings with a separator.
// Usage: (str-join separator list) -> string
// Example: (str-join ", " (list "a" "b" "c")) -> "a, b, c"
func builtinStrJoin(ctx context.Context, args []Value) (Value, error) {
	if len(args) != 2 {
		return Value{}, fmt.Errorf("str-join expects 2 arguments (separator, list)")
	}
	sep, err := args[0].AsString()
	if err != nil {
		return Value{}, fmt.Errorf("str-join: separator must be string: %w", err)
	}
	list, err := args[1].AsList()
	if err != nil {
		return Value{}, fmt.Errorf("str-join: second argument must be list: %w", err)
	}
	parts := make([]string, len(list))
	for i, v := range list {
		s, convErr := v.AsString()
		if convErr != nil {
			return Value{}, fmt.Errorf("str-join: list element %d must be string: %w", i, convErr)
		}
		parts[i] = s
	}
	return VString(strings.Join(parts, sep)), nil
}

// builtinStrSplit splits a string by a separator into a list.
// Usage: (str-split separator string) -> list
// Example: (str-split ", " "a, b, c") -> (list "a" "b" "c")
func builtinStrSplit(ctx context.Context, args []Value) (Value, error) {
	if len(args) != 2 {
		return Value{}, fmt.Errorf("str-split expects 2 arguments (separator, string)")
	}
	sep, err := args[0].AsString()
	if err != nil {
		return Value{}, fmt.Errorf("str-split: separator must be string: %w", err)
	}
	str, err := args[1].AsString()
	if err != nil {
		return Value{}, fmt.Errorf("str-split: second argument must be string: %w", err)
	}
	parts := strings.Split(str, sep)
	result := make([]Value, len(parts))
	for i, p := range parts {
		result[i] = VString(p)
	}
	return VList(result), nil
}

// builtinStrFind checks if a substring exists in a string.
// Usage: (str-find substring string) -> bool
// Example: (str-find "world" "hello world") -> #t
func builtinStrFind(ctx context.Context, args []Value) (Value, error) {
	if len(args) != 2 {
		return Value{}, fmt.Errorf("str-find expects 2 arguments (substring, string)")
	}
	substr, err := args[0].AsString()
	if err != nil {
		return Value{}, fmt.Errorf("str-find: substring must be string: %w", err)
	}
	str, err := args[1].AsString()
	if err != nil {
		return Value{}, fmt.Errorf("str-find: second argument must be string: %w", err)
	}
	return VBool(strings.Contains(str, substr)), nil
}

// builtinStrTrim removes leading and trailing whitespace from a string.
// Usage: (str-trim string) -> string
// Example: (str-trim "  hello  ") -> "hello"
func builtinStrTrim(ctx context.Context, args []Value) (Value, error) {
	if len(args) != 1 {
		return Value{}, fmt.Errorf("str-trim expects 1 argument (string)")
	}
	str, err := args[0].AsString()
	if err != nil {
		return Value{}, fmt.Errorf("str-trim: argument must be string: %w", err)
	}
	return VString(strings.TrimSpace(str)), nil
}

// builtinStrReplace replaces all occurrences of old with new in a string.
// Usage: (str-replace old new string) -> string
// Example: (str-replace "world" "Filo" "hello world") -> "hello Filo"
func builtinStrReplace(ctx context.Context, args []Value) (Value, error) {
	if len(args) != 3 {
		return Value{}, fmt.Errorf("str-replace expects 3 arguments (old, new, string)")
	}
	old, err := args[0].AsString()
	if err != nil {
		return Value{}, fmt.Errorf("str-replace: old must be string: %w", err)
	}
	newStr, err := args[1].AsString()
	if err != nil {
		return Value{}, fmt.Errorf("str-replace: new must be string: %w", err)
	}
	str, err := args[2].AsString()
	if err != nil {
		return Value{}, fmt.Errorf("str-replace: third argument must be string: %w", err)
	}
	return VString(strings.ReplaceAll(str, old, newStr)), nil
}

// builtinStrUpper converts a string to uppercase.
// Usage: (str-upper string) -> string
// Example: (str-upper "hello") -> "HELLO"
func builtinStrUpper(ctx context.Context, args []Value) (Value, error) {
	if len(args) != 1 {
		return Value{}, fmt.Errorf("str-upper expects 1 argument (string)")
	}
	str, err := args[0].AsString()
	if err != nil {
		return Value{}, fmt.Errorf("str-upper: argument must be string: %w", err)
	}
	return VString(strings.ToUpper(str)), nil
}

// builtinStrLower converts a string to lowercase.
// Usage: (str-lower string) -> string
// Example: (str-lower "HELLO") -> "hello"
func builtinStrLower(ctx context.Context, args []Value) (Value, error) {
	if len(args) != 1 {
		return Value{}, fmt.Errorf("str-lower expects 1 argument (string)")
	}
	str, err := args[0].AsString()
	if err != nil {
		return Value{}, fmt.Errorf("str-lower: argument must be string: %w", err)
	}
	return VString(strings.ToLower(str)), nil
}

// builtinStrConcat concatenates multiple strings.
// Usage: (str-concat strings...) -> string
// Example: (str-concat "hello" " " "world") -> "hello world"
func builtinStrConcat(ctx context.Context, args []Value) (Value, error) {
	var b strings.Builder
	for i, a := range args {
		s, err := a.AsString()
		if err != nil {
			return Value{}, fmt.Errorf("str-concat: argument %d must be string: %w", i, err)
		}
		b.WriteString(s)
	}
	return VString(b.String()), nil
}

// builtinStrLen returns the length of a string in bytes.
// Usage: (str-len string) -> number
// Example: (str-len "hello") -> 5
func builtinStrLen(ctx context.Context, args []Value) (Value, error) {
	if len(args) != 1 {
		return Value{}, fmt.Errorf("str-len expects 1 argument (string)")
	}
	str, err := args[0].AsString()
	if err != nil {
		return Value{}, fmt.Errorf("str-len: argument must be string: %w", err)
	}
	return VNum(float64(len(str))), nil
}

// builtinStrSub extracts a substring from a string.
// Usage: (str-sub start end string) -> string
// Indices are 0-based. End is exclusive.
// Example: (str-sub 0 5 "hello world") -> "hello"
func builtinStrSub(ctx context.Context, args []Value) (Value, error) {
	if len(args) != 3 {
		return Value{}, fmt.Errorf("str-sub expects 3 arguments (start, end, string)")
	}
	startF, err := args[0].AsNumber()
	if err != nil {
		return Value{}, fmt.Errorf("str-sub: start must be number: %w", err)
	}
	endF, err := args[1].AsNumber()
	if err != nil {
		return Value{}, fmt.Errorf("str-sub: end must be number: %w", err)
	}
	str, err := args[2].AsString()
	if err != nil {
		return Value{}, fmt.Errorf("str-sub: third argument must be string: %w", err)
	}
	start := int(startF)
	end := int(endF)
	if start < 0 {
		start = 0
	}
	if end > len(str) {
		end = len(str)
	}
	if start > end {
		return VString(""), nil
	}
	return VString(str[start:end]), nil
}
