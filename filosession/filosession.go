// Package filosession provides Filo builtins for accessing the current
// session's user. Useful in validate_expr ("only sysop can approve") and
// pre_save (auto-fill created_by).
//
// Builtins:
//
//	(session-user-id)        ; reference_id of the logged user, or ""
//	(session-user-username)  ; username, or ""
//	(session-user-email)     ; email, or ""
//	(session-sysop?)         ; bool
//
// The user pointer may be nil — in that case all string getters return ""
// and (session-sysop?) returns false.
package filosession

import (
	"context"
	"fmt"

	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/filo"
)

type Context struct {
	user *db.User
}

func NewContext(user *db.User) *Context {
	return &Context{user: user}
}

func RegisterSessionBuiltins(eng *filo.Engine, ctx *Context) {
	eng.MustRegisterBuiltin("session-user-id", ctx.userID)
	eng.MustRegisterBuiltin("session-user-username", ctx.userUsername)
	eng.MustRegisterBuiltin("session-user-email", ctx.userEmail)
	eng.MustRegisterBuiltin("session-sysop?", ctx.sysop)
}

func (c *Context) userID(_ context.Context, args []filo.Value) (filo.Value, error) {
	if err := expectArity("session-user-id", args, 0); err != nil {
		return filo.Value{}, err
	}
	if c.user == nil {
		return filo.VString(""), nil
	}
	return filo.VString(c.user.ReferenceID), nil
}

func (c *Context) userUsername(_ context.Context, args []filo.Value) (filo.Value, error) {
	if err := expectArity("session-user-username", args, 0); err != nil {
		return filo.Value{}, err
	}
	if c.user == nil {
		return filo.VString(""), nil
	}
	return filo.VString(c.user.Username), nil
}

func (c *Context) userEmail(_ context.Context, args []filo.Value) (filo.Value, error) {
	if err := expectArity("session-user-email", args, 0); err != nil {
		return filo.Value{}, err
	}
	if c.user == nil {
		return filo.VString(""), nil
	}
	return filo.VString(c.user.Email), nil
}

func (c *Context) sysop(_ context.Context, args []filo.Value) (filo.Value, error) {
	if err := expectArity("session-sysop?", args, 0); err != nil {
		return filo.Value{}, err
	}
	if c.user == nil {
		return filo.VBool(false), nil
	}
	return filo.VBool(c.user.Sysop), nil
}

func expectArity(name string, args []filo.Value, want int) error {
	if len(args) != want {
		return fmt.Errorf("%s expects %d arguments, got %d", name, want, len(args))
	}
	return nil
}
