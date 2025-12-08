package ui

import (
	"embed"
	"encoding/json"
	"net/url"

	"github.com/crgimenes/devengine/db"
)

type FieldRuntimeContext struct {
	FormID      int64
	RecordID    *int64
	Field       *db.EAVField
	Value       any
	AllValues   map[string]any
	IsNewRecord bool
}

type ValidationResult struct {
	OK      bool
	Message string
}

type FieldUI interface {
	ID() string
	Label() string

	HasPersistence() bool
	SupportsReadOnly() bool
	IsGroupingField() bool
	RecommendedPrimitiveKind() string

	TemplatesFS() embed.FS

	RenderOptions(ctx FieldRuntimeContext) (string, any, error)
	ParseOptions(form url.Values) (json.RawMessage, error)

	BeforeSave(ctx *FieldRuntimeContext) (any, error)
	Validate(ctx FieldRuntimeContext) ValidationResult
}
