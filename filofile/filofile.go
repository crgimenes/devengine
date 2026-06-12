// Package filofile provides Filo builtins for reading filemanager metadata
// by the file's opaque filename. Useful in validate_expr ("the avatar field
// must reference an existing image") and pos_load (decorate a record with
// human-readable file info).
//
// Builtins:
//
//	(file-exists? "<filename>")        ; bool
//	(file-original-name "<filename>")  ; original_filename, or ""
//	(file-size "<filename>")           ; bytes, or 0
//	(file-hash "<filename>")           ; SHA-256 hex, or ""
//	(file-type "<filename>")           ; MIME type, or ""
package filofile

import (
	"context"
	"fmt"

	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/filo"
)

// Storage is the slice of the db contract these builtins need.
// Implemented by db.Store.
type Storage interface {
	GetFileByFilename(filename string) (*db.File, error)
}

type Context struct {
	storage Storage
}

func NewContext(storage Storage) *Context {
	return &Context{storage: storage}
}

func RegisterFileBuiltins(eng *filo.Engine, ctx *Context) {
	eng.MustRegisterBuiltin("file-exists?", ctx.fileExists)
	eng.MustRegisterBuiltin("file-original-name", ctx.fileOriginalName)
	eng.MustRegisterBuiltin("file-size", ctx.fileSize)
	eng.MustRegisterBuiltin("file-hash", ctx.fileHash)
	eng.MustRegisterBuiltin("file-type", ctx.fileType)
}

func (c *Context) fileExists(_ context.Context, args []filo.Value) (filo.Value, error) {
	f, err := c.lookup("file-exists?", args)
	if err != nil {
		return filo.Value{}, err
	}
	return filo.VBool(f != nil), nil
}

func (c *Context) fileOriginalName(_ context.Context, args []filo.Value) (filo.Value, error) {
	f, err := c.lookup("file-original-name", args)
	if err != nil {
		return filo.Value{}, err
	}
	if f == nil {
		return filo.VString(""), nil
	}
	return filo.VString(f.OriginalFilename), nil
}

func (c *Context) fileSize(_ context.Context, args []filo.Value) (filo.Value, error) {
	f, err := c.lookup("file-size", args)
	if err != nil {
		return filo.Value{}, err
	}
	if f == nil {
		return filo.VNum(0), nil
	}
	return filo.VNum(float64(f.Filesize)), nil
}

func (c *Context) fileHash(_ context.Context, args []filo.Value) (filo.Value, error) {
	f, err := c.lookup("file-hash", args)
	if err != nil {
		return filo.Value{}, err
	}
	if f == nil {
		return filo.VString(""), nil
	}
	return filo.VString(f.Filehash), nil
}

func (c *Context) fileType(_ context.Context, args []filo.Value) (filo.Value, error) {
	f, err := c.lookup("file-type", args)
	if err != nil {
		return filo.Value{}, err
	}
	if f == nil {
		return filo.VString(""), nil
	}
	return filo.VString(f.Filetype), nil
}

func (c *Context) lookup(name string, args []filo.Value) (*db.File, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("%s expects 1 argument (filename), got %d", name, len(args))
	}
	filename, err := args[0].AsString()
	if err != nil {
		return nil, fmt.Errorf("%s: argument must be string: %w", name, err)
	}
	if filename == "" {
		return nil, nil
	}
	return c.storage.GetFileByFilename(filename)
}
