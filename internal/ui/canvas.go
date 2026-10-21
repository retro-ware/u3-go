// Package ui is Ultima III's screen, keyboard, gamepad, sound and music: the
// Go port of synthwave-pixel's TypeScript web UI (web/src/ui, MIT licensed,
// after LairWare's Macintosh port), drawn natively with Ebiten.
//
// # Drawing
//
// The TypeScript drew on an HTML canvas. Here the screen is a software
// canvas, an *image.RGBA of 40 x 24 cells, that the Screen draws into
// exactly as the TypeScript did (retained: the border, text and stats when
// they change, the viewport twelve times a second). Ebiten only uploads the
// finished picture each frame, so the same code renders without a window
// for the -shot check and the scripted tests.
//
// # Threading
//
// The game runs on its own goroutine and calls the Screen (game.IO). The
// game goroutine holds Screen.mu whenever it runs; it lets go only while it
// waits (a key, a timeout, a pause). The frame loop takes the lock to
// animate and copy the canvas, so it only ever sees the World while the
// game is blocked in an IO call, as package game asks.
package ui

import (
	"image"
	"image/color"
	"math"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/vector"
)

// Canvas is a picture with the canvas operations the screen needs.
type Canvas struct {
	*image.RGBA
}

// NewCanvas is a black canvas w x h pixels.
func NewCanvas(w, h int) *Canvas {
	c := &Canvas{image.NewRGBA(image.Rect(0, 0, w, h))}
	c.Fill(c.Bounds(), color.RGBA{0, 0, 0, 255})
	return c
}

// rgb is an opaque colour.
func rgb(r, g, b uint8) color.RGBA { return color.RGBA{r, g, b, 255} }

// hexColour reads "#rgb" or "#rrggbb" ("" and anything else is white).
func hexColour(s string) color.RGBA {
	if len(s) == 4 && s[0] == '#' {
		s = "#" + string([]byte{s[1], s[1], s[2], s[2], s[3], s[3]})
	}
	if len(s) != 7 || s[0] != '#' {
		return rgb(255, 255, 255)
	}
	var v [3]uint8
	for i := range 3 {
		v[i] = hexByte(s[1+2*i])<<4 | hexByte(s[2+2*i])
	}
	return rgb(v[0], v[1], v[2])
}

func hexByte(c byte) uint8 {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10
	}
	return 0
}

// Fill paints r with c, blending when c is not opaque.
func (c *Canvas) Fill(r image.Rectangle, col color.RGBA) {
	r = r.Intersect(c.Bounds())
	if r.Empty() {
		return
	}
	if col.A == 255 {
		for y := r.Min.Y; y < r.Max.Y; y++ {
			row := c.Pix[c.PixOffset(r.Min.X, y):c.PixOffset(r.Max.X, y)]
			for i := 0; i < len(row); i += 4 {
				row[i], row[i+1], row[i+2], row[i+3] = col.R, col.G, col.B, 255
			}
		}
		return
	}
	xdraw.Draw(c, r, image.NewUniform(col), image.Point{}, xdraw.Over)
}

// FillAlpha paints r with an opaque colour at the given opacity (0..1).
func (c *Canvas) FillAlpha(r image.Rectangle, col color.RGBA, alpha float64) {
	c.Fill(r, premul(col, alpha))
}

// premul is col at opacity alpha, premultiplied as image.RGBA wants.
func premul(col color.RGBA, alpha float64) color.RGBA {
	a := math.Max(0, math.Min(1, alpha))
	return color.RGBA{uint8(float64(col.R) * a), uint8(float64(col.G) * a), uint8(float64(col.B) * a), uint8(255 * a)}
}

// Invert inverts r's colours (the canvas's "difference" with white).
func (c *Canvas) Invert(r image.Rectangle) {
	r = r.Intersect(c.Bounds())
	for y := r.Min.Y; y < r.Max.Y; y++ {
		row := c.Pix[c.PixOffset(r.Min.X, y):c.PixOffset(r.Max.X, y)]
		for i := 0; i < len(row); i += 4 {
			row[i], row[i+1], row[i+2] = 255-row[i], 255-row[i+1], 255-row[i+2]
		}
	}
}

// Multiply tints r: each channel times the colour's ("multiply"), so white
// takes the colour and black stays black.
func (c *Canvas) Multiply(r image.Rectangle, col color.RGBA) {
	r = r.Intersect(c.Bounds())
	for y := r.Min.Y; y < r.Max.Y; y++ {
		row := c.Pix[c.PixOffset(r.Min.X, y):c.PixOffset(r.Max.X, y)]
		for i := 0; i < len(row); i += 4 {
			row[i] = uint8(int(row[i]) * int(col.R) / 255)
			row[i+1] = uint8(int(row[i+1]) * int(col.G) / 255)
			row[i+2] = uint8(int(row[i+2]) * int(col.B) / 255)
		}
	}
}

// Copy is a copy of the pixels in r, for Restore.
func (c *Canvas) Copy(r image.Rectangle) *image.RGBA {
	r = r.Intersect(c.Bounds())
	out := image.NewRGBA(r)
	xdraw.Draw(out, r, c, r.Min, xdraw.Src)
	return out
}

// Restore puts back pixels taken with Copy.
func (c *Canvas) Restore(saved *image.RGBA) {
	if saved != nil {
		xdraw.Draw(c, saved.Rect, saved, saved.Rect.Min, xdraw.Src)
	}
}

// Blit draws the src rectangle of img scaled into dst, nearest neighbour,
// blended over what is there (over) or replacing it.
func (c *Canvas) Blit(img image.Image, src, dst image.Rectangle, over bool) {
	if dst.Empty() || src.Empty() {
		return
	}
	op := xdraw.Src
	if over {
		op = xdraw.Over
	}
	if src.Dx() == dst.Dx() && src.Dy() == dst.Dy() {
		xdraw.Draw(c, dst, img, src.Min, op)
		return
	}
	xdraw.NearestNeighbor.Scale(c, dst, img, src, op, nil)
}

// BlitFlipped is Blit mirrored left to right.
func (c *Canvas) BlitFlipped(img image.Image, src, dst image.Rectangle, over bool) {
	tmp := image.NewRGBA(image.Rect(0, 0, dst.Dx(), dst.Dy()))
	xdraw.NearestNeighbor.Scale(tmp, tmp.Rect, img, src, xdraw.Src, nil)
	w := tmp.Rect.Dx()
	for y := 0; y < tmp.Rect.Dy(); y++ {
		row := tmp.Pix[y*tmp.Stride : y*tmp.Stride+w*4]
		for i, j := 0, w-1; i < j; i, j = i+1, j-1 {
			for k := range 4 {
				row[i*4+k], row[j*4+k] = row[j*4+k], row[i*4+k]
			}
		}
	}
	c.Blit(tmp, tmp.Rect, dst, over)
}

// Path is a polygon (or several) in canvas pixels, for filling or stroking.
type Path [][]vec

type vec struct{ X, Y float64 }

// FillPolygon fills the closed polygons of p with col (antialiased).
func (c *Canvas) FillPolygon(p Path, col color.RGBA) {
	c.FillPolygonClip(p, col, c.Bounds())
}

// FillPolygonClip is FillPolygon inside clip only.
func (c *Canvas) FillPolygonClip(p Path, col color.RGBA, clip image.Rectangle) {
	clip = clip.Intersect(c.Bounds())
	if clip.Empty() {
		return
	}
	// The rasterizer covers just the clip; its Draw puts its origin at clip.Min.
	z := vector.NewRasterizer(clip.Dx(), clip.Dy())
	ox, oy := float32(clip.Min.X), float32(clip.Min.Y)
	for _, poly := range p {
		if len(poly) < 3 {
			continue
		}
		z.MoveTo(float32(poly[0].X)-ox, float32(poly[0].Y)-oy)
		for _, v := range poly[1:] {
			z.LineTo(float32(v.X)-ox, float32(v.Y)-oy)
		}
		z.ClosePath()
	}
	z.Draw(c, clip, image.NewUniform(col), image.Point{})
}

// StrokeLines strokes each segment (x1, y1, x2, y2) width pixels wide, with
// round ends when round is set.
func (c *Canvas) StrokeLines(segs [][4]float64, width float64, col color.RGBA, round bool) {
	var p Path
	for _, s := range segs {
		p = append(p, lineQuad(s[0], s[1], s[2], s[3], width))
		if round {
			p = append(p, circle(s[0], s[1], width/2), circle(s[2], s[3], width/2))
		}
	}
	c.FillPolygon(p, col)
}

// StrokePolygon outlines a closed polygon.
func (c *Canvas) StrokePolygon(poly []vec, width float64, col color.RGBA, clip image.Rectangle) {
	var p Path
	for i := range poly {
		a, b := poly[i], poly[(i+1)%len(poly)]
		p = append(p, lineQuad(a.X, a.Y, b.X, b.Y, width))
	}
	c.FillPolygonClip(p, col, clip)
}

// lineQuad is the rectangle a segment covers when stroked width wide.
func lineQuad(x1, y1, x2, y2, width float64) []vec {
	dx, dy := x2-x1, y2-y1
	l := math.Hypot(dx, dy)
	if l == 0 {
		return circle(x1, y1, width/2)
	}
	nx, ny := -dy/l*width/2, dx/l*width/2
	return []vec{{x1 + nx, y1 + ny}, {x2 + nx, y2 + ny}, {x2 - nx, y2 - ny}, {x1 - nx, y1 - ny}}
}

// circle is a disc as a polygon.
func circle(cx, cy, r float64) []vec {
	n := max(12, int(r*2))
	out := make([]vec, n)
	for i := range out {
		a := 2 * math.Pi * float64(i) / float64(n)
		out[i] = vec{cx + r*math.Cos(a), cy + r*math.Sin(a)}
	}
	return out
}

// ring is the band between radii r-w/2 and r+w/2, as two opposite polygons
// (the rasterizer's non-zero rule leaves the hole).
func ring(cx, cy, r, w float64) Path {
	outer := circle(cx, cy, r+w/2)
	inner := circle(cx, cy, r-w/2)
	for i, j := 0, len(inner)-1; i < j; i, j = i+1, j-1 {
		inner[i], inner[j] = inner[j], inner[i]
	}
	return Path{outer, inner}
}

// toRGBA converts any image to *image.RGBA (premultiplied), for fast blits.
func toRGBA(img image.Image) *image.RGBA {
	if r, ok := img.(*image.RGBA); ok && r.Rect.Min == (image.Point{}) {
		return r
	}
	b := img.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	xdraw.Draw(out, out.Rect, img, b.Min, xdraw.Src)
	return out
}
