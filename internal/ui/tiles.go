package ui

import (
	"image"

	"github.com/retro-ware/u3-go/internal/game"
)

// TileSheet is every tile of a set at size pixels on a 16-wide grid, each
// on grass as the map draws it, with its second frame beside it (a check
// of the tile sets in place of tiles.ts's tile viewer page).
func TileSheet(g *Graphics, size int) *image.RGBA {
	const tiles, across = 96, 8
	c := NewCanvas(across*2*size, (tiles+across-1)/across*size)
	for i := range tiles {
		x, y := (i%across)*2*size, (i/across)*size
		for frame := range 2 {
			g.drawShape(c, game.ShapeGrass, x+frame*size, y, size, shapeOpts{})
			g.drawShape(c, i*2, x+frame*size, y, size, shapeOpts{masked: true, altFrame: frame == 1})
		}
	}
	return c.RGBA
}
