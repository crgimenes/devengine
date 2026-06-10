// Package filoeav provides Filo builtins for read-only access to EAV
// records. Useful in validate_expr (cross-record uniqueness, referential
// checks) and pos_load (decorate display values pulled from other entities).
//
// Builtins:
//
//	(eav-exists? "<entity>" "<record-ref-id>")
//	(eav-count "<entity>")
//	(eav-count-where "<entity>" "<attribute>" value)
//	(eav-get-value "<entity>" "<record-ref-id>" "<attribute>")
//
// The "entity" string is the entity_type machine_name. "<record-ref-id>" is
// the opaque reference_id of the record. Soft-deleted records and
// attributes are excluded.
package filoeav

import (
	"context"
	"errors"
	"fmt"

	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/filo"
)

type Context struct {
	storage *db.SQLite
}

func NewContext(storage *db.SQLite) *Context {
	return &Context{storage: storage}
}

func RegisterEAVBuiltins(eng *filo.Engine, ctx *Context) {
	eng.MustRegisterBuiltin("eav-exists?", ctx.exists)
	eng.MustRegisterBuiltin("eav-count", ctx.count)
	eng.MustRegisterBuiltin("eav-count-where", ctx.countWhere)
	eng.MustRegisterBuiltin("eav-get-value", ctx.getValue)
}

func (c *Context) exists(_ context.Context, args []filo.Value) (filo.Value, error) {
	if len(args) != 2 {
		return filo.Value{}, fmt.Errorf("eav-exists? expects 2 arguments (entity, record-ref-id)")
	}
	entityName, err := args[0].AsString()
	if err != nil {
		return filo.Value{}, fmt.Errorf("eav-exists?: entity must be string: %w", err)
	}
	refID, err := args[1].AsString()
	if err != nil {
		return filo.Value{}, fmt.Errorf("eav-exists?: record-ref-id must be string: %w", err)
	}

	et, err := c.storage.GetEAVEntityTypeByMachineName(entityName)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			et = nil
		} else {
			return filo.Value{}, err
		}
	}
	if et == nil {
		return filo.VBool(false), nil
	}

	rec, err := c.storage.GetEAVRecordByRefID(refID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return filo.VBool(false), nil
		}
		return filo.Value{}, err
	}
	if rec == nil || rec.EntityTypeID != et.ID {
		return filo.VBool(false), nil
	}
	return filo.VBool(true), nil
}

func (c *Context) count(_ context.Context, args []filo.Value) (filo.Value, error) {
	if len(args) != 1 {
		return filo.Value{}, fmt.Errorf("eav-count expects 1 argument (entity)")
	}
	entityName, err := args[0].AsString()
	if err != nil {
		return filo.Value{}, fmt.Errorf("eav-count: entity must be string: %w", err)
	}

	et, err := c.storage.GetEAVEntityTypeByMachineName(entityName)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			et = nil
		} else {
			return filo.Value{}, err
		}
	}
	if et == nil {
		return filo.VNum(0), nil
	}

	n, err := c.storage.CountEAVRecords(et.ID)
	if err != nil {
		return filo.Value{}, err
	}
	return filo.VNum(float64(n)), nil
}

func (c *Context) countWhere(_ context.Context, args []filo.Value) (filo.Value, error) {
	if len(args) != 3 {
		return filo.Value{}, fmt.Errorf("eav-count-where expects 3 arguments (entity, attribute, value)")
	}
	entityName, err := args[0].AsString()
	if err != nil {
		return filo.Value{}, fmt.Errorf("eav-count-where: entity must be string: %w", err)
	}
	attrName, err := args[1].AsString()
	if err != nil {
		return filo.Value{}, fmt.Errorf("eav-count-where: attribute must be string: %w", err)
	}

	et, attr, err := c.storage.LookupEAVEntityTypeAndAttribute(entityName, attrName)
	if err != nil {
		return filo.Value{}, err
	}
	if et == nil || attr == nil {
		return filo.VNum(0), nil
	}

	_, param, err := matchValueColumn(attr.PrimitiveKind, args[2])
	if err != nil {
		return filo.Value{}, fmt.Errorf("eav-count-where: %w", err)
	}

	n, err := c.storage.CountEAVRecordsWhere(et.ID, attr.ID, attr.PrimitiveKind, param)
	if err != nil {
		return filo.Value{}, err
	}
	return filo.VNum(float64(n)), nil
}

func (c *Context) getValue(_ context.Context, args []filo.Value) (filo.Value, error) {
	if len(args) != 3 {
		return filo.Value{}, fmt.Errorf("eav-get-value expects 3 arguments (entity, record-ref-id, attribute)")
	}
	entityName, err := args[0].AsString()
	if err != nil {
		return filo.Value{}, fmt.Errorf("eav-get-value: entity must be string: %w", err)
	}
	refID, err := args[1].AsString()
	if err != nil {
		return filo.Value{}, fmt.Errorf("eav-get-value: record-ref-id must be string: %w", err)
	}
	attrName, err := args[2].AsString()
	if err != nil {
		return filo.Value{}, fmt.Errorf("eav-get-value: attribute must be string: %w", err)
	}

	et, attr, err := c.storage.LookupEAVEntityTypeAndAttribute(entityName, attrName)
	if err != nil {
		return filo.Value{}, err
	}
	if et == nil || attr == nil {
		return filo.VString(""), nil
	}

	rec, err := c.storage.GetEAVRecordByRefID(refID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return filo.VString(""), nil
		}
		return filo.Value{}, err
	}
	if rec == nil || rec.EntityTypeID != et.ID {
		return filo.VString(""), nil
	}

	values, err := c.storage.GetEAVValuesByRecordID(rec.ID)
	if err != nil {
		return filo.Value{}, err
	}
	for _, v := range values {
		if v.AttributeID == attr.ID {
			return valueToFilo(attr.PrimitiveKind, v), nil
		}
	}
	return filo.VString(""), nil
}



// matchValueColumn returns the v_* column name and a Go-typed value that
// fits the attribute's primitive kind, given the raw Filo value the caller
// passed in.
func matchValueColumn(primitive string, v filo.Value) (string, any, error) {
	switch primitive {
	case "BOOL":
		if v.Kind != filo.KBool {
			return "", nil, fmt.Errorf("attribute is BOOL but value is %s", filoKind(v.Kind))
		}
		return "v_bool", v.Bool, nil
	case "INT":
		if v.Kind != filo.KNumber {
			return "", nil, fmt.Errorf("attribute is INT but value is %s", filoKind(v.Kind))
		}
		return "v_int", int64(v.Num), nil
	case "REAL":
		if v.Kind != filo.KNumber {
			return "", nil, fmt.Errorf("attribute is REAL but value is %s", filoKind(v.Kind))
		}
		return "v_real", v.Num, nil
	case "TEXT":
		if v.Kind != filo.KString {
			return "", nil, fmt.Errorf("attribute is TEXT but value is %s", filoKind(v.Kind))
		}
		return "v_text", v.Str, nil
	case "DATETIME":
		if v.Kind != filo.KString {
			return "", nil, fmt.Errorf("attribute is DATETIME but value is %s", filoKind(v.Kind))
		}
		return "v_datetime", v.Str, nil
	}
	return "", nil, fmt.Errorf("unknown primitive kind %q", primitive)
}

func valueToFilo(primitive string, v db.EAVValue) filo.Value {
	switch primitive {
	case "BOOL":
		if v.VBool == nil {
			return filo.VBool(false)
		}
		return filo.VBool(*v.VBool)
	case "INT":
		if v.VInt == nil {
			return filo.VNum(0)
		}
		return filo.VNum(float64(*v.VInt))
	case "REAL":
		if v.VReal == nil {
			return filo.VNum(0)
		}
		return filo.VNum(*v.VReal)
	case "TEXT":
		if v.VText == nil {
			return filo.VString("")
		}
		return filo.VString(*v.VText)
	case "DATETIME":
		if v.VDatetime == nil {
			return filo.VString("")
		}
		return filo.VString(*v.VDatetime)
	}
	return filo.VString("")
}

func filoKind(k filo.Kind) string {
	switch k {
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
	}
	return "unknown"
}
