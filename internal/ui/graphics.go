package ui

import (
	"bytes"
	"errors"
	"image"
	_ "image/gif"  // LairWare's sheets
	_ "image/jpeg" // LairWare's scenes and dungeon
	_ "image/png"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/retro-ware/u3-go/assets"
	"github.com/retro-ware/u3-go/internal/game"
	"github.com/retro-ware/u3-go/internal/res"
)

// A tile set is a family of images named "<Set>-Tiles", "-Font" and "-UI"
// (after LairWare's layout, its Graphics folder's "_About This Folder_"):
//
//	Tiles  12 columns x 16 rows. Column pairs hold the two animation frames
//	       of a tile: tile index i is at row i % 16, column (i / 16) * 2, and
//	       its alternate frame is one column to the right. The sheet's alpha
//	       (or the set's "-Mask", black opaque) is the transparency used when
//	       drawing creatures over terrain; sets with neither draw them opaque.
//	Font   96 glyphs in one row, from ASCII 0x20.
//	UI     16 columns x 3 rows of border pieces, cursor frames, moon phases.
//
// Sheets come from the player's import (data/mac/Graphics) and, for the
// port's own art, from package assets. Images may be any resolution; they
// are scaled to the cell when drawn. This file also owns the tile animation
// state that FullUpdate() advanced every idle tick in the original.

const (
	tileColumns = 12
	tileRows    = 16
	fontGlyphs  = 96
	uiColumns   = 16
	// exodusIndex is the Exodus tile (map value 0x7c); its four light panels
	// are the alternate-frame cells of tiles 32-35 (column 5).
	exodusIndex       = game.MapValueExodus >> 2
	exodusPanelColumn = 5
)

// TileSets are the sets the Settings menu lists, best first: Standard (the
// port's art), Lairware (the Mac's own), the colour sets, the Apple II
// colour set, the monochrome sets and the 16-pixel 8-bit and early PC sets.
var TileSets = []string{
	"Standard", "Lairware", "PC VGA", "Nintendo", "PC Ultima V", "PC MCGA",
	"Apple II Color", "Apple II Mono", "Macintosh B&W", "PC EGA", "PC CGA", "Commodore 64",
}

// sceneNames are the pictures a set paints in its own style: the four
// full-window scenes and the title logo.
var sceneNames = []string{"Fountain", "Rod", "Shrine", "TimeLord", "Exodus"}

// classFigureSets draw one figure per class in cells 68-78; the others show
// the shared figures there (game.ClassFallbackTile).
var classFigureSets = map[string]bool{"Standard": true}

// fileStem is a set's name as its embedded files carry it ("&" dropped:
// "Macintosh B&W" is "Macintosh BW-Tiles.png").
func fileStem(set string) string { return strings.ReplaceAll(set, "&", "") }

// Graphics is one loaded tile set and its animation state. (GraphicsSet)
type Graphics struct {
	Name string

	masked, opaque *image.RGBA // the sheet with its transparency, and over black
	tileSize       int
	font           *image.RGBA
	fontW, fontH   int
	ui             *image.RGBA
	uiSize         int

	dungeonShapes *image.RGBA   // the 3000x512 first-person sheet (dungeonview.go)
	dungeonMasks  *image.RGBA   // its side walls' mask
	dungeonStyle  *DungeonStyle // the style it was painted from, or nil for a loaded sheet
	scenes        map[string]*image.RGBA
	moons         *image.RGBA // moons painted for the set (moonart.go), or nil for the UI sheet's
	classFigures  bool
	display       DisplayStyle

	// Animation (gShapeSwapped, the scroll offsets, exoduslitez, twiddleFlag, animFlag).
	swapped             [tileColumns * tileRows]bool
	scroll              map[int]int
	exodusFrame         int
	exodusSkip          int
	twiddle             []int
	animPhase, animSkip int
}

// source finds the sheets and pictures of a set in the import and the
// embedded assets.
type source struct {
	data string // the -data directory
}

// load decodes the first file found among the names, in the import's
// Graphics folder and then the embedded assets as the order says.
func (s source) load(names []string, embedded bool, imported bool) image.Image {
	for _, n := range names {
		if imported {
			for _, ext := range []string{".png", ".gif", ".pict", ".jpg"} {
				if img := decodeFile(filepath.Join(s.data, "mac", "Graphics", n+ext)); img != nil {
					return img
				}
			}
		}
		if embedded {
			if b, err := fs.ReadFile(assets.FS, "graphics/"+fileStem(n)+".png"); err == nil {
				if img, _, err := image.Decode(bytes.NewReader(b)); err == nil {
					return img
				}
			}
		}
	}
	return nil
}

// macFile decodes one of the Mac's loose pictures (data/mac/Fountain.jpg).
func (s source) macFile(name string) image.Image {
	for _, ext := range []string{".jpg", ".png"} {
		if img := decodeFile(filepath.Join(s.data, "mac", name+ext)); img != nil {
			return img
		}
	}
	return nil
}

func decodeFile(path string) image.Image {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	if strings.EqualFold(filepath.Ext(path), ".pict") {
		img, err := decodePICT(b)
		if err != nil {
			return nil
		}
		return img
	}
	img, _, err := image.Decode(bytes.NewReader(b))
	if err != nil {
		return nil
	}
	return img
}

// sheetName is the name a set's files carry in the import: the Lairware set
// is LairWare's "Standard" sheets.
func sheetName(set string) string {
	if set == "Lairware" {
		return "Standard"
	}
	return set
}

// tilesFor finds a set's tile sheet: the port's own for Standard and Apple
// II Color, else the import's, the PC CGA set falling back on the PC
// release's SHAPES.ULT.
func (s source) tilesFor(set string) image.Image {
	name := sheetName(set) + "-Tiles"
	own := ownSheet(set)
	img := s.load([]string{name}, own, !own)
	if img == nil && set == "PC CGA" {
		img = pcSheet(s.data)
	}
	return img
}

// ownSheet says whether set's tiles are the port's (embedded) rather than the
// import's: LairWare ships sheets under the same names, in its own layout.
func ownSheet(set string) bool { return set == "Standard" || set == "Apple II Color" }

// Available lists the tile sets whose sheets are found, in TileSets' order.
func Available(data string) []string {
	s := source{data}
	var out []string
	for _, set := range TileSets {
		if s.tilesFor(set) != nil {
			out = append(out, set)
		}
	}
	return out
}

// LoadGraphics loads the named set. Missing optional files fall back to the
// Standard set's, as the original did.
func LoadGraphics(name, data string) (*Graphics, error) {
	s := source{data}
	tiles := s.tilesFor(name)
	if tiles == nil {
		return nil, errors.New("no tile sheet for " + name)
	}
	sheet := toRGBA(tiles)
	// The mask comes from where the sheet did: LairWare's Standard-Mask is for
	// its own 32-pixel sheet, not the port's.
	if mask := s.load([]string{sheetName(name) + "-Mask"}, false, !ownSheet(name)); mask != nil {
		sheet = applyMask(sheet, mask)
	}
	// The UI: the port's own frame for its sets, LairWare's, then Standard's.
	ui := s.load([]string{sheetName(name) + "-UI"}, name != "Lairware", true)
	if ui == nil {
		ui = s.load([]string{"Standard-UI"}, true, true)
	}
	font := s.load([]string{sheetName(name) + "-Font", "Standard-Font"}, false, true)
	if font == nil {
		font = pcFont(data)
	}
	if ui == nil || font == nil {
		return nil, errors.New("font or UI sheet missing for " + name)
	}
	g := &Graphics{
		Name:         name,
		masked:       sheet,
		tileSize:     sheet.Rect.Dx() / tileColumns,
		font:         toRGBA(font),
		ui:           toRGBA(ui),
		scenes:       map[string]*image.RGBA{},
		classFigures: classFigureSets[name],
		display:      displayOf(name),
		scroll:       map[int]int{},
		twiddle:      []int{3, 2, 1},
		animPhase:    16,
		animSkip:     5,
	}
	opaque := NewCanvas(sheet.Rect.Dx(), sheet.Rect.Dy())
	opaque.Blit(sheet, sheet.Rect, sheet.Rect, true)
	g.opaque = opaque.RGBA
	g.fontW, g.fontH = g.font.Rect.Dx()/fontGlyphs, g.font.Rect.Dy()
	g.uiSize = g.ui.Rect.Dx() / uiColumns

	// Dungeon art: the Lairware set's photographic sheet, else painted from
	// the set's style over LairWare's mask (or one made from the geometry).
	masks := s.macFile("DungeonMasks")
	if masks == nil {
		masks = syntheticDungeonMask()
	}
	g.dungeonMasks = toRGBA(masks)
	if name == "Lairware" {
		if shapes := s.macFile("DungeonShapes"); shapes != nil {
			g.dungeonShapes = toRGBA(shapes)
		}
	}
	if g.dungeonShapes == nil {
		style := dungeonStyles[name]
		if style == nil {
			style = dungeonStyles["Standard"]
		}
		g.dungeonStyle = style
		g.dungeonShapes = paintDungeonSheet(style, g.dungeonMasks)
	}

	for _, scene := range sceneNames {
		var img image.Image
		if name == "Lairware" {
			img = s.macFile(scene)
		} else {
			img = s.load([]string{fileStem(name) + "-" + scene}, true, false)
		}
		if img == nil && scene == "Exodus" {
			if img = s.macFile("Exodus"); img == nil {
				img = s.load([]string{"Standard-Exodus"}, true, false)
			}
		}
		if img != nil {
			g.scenes[scene] = toRGBA(img)
		}
	}
	if style, ok := moonStyles[name]; ok {
		g.moons = paintMoons(style)
	}
	return g, nil
}

// applyMask makes the sheet clear where the mask is light (black is opaque).
func applyMask(sheet *image.RGBA, mask image.Image) *image.RGBA {
	out := image.NewRGBA(sheet.Rect)
	copy(out.Pix, sheet.Pix)
	m := toRGBA(mask)
	w, h := min(m.Rect.Dx(), out.Rect.Dx()), min(m.Rect.Dy(), out.Rect.Dy())
	for y := range h {
		for x := range w {
			i := m.PixOffset(x, y)
			a := 255 - (int(m.Pix[i])+int(m.Pix[i+1])+int(m.Pix[i+2]))/3
			o := out.PixOffset(x, y)
			for k := range 4 {
				out.Pix[o+k] = uint8(int(out.Pix[o+k]) * a / 255)
			}
		}
	}
	return out
}

// pcSheet lays the PC release's own shapes (64 tiles, 16 pixels, CGA) out
// as a LairWare sheet. The cells the PC has no tile for get the figure the
// map would show there: the class figures the fighter, cleric, wizard and
// thief, the monster variants their monster; Exodus' panel cells keep
// Exodus, so he stands still.
func pcSheet(data string) image.Image {
	b, err := os.ReadFile(filepath.Join(data, "pc", "SHAPES.ULT"))
	if err != nil {
		return nil
	}
	tiles := res.Shapes(b)
	if len(tiles) == 0 {
		return nil
	}
	const size = 16
	c := NewCanvas(tileColumns*size, tileRows*size)
	for i := range tileColumns / 2 * tileRows {
		from := i
		switch {
		case i >= 64 && i < 68:
			from = 20 + i - 64
		case i >= 80:
			from = 23 + (i-80)/2
		case i >= len(tiles):
			from = 0
		}
		for f := range 2 {
			src := tiles[from][f]
			if i >= 32 && i < 36 && f == 1 {
				src = tiles[exodusIndex][0]
			}
			x, y := (i/tileRows*2+f)*size, i%tileRows*size
			c.Blit(src, src.Bounds(), image.Rect(x, y, x+size, y+size), false)
		}
	}
	return c.RGBA
}

// pcFont is the PC release's CHARSET.ULT as a font strip, for a set whose
// font is not in the import.
func pcFont(data string) image.Image {
	b, err := os.ReadFile(filepath.Join(data, "pc", "CHARSET.ULT"))
	if err != nil {
		return nil
	}
	glyphs := res.Charset(b)
	if len(glyphs) < 32+fontGlyphs {
		return nil
	}
	c := NewCanvas(fontGlyphs*8, 8)
	for i := range fontGlyphs {
		g := glyphs[32+i]
		c.Blit(g, g.Bounds(), image.Rect(i*8, 0, i*8+8, 8), false)
	}
	return c.RGBA
}

// tileRect is the sheet rectangle of a tile index, honouring the animation
// frame. (GetTileRectForIndex)
func (g *Graphics) tileRect(index int, frame int) image.Rectangle {
	s := g.tileSize
	if index == exodusIndex {
		return image.Rect(exodusPanelColumn*s, g.exodusFrame*s, (exodusPanelColumn+1)*s, (g.exodusFrame+1)*s)
	}
	if !g.classFigures && game.IsClassTile(index) {
		index = game.ClassFallbackTileFor(index)
	}
	if index < 0 || index >= len(g.swapped) {
		index = 0
	}
	alt := g.swapped[index]
	if frame >= 0 {
		alt = frame == 1
	}
	x := (index/tileRows*2 + b2i(alt)) * s
	y := index % tileRows * s
	return image.Rect(x, y, x+s, y+s)
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// shapeOpts say how a shape is drawn.
type shapeOpts struct {
	masked   bool // through the transparency (creatures and objects)
	flip     bool // mirrored (the horse facing east)
	altFrame bool // its second frame (the "HIT" balls)
}

// drawShape draws a shape (tile index * 2, or ShapeDoor) at pixel (dx, dy),
// size pixels square. Water and friends scroll vertically.
func (g *Graphics) drawShape(c *Canvas, shape, dx, dy, size int, o shapeOpts) {
	src := g.opaque
	if o.masked {
		src = g.masked
	}
	var r image.Rectangle
	switch {
	case shape == game.ShapeDoor:
		// Doors are the alternate frame of the letter "I".
		r = g.tileRect(game.ShapeDoor>>1, 1)
	case o.altFrame:
		r = g.tileRect(shape>>1, 1)
	default:
		r = g.tileRect(shape>>1, -1)
	}
	dst := image.Rect(dx, dy, dx+size, dy+size)
	if o.flip {
		c.BlitFlipped(src, r, dst, o.masked)
		return
	}
	sp := g.scroll[shape]
	if sp == 0 {
		c.Blit(src, r, dst, o.masked)
		return
	}
	// Scrolling: the bottom of the tile drawn at the top and the top below it.
	top := r.Dy() - sp
	split := dy + top*size/r.Dy()
	c.Blit(src, image.Rect(r.Min.X, r.Min.Y+sp, r.Max.X, r.Max.Y), image.Rect(dx, dy, dx+size, split), o.masked)
	c.Blit(src, image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+sp), image.Rect(dx, split, dx+size, dy+size), o.masked)
}

// drawGlyph draws a character of the bitmap font, the whole glyph in the square cell.
func (g *Graphics) drawGlyph(c *Canvas, ch byte, dx, dy, size int) {
	code := int(ch) - 0x20
	if code < 0 || code >= fontGlyphs {
		code = 0
	}
	c.Blit(g.font, image.Rect(code*g.fontW, 0, (code+1)*g.fontW, g.fontH), image.Rect(dx, dy, dx+size, dy+size), true)
}

// drawMoon draws a moon's phase (0..7): Trammel's eight then Felucca's.
func (g *Graphics) drawMoon(c *Canvas, moon, phase, dx, dy, size int) {
	column := moon*8 + phase&7
	if g.moons != nil {
		c.Blit(g.moons, image.Rect(column*moonCell, 0, (column+1)*moonCell, moonCell), image.Rect(dx, dy, dx+size, dy+size), false)
		return
	}
	g.drawUIPiece(c, column, 2, dx, dy, size)
}

// drawUIPiece draws piece (column, row) of the UI sheet.
func (g *Graphics) drawUIPiece(c *Canvas, column, row, dx, dy, size int) {
	s := g.uiSize
	c.Blit(g.ui, image.Rect(column*s, row*s, (column+1)*s, (row+1)*s), image.Rect(dx, dy, dx+size, dy+size), true)
}

// flag shapes that flap (castle, town, ship), and how many ticks each waits.
var (
	flagShapes = []int{game.ShapeCastle, game.ShapeTown, game.ShapeFrigate}
	flagWaits  = []int{3, 2, 1}
)

// twiddleFlags is one idle tick of TwiddleFlags(): each flag's counter
// counts down and the flag flaps when it passes zero, then waits again.
// From the counts (3, 2, 1) the castle flaps every 4th tick, the town every
// 3rd and the ship every 2nd.
func twiddleFlags(counters []int) []bool {
	out := make([]bool, len(counters))
	for i := range counters {
		counters[i]--
		if counters[i] >= 0 {
			continue
		}
		counters[i] = flagWaits[i]
		out[i] = true
	}
	return out
}

// tick advances the animations by one idle tick: ScrollThings(),
// TwiddleFlags() and AnimateTiles() of UltimaMain.c. Water and lava scroll
// slowly, force fields and moongates twice as fast; flags flap on their own
// periods; creatures cycle their two frames one type at a time.
func (g *Graphics) tick() {
	step := max(1, (g.tileSize+16)/32)
	scrollBy := func(shape, amount int) { g.scroll[shape] = (g.scroll[shape] + amount) % g.tileSize }
	scrollBy(game.ShapeForceField, step*2)
	scrollBy(game.ShapeMoonGate, step*2)
	scrollBy(game.ShapeWater, step)
	scrollBy(game.ShapeLava, step)

	// Exodus' lights: every other tick the next panel down (3, 2, 1, 0). (ExodusLights)
	g.exodusSkip ^= 1
	if g.exodusSkip == 1 {
		g.exodusFrame = (g.exodusFrame + 3) & 3
	}
	for i, flap := range twiddleFlags(g.twiddle) {
		if flap {
			g.swap(flagShapes[i])
		}
	}
	// Creatures: every tick but the 5th, advance one creature type.
	for {
		g.animSkip--
		if g.animSkip < 1 {
			g.animSkip = 5
			return
		}
		g.animPhase--
		if g.animPhase < 0 {
			g.animPhase = 19
		}
		shape := g.animPhase*2 + 32 // 0x20 .. 0x46
		if shape >= 0x44 {
			g.swap(shape - 0x2a) // also Serpent, Man-O-War, Pirate
		}
		if shape == 0x20 {
			g.swap(game.ShapeRanger) // the party symbol
		}
		if shape == 62 {
			shape = 24
		}
		if shape > 62 {
			shape += 64
		}
		g.swap(shape)
		if shape >= 0x2e && shape <= 0x3c {
			// Monster types with variants: their variant tiles swap too.
			v := ((shape>>1)-23)*2 + 80
			g.swap(v * 2)
			g.swap((v + 1) * 2)
		}
	}
}

// swap toggles a shape's frame, and the class figures that stand in for it. (SwapShape)
func (g *Graphics) swap(shape int) {
	index := shape >> 1
	if index >= len(g.swapped) {
		return
	}
	g.swapped[index] = !g.swapped[index]
	for c := range game.ClassCount {
		if game.ClassFallbackTile[c] == index {
			g.swapped[game.FirstClassTile+c] = !g.swapped[game.FirstClassTile+c]
		}
	}
}
