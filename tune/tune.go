// Package tune holds the product's tuning files, see docs/simulation.md#the-tuning-system. The
// game plays the base config, so that it runs on the world the product's goals are judged in, with
// the play overlay on top.
package tune

import _ "embed"

// BaseYAML is base.yaml, the base config of the product's search and the config of the game.
//
//go:embed base.yaml
var BaseYAML string //nolint:gochecknoglobals // go:embed needs a package variable

// PlayYAML is play.yaml, the overrides the game applies on top of BaseYAML, and the search does not.
//
//go:embed play.yaml
var PlayYAML string //nolint:gochecknoglobals // go:embed needs a package variable
