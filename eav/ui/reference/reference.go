// Package reference registers the "reference" field plugin: a foreign key
// from one EAV record to another, stored as TEXT (the target's reference_id).
//
// Options:
//
//	{
//	  "entity":      "<target_entity_machine_name>",
//	  "display":     "<attribute_machine_name>",
//	  "allow_empty": false
//	}
//
// At save time Parse just trims the submitted reference_id. Validate looks
// the target up via filoeav-style helpers and rejects values that don't
// resolve to an active record of the configured entity. The runtime
// template uses the choices provider to render the <select>.
package reference

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/devengine/eav/ui"
	"github.com/crgimenes/devengine/templates"
)

//go:embed field_reference.go.tmpl
var templatesFS embed.FS

func init() {
	templates.RegisterFS(templatesFS)
	ui.Register("reference", func() ui.FieldUI { return Plugin{} })
	templates.RegisterReferenceChoicesProvider(choicesFor)
}

type Options struct {
	Entity     string `json:"entity,omitempty"`
	Display    string `json:"display,omitempty"`
	AllowEmpty bool   `json:"allow_empty,omitempty"`
}

type Plugin struct{}

func (Plugin) ID() string               { return "reference" }
func (Plugin) PrimitiveKinds() []string { return []string{"TEXT"} }
func (Plugin) HasPersistence() bool     { return true }
func (Plugin) SupportsReadOnly() bool   { return true }

func (Plugin) Defaults() map[string]any {
	return map[string]any{}
}

func (Plugin) ParseOptions(raw string) any {
	var o Options
	if raw == "" {
		return o
	}
	_ = json.Unmarshal([]byte(raw), &o)
	return o
}

func (Plugin) Parse(raw string, _ any) (any, error) {
	return strings.TrimSpace(raw), nil
}

func (Plugin) Validate(value, opts any) error {
	s, ok := value.(string)
	if !ok {
		return errors.New("reference: value must be a string")
	}
	o, _ := opts.(Options)
	if s == "" {
		if o.AllowEmpty {
			return nil
		}
		return errors.New("reference: empty value not allowed")
	}
	if o.Entity == "" {
		return errors.New("reference: missing 'entity' option")
	}
	if db.Storage == nil {
		return errors.New("reference: db not initialized")
	}
	et, err := db.Storage.GetEAVEntityTypeByMachineName(o.Entity)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return fmt.Errorf("reference: entity %q not found", o.Entity)
		}
		return err
	}
	rec, err := db.Storage.GetEAVRecordByRefID(s)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return fmt.Errorf("reference: %q is not a valid record", s)
		}
		return err
	}
	if rec == nil || rec.EntityTypeID != et.ID {
		return fmt.Errorf("reference: %q does not belong to %q", s, o.Entity)
	}
	return nil
}

// choicesFor is the function templates calls to populate the <select>. It is
// resilient to errors: lookup failures result in an empty list rather than a
// template explosion.
func choicesFor(entity, displayAttr string) []templates.ReferenceChoice {
	if db.Storage == nil || entity == "" {
		return nil
	}

	et, err := db.Storage.GetEAVEntityTypeByMachineName(entity)
	if err != nil || et == nil {
		return nil
	}

	var display *db.EAVAttribute
	if displayAttr != "" {
		attrs, err := db.Storage.ListEAVAttributesByEntityTypeID(et.ID)
		if err == nil {
			for i := range attrs {
				if attrs[i].MachineName == displayAttr {
					display = &attrs[i]
					break
				}
			}
		}
	}

	records, _, err := db.Storage.ListEAVRecordsByEntityTypeID(et.ID, 1000, 0)
	if err != nil {
		return nil
	}

	out := make([]templates.ReferenceChoice, 0, len(records))
	for _, rec := range records {
		label := rec.ReferenceID
		if display != nil {
			values, err := db.Storage.GetEAVValuesByRecordID(rec.ID)
			if err == nil {
				label = db.FormatEAVValue(display.PrimitiveKind, values, display.ID, rec.ReferenceID)
			}
		}
		out = append(out, templates.ReferenceChoice{
			Value: rec.ReferenceID,
			Label: label,
		})
	}
	return out
}
