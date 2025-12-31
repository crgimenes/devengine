package filo

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func addTwoBuiltin(ctx context.Context, args []Value) (Value, error) {
	if len(args) != 2 {
		return Value{}, errors.New("expected two numbers")
	}

	first, err := args[0].AsNumber()
	if err != nil {
		return Value{}, err
	}

	second, err := args[1].AsNumber()
	if err != nil {
		return Value{}, err
	}

	return VNum(first + second), nil
}

func registerMathBuiltins(eng *Engine) {
	eng.RegisterBuiltin("add-two", addTwoBuiltin)
}

func fullNameBuiltin(ctx context.Context, args []Value) (Value, error) {
	if len(args) != 2 {
		return Value{}, errors.New("expected first and last name")
	}

	first, err := args[0].AsString()
	if err != nil {
		return Value{}, err
	}

	last, err := args[1].AsString()
	if err != nil {
		return Value{}, err
	}

	combined := strings.TrimSpace(first + " " + last)
	return VString(combined), nil
}

func registerStringBuiltins(eng *Engine) {
	eng.RegisterBuiltin("full-name", fullNameBuiltin)
}

func minMaxBuiltin(ctx context.Context, args []Value) (Value, error) {
	if len(args) != 1 {
		return Value{}, errors.New("expected one list")
	}

	list, err := args[0].AsList()
	if err != nil {
		return Value{}, err
	}

	if len(list) == 0 {
		return Value{}, errors.New("list cannot be empty")
	}

	minVal, err := list[0].AsNumber()
	if err != nil {
		return Value{}, err
	}

	maxVal := minVal
	for i := 1; i < len(list); i++ {
		current, convErr := list[i].AsNumber()
		if convErr != nil {
			return Value{}, convErr
		}

		if current < minVal {
			minVal = current
		}

		if current > maxVal {
			maxVal = current
		}
	}

	return VList([]Value{VNum(minVal), VNum(maxVal)}), nil
}

func registerAggregatorBuiltins(eng *Engine) {
	eng.RegisterBuiltin("min-max", minMaxBuiltin)
}

func run(t *testing.T, script string, globals map[string]Value, cfg EvalConfig) (Value, map[string]Value) {
	t.Helper()
	eng := NewEngine()
	ctx := context.Background()
	res, g, err := eng.RunScript(ctx, script, globals, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return res, g
}

func defaultCfg() EvalConfig {
	return EvalConfig{StepLimit: 10000, RecursionLimit: 64, Timeout: 200 * time.Millisecond}
}

func TestArithmetic(t *testing.T) {
	cfg := defaultCfg()
	cases := []struct {
		name   string
		script string
		want   float64
	}{
		{"add", "(+ 1 2 3)", 6},
		{"sub", "(- 10 3 2)", 5},
		{"unary-sub", "(- 5)", -5},
		{"mul", "(* 2 3 4)", 24},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			val, _ := run(t, tc.script, nil, cfg)
			got, err := val.AsNumber()
			if err != nil {
				t.Fatalf("expected number: %v", err)
			}
			if got != tc.want {
				t.Fatalf("want %v got %v", tc.want, got)
			}
		})
	}
}

func TestComparisonAndBoolean(t *testing.T) {
	cfg := defaultCfg()
	cases := []struct {
		name   string
		script string
		want   bool
	}{
		{"eq", "(= 1 1)", true},
		{"neq", "(!= 1 2)", true},
		{"lt", "(< 1 2 3)", true},
		{"gt", "(> 3 2 1)", true},
		{"and", "(and #t #t)", true},
		{"or", "(or #f #t)", true},
		{"not", "(not #f)", true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			val, _ := run(t, tc.script, nil, cfg)
			got, err := val.AsBool()
			if err != nil {
				t.Fatalf("expected bool: %v", err)
			}
			if got != tc.want {
				t.Fatalf("want %v got %v", tc.want, got)
			}
		})
	}
}

func TestStrings(t *testing.T) {
	cfg := defaultCfg()
	val, _ := run(t, "(if (= \"go\" \"go\") \"ok\" \"fail\")", nil, cfg)
	got, err := val.AsString()
	if err != nil {
		t.Fatalf("expected string: %v", err)
	}
	if got != "ok" {
		t.Fatalf("unexpected: %s", got)
	}
}

func TestIfOptionalElse(t *testing.T) {
	cfg := defaultCfg()
	// Test if with condition true - should return the then-branch
	val, _ := run(t, "(if #t \"yes\")", nil, cfg)
	got, err := val.AsString()
	if err != nil {
		t.Fatalf("expected string: %v", err)
	}
	if got != "yes" {
		t.Fatalf("expected 'yes', got %s", got)
	}

	// Test if with condition false and no else - should return empty list
	val2, _ := run(t, "(if #f \"yes\")", nil, cfg)
	list, err := val2.AsList()
	if err != nil {
		t.Fatalf("expected list: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected empty list, got %v", list)
	}

	// Test if with 3 args still works
	val3, _ := run(t, "(if #f \"yes\" \"no\")", nil, cfg)
	got, err = val3.AsString()
	if err != nil {
		t.Fatalf("expected string: %v", err)
	}
	if got != "no" {
		t.Fatalf("expected 'no', got %s", got)
	}
}

func TestLetLetvValues(t *testing.T) {
	cfg := defaultCfg()
	val, _ := run(t, "(let ((a 10) (b 20)) (+ a b))", nil, cfg)
	sum, err := val.AsNumber()
	if err != nil {
		t.Fatalf("expected number: %v", err)
	}
	if sum != 30 {
		t.Fatalf("unexpected sum %v", sum)
	}

	val2, _ := run(t, "(letv (a b) (values 2 3) (+ a b))", nil, cfg)
	num, err := val2.AsNumber()
	if err != nil {
		t.Fatalf("expected number: %v", err)
	}
	if num != 5 {
		t.Fatalf("unexpected result %v", num)
	}
}

func TestSetAndGlobals(t *testing.T) {
	cfg := defaultCfg()
	globals := map[string]Value{
		"Address": VString("http://localhost:3210"),
		"ENV":     VString("prod"),
	}
	script := "(let ((env ENV)) (set Address \"http://localhost:3210\") (if (= env \"prod\") (set Address \"https://app.example.com\") (set Address \"http://localhost:3210\")))"
	_, g := run(t, script, globals, cfg)
	got, ok := g["Address"]
	if !ok {
		t.Fatalf("expected Address in globals")
	}
	s, err := got.AsString()
	if err != nil {
		t.Fatalf("expected string: %v", err)
	}
	if s != "https://app.example.com" {
		t.Fatalf("unexpected address %s", s)
	}
}

func TestCalculatedFieldExample(t *testing.T) {
	cfg := defaultCfg()
	globals := map[string]Value{
		"field:for":   VNum(8),
		"field:bonus": VNum(3),
	}
	script := "(let ((forca field:for) (bonus field:bonus)) (+ (* forca 2) bonus))"
	val, _ := run(t, script, globals, cfg)
	num, err := val.AsNumber()
	if err != nil {
		t.Fatalf("expected number: %v", err)
	}
	if num != 19 {
		t.Fatalf("unexpected calculated value %v", num)
	}
}

func TestGlobalsReadmeExample(t *testing.T) {
	cfg := defaultCfg()
	eng := NewEngine()
	registerMathBuiltins(eng)
	registerStringBuiltins(eng)
	registerAggregatorBuiltins(eng)
	globals := map[string]Value{
		"field:a": VNum(10),
		"field:b": VNum(5),
	}
	ctx := context.Background()
	val, _, err := eng.RunScript(ctx, "(+ field:a field:b)", globals, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	num, convErr := val.AsNumber()
	if convErr != nil {
		t.Fatalf("expected number: %v", convErr)
	}
	if num != 15 {
		t.Fatalf("unexpected result %v", num)
	}
}

func TestFunctionsAndRecursion(t *testing.T) {
	cfg := defaultCfg()
	factScript := "(let () (def fact (fn (n) (if (<= n 1) 1 (* n (fact (- n 1)))))) (fact 5))"
	val, _ := run(t, factScript, nil, cfg)
	num, err := val.AsNumber()
	if err != nil {
		t.Fatalf("expected number: %v", err)
	}
	if num != 120 {
		t.Fatalf("unexpected factorial %v", num)
	}

	loopScript := "(let () (def loop (fn (n) (loop n))) (loop 0))"
	eng := NewEngine()
	ctx := context.Background()
	_, _, err = eng.RunScript(ctx, loopScript, nil, EvalConfig{StepLimit: 100, RecursionLimit: 5, Timeout: 100 * time.Millisecond})
	if err == nil {
		t.Fatalf("expected error due to limits")
	}
}

func TestListsAndHOF(t *testing.T) {
	cfg := defaultCfg()
	sumScript := "(let ((xs (list 1 2 3 4))) (fold (fn (acc x) (+ acc x)) 0 xs))"
	val, _ := run(t, sumScript, nil, cfg)
	num, err := val.AsNumber()
	if err != nil {
		t.Fatalf("expected number: %v", err)
	}
	if num != 10 {
		t.Fatalf("unexpected sum %v", num)
	}

	mapScript := "(let ((xs (list 1 2 3))) (map (fn (x) (* x x)) xs))"
	val2, _ := run(t, mapScript, nil, cfg)
	list, err := val2.AsList()
	if err != nil {
		t.Fatalf("expected list: %v", err)
	}
	expected := []float64{1, 4, 9}
	if len(list) != len(expected) {
		t.Fatalf("unexpected list length")
	}
	for i, v := range list {
		got, convErr := v.AsNumber()
		if convErr != nil {
			t.Fatalf("expected number: %v", convErr)
		}
		if got != expected[i] {
			t.Fatalf("index %d expected %v got %v", i, expected[i], got)
		}
	}

	headScript := "(head (list 10 20 30))"
	val3, _ := run(t, headScript, nil, cfg)
	num, err = val3.AsNumber()
	if err != nil {
		t.Fatalf("expected number: %v", err)
	}
	if num != 10 {
		t.Fatalf("unexpected head %v", num)
	}

	lengthScript := "(length (list 1 2 3 4))"
	val4, _ := run(t, lengthScript, nil, cfg)
	num, err = val4.AsNumber()
	if err != nil {
		t.Fatalf("expected number: %v", err)
	}
	if num != 4 {
		t.Fatalf("unexpected length %v", num)
	}

	nthScript := "(nth (list 10 20 30) 1)"
	val5, _ := run(t, nthScript, nil, cfg)
	num, err = val5.AsNumber()
	if err != nil {
		t.Fatalf("expected number: %v", err)
	}
	if num != 20 {
		t.Fatalf("unexpected nth %v", num)
	}
}

func TestSecurityLimits(t *testing.T) {
	eng := NewEngine()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := eng.RunScript(ctx, "(+ 1 2)", nil, EvalConfig{StepLimit: 10, RecursionLimit: 10, Timeout: 100 * time.Millisecond})
	if err == nil {
		t.Fatalf("expected cancellation error")
	}

	slowScript := "(let () (def loop (fn (n) (loop n))) (loop 0))"
	_, _, err = eng.RunScript(context.Background(), slowScript, nil, EvalConfig{StepLimit: 50, RecursionLimit: 20, Timeout: 10 * time.Millisecond})
	if err == nil {
		t.Fatalf("expected timeout or limit error")
	}
}

func TestRegisterBuiltin(t *testing.T) {
	eng := NewEngine()
	registerMathBuiltins(eng)
	ctx := context.Background()
	val, _, err := eng.RunScript(ctx, "(add-two 10 32)", nil, defaultCfg())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	num, convErr := val.AsNumber()
	if convErr != nil {
		t.Fatalf("expected number: %v", convErr)
	}
	if num != 42 {
		t.Fatalf("unexpected result %v", num)
	}
}

func TestRegisterStringFormatterExample(t *testing.T) {
	eng := NewEngine()
	registerStringBuiltins(eng)
	ctx := context.Background()
	val, _, err := eng.RunScript(ctx, "(full-name \"Ada\" \"Lovelace\")", nil, defaultCfg())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	name, convErr := val.AsString()
	if convErr != nil {
		t.Fatalf("expected string: %v", convErr)
	}
	if name != "Ada Lovelace" {
		t.Fatalf("unexpected name %s", name)
	}
}

func TestRegisterAggregatorExample(t *testing.T) {
	eng := NewEngine()
	registerAggregatorBuiltins(eng)
	ctx := context.Background()
	val, _, err := eng.RunScript(ctx, "(min-max (list 4 7 1 9))", nil, defaultCfg())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	list, convErr := val.AsList()
	if convErr != nil {
		t.Fatalf("expected list: %v", convErr)
	}
	if len(list) != 2 {
		t.Fatalf("unexpected list length %d", len(list))
	}
	minVal, err := list[0].AsNumber()
	if err != nil {
		t.Fatalf("expected number: %v", err)
	}
	maxVal, err := list[1].AsNumber()
	if err != nil {
		t.Fatalf("expected number: %v", err)
	}
	if minVal != 1 || maxVal != 9 {
		t.Fatalf("unexpected min/max %v %v", minVal, maxVal)
	}
}

func TestAutoLevelExample(t *testing.T) {
	eng := NewEngine()
	cfg := defaultCfg()
	script := `(let ()
  (def thresholds (list 0 300 900 2700 6500 15000))

  (def auto-level (fn (xp thresholds)
    (fold (fn (lvl threshold)
            (if (>= xp threshold) (+ lvl 1) lvl))
         0
         thresholds)))

  (def auto-level-progress (fn (xp thresholds)
    (let ((lvl (auto-level xp thresholds))
          (total (length thresholds)))
      (if (>= lvl total)
          (values lvl 0)
          (let ((next (nth thresholds lvl)))
            (values lvl (- next xp)))))))

  (auto-level-progress 1200 thresholds))`
	ctx := context.Background()
	val, _, err := eng.RunScript(ctx, script, nil, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tuple, convErr := val.AsTuple()
	if convErr != nil {
		t.Fatalf("expected tuple: %v", convErr)
	}
	if len(tuple) != 2 {
		t.Fatalf("unexpected tuple length %d", len(tuple))
	}

	level, err := tuple[0].AsNumber()
	if err != nil {
		t.Fatalf("expected level number: %v", err)
	}
	remaining, err := tuple[1].AsNumber()
	if err != nil {
		t.Fatalf("expected remaining xp number: %v", err)
	}

	if level != 3 || remaining != 1500 {
		t.Fatalf("unexpected auto level result %v %v", level, remaining)
	}
}
