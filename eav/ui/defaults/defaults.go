// Package defaults blank-imports the canonical engine plugins (text, int,
// bool). Apps that want the full default set import this package once;
// apps that want a custom subset import each plugin package directly.
package defaults

import (
	_ "github.com/crgimenes/devengine/eav/ui/boolp"
	_ "github.com/crgimenes/devengine/eav/ui/intp"
	_ "github.com/crgimenes/devengine/eav/ui/text"
)
