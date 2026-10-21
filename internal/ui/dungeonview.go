package ui

import (
	"image"

	"github.com/retro-ware/u3-go/internal/game"
)

// The first-person dungeon view: the drawing half of UltimaDngn.c
// (DrawWall, DrawDoor, DrawChest, DrawLadder, DrawDungeonBackGround,
// DimDungeon, ShowSecret), after dungeonView.ts.
//
// The view is composed at 600x512, the size the original used, and scaled
// into the map window. Wall pieces come from a 3000x512 sheet: the first
// 1200 pixels hold the wall pieces (a mask sheet gives the angled side
// walls their shape), and 1800..2400 is the empty corridor. The tables are
// in the original's half-resolution units and doubled when used.

// Wall piece rectangles for the 32 view locations (and 33-35: the chest and ladder art).
var (
	dngL = []int{0, 0, 225, 0, 75, 225, 0, 75, 187, 225, 0, 75, 113, 187, 225, 0, 75, 113, 170, 188, 226, 0, 75, 112, 130, 170, 188, 225, 112, 130, 152, 168, 0, 300, 375, 377}
	dngT = []int{0, 0, 0, 64, 64, 64, 64, 65, 65, 64, 96, 96, 96, 96, 96, 96, 96, 97, 97, 96, 96, 112, 112, 112, 112, 112, 112, 112, 112, 112, 112, 112, 0, 208, 208, 208}
	dngR = []int{300, 75, 300, 75, 225, 300, 75, 113, 225, 300, 75, 113, 187, 225, 300, 74, 112, 130, 187, 225, 300, 75, 112, 130, 170, 188, 225, 300, 132, 148, 170, 188, 300, 375, 377, 381}
	dngB = []int{256, 256, 256, 192, 192, 192, 192, 191, 191, 192, 160, 160, 160, 160, 160, 160, 160, 159, 159, 160, 160, 144, 144, 144, 144, 144, 144, 144, 144, 144, 144, 144, 256, 240, 212, 256}
	// Where in the sheet each location's piece is, relative to its screen rectangle.
	ofX = []int{600, 0, 0, 1200, 1200, 1200, 300, 0, 0, 300, 1200, 1200, 1200, 1200, 1200, 300, 300, 0, 0, 300, 300, 1200, 1200, 1200, 1200, 1200, 1200, 1200, 300, 0, 0, 300}
	ofY = []int{0, 0, 0, -64, -64, -64, -64, 0, 0, -64, 34, 34, 34, 34, 34, 34, -64, 0, 0, -64, 34, 84, 84, 84, 84, 84, 84, 84, -64, 0, 0, -64}
	// Whether the piece goes through the mask (side walls seen at an angle).
	useMask = []bool{false, true, true, false, false, false, true, true, true, true, false, false, false, false, false, true, true, true, true, true, true, false, false, false, false, false, false, false, true, true, true, true}

	// Chest and ladder placement for locations 0..20.
	chL = []int{112, 0, 264, 56, 131, 226, 0, 75, 207, 272, 66, 103, 141, 188, 226, 28, 75, 113, 179, 216, 254}
	chT = []int{208, 208, 208, 168, 168, 168, 168, 168, 168, 168, 148, 148, 148, 148, 148, 148, 148, 148, 148, 148, 148}
	chR = []int{186, 36, 300, 74, 169, 244, 18, 93, 225, 300, 74, 112, 159, 197, 234, 46, 84, 121, 187, 225, 272}
	chB = []int{240, 240, 240, 184, 184, 184, 184, 184, 184, 184, 156, 156, 156, 156, 156, 156, 156, 156, 156, 156, 156}
	// 0 seen head on, 1 only the left half visible, 2 only the right half.
	chSide = []int{0, 2, 1, 1, 0, 2, 2, 2, 1, 1, 1, 1, 0, 2, 2, 0, 2, 2, 1, 1, 0}

	// Door polygons.
	doorX1 = []int{74, 0, 300, 0, 114, 300, 20, 88, 212, 280, 36, 79, 135, 221, 264, 24, 87, 118, 182, 213, 276, 40, 87, 114, 143, 186, 213, 260, 118, 136, 164, 182}
	doorX2 = []int{226, 40, 260, 37, 186, 263, 48, 102, 198, 252, 64, 108, 164, 192, 236, 48, 101, 124, 176, 199, 252, 54, 100, 127, 156, 173, 200, 246, 126, 140, 160, 174}
	doorY1 = []int{256, 256, 256, 191, 191, 191, 182, 179, 179, 182, 159, 159, 159, 159, 159, 154, 153, 153, 153, 153, 154, 143, 143, 143, 143, 143, 143, 143, 140, 138, 138, 140}
	doorY2 = []int{64, 75, 75, 95, 95, 95, 96, 100, 100, 96, 112, 112, 112, 112, 112, 112, 110, 110, 110, 110, 112, 120, 120, 120, 120, 120, 120, 120, 122, 124, 124, 122}
	doorY3 = []int{64, 92, 92, 95, 95, 95, 102, 106, 106, 102, 112, 112, 112, 112, 112, 116, 114, 115, 115, 114, 116, 120, 120, 120, 120, 120, 120, 120, 124, 126, 126, 124}
	doorY4 = []int{256, 220, 220, 191, 191, 191, 170, 168, 168, 170, 159, 159, 159, 159, 159, 149, 147, 148, 148, 147, 149, 143, 143, 143, 143, 143, 143, 143, 137, 134, 134, 137}
)

const (
	dungeonViewW, dungeonViewH   = 600, 512
	dungeonSheetW, dungeonSheetH = 3000, 512
)

// sheetRegion is one rectangle of the sheet the renderer samples.
type sheetRegion struct {
	name   string
	index  int // the wall's location, or -1
	sx, sy int
	w, h   int
	dx, dy int // where it lands in the view
	masked bool
}

// sheetRegions are every rectangle of the shapes sheet the renderer reads:
// the 32 wall positions, the chest, the two ladder pieces and the corridor.
// Another set's sheet must carry its art in these places.
func sheetRegions() []sheetRegion {
	var out []sheetRegion
	for i := range 32 {
		x, y := dngL[i]*2, dngT[i]*2
		out = append(out, sheetRegion{name: "wall", index: i, sx: x + ofX[i]*2, sy: y + ofY[i]*2,
			w: (dngR[i] - dngL[i]) * 2, h: (dngB[i] - dngT[i]) * 2, dx: x, dy: y, masked: useMask[i]})
	}
	for i, name := range map[int]string{33: "chest", 34: "ladder rung", 35: "ladder rail"} {
		out = append(out, sheetRegion{name: name, index: -1, sx: dngL[i] * 2, sy: dngT[i] * 2,
			w: (dngR[i] - dngL[i]) * 2, h: (dngB[i] - dngT[i]) * 2})
	}
	return append(out, sheetRegion{name: "corridor", index: -1, sx: 1800, w: 600, h: 512})
}

// dungeonRenderer composes the first-person view from a set's sheet.
type dungeonRenderer struct {
	shapes, masked *image.RGBA
	style          *DungeonStyle
	view           *Canvas
	gfx            *Graphics // for the font of the secret writing
}

func newDungeonRenderer(g *Graphics) *dungeonRenderer {
	if g.dungeonShapes == nil || g.dungeonMasks == nil {
		return nil
	}
	return &dungeonRenderer{
		shapes: g.dungeonShapes,
		masked: applyMask(g.dungeonShapes, g.dungeonMasks),
		style:  g.dungeonStyle,
		view:   NewCanvas(dungeonViewW, dungeonViewH),
		gfx:    g,
	}
}

// render composes the view for a draw list. A torch below 3 dims it; secret is writing over it.
func (d *dungeonRenderer) render(ops []game.DungeonDrawOp, torch int, secret string) *Canvas {
	v := d.view
	v.Blit(d.shapes, image.Rect(1800, 0, 2400, 512), v.Rect, false)
	for _, op := range ops {
		switch op.Kind {
		case "wall":
			d.wall(op.Location)
		case "door":
			d.door(op.Location)
		case "chest":
			d.chest(op.Location)
		case "ladder":
			d.ladder(op.Location, op.Down)
		}
	}
	if secret != "" {
		d.secret(secret)
	}
	if torch < 3 {
		v.FillAlpha(v.Rect, rgb(0, 0, 0), 0.57)
	}
	return v
}

// wall mirrors DrawWall().
func (d *dungeonRenderer) wall(at int) {
	if at < 0 || at > 31 {
		return
	}
	x, y := dngL[at]*2, dngT[at]*2
	w, h := (dngR[at]-dngL[at])*2, (dngB[at]-dngT[at])*2
	sx, sy := x+ofX[at]*2, y+ofY[at]*2
	src := d.shapes
	if useMask[at] {
		src = d.masked
	}
	d.view.Blit(src, image.Rect(sx, sy, sx+w, sy+h), image.Rect(x, y, x+w, y+h), useMask[at])
}

// door mirrors DrawDoor(): a black trapezoid in the wall (a painted style
// may fill and outline it its own way).
func (d *dungeonRenderer) door(at int) {
	if at < 0 || at > 31 {
		return
	}
	poly := []vec{
		{float64(doorX1[at] * 2), float64(doorY1[at] * 2)},
		{float64(doorX1[at] * 2), float64(doorY2[at] * 2)},
		{float64(doorX2[at] * 2), float64(doorY3[at] * 2)},
		{float64(doorX2[at] * 2), float64(doorY4[at] * 2)},
	}
	fill := rgb(0, 0, 0)
	if d.style != nil && d.style.DoorFill != "" {
		fill = hexColour(d.style.DoorFill)
	}
	d.view.FillPolygon(Path{poly}, fill)
	if d.style != nil && d.style.DoorLine != "" {
		d.view.StrokePolygon(poly, 2, hexColour(d.style.DoorLine), d.view.Rect)
	}
}

// chest mirrors DrawChest().
func (d *dungeonRenderer) chest(at int) {
	if at < 0 || at > 20 {
		return
	}
	const shape = 33
	sx, sw := dngL[shape]*2, (dngR[shape]-dngL[shape])*2
	sy, sh := dngT[shape]*2, (dngB[shape]-dngT[shape])*2
	half := sw / 2
	switch chSide[at] {
	case 1:
		sw -= half
	case 2:
		sx += half - 1
		sw -= half - 1
	}
	dx, dy := chL[at]*2, chT[at]*2
	dw, dh := (chR[at]+1)*2-dx, (chB[at]+1)*2-dy
	d.view.Blit(d.shapes, image.Rect(sx, sy, sx+sw, sy+sh), image.Rect(dx, dy, dx+dw, dy+dh), false)
}

// ladder mirrors DrawLadder(): rails and a rung from two small textures.
func (d *dungeonRenderer) ladder(at int, down bool) {
	if at < 0 || at > 20 {
		return
	}
	left := float64(chL[at] * 2)
	top := float64(chT[at] * 2)
	right := float64(chR[at] * 2)
	bottom := float64(chB[at] * 2)
	top = bottom - (bottom-top)*1.5
	base := bottom
	if !down {
		swap := 512 - bottom
		bottom = 512 - top
		top = swap
		base = top
	}
	side := chSide[at]
	height := ((bottom-top)*1.25 - (bottom - top)) / 2
	width := (bottom - top) / 12
	rung := (bottom-top)/2 + top - width/2
	half := left + (right-left)/2
	if side == 1 {
		half = right
	}
	if side == 2 {
		half = left
	}
	// The hole in the floor or ceiling.
	d.view.Fill(frect(left-width-2, base-height, right-left+2*width+4, 2*height), rgb(0, 0, 0))
	rail := func(x float64) { d.piece(35, x, top, width, bottom-top) }
	bar := func(x1, x2 float64) { d.piece(34, x1, rung, x2-x1, width) }
	if side == 0 || side == 1 {
		rail(left)
		bar(left, half+1)
	}
	if side == 0 || side == 2 {
		rail(right - width)
		bar(half, right)
	}
}

func (d *dungeonRenderer) piece(shape int, dx, dy, dw, dh float64) {
	sx, sy := dngL[shape]*2, dngT[shape]*2
	sw, sh := (dngR[shape]-dngL[shape])*2, (dngB[shape]-dngT[shape])*2
	d.view.Blit(d.shapes, image.Rect(sx, sy, sx+sw, sy+sh), frect(dx, dy, dw, dh), false)
}

// frect is a rectangle from fractional x, y, width and height.
func frect(x, y, w, h float64) image.Rectangle {
	return image.Rect(int(x+0.5), int(y+0.5), int(x+w+0.5), int(y+h+0.5))
}

// secret mirrors ShowSecret(): ghostly writing across the middle, in the
// set's font at 48 pixels: a light shadow, a dark one, then grey.
func (d *dungeonRenderer) secret(text string) {
	const size = 40
	w := len(text) * size
	x0, y0 := 300-w/2, 282-size/2
	layer := func(dx, dy int, col [3]uint8) {
		tmp := NewCanvas(w, size)
		for i := range len(text) {
			d.gfx.drawGlyph(tmp, text[i], i*size, 0, size)
		}
		for y := range size {
			for x := range w {
				i := tmp.PixOffset(x, y)
				lum := int(tmp.Pix[i]) // the glyphs are white on black
				if lum < 128 {
					continue
				}
				px, py := x0+dx+x, y0+dy+y
				if !(image.Point{px, py}.In(d.view.Rect)) {
					continue
				}
				j := d.view.PixOffset(px, py)
				for k := range 3 {
					d.view.Pix[j+k] = uint8((int(d.view.Pix[j+k])*4 + int(col[k])*6) / 10)
				}
			}
		}
	}
	layer(1, 1, [3]uint8{255, 255, 255})
	layer(-1, -1, [3]uint8{0, 0, 0})
	layer(0, 0, [3]uint8{0x88, 0x88, 0x88})
}

// syntheticDungeonMask draws the side walls' mask from the view's geometry,
// for an import without LairWare's DungeonMasks.png (the PC release): each
// angled wall is the band between its near edge and the vanishing point.
func syntheticDungeonMask() image.Image {
	m := NewCanvas(1200, 512)
	m.Fill(m.Rect, rgb(255, 255, 255))
	const vpX, vpY = 300.0, 256.0
	for _, r := range sheetRegions() {
		if r.name != "wall" || !r.masked {
			continue
		}
		left := leftWalls[r.index]
		nearX := float64(r.dx)
		if !left {
			nearX = float64(r.dx + r.w)
		}
		top := float64(r.dy)
		for x := range r.w {
			sx := float64(r.dx + x)
			dist := sx - nearX
			if !left {
				dist = nearX - sx - 1
			}
			span := vpX - nearX
			if !left {
				span = nearX - vpX
			}
			t := top + dist*(vpY-top)/span
			for y := int(t + 0.5); y < int(512-t+0.5); y++ {
				py := r.sy + y - r.dy
				if py >= 0 && py < 512 && r.sx+x < 1200 {
					i := m.PixOffset(r.sx+x, py)
					m.Pix[i], m.Pix[i+1], m.Pix[i+2] = 0, 0, 0
				}
			}
		}
	}
	return m.RGBA
}
