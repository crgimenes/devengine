package uiplugins

import (
	_ "github.com/crgimenes/devengine/eav/ui/audio"
	_ "github.com/crgimenes/devengine/eav/ui/boolean"
	_ "github.com/crgimenes/devengine/eav/ui/decimal"
	_ "github.com/crgimenes/devengine/eav/ui/divider"
	_ "github.com/crgimenes/devengine/eav/ui/group"
	_ "github.com/crgimenes/devengine/eav/ui/groupaccordion"
	_ "github.com/crgimenes/devengine/eav/ui/image"
	_ "github.com/crgimenes/devengine/eav/ui/integer"
	_ "github.com/crgimenes/devengine/eav/ui/list"
	_ "github.com/crgimenes/devengine/eav/ui/text"
	_ "github.com/crgimenes/devengine/eav/ui/textarea"
	_ "github.com/crgimenes/devengine/eav/ui/video"
)

// Init ensures plugin packages are linked.
func Init() {}
