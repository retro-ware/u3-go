package ui

import (
	"image/png"
	"os"
	"testing"
)

// TestPaintDungeonSheet paints every style over the synthetic mask and
// checks the walls came out (something other than the ground in each piece).
func TestPaintDungeonSheet(t *testing.T) {
	mask := toRGBA(syntheticDungeonMask())
	for name, st := range dungeonStyles {
		sheet := paintDungeonSheet(st, mask)
		bg := hexColour(st.Bg)
		for _, r := range sheetRegions() {
			if r.name != "wall" {
				continue
			}
			painted := false
			for y := r.sy; y < r.sy+r.h && !painted; y++ {
				for x := r.sx; x < r.sx+r.w; x++ {
					if sheet.RGBAAt(x, y) != bg {
						painted = true
						break
					}
				}
			}
			if !painted {
				t.Errorf("%s: wall %d is blank", name, r.index)
			}
		}
		if out := os.Getenv("U3_DUMP"); out != "" && name == "Standard" {
			f, _ := os.Create(out)
			png.Encode(f, sheet)
			f.Close()
		}
	}
}
