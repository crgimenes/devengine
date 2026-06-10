// Package subform registers the "subform" field plugin: a read-only listing
// of records that point back at the current record through a reference
// field. Useful for master/detail relationships ("this customer's orders",
// "this entity's attachments"). Subform elements are UI-only — they don't
// hold an EAV value themselves; they render based on the form's runtime
// context.
//
// Options:
//
//	{
//	  "target_entity":     "<machine_name_of_child_entity>",
//	  "target_attr":       "<reference_attr_on_child_pointing_at_parent>",
//	  "display":           "<attribute_on_child_to_show_as_label>",
//	  "limit":             50
//	}
//
// Editing and creating child records is not handled here yet; the partial
// links each row to the existing form runtime route for that entity.
package subform

import (
	"encoding/json"

	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/devengine/eav/ui"
	"github.com/crgimenes/devengine/templates"
)

func init() {
	ui.Register("subform", func() ui.FieldUI { return Plugin{} })
	templates.RegisterSubformRecordsProvider(recordsFor)
}

type Options struct {
	TargetEntity string `json:"target_entity,omitempty"`
	TargetAttr   string `json:"target_attr,omitempty"`
	Display      string `json:"display,omitempty"`
	Limit        int    `json:"limit,omitempty"`
}

type Plugin struct{}

func (Plugin) ID() string                  { return "subform" }
func (Plugin) PrimitiveKinds() []string    { return nil }
func (Plugin) HasPersistence() bool        { return false }
func (Plugin) SupportsReadOnly() bool      { return true }
func (Plugin) Parse(string, any) (any, error) { return nil, nil }
func (Plugin) Validate(any, any) error     { return nil }

func (Plugin) ParseOptions(raw string) any {
	var o Options
	if raw == "" {
		return o
	}
	_ = json.Unmarshal([]byte(raw), &o)
	return o
}

// recordsFor returns the child records of `targetEntity` whose `targetAttr`
// value equals `parentRef`. Errors collapse to an empty list so the partial
// renders cleanly even when configuration is wrong.
func recordsFor(targetEntity, targetAttr, parentRef, displayAttr string) []templates.SubformRecord {
	if db.Storage == nil || targetEntity == "" || targetAttr == "" || parentRef == "" {
		return nil
	}

	et, err := db.Storage.GetEAVEntityTypeByMachineName(targetEntity)
	if err != nil || et == nil {
		return nil
	}

	attrs, err := db.Storage.ListEAVAttributesByEntityTypeID(et.ID)
	if err != nil {
		return nil
	}

	var ptrAttrID int64
	var displayID int64
	var displayKind string
	for i := range attrs {
		if attrs[i].MachineName == targetAttr {
			ptrAttrID = attrs[i].ID
		}
		if displayAttr != "" && attrs[i].MachineName == displayAttr {
			displayID = attrs[i].ID
			displayKind = attrs[i].PrimitiveKind
		}
	}
	if ptrAttrID == 0 {
		return nil
	}

	records, err := db.Storage.ListEAVRecordsByAttributeValue(et.ID, ptrAttrID, parentRef)
	if err != nil {
		return nil
	}

	var out []templates.SubformRecord
	for _, rec := range records {
		label := rec.ReferenceID
		if displayID != 0 {
			vals, err := db.Storage.GetEAVValuesByRecordID(rec.ID)
			if err == nil {
				label = db.FormatEAVValue(displayKind, vals, displayID, rec.ReferenceID)
			}
		}
		out = append(out, templates.SubformRecord{ReferenceID: rec.ReferenceID, Label: label})
	}
	return out
}


