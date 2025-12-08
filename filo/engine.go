package filo

import (
	"context"
	"fmt"
	"time"
)

type Engine struct {
	builtins map[string]builtinFunc
}

type EvalConfig struct {
	StepLimit      int
	RecursionLimit int
	Timeout        time.Duration
}

func NewEngine() *Engine {
	return &Engine{builtins: defaultBuiltins()}
}

func (e *Engine) RegisterBuiltin(name string, fn Builtin) {
	e.builtins[name] = func(ctx context.Context, _ *evaluator, args []Value) (Value, error) {
		return fn(ctx, args)
	}
}

func (e *Engine) RunScript(ctx context.Context, src string, globals map[string]Value, cfg EvalConfig) (Value, map[string]Value, error) {
	ast, err := parse(src)
	if err != nil {
		return Value{}, nil, err
	}

	root := NewEnv()
	for k, v := range globals {
		root.Define(k, v)
	}

	runCtx := ctx
	var cancel context.CancelFunc
	if cfg.Timeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, cfg.Timeout)
	}
	if cancel != nil {
		defer cancel()
	}

	ev := newEvaluator(runCtx, cfg, root, e.builtins)

	var result Value
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic in script: %v", r)
		}
	}()

	result, err = ev.eval(ast, root)
	if err != nil {
		return Value{}, nil, err
	}

	newGlobals := make(map[string]Value, len(root.bind))
	for k, v := range root.bind {
		newGlobals[k] = v
	}
	return result, newGlobals, nil
}
