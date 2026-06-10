// Package datetime registers the "datetime" field plugin (DATETIME primitive).
//
// Input format: HTML5 datetime-local strings ("2006-01-02T15:04" or
// "2006-01-02T15:04:05"). Values are stored as ISO-8601 UTC, matching the
// schema contract for v_datetime columns.
package datetime

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/crgimenes/devengine/eav/ui"
)

func init() {
	ui.Register("datetime", func() ui.FieldUI { return Plugin{} })
}

type Options struct {
	Min     string `json:"min,omitempty"`
	Max     string `json:"max,omitempty"`
	Step    int    `json:"step,omitempty"`
	Seconds bool   `json:"seconds,omitempty"`
}

type Plugin struct{}

func (Plugin) ID() string               { return "datetime" }
func (Plugin) PrimitiveKinds() []string { return []string{"DATETIME"} }
func (Plugin) HasPersistence() bool     { return true }
func (Plugin) SupportsReadOnly() bool   { return true }

func (Plugin) ParseOptions(raw string) any {
	var o Options
	if raw == "" {
		return o
	}
	_ = json.Unmarshal([]byte(raw), &o)
	return o
}

func (Plugin) Parse(raw string, _ any) (any, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", nil
	}
	t, err := parseLocal(s)
	if err != nil {
		return nil, fmt.Errorf("datetime: invalid value %q", s)
	}
	return t.UTC().Format(time.RFC3339), nil
}

func (Plugin) Validate(value, opts any) error {
	s, ok := value.(string)
	if !ok {
		return errors.New("datetime: value must be a string")
	}
	if s == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return fmt.Errorf("datetime: invalid RFC3339 %q", s)
	}
	o, _ := opts.(Options)
	if o.Min != "" {
		min, err := parseLocal(o.Min)
		if err == nil && t.Before(min) {
			return fmt.Errorf("datetime: before minimum %s", o.Min)
		}
	}
	if o.Max != "" {
		max, err := parseLocal(o.Max)
		if err == nil && t.After(max) {
			return fmt.Errorf("datetime: after maximum %s", o.Max)
		}
	}
	return nil
}

// parseLocal accepts the HTML5 datetime-local formats with and without
// seconds, plus full RFC3339 (so existing stored values round-trip).
func parseLocal(s string) (time.Time, error) {
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
	} {
		t, err := time.Parse(layout, s)
		if err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, errors.New("unparseable datetime")
}
