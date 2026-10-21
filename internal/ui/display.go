package ui

import (
	"image"
	"image/color"
	"math"
)

// How each tile set's machine showed its picture beyond what its sheets
// hold (display.ts): a monochrome monitor's one colour, a light ground.
// Whatever is drawn over the tiles in code (the first-person dungeon, the
// combat marker) reads these so it matches the tiles round it.

// DisplayStyle is a set's display.
type DisplayStyle struct {
	Phosphor    string // a monochrome monitor's colour, where the others draw white
	LightGround bool   // the ground is light (the Macintosh set's white)
}

var displayStyles = map[string]DisplayStyle{
	"Apple II Mono": {Phosphor: "#8cf88c"},
	"Macintosh B&W": {LightGround: true},
}

// displayOf is a set's display style; a dark ground, in white, for one not listed.
func displayOf(set string) DisplayStyle { return displayStyles[set] }

// markerColour is the combat marker's colour elapsed (0..1) into the
// member's turn: white, or the phosphor, fading to half as the turn runs
// out; on a light ground, black fading up to grey.
func markerColour(d DisplayStyle, elapsed float64) color.RGBA {
	fade := 127 * math.Max(0, math.Min(1, elapsed))
	level := (255 - fade) / 255
	if d.LightGround {
		level = fade / 255
	}
	base := rgb(255, 255, 255)
	if d.Phosphor != "" {
		base = hexColour(d.Phosphor)
	}
	return rgb(uint8(math.Round(float64(base.R)*level)), uint8(math.Round(float64(base.G)*level)), uint8(math.Round(float64(base.B)*level)))
}

// Moon phases painted for the 8-bit and 16-colour sets (moonArt.ts): flat
// pixel moons on a 16 by 16 grid in each set's palette. Phases run 0 new,
// 1 to 3 a crescent growing on the left, 4 full, 5 to 7 shrinking on the
// right; Trammel and Felucca each have a colour.

// MoonStyle is a set's moon colours.
type MoonStyle struct{ Trammel, Felucca, Dark string }

var moonStyles = map[string]MoonStyle{
	"Standard":       {"#ffffff", "#ffff55", "#555555"}, // as PC EGA
	"Commodore 64":   {"#8e8dff", "#f3eb5b", "#3b3b3b"},
	"Apple II Color": {"#15cffd", "#ff6a3c", "#303030"},
	"Apple II Mono":  {"#8cf88c", "#8cf88c", "#204320"},
	"PC CGA":         {"#55ffff", "#ff55ff", "#000000"},
	"PC EGA":         {"#ffffff", "#ffff55", "#555555"},
}

const (
	moonGrid  = 16
	moonScale = 4
	moonCell  = moonGrid * moonScale
)

// moonPixel says whether logical pixel (x, y) of a phase is lit; ok is
// false off the disc.
func moonPixel(phase, x, y int) (lit, ok bool) {
	cx := (float64(x) + 0.5 - moonGrid/2) / 6.5
	cy := (float64(y) + 0.5 - moonGrid/2) / 6.5
	if cx*cx+cy*cy > 1 {
		return false, false
	}
	half := math.Sqrt(1 - cy*cy)
	k := float64(phase) / 4
	if phase > 4 {
		k = float64(8-phase) / 4
	}
	terminator := half * math.Cos(math.Pi*k)
	if phase <= 4 {
		return cx < -terminator, true
	}
	return cx > terminator, true
}

// paintMoons paints the sixteen moons, Trammel's eight then Felucca's, as one strip.
func paintMoons(s MoonStyle) *image.RGBA {
	c := NewCanvas(moonCell*16, moonCell)
	for column := range 16 {
		lit := hexColour(s.Trammel)
		if column >= 8 {
			lit = hexColour(s.Felucca)
		}
		for y := range moonGrid {
			for x := range moonGrid {
				on, ok := moonPixel(column&7, x, y)
				if !ok {
					continue
				}
				col := hexColour(s.Dark)
				if on {
					col = lit
				}
				px := column*moonCell + x*moonScale
				c.Fill(image.Rect(px, y*moonScale, px+moonScale, (y+1)*moonScale), col)
			}
		}
	}
	return c.RGBA
}

// The Scanlines setting (scanlines.ts): one dark band for each of the
// Apple II's 192 screen lines, drawn in the window's own pixels.

const (
	screenLines   = 192
	scanlineLevel = 0.65 // the brightness a band lets through
)

// scanlineBands are the dark bands for a picture height pixels tall, as
// [top, rows]: the lower half of each line at an even whole-pixel pitch.
// Under two pixels a line there is no room, and there are none.
func scanlineBands(height int) [][2]int {
	pitch := int(math.Round(float64(height) / screenLines))
	if pitch < 2 {
		return nil
	}
	dark := pitch / 2
	var out [][2]int
	for top := pitch - dark; top < height; top += pitch {
		out = append(out, [2]int{top, min(dark, height-top)})
	}
	return out
}
