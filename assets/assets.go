// Package assets holds the art and sound effects of synthwave-pixel's
// Ultima III port that are its own work (MIT, see README.md and LICENSE).
package assets

import "embed"

// FS has graphics/*.png and sounds/*.wav.
//
//go:embed graphics sounds
var FS embed.FS
