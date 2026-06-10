// Package defaults blank-imports the canonical engine plugins (text,
// textarea, int, decimal, bool, datetime, select). Apps that want the full
// default set import this package once; apps that want a custom subset
// import each plugin package directly.
package defaults

import (
	_ "github.com/crgimenes/devengine/eav/ui/boolp"
	_ "github.com/crgimenes/devengine/eav/ui/datetime"
	_ "github.com/crgimenes/devengine/eav/ui/decimal"
	_ "github.com/crgimenes/devengine/eav/ui/intp"
	_ "github.com/crgimenes/devengine/eav/ui/reference"
	_ "github.com/crgimenes/devengine/eav/ui/selectp"
	_ "github.com/crgimenes/devengine/eav/ui/subform"
	_ "github.com/crgimenes/devengine/eav/ui/text"
	_ "github.com/crgimenes/devengine/eav/ui/textarea"
)
