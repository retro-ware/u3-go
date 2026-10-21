package res

import (
	"errors"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// TileSet is one look of the game: each tile's two animation frames, indexed
// by map value / 4, and the font's glyphs from ASCII 32 (nil if the set has none).
type TileSet struct {
	Name  string
	Tiles [][2]image.Image
	Font  []image.Image
}

// A LairWare sheet is 12 columns by 16 rows; each tile's two frames sit side
// by side, tiles run down the columns. A font sheet is one row of 96 glyphs.
const (
	sheetCols, sheetRows = 12, 16
	fontGlyphs           = 96
)

// CGA palette 1, high intensity: black, cyan, magenta, white.
var cga = color.Palette{
	color.RGBA{0, 0, 0, 255}, color.RGBA{85, 255, 255, 255},
	color.RGBA{255, 85, 255, 255}, color.RGBA{255, 255, 255, 255},
}

// TileSets returns the tile sets found under dir: the PC's own shapes
// (dir/pc/SHAPES.ULT and CHARSET.ULT) and every LairWare sheet in
// dir/mac/Graphics. Sets that fail to decode are left out.
func TileSets(dir string) []TileSet {
	var sets []TileSet
	if s, err := pcTileSet(filepath.Join(dir, "pc")); err == nil {
		sets = append(sets, s)
	}
	sheets, _ := filepath.Glob(filepath.Join(dir, "mac", "Graphics", "*-Tiles.*"))
	sort.Strings(sheets)
	for _, p := range sheets {
		if s, err := sheetTileSet(p); err == nil {
			sets = append(sets, s)
		}
	}
	return sets
}

func pcTileSet(dir string) (TileSet, error) {
	shapes, err := os.ReadFile(filepath.Join(dir, "SHAPES.ULT"))
	if err != nil {
		return TileSet{}, err
	}
	chars, err := os.ReadFile(filepath.Join(dir, "CHARSET.ULT"))
	if err != nil {
		return TileSet{}, err
	}
	tiles, font := Shapes(shapes), Charset(chars)
	if len(tiles) == 0 || len(font) < 32+fontGlyphs {
		return TileSet{}, errors.New("short SHAPES.ULT or CHARSET.ULT")
	}
	return TileSet{Name: "PC CGA (original)", Tiles: tiles, Font: font[32 : 32+fontGlyphs]}, nil
}

// pcAltFrames is where SHAPES.ULT keeps second frames: tiles 16-31 (the
// people and monsters) have theirs at 64-79; the others have one frame.
const pcFirstAnimated, pcAnimated, pcAltFrames = 16, 16, 64

// Shapes decodes SHAPES.ULT: 16x16 CGA tiles of 64 bytes each.
func Shapes(b []byte) [][2]image.Image {
	all := cgaTiles(b, 16, 16)
	if len(all) < pcAltFrames {
		return nil
	}
	tiles := make([][2]image.Image, pcAltFrames)
	for i := range tiles {
		tiles[i] = [2]image.Image{all[i], all[i]}
		if j := pcAltFrames + i - pcFirstAnimated; i >= pcFirstAnimated && i < pcFirstAnimated+pcAnimated && j < len(all) {
			tiles[i][1] = all[j]
		}
	}
	return tiles
}

// Charset decodes CHARSET.ULT: 128 8x8 CGA glyphs in ASCII order.
func Charset(b []byte) []image.Image { return cgaTiles(b, 8, 8) }

// cgaTiles cuts 2-bit CGA images of w x h, each stored as its even rows
// then its odd rows, as in CGA memory.
func cgaTiles(b []byte, w, h int) []image.Image {
	row := w / 4
	size := row * h
	var out []image.Image
	for at := 0; at+size <= len(b); at += size {
		img := image.NewPaletted(image.Rect(0, 0, w, h), cga)
		for y := range h {
			src := b[at+(y%2)*(size/2)+(y/2)*row:]
			for x := range w {
				img.Pix[y*w+x] = src[x/4] >> (6 - 2*(x%4)) & 3
			}
		}
		out = append(out, img)
	}
	return out
}

func sheetTileSet(path string) (TileSet, error) {
	sheet, err := decode(path)
	if err != nil {
		return TileSet{}, err
	}
	base := strings.TrimSuffix(path, filepath.Ext(path))
	base = strings.TrimSuffix(base, "Tiles")
	if mask, err := decodeAny(base + "Mask"); err == nil {
		sheet = applyMask(sheet, mask)
	}
	size := sheet.Bounds().Dx() / sheetCols
	cell := func(col, row int) image.Image {
		r := image.Rect(col*size, row*size, (col+1)*size, (row+1)*size).Add(sheet.Bounds().Min)
		return sheet.(interface {
			SubImage(image.Rectangle) image.Image
		}).SubImage(r)
	}
	s := TileSet{Name: strings.TrimSuffix(filepath.Base(base), "-")}
	for i := range sheetCols / 2 * sheetRows {
		s.Tiles = append(s.Tiles, [2]image.Image{cell(i/sheetRows*2, i%sheetRows), cell(i/sheetRows*2+1, i%sheetRows)})
	}
	if font, err := decodeAny(base + "Font"); err == nil {
		gw, gh := font.Bounds().Dx()/fontGlyphs, font.Bounds().Dy()
		for i := range fontGlyphs {
			r := image.Rect(i*gw, 0, (i+1)*gw, gh).Add(font.Bounds().Min)
			s.Font = append(s.Font, font.(interface {
				SubImage(image.Rectangle) image.Image
			}).SubImage(r))
		}
	}
	return s, nil
}

func decode(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	return img, err
}

func decodeAny(base string) (image.Image, error) {
	for _, ext := range []string{".png", ".gif"} {
		if img, err := decode(base + ext); err == nil {
			return img, nil
		}
	}
	return nil, os.ErrNotExist
}

// applyMask makes the sheet clear where the mask is white (black is opaque).
func applyMask(sheet, mask image.Image) image.Image {
	out := image.NewNRGBA(sheet.Bounds())
	draw.Draw(out, out.Bounds(), sheet, sheet.Bounds().Min, draw.Src)
	mb := mask.Bounds()
	for y := range min(mb.Dy(), out.Rect.Dy()) {
		for x := range min(mb.Dx(), out.Rect.Dx()) {
			if g := color.GrayModel.Convert(mask.At(mb.Min.X+x, mb.Min.Y+y)).(color.Gray); g.Y > 127 {
				out.Pix[out.PixOffset(x+out.Rect.Min.X, y+out.Rect.Min.Y)+3] = 0
			}
		}
	}
	return out
}
