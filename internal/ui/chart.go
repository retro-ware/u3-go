package ui

import (
	"math"

	"github.com/retro-ware/u3-go/internal/game"
)

// Sosaria's chart (chart.ts): the page of the map under what the party has
// seen, as a map the party set out with would be: coasts in ink on old
// paper, the land a shade darker than the sea, and nothing else. The
// squares the party has seen are drawn over it in the set's tiles; a town
// known of but not yet seen is a sign in the same ink (drawPlaceSign).

var (
	paper = [3]float64{226, 203, 160}
	ink   = hexColour("#4a3524")
)

// grain is a small seeded generator (mulberry32), so the paper's grain is the same every time.
func grain(seed uint32) func() float64 {
	a := seed
	return func() float64 {
		a += 0x6d2b79f5
		t := (a ^ a>>15) * (a | 1)
		t ^= t + (t^t>>7)*(t|61)
		return float64(t^t>>14) / 4294967296
	}
}

// drawChart draws the chart, sq pixels a square, from which squares are land; the map wraps.
func drawChart(sq float64, isLand func(x, y int) bool) *Canvas {
	size := int(math.Round(sq * game.MapSide))
	c := NewCanvas(size, size)
	rand := grain(1983)
	for i := 0; i < size*size; i++ {
		d := (rand() - 0.5) * 18
		c.Pix[i*4] = uint8(paper[0] + d)
		c.Pix[i*4+1] = uint8(paper[1] + d)
		c.Pix[i*4+2] = uint8(paper[2] + d*0.8)
	}
	land := premul(rgb(120, 88, 40), 0.16)
	for y := range game.MapSide {
		for x := range game.MapSide {
			if isLand(x, y) {
				c.Fill(frect(float64(x)*sq, float64(y)*sq, sq, sq), land)
			}
		}
	}
	// The coasts: a wide pale line under a fine dark one, so a coast reads as drawn.
	var segs [][4]float64
	for _, s := range game.CoastSegments(isLand, game.MapSide, true) {
		segs = append(segs, [4]float64{s.X1 * sq, s.Y1 * sq, s.X2 * sq, s.Y2 * sq})
	}
	c.StrokeLines(segs, math.Max(3, sq*0.45), premul(ink, 0.25), true)
	c.StrokeLines(segs, math.Max(1.2, sq*0.15), ink, true)
	// The paper's edges darkened with age.
	half := float64(size) / 2
	for y := range size {
		for x := range size {
			t := (math.Hypot(float64(x)-half, float64(y)-half) - float64(size)*0.35) / (float64(size) * 0.4)
			if t <= 0 {
				continue
			}
			a := 0.35 * math.Min(1, t)
			i := c.PixOffset(x, y)
			for k, v := range [3]float64{90, 60, 20} {
				c.Pix[i+k] = uint8(float64(c.Pix[i+k])*(1-a) + v*a)
			}
		}
	}
	return c
}

// drawPlaceSign marks a place on the chart before the party has seen it,
// centred on (cx, cy), sq across: a town a ring about a dot, a castle a square about a dot.
func drawPlaceSign(c *Canvas, cx, cy, sq float64, castle bool) {
	r := sq * 0.8
	w := math.Max(1.5, sq*0.22)
	if castle {
		c.StrokePolygon([]vec{{cx - r, cy - r}, {cx + r, cy - r}, {cx + r, cy + r}, {cx - r, cy + r}}, w, ink, c.Bounds())
	} else {
		c.FillPolygon(ring(cx, cy, r, w), ink)
	}
	c.FillPolygon(Path{circle(cx, cy, math.Max(1.2, sq*0.24))}, ink)
}
