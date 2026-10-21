package ui

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// An imported Standard-Mask (LairWare's, for its own sheet) must not cut
// into the port's embedded Standard tiles.
func TestImportedMaskSkipsOwnSheet(t *testing.T) {
	data := t.TempDir()
	dir := filepath.Join(data, "mac", "Graphics")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	white := image.NewRGBA(image.Rect(0, 0, 768, 1024)) // white: cut everything
	draw.Draw(white, white.Rect, image.NewUniform(color.White), image.Point{}, draw.Src)
	write := func(name string, img image.Image) {
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, img); err != nil {
			t.Fatal(err)
		}
	}
	write("Standard-Mask.png", white)
	write("Standard-Font.png", image.NewRGBA(image.Rect(0, 0, fontGlyphs*8, 8))) // Standard's font is LairWare's

	g, err := LoadGraphics("Standard", data)
	if err != nil {
		t.Fatal(err)
	}
	r := g.tileRect(4, 0) // mountains
	if g.masked.RGBAAt(r.Min.X+r.Dx()/2, r.Min.Y+r.Dy()/2).A == 0 {
		t.Error("the imported mask was applied to the embedded Standard sheet")
	}
}
