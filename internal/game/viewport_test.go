package game

import "testing"

const viewCentreCell = ViewCentre*ViewSize + ViewCentre

func flatWorld(t *testing.T, fill byte) *World {
	w := newWorld(t)
	for i := range w.Current.Tiles {
		w.Current.Tiles[i] = fill
	}
	clear(w.Monsters().Bytes)
	w.X, w.Y = 32, 32
	return w
}

func TestViewportCentresOnParty(t *testing.T) {
	w := newWorld(t)
	v := BuildViewport(w, w.X, w.Y)
	c := v.Cells[viewCentreCell]
	if len(v.Cells) != 121 || v.OriginX != w.X-5 || v.OriginY != w.Y-5 || c.Overlay != ShapeRanger || c.Base != w.GetXYVal(w.X, w.Y)>>1 {
		t.Fatalf("%+v", c)
	}
}

func TestViewportWraps(t *testing.T) {
	w := newWorld(t)
	w.X, w.Y = 0, 0
	b := BuildViewport(w, 0, 0).Cells[0].Base
	if b != w.GetXYVal(59, 59)>>1 && b != ShapeVoid {
		t.Fatal(b)
	}
}

func TestViewportHidesBehindMountains(t *testing.T) {
	w := flatWorld(t, MapValueGrass)
	w.PutXYVal(MapValueMountains, 32, 31)
	v := BuildViewport(w, w.X, w.Y)
	at := func(x, y int) int { return v.Cells[y*ViewSize+x].Base }
	if at(5, 4) != ShapeMountains || at(5, 3) != ShapeVoid || at(5, 0) != ShapeVoid || at(4, 3) != ShapeVoid ||
		at(3, 3) != ShapeGrass || at(6, 4) != ShapeGrass {
		t.Fatal("line of sight wrong")
	}
}

func TestViewportCreatureOverTerrain(t *testing.T) {
	w := flatWorld(t, MapValueGrass)
	m := w.Monsters()
	m.SetType(3, MapValueOrc)
	m.SetTileUnder(3, MapValueBrush)
	m.SetPosition(3, 34, 32)
	w.PutXYVal(MapValueOrc, 34, 32)
	c := BuildViewport(w, w.X, w.Y).Cells[5*ViewSize+7]
	if c.Base != ShapeBrush || c.Overlay != MapValueOrc>>1 {
		t.Fatalf("%+v", c)
	}
}

func TestViewportWaterUnderStraySeaCreature(t *testing.T) {
	w := flatWorld(t, MapValueGrass)
	w.PutXYVal(MapValuePirate, 34, 32)
	c := BuildViewport(w, w.X, w.Y).Cells[5*ViewSize+7]
	if c.Base != ShapeWater || c.Overlay != ShapePirate {
		t.Fatalf("%+v", c)
	}
}

func TestViewportVariantRows(t *testing.T) {
	w := flatWorld(t, MapValueGrass)
	w.PutXYVal(MapValueOrc+1, 33, 32) // Goblin
	if b := BuildViewport(w, w.X, w.Y).Cells[5*ViewSize+6].Base; b != 82*2 {
		t.Fatal(b)
	}
}

func TestViewportLoneIIsADoor(t *testing.T) {
	w := flatWorld(t, MapValueFloor)
	w.PutXYVal(MapValueLetterI, 33, 32)
	w.PutXYVal(MapValueLetterI, 30, 32)
	w.PutXYVal(MapValueLetterA, 29, 32) // "AI" is a sign
	v := BuildViewport(w, w.X, w.Y)
	if v.Cells[5*ViewSize+6].Overlay != ShapeDoor {
		t.Fatal("door")
	}
	if c := v.Cells[5*ViewSize+3]; c.Base != MapValueLetterI>>1 || c.HasOverlay {
		t.Fatalf("%+v", c)
	}
}

func TestViewportFollowerLine(t *testing.T) {
	w := flatWorld(t, MapValueGrass)
	io := newFakeIO(w.Resources)
	Move(w, io, MoveNorth)
	Move(w, io, MoveNorth)
	Move(w, io, MoveEast)
	v := BuildViewport(w, w.X, w.Y)
	at := func(dx, dy int) ViewCell { return v.Cells[(ViewCentre+dy)*ViewSize+ViewCentre+dx] }
	if !at(0, 0).Party || at(-1, 0).Follower != 1 || at(-1, 1).Follower != 2 || at(-1, 2).Follower != 3 {
		t.Fatal("followers wrong")
	}
}

func TestViewportDoorsAndLetterI(t *testing.T) {
	const moon, deathGulch = 403, 411
	drawnAt := func(w *World, x, y int) int {
		v := BuildViewport(w, w.X, w.Y)
		c := v.Cells[(y-v.OriginY)*ViewSize+x-v.OriginX]
		if c.HasOverlay {
			return c.Overlay
		}
		return c.Base
	}
	standing := func(id, x, y int) *World {
		w := newWorld(t)
		w.EnterMap(id)
		w.X, w.Y = x, y
		return w
	}
	w := standing(moon, 19, 49)
	if w.GetXYVal(21, 50) != MapValueLetterI || w.GetXYVal(22, 50) != MapValueLetterI ||
		drawnAt(w, 21, 50) != MapValueLetterI>>1 || drawnAt(w, 22, 50) != MapValueLetterI>>1 {
		t.Fatal("CAPESSII")
	}
	for _, g := range [][4]int{{11, 11, 12, 11}, {21, 11, 20, 11}} {
		w := standing(deathGulch, g[2], g[3])
		if w.GetXYVal(g[0], g[1]) != MapValueLetterI || drawnAt(w, g[0], g[1]) != ShapeDoor {
			t.Fatal("gate", g)
		}
	}
}
