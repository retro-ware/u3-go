package game

// The 11x11 window of shapes the renderer draws around the party: DrawMap(),
// HideMonsters() and ShowMonsters() from UltimaGraphics.c reduced to a pure
// function that reads the world and returns what to draw.

// ViewSize is the viewport's side; ViewCentre the party's cell on it.
const (
	ViewSize     = 11
	ViewCentre   = 5
	centreOffset = ViewCentre*ViewSize + ViewCentre // 60, 0x3C in the C source
)

// Hit is a hit landing on a cell: the frame 1..3 and the ball shape.
type Hit struct{ Frame, Shape int }

// ViewCell is one cell: a base shape (terrain, drawn opaque) and an
// optional overlay (a creature or object drawn through the mask).
type ViewCell struct {
	Base       int
	Overlay    int
	HasOverlay bool
	Flip       bool // draw the overlay flipped (horse facing east)
	AltFrame   bool // draw the overlay from its alternate frame (the "HIT" balls)
	Hit        *Hit // a hit landing here: the "HIT" tile, or a burst with the Standard tiles
	Party      bool // the party stands here on foot; the screen may draw the members
	Follower   int  // member index of a follower walking behind the leader here, or -1
}

func baseCell(s int) ViewCell { return ViewCell{Base: s, Follower: -1} }

func overlayCell(base, overlay int) ViewCell {
	return ViewCell{Base: base, Overlay: overlay, HasOverlay: true, Follower: -1}
}

// Viewport is the view and the map coordinates of its top-left cell (stx, sty).
type Viewport struct {
	Cells            []ViewCell
	OriginX, OriginY int
}

// fillShapes is step 1 of DrawMap(): the 11x11 map values around (x, y) as
// shapes. An "I" that is not part of a word is drawn as a door.
func fillShapes(w *World, x, y int) []int {
	shapes := make([]int, ViewSize*ViewSize)
	offset := 0
	for ym := y - ViewCentre; ym <= y+ViewCentre; ym++ {
		for xm := x - ViewCentre; xm <= x+ViewCentre; xm++ {
			val := w.GetXYVal(xm, ym)
			shapes[offset] = MonsterVariantShape(val)
			if val == MapValueLetterI && xm > 0 && xm < w.MapSize()-1 && ym > 0 {
				// An "I" is a door unless a letter to its left, right or above
				// makes it text. Another "I" never counts, so two doors side
				// by side do not vouch for each other; looking past a run of
				// "I"s reads Moon's "CAPESSII" right, which the Mac spoiled.
				isText := func(v int) bool { return IsLetter(v) && v != MapValueLetterI }
				pastAnyIs := func(dx int) int {
					x := xm + dx
					for x > 0 && x < w.MapSize()-1 && w.GetXYVal(x, ym) == MapValueLetterI {
						x += dx
					}
					return w.GetXYVal(x, ym)
				}
				if !isText(pastAnyIs(-1)) && !isText(pastAnyIs(1)) && !IsLetter(w.GetXYVal(xm, ym-1)) {
					shapes[offset] = ShapeDoor
				}
			}
			offset++
		}
	}
	return shapes
}

// applyLineOfSight is step 2 of DrawMap(): from the outermost cells inward,
// trace a path toward the party; one that runs into mountains, forest, a
// wall or an already hidden cell hides the cell, and so what is behind it.
func applyLineOfSight(shapes []int) {
	blocks := func(s int) bool { return s == ShapeForest || s == ShapeMountains || s == ShapeWall || s == ShapeVoid }
	toward := func(v int) int {
		switch {
		case v < ViewCentre:
			return 1
		case v > ViewCentre:
			return -1
		}
		return 0
	}
	for offset := ViewSize*ViewSize - 1; offset >= 0; offset-- {
		xt, yt := offset%ViewSize, offset/ViewSize
		cursor := offset
		for {
			next := cursor + toward(xt) + toward(yt)*ViewSize
			if next == centreOffset {
				break // reached the party: visible
			}
			cursor = next
			if blocks(shapes[cursor]) {
				shapes[offset] = ShapeVoid
				break
			}
			xt += toward(xt)
			yt += toward(yt)
		}
	}
}

func isBall(s int) bool { return s >= ShapeMagicBall && s <= ShapeFireBall }

// applyOverlays is step 3 (HideMonsters / ShowMonsters on the surface and in
// towns): a monster's terrain comes from its tileUnder; chests, horses and
// ships encode or imply theirs.
func applyOverlays(w *World, shapes []int, originX, originY int) []ViewCell {
	cells := make([]ViewCell, len(shapes))
	for i, s := range shapes {
		cells[i] = baseCell(s)
	}
	mons := w.Monsters()
	for i := range MonsterSlots {
		if mons.Type(i) == 0 {
			continue
		}
		xm, ym := mons.X(i)-originX, mons.Y(i)-originY
		if w.OnSurface() {
			xm, ym = w.Constrain(xm), w.Constrain(ym)
		}
		if xm < 0 || xm >= ViewSize || ym < 0 || ym >= ViewSize {
			continue
		}
		offset := ym*ViewSize + xm
		s := shapes[offset]
		if s == ShapeVoid || isBall(s) {
			continue
		}
		cells[offset] = overlayCell(mons.TileUnder(i)>>1, s)
	}

	for offset := range cells {
		if cells[offset].HasOverlay {
			continue
		}
		xm, ym := offset%ViewSize, offset/ViewSize
		switch s := cells[offset].Base; s {
		case ShapeJester, ShapeSnakeBottom, ShapeSnakeTop, ShapeFrigate, ShapeWhirlpool, ShapeSerpent, ShapeManOWar, ShapePirate:
			// The jester in Lord British's castle and the sea creatures stand on water.
			cells[offset] = overlayCell(ShapeWater, s)
		case ShapeHorse:
			cells[offset] = overlayCell(ShapeGrass, s)
		case ShapeShrine:
			// Ambrosia's shrines sit on whatever is to their left.
			cells[offset] = overlayCell(w.GetXYVal(originX+xm-1, originY+ym)>>1, s)
		case ShapeChest:
			// A chest's low two map bits say what it sits on: 0 floor, else grass/brush/forest.
			under := (w.GetXYVal(originX+xm, originY+ym) & 0x3) * 2
			if under == 0 {
				under = ShapeFloor
			}
			cells[offset] = overlayCell(under, ShapeChest)
		case ShapeDoor:
			// A door over the terrain to its left (grass if that is not terrain).
			var neighbour int
			if mon := mons.At(originX+xm-1, originY+ym); mon >= 0 {
				neighbour = mons.TileUnder(mon) >> 1
			} else {
				neighbour = w.GetXYVal(originX+xm-1, originY+ym) >> 1
			}
			if neighbour > ShapeFloor {
				neighbour = ShapeGrass
			}
			cells[offset] = overlayCell(neighbour, ShapeDoor)
		}
	}

	// The party itself, unless a spell ball is being drawn over it.
	if w.Party.Location() != LocationCombat {
		c := &cells[centreOffset]
		top := c.Base
		if c.HasOverlay {
			top = c.Overlay
		}
		if !isBall(top) {
			c.Overlay, c.HasOverlay = w.Party.Shape(), true
			c.Flip = w.Party.Shape() == ShapeHorse && w.HorseFacingEast
			c.Party = w.Party.Shape() == 0x7e // on foot (the ranger figure)
			if c.Party {
				markFollowers(w, cells, originX+ViewCentre, originY+ViewCentre)
			}
		}
	}
	return cells
}

// buildCombatViewport is the arena with monsters and members over the
// terrain they stand on (the combat branch of HideMonsters / ShowMonsters).
func buildCombatViewport(w *World) *Viewport {
	c := w.Combat
	cells := make([]ViewCell, len(c.Tiles))
	for i, s := range c.Tiles {
		cells[i] = baseCell(int(s))
	}
	for _, m := range c.Monsters {
		if m.HP > 0 {
			cells[m.Y*ViewSize+m.X] = overlayCell(m.TileUnder, m.Shape)
		}
	}
	for _, p := range c.Members {
		if p.X <= 10 && p.Y <= 10 {
			cells[p.Y*ViewSize+p.X] = overlayCell(p.TileUnder, p.Shape)
		}
	}
	return &Viewport{Cells: cells}
}

// markFollowers puts the living members after the leader, in a town or
// castle, on the squares the leader came through (World.Trail).
func markFollowers(w *World, cells []ViewCell, x, y int) {
	var living []int
	for m := range 4 {
		if w.Party.MemberSlot(m) >= 0 && w.MemberAlive(m) {
			living = append(living, m)
		}
	}
	size := w.MapSize()
	wrap := func(d int) int { return ((d+size/2)%size+size)%size - size/2 } // shortest way round
	for i, pos := range w.Trail {
		if i+1 >= len(living) {
			return
		}
		ddx, ddy := wrap(pos.X-x), wrap(pos.Y-y)
		if abs(ddx) > 3 || abs(ddy) > 3 {
			continue // a moongate or whirlpool left this square behind
		}
		vx, vy := ddx+ViewCentre, ddy+ViewCentre
		if vx < 0 || vx >= ViewSize || vy < 0 || vy >= ViewSize {
			continue
		}
		if c := &cells[vy*ViewSize+vx]; c.Base != ShapeVoid {
			c.Follower = living[i+1]
		}
	}
}

// BuildViewport is the view centred on (x, y), the party's square by
// default (pass w.X, w.Y), or the arena during combat.
func BuildViewport(w *World, x, y int) *Viewport {
	var view *Viewport
	if w.Combat != nil {
		view = buildCombatViewport(w)
	} else {
		shapes := fillShapes(w, x, y)
		applyLineOfSight(shapes)
		ox, oy := x-ViewCentre, y-ViewCentre
		view = &Viewport{Cells: applyOverlays(w, shapes, ox, oy), OriginX: ox, OriginY: oy}
	}
	// A spell ball or cannon shot in flight is drawn over whatever is there.
	if b := w.Ball; b != nil {
		vx, vy := b.X-view.OriginX, b.Y-view.OriginY
		if w.Combat == nil && w.OnSurface() {
			vx, vy = w.Constrain(vx), w.Constrain(vy)
		}
		if vx >= 0 && vx < ViewSize && vy >= 0 && vy < ViewSize {
			c := &view.Cells[vy*ViewSize+vx]
			if b.Hit != 0 {
				c.Hit = &Hit{b.Hit, b.Shape}
			} else {
				*c = overlayCell(c.Base, b.Shape)
			}
		}
	}
	return view
}
