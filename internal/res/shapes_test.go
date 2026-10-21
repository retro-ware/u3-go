package res

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTileSets(t *testing.T) {
	dir := t.TempDir()
	graphics := refDir + "/Lairware/Resources/Graphics"
	if _, err := os.Stat(graphics); err != nil {
		t.Skip("ref/ missing")
	}
	os.Mkdir(filepath.Join(dir, "mac"), 0o755)
	abs, _ := filepath.Abs(graphics)
	os.Symlink(abs, filepath.Join(dir, "mac", "Graphics"))
	if pc := os.Getenv("U3_PC_DIR"); pc != "" {
		os.Symlink(pc, filepath.Join(dir, "pc"))
	}
	sets := TileSets(dir)
	if len(sets) < 10 {
		t.Fatalf("%d sets", len(sets))
	}
	for _, s := range sets {
		if len(s.Tiles) < 64 || s.Tiles[16][1] == nil {
			t.Errorf("%s: %d tiles", s.Name, len(s.Tiles))
		}
		t.Log(s.Name, len(s.Tiles), len(s.Font), s.Tiles[0][0].Bounds().Dx())
	}
}

func TestCGATiles(t *testing.T) {
	// One 8x8 glyph: row 0 (even half) all colour 3, row 1 (odd half) colour 1.
	b := make([]byte, 16)
	b[0], b[1] = 255, 255
	b[8], b[9] = 85, 85
	g := Charset(b)[0]
	if c := g.At(0, 0); c != cga[3] {
		t.Errorf("row 0 %v", c)
	}
	if c := g.At(7, 1); c != cga[1] {
		t.Errorf("row 1 %v", c)
	}
}
