package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/retro-ware/u3-go/internal/res"
)

func TestImport(t *testing.T) {
	srcs := map[string]string{"mac": "../../ref/ultima3/Lairware", "pc": os.Getenv("U3_PC_DIR")}
	for kind, src := range srcs {
		if _, err := os.Stat(src); src == "" || err != nil {
			continue
		}
		dir := t.TempDir()
		if err := importGame(src, dir); err != nil {
			t.Fatal(kind, err)
		}
		if _, err := os.Stat(filepath.Join(dir, "COVER.PNG")); err != nil {
			t.Error(kind, err)
		}
		r, err := res.Load(dir)
		if err != nil || len(r.Maps) != 21 {
			t.Error(kind, err)
		}
	}
}
