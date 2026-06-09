// Package ui defines the contract for form field plugins.
//
// A plugin owns how a single field type is parsed from form input, validated,
// and described to the admin (options). Rendering is delegated to template
// partials whose name is derived from the plugin id (field_<id>).
package ui

// FieldUI is implemented by every plugin and registered in the global registry.
type FieldUI interface {
	ID() string
	PrimitiveKinds() []string
	HasPersistence() bool
	SupportsReadOnly() bool
	ParseOptions(rawJSON string) any
	Parse(raw string, opts any) (any, error)
	Validate(value any, opts any) error
}
