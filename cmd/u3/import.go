package main

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/retro-ware/gameimport"
	"github.com/retro-ware/u3-go/internal/res"
)

// pcFile says whether name is one of the PC release's data files.
func pcFile(name string) bool {
	switch b := strings.ToUpper(path.Base(name)); b {
	case "BRAND.IMG", "FOUNTAIN.IMG", "SHRINE.IMG", "TIME.IMG", "DUNGEON.DAT", "NAME.DAT", "ANIMATE.DAT":
		return true
	default:
		return path.Ext(b) == ".ULT"
	}
}

// macRoot gives the part of name up to its app's Resources folder (the
// LairWare layout), or false.
func macRoot(name string) (string, bool) {
	i := strings.LastIndex(name, "Resources/")
	if i < 0 || i > 0 && name[i-1] != '/' {
		return "", false
	}
	return name[:i+len("Resources/")], true
}

// importGame copies either release's files from src (folder, zip, disk image,
// DMG, ISO or loose files) into dir/mac (the LairWare Resources folder as it
// is) or dir/pc, checks they load and writes dir/COVER.PNG.
func importGame(src, dir string) error {
	files, err := gameimport.Find(src, func(name string) bool {
		_, mac := macRoot(name)
		return mac || pcFile(name)
	})
	if err != nil {
		return err
	}
	var macFork string
	for name := range files {
		if b := path.Base(name); b == "MainResources.rsrc" || b == "Ultima III.rsrc" {
			macFork = name
		}
	}
	wrote := false
	if root, ok := macRoot(macFork); ok {
		for name, b := range files {
			if strings.HasPrefix(name, root) {
				if err := write(filepath.Join(dir, "mac", filepath.FromSlash(name[len(root):])), b); err != nil {
					return err
				}
			}
		}
		if _, err := res.LoadMac(filepath.Join(dir, "mac")); err != nil {
			return err
		}
		wrote = true
	}
	var pcDir string
	for name := range files {
		if strings.EqualFold(path.Base(name), "SOSARIA.ULT") {
			pcDir = path.Dir(name)
		}
	}
	if pcDir != "" {
		for name, b := range files {
			if path.Dir(name) == pcDir && pcFile(name) {
				if err := write(filepath.Join(dir, "pc", strings.ToUpper(path.Base(name))), b); err != nil {
					return err
				}
			}
		}
		if _, err := res.LoadDOS(filepath.Join(dir, "pc")); err != nil {
			return err
		}
		wrote = true
	}
	if !wrote {
		return fmt.Errorf("no Ultima III files (LairWare's Ultima III.rsrc or the PC's SOSARIA.ULT) in %s", src)
	}
	return writeCover(dir)
}

func write(name string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		return err
	}
	return os.WriteFile(name, b, 0o644)
}

// writeCover draws dir/COVER.PNG for goplay's label: "ULTIMA III" in the
// game's own font over a strip of its tiles, 320x200 on black.
func writeCover(dir string) error {
	sets := res.TileSets(dir)
	var set *res.TileSet
	for i := range sets {
		if sets[i].Font != nil && (set == nil || sets[i].Name == "Standard") {
			set = &sets[i]
		}
	}
	if set == nil {
		return errors.New("no tile set to draw the cover with")
	}
	const w, h, tile = 320, 200, 32
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.Black), image.Point{}, draw.Src)
	// Tiles by map value / 4: mountains, forest, a town, a castle, the four
	// first classes' figures and a dragon.
	row := []int{4, 3, 6, 7, 20, 21, 22, 23, 29}
	for i, t := range row {
		scaled(img, set.Tiles[t][0], image.Rect(16+i*tile, 130, 16+(i+1)*tile, 130+tile))
	}
	title := "ULTIMA III"
	gw := 28
	x0 := (w - len(title)*gw) / 2
	for i, c := range title {
		if c == ' ' {
			continue
		}
		scaled(img, set.Font[c-' '], image.Rect(x0+i*gw, 50, x0+(i+1)*gw, 50+gw*set.Font[0].Bounds().Dy()/set.Font[0].Bounds().Dx()))
	}
	f, err := os.Create(filepath.Join(dir, "COVER.PNG"))
	if err != nil {
		return err
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// scaled draws src into r of dst, nearest neighbour.
func scaled(dst *image.RGBA, src image.Image, r image.Rectangle) {
	sb := src.Bounds()
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			c := src.At(sb.Min.X+(x-r.Min.X)*sb.Dx()/r.Dx(), sb.Min.Y+(y-r.Min.Y)*sb.Dy()/r.Dy())
			if _, _, _, a := c.RGBA(); a > 0 {
				dst.Set(x, y, c)
			}
		}
	}
}
