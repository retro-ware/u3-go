package ui

import (
	"image"
	"image/color"
	"math"
)

// Paints a first-person dungeon sheet in the style of a tile set, in the
// layout the renderer expects (dungeonArt.ts). LairWare's Mac version had
// one photographic sheet (kept for the Lairware set); the 8-bit and PC
// originals drew wireframes or flat bricks, so each other set gets a small
// style record and its sheet is painted from it when the set is chosen.
//
// The geometry comes from the renderer's tables (sheetRegions) and the
// mask, traced column by column so the lines and brick courses of each
// angled side wall follow its silhouette.

// DungeonStyle is how a set's dungeon looks.
type DungeonStyle struct {
	Brick     bool    // filled walls with mortar lines, else a wireframe
	Bg        string  // the ground, and the inside of wire walls unless Fill says otherwise
	Line      string  // wire: the line colour; brick: the mortar
	Fill      string  // wire only: fill walls with this instead of Bg
	Face      string  // brick: the facing wall colour
	Side      string  // brick: the side wall colour
	Dither    bool    // brick: checker-dither the side walls with the mortar
	Variation float64 // brick: how much each brick's shade varies, 0..1
	Floor     string
	FloorLine string
	Ceiling   string
	Wood      string // chest and ladder
	DoorFill  string // a doorway's fill (default black)
	DoorLine  string // and outline (default none)
}

var vgaDungeon = &DungeonStyle{Brick: true, Bg: "#0c0c10", Line: "#2a2a30", Face: "#7c7c84", Side: "#5c5c64",
	Variation: 0.22, Floor: "#3a3a40", FloorLine: "#4a4a50", Ceiling: "#1a1a20", Wood: "#9a6a3a"}

// dungeonStyles has a style for every set but Lairware. Standard takes the VGA dungeon.
var dungeonStyles = map[string]*DungeonStyle{
	"Standard":       vgaDungeon,
	"PC VGA":         vgaDungeon,
	"Apple II Mono":  {Bg: "#000", Line: "#8cf88c", Wood: "#8cf88c", DoorFill: "#000", DoorLine: "#8cf88c"},
	"Apple II Color": {Bg: "#000", Line: "#fff", Wood: "#ff8000", DoorFill: "#000", DoorLine: "#fff"},
	"Commodore 64":   {Bg: "#000", Line: "#8e8dff", Wood: "#a57a4c", DoorFill: "#000", DoorLine: "#8e8dff"},
	"Macintosh B&W":  {Bg: "#fff", Line: "#000", Wood: "#000", DoorFill: "#000"},
	"PC CGA":         {Bg: "#000", Line: "#55ffff", Fill: "#0000aa", Wood: "#ffffff", DoorFill: "#000", DoorLine: "#55ffff"},
	"PC EGA": {Brick: true, Bg: "#000", Line: "#000", Face: "#aa5500", Side: "#aa5500", Dither: true,
		Floor: "#555555", FloorLine: "#aaaaaa", Ceiling: "#000", Wood: "#ffff55"},
	"Nintendo": {Brick: true, Bg: "#000", Line: "#301810", Face: "#a84030", Side: "#782c20",
		Floor: "#404040", FloorLine: "#585858", Ceiling: "#000", Wood: "#d09050"},
	"PC MCGA": {Brick: true, Bg: "#101010", Line: "#3a2a1a", Face: "#8a6a4a", Side: "#66503a", Variation: 0.18,
		Floor: "#484848", FloorLine: "#585858", Ceiling: "#202020", Wood: "#b07a3a"},
	"PC Ultima V": {Brick: true, Bg: "#0a0c10", Line: "#1a2028", Face: "#5c6c7c", Side: "#44505c", Variation: 0.2,
		Floor: "#2c3038", FloorLine: "#3c4048", Ceiling: "#101418", Wood: "#8a5a2a"},
}

const (
	vpX, vpY      = 300, 256
	courses       = 8 // brick courses per wall height
	bricksPerCell = 4 // bricks along one cell of side wall
)

// leftWalls are the wall positions whose near edge is on the left.
var leftWalls = map[int]bool{1: true, 6: true, 7: true, 15: true, 16: true, 17: true, 28: true, 29: true}

// silhouette is the opaque extent of each column of a masked piece.
type silhouette struct {
	first, last int
	top, bottom []int // per column; -1 where transparent
}

func trace(mask *image.RGBA, r sheetRegion) *silhouette {
	s := &silhouette{first: -1, last: -1, top: make([]int, r.w), bottom: make([]int, r.w)}
	b := mask.Bounds()
	for x := range r.w {
		s.top[x], s.bottom[x] = -1, -1
		mx := r.sx + x
		if mx >= b.Dx() {
			break
		}
		for y := range r.h {
			my := r.sy + y
			if my < 0 || my >= b.Dy() || mask.Pix[mask.PixOffset(mx, my)] >= 128 {
				continue // white in the mask is transparent
			}
			if s.top[x] < 0 {
				s.top[x] = y
			}
			s.bottom[x] = y
		}
		if s.top[x] >= 0 {
			if s.first < 0 {
				s.first = x
			}
			s.last = x
		}
	}
	if s.first < 0 {
		return nil
	}
	return s
}

// rng is a small deterministic xorshift, so a set's bricks look the same every visit.
func rng(seed uint32) func() float64 {
	s := seed
	if s == 0 {
		s = 1
	}
	return func() float64 {
		s ^= s << 13
		s ^= s >> 17
		s ^= s << 5
		return float64(s%10000) / 10000
	}
}

// shade scales a colour's channels by factor.
func shade(c color.RGBA, factor float64) color.RGBA {
	f := func(v uint8) uint8 { return uint8(math.Max(0, math.Min(255, math.Round(float64(v)*factor)))) }
	return rgb(f(c.R), f(c.G), f(c.B))
}

// paintDungeonSheet paints the whole sheet for a style over the mask the renderer will apply.
func paintDungeonSheet(st *DungeonStyle, mask *image.RGBA) *image.RGBA {
	c := NewCanvas(dungeonSheetW, dungeonSheetH)
	c.Fill(c.Rect, hexColour(st.Bg))
	p := painter{c, st}
	for _, r := range sheetRegions() {
		switch {
		case r.name == "corridor":
			p.corridor(r)
		case r.name == "chest":
			p.chest(r)
		case r.name == "ladder rung" || r.name == "ladder rail":
			c.Fill(image.Rect(r.sx, r.sy, r.sx+r.w, r.sy+r.h), hexColour(st.Wood))
		case r.masked:
			if s := trace(mask, r); s != nil {
				p.sideWall(r, s, leftWalls[r.index])
			}
		default:
			p.facingWall(r)
		}
	}
	return c.RGBA
}

type painter struct {
	c  *Canvas
	st *DungeonStyle
}

func colourOr(s, fallback string) color.RGBA {
	if s == "" {
		return hexColour(fallback)
	}
	return hexColour(s)
}

// corridor is the empty corridor: ceiling and floor triangles meeting at
// the vanishing point, floor joints, and the far end fading to the ground.
func (p painter) corridor(r sheetRegion) {
	if !p.st.Brick {
		return // nothing but the ground: wire walls draw themselves
	}
	ox := float64(r.sx)
	clip := image.Rect(r.sx, 0, r.sx+600, 512)
	p.c.FillPolygon(Path{{{ox, 0}, {ox + 600, 0}, {ox + vpX, vpY}}}, colourOr(p.st.Ceiling, p.st.Bg))
	floor := []vec{{ox, 512}, {ox + 600, 512}, {ox + vpX, vpY}}
	p.c.FillPolygon(Path{floor}, colourOr(p.st.Floor, p.st.Bg))
	// Floor lines, clipped to the floor: strokes drawn into a scratch canvas, then copied through the floor's shape.
	lines := &Canvas{image.NewRGBA(image.Rect(0, 0, 600, 512))}
	var segs [][4]float64
	for i := -6; i <= 6; i++ {
		segs = append(segs, [4]float64{float64(vpX + i*150), 512, vpX, vpY})
	}
	for z := 1; z <= 8; z++ {
		y := vpY + 256/float64(z)
		segs = append(segs, [4]float64{0, y, 600, y})
	}
	lines.StrokeLines(segs, 2, colourOr(p.st.FloorLine, p.st.Line), false)
	floorMask := &Canvas{image.NewRGBA(image.Rect(0, 0, 600, 512))}
	floorMask.FillPolygon(Path{{{0, 512}, {600, 512}, {vpX, vpY}}}, rgb(255, 255, 255))
	for i := 0; i < len(lines.Pix); i += 4 {
		lines.Pix[i+3] = uint8(int(lines.Pix[i+3]) * int(floorMask.Pix[i+3]) / 255)
		for k := range 3 {
			lines.Pix[i+k] = uint8(int(lines.Pix[i+k]) * int(floorMask.Pix[i+3]) / 255)
		}
	}
	p.c.Blit(lines, lines.Rect, clip, true)
	// The radial fade: the ground at full strength out to 35% of 330 pixels, then fading out.
	bg := hexColour(p.st.Bg)
	for y := range 512 {
		for x := range 600 {
			d := math.Hypot(float64(x-vpX), float64(y-vpY))
			t := (d - 10) / 320
			a := 1.0
			if t > 0.35 {
				a = math.Max(0, 1-(t-0.35)/0.65)
			}
			if a <= 0 {
				continue
			}
			i := p.c.PixOffset(r.sx+x, y)
			p.c.Pix[i] = uint8(float64(p.c.Pix[i])*(1-a) + float64(bg.R)*a)
			p.c.Pix[i+1] = uint8(float64(p.c.Pix[i+1])*(1-a) + float64(bg.G)*a)
			p.c.Pix[i+2] = uint8(float64(p.c.Pix[i+2])*(1-a) + float64(bg.B)*a)
		}
	}
}

// facingWall is a wall seen head on: a framed rectangle, or bricks laid in
// screen space, halving with distance, so neighbouring pieces tile.
func (p painter) facingWall(r sheetRegion) {
	area := image.Rect(r.sx, r.sy, r.sx+r.w, r.sy+r.h)
	if !p.st.Brick {
		p.c.Fill(area, colourOr(p.st.Fill, p.st.Bg))
		x, y, w, h := float64(r.sx+1), float64(r.sy+1), float64(r.w-2), float64(r.h-2)
		p.c.StrokePolygon([]vec{{x, y}, {x + w, y}, {x + w, y + h}, {x, y + h}}, 2, hexColour(p.st.Line), area)
		return
	}
	bh := math.Max(4, float64(r.h)/courses)
	bw := bh * 2
	mortar := math.Max(1, math.Round(bh/12))
	random := rng(uint32(1000 + r.index))
	face := hexColour(p.st.Face)
	sub := &Canvas{p.c.SubImage(area).(*image.RGBA)}
	sub.Fill(area, hexColour(p.st.Line))
	dx, dy := float64(r.dx), float64(r.dy)
	for row := int(math.Floor(dy / bh)); row <= int(math.Ceil((dy+float64(r.h))/bh)); row++ {
		stagger := 0.0
		if row%2 != 0 {
			stagger = bw / 2
		}
		for col := int(math.Floor((dx-stagger)/bw)) - 1; col <= int(math.Ceil((dx+float64(r.w)-stagger)/bw))+1; col++ {
			x := float64(col)*bw + stagger - dx + float64(r.sx)
			y := float64(row)*bh - dy + float64(r.sy)
			c := face
			if p.st.Variation != 0 {
				c = shade(face, 1+(random()-0.5)*2*p.st.Variation)
			}
			sub.Fill(frect(x+mortar, y+mortar, bw-mortar, bh-mortar), c)
		}
	}
}

// sideWall is an angled wall, traced from the mask.
func (p painter) sideWall(r sheetRegion, s *silhouette, left bool) {
	area := image.Rect(r.sx, r.sy, r.sx+r.w, r.sy+r.h)
	sub := &Canvas{p.c.SubImage(area).(*image.RGBA)}
	near, far := s.last, s.first
	if left {
		near, far = s.first, s.last
	}
	topAt := func(x int) float64 { return float64(r.sy + s.top[x]) }
	bottomAt := func(x int) float64 { return float64(r.sy + s.bottom[x] + 1) }
	column := func(x int, c color.RGBA) {
		sub.Fill(image.Rect(r.sx+x, int(topAt(x)), r.sx+x+1, int(bottomAt(x))), c)
	}
	line := hexColour(p.st.Line)

	if !p.st.Brick {
		fill := colourOr(p.st.Fill, p.st.Bg)
		var outline []vec
		outline = append(outline, vec{float64(r.sx + s.first + 1), topAt(s.first) + 1})
		for x := s.first; x <= s.last; x++ {
			if s.top[x] >= 0 {
				column(x, fill)
				outline = append(outline, vec{float64(r.sx+x) + 0.5, topAt(x) + 1})
			}
		}
		outline = append(outline, vec{float64(r.sx+s.last) + 0.5, bottomAt(s.last) - 1})
		for x := s.last; x >= s.first; x-- {
			if s.top[x] >= 0 {
				outline = append(outline, vec{float64(r.sx+x) + 0.5, bottomAt(x) - 1})
			}
		}
		p.c.StrokePolygon(outline, 2, line, area)
		return
	}

	side := colourOr(p.st.Side, p.st.Face)
	for x := s.first; x <= s.last; x++ {
		if s.top[x] >= 0 {
			column(x, side)
		}
	}
	if p.st.Dither {
		for x := s.first; x <= s.last; x++ {
			if s.top[x] < 0 {
				continue
			}
			for y := int(topAt(x)) + (x+r.sx)&1; y < int(bottomAt(x)); y += 2 {
				sub.Fill(image.Rect(r.sx+x, y, r.sx+x+1, y+1), line)
			}
		}
	}
	mortar := math.Max(1, math.Round((bottomAt(near)-topAt(near))/courses/12))
	// Courses: a fixed fraction of the way down every column.
	var segs [][4]float64
	for i := 1; i < courses; i++ {
		px, py := -1.0, 0.0
		for x := s.first; x <= s.last; x++ {
			if s.top[x] < 0 {
				continue
			}
			y := topAt(x) + (bottomAt(x)-topAt(x))*float64(i)/courses
			if px >= 0 {
				segs = append(segs, [4]float64{px, py, float64(r.sx + x), y})
			}
			px, py = float64(r.sx+x), y
		}
	}
	strokes := &Canvas{image.NewRGBA(image.Rect(0, 0, r.w, r.h))}
	for i := range segs {
		segs[i][0] -= float64(r.sx)
		segs[i][2] -= float64(r.sx)
		segs[i][1] -= float64(r.sy)
		segs[i][3] -= float64(r.sy)
	}
	strokes.StrokeLines(segs, mortar, line, false)
	p.c.Blit(strokes, strokes.Rect, area, true)
	// Joints: a joint a fraction t along the one-cell wall sits where perspective puts it.
	nearX := float64(r.dx + near - vpX)
	farX := float64(r.dx + far - vpX)
	z0 := math.Abs(farX) / math.Max(1, math.Abs(nearX)-math.Abs(farX))
	xAt := func(t float64) float64 { return nearX*(z0/(z0+t)) + vpX - float64(r.dx) }
	for i := range courses {
		stagger := 0.0
		if i%2 != 0 {
			stagger = 0.5
		}
		for j := 0; j <= bricksPerCell; j++ {
			t := (float64(j) + stagger) / bricksPerCell
			if t <= 0 || t >= 1 {
				continue
			}
			x := int(math.Round(xAt(t)))
			if x < s.first || x > s.last || s.top[x] < 0 {
				continue
			}
			y0 := topAt(x) + (bottomAt(x)-topAt(x))*float64(i)/courses
			y1 := topAt(x) + (bottomAt(x)-topAt(x))*float64(i+1)/courses
			sub.Fill(frect(float64(r.sx+x), y0, mortar, y1-y0), line)
		}
	}
}

// chest is a box with a lid and a lock.
func (p painter) chest(r sheetRegion) {
	lid := float64(r.h) * 0.35
	x, y, w, h := float64(r.sx), float64(r.sy), float64(r.w), float64(r.h)
	area := image.Rect(r.sx, r.sy, r.sx+r.w, r.sy+r.h)
	line := hexColour(p.st.Line)
	if !p.st.Brick {
		p.c.Fill(area, colourOr(p.st.Fill, p.st.Bg))
		p.c.StrokePolygon([]vec{{x + 1, y + 1}, {x + w - 1, y + 1}, {x + w - 1, y + h - 1}, {x + 1, y + h - 1}}, 2, line, area)
		p.c.StrokeLines([][4]float64{{x + 1, y + lid, x + w - 1, y + lid}}, 2, line, false)
		lx, ly := x+w/2-4, y+lid-4
		p.c.StrokePolygon([]vec{{lx, ly}, {lx + 8, ly}, {lx + 8, ly + 12}, {lx, ly + 12}}, 2, line, area)
		return
	}
	wood := hexColour(p.st.Wood)
	p.c.Fill(area, wood)
	p.c.Fill(frect(x, y, w, lid), shade(wood, 0.6))
	p.c.Fill(frect(x, y+lid, w, 2), shade(wood, 0.6))
	p.c.Fill(frect(x+w/2-5, y+lid-5, 10, 14), line)
	p.c.Fill(frect(x+w/2-2, y+lid+1, 4, 4), wood)
}
