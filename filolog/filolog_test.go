package filolog

import (
	"context"
	"testing"

	"github.com/crgimenes/filo"
)

func TestBuiltinLogPrint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    []filo.Value
		wantErr bool
	}{
		{
			name:    "single string",
			args:    []filo.Value{filo.VString("hello")},
			wantErr: false,
		},
		{
			name:    "multiple args",
			args:    []filo.Value{filo.VString("value:"), filo.VNum(42)},
			wantErr: false,
		},
		{
			name:    "no args - error",
			args:    []filo.Value{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := builtinLogPrint(context.Background(), tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("builtinLogPrint() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBuiltinLogPrintf(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    []filo.Value
		wantErr bool
	}{
		{
			name:    "format with args",
			args:    []filo.Value{filo.VString("User %s has %d points"), filo.VString("Alice"), filo.VNum(100)},
			wantErr: false,
		},
		{
			name:    "format only",
			args:    []filo.Value{filo.VString("Simple message")},
			wantErr: false,
		},
		{
			name:    "no args - error",
			args:    []filo.Value{},
			wantErr: true,
		},
		{
			name:    "non-string format - error",
			args:    []filo.Value{filo.VNum(42)},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := builtinLogPrintf(context.Background(), tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("builtinLogPrintf() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBuiltinLogGlobals(t *testing.T) {
	t.Parallel()

	// Test with empty globals
	ctx := NewContext(nil)
	_, err := ctx.builtinLogGlobals(context.Background(), []filo.Value{})
	if err != nil {
		t.Errorf("builtinLogGlobals() with nil globals unexpected error = %v", err)
	}

	// Test with populated globals
	globals := map[string]filo.Value{
		"field:name":  filo.VString("Alice"),
		"field:age":   filo.VNum(25),
		"field:admin": filo.VBool(true),
	}
	ctx = NewContext(globals)
	_, err = ctx.builtinLogGlobals(context.Background(), []filo.Value{})
	if err != nil {
		t.Errorf("builtinLogGlobals() with globals unexpected error = %v", err)
	}

	// Test error on args
	_, err = ctx.builtinLogGlobals(context.Background(), []filo.Value{filo.VString("extra")})
	if err == nil {
		t.Error("builtinLogGlobals() expected error for extra args")
	}
}

func TestValueToString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		val  filo.Value
		want string
	}{
		{"bool true", filo.VBool(true), "true"},
		{"bool false", filo.VBool(false), "false"},
		{"integer", filo.VNum(42), "42"},
		{"float", filo.VNum(3.14), "3.14"},
		{"string", filo.VString("hello"), "hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := valueToString(tt.val)
			if got != tt.want {
				t.Errorf("valueToString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRegisterLogBuiltins(t *testing.T) {
	t.Parallel()

	eng := filo.NewEngine()
	ctx := NewContext(nil)

	// Should not panic
	RegisterLogBuiltins(eng, ctx)

	// Verify builtins are registered by running a simple script
	globals := map[string]filo.Value{}
	cfg := filo.EvalConfig{StepLimit: 1000, RecursionLimit: 100}

	_, _, err := eng.RunScript(context.Background(), `(log-print "test")`, globals, cfg)
	if err != nil {
		t.Errorf("RunScript with log-print failed: %v", err)
	}

	_, _, err = eng.RunScript(context.Background(), `(log-printf "test %d" 42)`, globals, cfg)
	if err != nil {
		t.Errorf("RunScript with log-printf failed: %v", err)
	}
}
