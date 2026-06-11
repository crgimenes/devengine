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
//	(eav-set-value "<entity>" "<record-ref-id>" "<attribute>" value)
//
// The "entity" string is the entity_type machine_name. "<record-ref-id>" is
// the opaque reference_id of the record. Soft-deleted records and
// attributes are excluded.
//
// eav-set-value writes through the executor given to NewContextTx when one
// is set, so script writes join the caller's transaction (button actions);
// otherwise it writes straight through the storage.
package filoeav

import (
	"context"
	"errors"
	"fmt"

	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/filo"
)

// Execer runs a write statement. A transaction adapter satisfies it so
// script writes join the caller's transaction.
type Execer interface {
	Exec(query string, args ...any) error
}

type Context struct {
	storage *db.SQLite
	exec    Execer // nil: writes go straight through storage
}

func NewContext(storage *db.SQLite) *Context {
	return &Context{storage: storage}
}

// NewContextTx routes eav-set-value writes through the given executor,
// typically the surrounding action's transaction.
func NewContextTx(storage *db.SQLite, exec Execer) *Context {
	return &Context{storage: storage, exec: exec}
}

func RegisterEAVBuiltins(eng *filo.Engine, ctx *Context) {
	eng.MustRegisterBuiltin("eav-exists?", ctx.exists)
	eng.MustRegisterBuiltin("eav-count", ctx.count)
	eng.MustRegisterBuiltin("eav-count-where", ctx.countWhere)
	eng.MustRegisterBuiltin("eav-get-value", ctx.getValue)
	eng.MustRegisterBuiltin("eav-set-value", ctx.setValue)
}

// setValue persists one typed value on a record and bumps the record rev so
// concurrent editors' optimistic locking notices the change.
func (c *Context) setValue(_ context.Context, args []filo.Value) (filo.Value, error) {
	if len(args) != 4 {
		return filo.Value{}, fmt.Errorf("eav-set-value expects 4 arguments (entity, record-ref-id, attribute, value)")
	}
	entityName, err := args[0].AsString()
	if err != nil {
		return filo.Value{}, fmt.Errorf("eav-set-value: entity must be string: %w", err)
	}
	refID, err := args[1].AsString()
	if err != nil {
		return filo.Value{}, fmt.Errorf("eav-set-value: record-ref-id must be string: %w", err)
	}
	attrName, err := args[2].AsString()
	if err != nil {
		return filo.Value{}, fmt.Errorf("eav-set-value: attribute must be string: %w", err)
	}

	et, attr, err := c.resolveAttr(entityName, attrName)
	if err != nil {
		return filo.Value{}, err
	}
	if et == nil || attr == nil {
		return filo.Value{}, fmt.Errorf("eav-set-value: unknown entity or attribute %q.%q", entityName, attrName)
	}

	rec, err := c.storage.GetEAVRecordByRefID(refID)
	if err != nil {
		return filo.Value{}, fmt.Errorf("eav-set-value: record %q: %w", refID, err)
	}
	if rec.EntityTypeID != et.ID {
		return filo.Value{}, fmt.Errorf("eav-set-value: record %q does not belong to %q", refID, entityName)
	}

	vBool, vInt, vReal, vText, vDatetime, err := typedColumns(attr.PrimitiveKind, args[3])
	if err != nil {
		return filo.Value{}, fmt.Errorf("eav-set-value (%s.%s): %w", entityName, attrName, err)
	}

	exec := c.exec
	if exec == nil {
		exec = c.storage
	}
	err = exec.Exec(`
		INSERT INTO eav_values (record_id, attribute_id, v_bool, v_int, v_real, v_text, v_datetime)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(record_id, attribute_id) DO UPDATE SET
			v_bool = excluded.v_bool,
			v_int = excluded.v_int,
			v_real = excluded.v_real,
			v_text = excluded.v_text,
			v_datetime = excluded.v_datetime,
			updated_at = datetime('now')
	`, rec.ID, attr.ID, vBool, vInt, vReal, vText, vDatetime)
	if err != nil {
		return filo.Value{}, fmt.Errorf("eav-set-value: %w", err)
	}
	err = exec.Exec(`UPDATE eav_records SET rev = rev + 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, rec.ID)
	if err != nil {
		return filo.Value{}, fmt.Errorf("eav-set-value: bump rev: %w", err)
	}
	return filo.VBool(true), nil
}

// typedColumns converts a Filo value into the typed column set expected by
// eav_values for the given primitive kind.
func typedColumns(kind string, v filo.Value) (vBool *bool, vInt *int64, vReal *float64, vText, vDatetime *string, err error) {
	switch kind {
	case "BOOL":
		if v.Kind != filo.KBool {
			return nil, nil, nil, nil, nil, errors.New("value must be a bool")
		}
		vBool = &v.Bool
	case "INT":
		if v.Kind != filo.KNumber {
			return nil, nil, nil, nil, nil, errors.New("value must be a number")
		}
		n := int64(v.Num)
		vInt = &n
	case "REAL":
		if v.Kind != filo.KNumber {
			return nil, nil, nil, nil, nil, errors.New("value must be a number")
		}
		vReal = &v.Num
	case "TEXT":
		if v.Kind != filo.KString {
			return nil, nil, nil, nil, nil, errors.New("value must be a string")
		}
		vText = &v.Str
	case "DATETIME":
		if v.Kind != filo.KString {
			return nil, nil, nil, nil, nil, errors.New("value must be a string")
		}
		vDatetime = &v.Str
	default:
		return nil, nil, nil, nil, nil, fmt.Errorf("unsupported primitive kind %q", kind)
	}
	return vBool, vInt, vReal, vText, vDatetime, nil
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

	var n int64
	const q = `SELECT COUNT(*) FROM eav_records
		WHERE entity_type_id = ? AND deleted_at IS NULL`
	err = c.storage.QueryRow(q, et.ID).Scan(&n)
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

	et, attr, err := c.resolveAttr(entityName, attrName)
	if err != nil {
		return filo.Value{}, err
	}
	if et == nil || attr == nil {
		return filo.VNum(0), nil
	}

	col, param, err := matchValueColumn(attr.PrimitiveKind, args[2])
	if err != nil {
		return filo.Value{}, fmt.Errorf("eav-count-where: %w", err)
	}

	q := `SELECT COUNT(*) FROM eav_records r
		JOIN eav_values v ON v.record_id = r.id
		WHERE r.entity_type_id = ?
		AND v.attribute_id = ?
		AND r.deleted_at IS NULL
		AND v.` + col + ` = ?`
	var n int64
	err = c.storage.QueryRow(q, et.ID, attr.ID, param).Scan(&n)
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

	et, attr, err := c.resolveAttr(entityName, attrName)
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

func (c *Context) resolveAttr(entityName, attrName string) (*db.EAVEntityType, *db.EAVAttribute, error) {
	et, err := c.storage.GetEAVEntityTypeByMachineName(entityName)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	if et == nil {
		return nil, nil, nil
	}
	attrs, err := c.storage.ListEAVAttributesByEntityTypeID(et.ID)
	if err != nil {
		return et, nil, err
	}
	for i := range attrs {
		if attrs[i].MachineName == attrName {
			return et, &attrs[i], nil
		}
	}
	return et, nil, nil
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
