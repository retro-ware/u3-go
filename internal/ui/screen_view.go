package ui

import (
	"image"
	"math"
	"math/rand/v2"
	"time"

	"github.com/retro-ware/u3-go/internal/game"
)

// The map viewport, its animation frame, flashes, pictures, mini maps and the ending.

// RedrawMap rebuilds the viewport from the world; the next frame paints it.
func (s *Screen) RedrawMap() {
	s.viewCovered = false
	s.view = game.BuildViewport(s.world, s.world.X, s.world.Y)
	game.RecordView(s.world, s.view) // what the view shows is on the map from now
	s.viewDirty = true
}

// ClearTiles blacks out the viewport.
func (s *Screen) ClearTiles() {
	s.view = nil
	s.viewCovered = true
	s.black(1, 1, 22, 22)
}

// markActiveMember outlines, in combat, the member whose turn it is: white
// fading to grey over the time they have, so it doubles as the timer.
func (s *Screen) markActiveMember() {
	c := s.world.Combat
	if c == nil || c.MarkedMember < 0 || c.MarkedMember >= len(c.Members) {
		return
	}
	me := c.Members[c.MarkedMember]
	if me.X > 10 || me.Y > 10 {
		return
	}
	cell := float64(s.cell)
	tile := 2 * cell
	width := 2 * tile / 16 // two Apple II pixels
	x := (1+2*float64(me.X))*cell - width/2
	y := (1+2*float64(me.Y))*cell - width/2
	now := c.MarkedFrozenAt
	if now == 0 {
		now = game.Now()
	}
	elapsed := 0.0
	if c.MarkedFor > 0 {
		elapsed = (now - c.MarkedAt) / c.MarkedFor
	}
	// ponytail: square corners where the canvas had roundRect; a polygon path if it matters.
	saved := s.c.RGBA
	s.c.RGBA = s.c.SubImage(s.cells(1, 1, 22, 22)).(*image.RGBA) // never paint over the border
	s.strokeRect(x+width/2, y+width/2, tile, tile, width, markerColour(s.gfx.display, elapsed))
	s.c.RGBA = saved
}

// paintViewport paints the cached viewport, or the dungeon view.
func (s *Screen) paintViewport() {
	w, cell := s.world, s.cell
	inDungeon := w.Party.Location() == game.LocationDungeon && w.Combat == nil
	overlay := inDungeon && w.MapMode != game.MapModeOff
	if s.overlayShown && !overlay {
		s.overlayShown = false
		s.redrawTextArea()
	}
	if inDungeon {
		var view *Canvas
		if w.Dungeon.Torch > 0 && s.dungeon != nil {
			view = s.dungeon.render(game.BuildDungeonView(w), w.Dungeon.Torch, game.SecretMessage(w))
		}
		if w.MapMode == game.MapModeFull {
			// The level fills the window; the first-person view shrinks into the overlay slot.
			s.overlayShown = true
			s.black(1, 1, 22, 22)
			span := topDownReach*2 + 1
			edge := cell + (22-span)*cell/2
			s.paintAutoMap(game.AroundParty(w), [2]int{-topDownReach, -topDownReach}, [2]int{span, span}, [2]int{edge, edge}, [2]int{0, 0})
			s.black(mapOverlayLeft, mapOverlayTop, mapOverlaySize, mapOverlaySize)
			if view != nil {
				vw := mapOverlaySize * cell
				vh := vw * 512 / 600
				x, y := mapOverlayLeft*cell, mapOverlayTop*cell+(mapOverlaySize*cell-vh)/2
				s.c.Blit(view, view.Rect, image.Rect(x, y, x+vw, y+vh), false)
			}
			return
		}
		if overlay {
			s.overlayShown = true
			half := mapOverlaySize >> 1
			s.paintAutoMap(game.AroundParty(w), [2]int{-half, -half}, [2]int{mapOverlaySize, mapOverlaySize},
				[2]int{mapOverlayLeft * cell, mapOverlayTop * cell}, [2]int{0, 0})
		}
		s.black(1, 1, 22, 22)
		if view != nil {
			// The original drew the 600x512 view at (84, 128) for 32-pixel cells.
			m := float64(cell) / 32
			s.c.Blit(view, view.Rect, frect(84*m, 128*m, 600*m, 512*m), false)
		}
		return
	}
	if s.view == nil {
		return
	}
	tile := cell * 2
	style := s.partyStyle()
	for i, c := range s.view.Cells {
		dx := cell + (i%game.ViewSize)*tile
		dy := cell + (i/game.ViewSize)*tile
		s.gfx.drawShape(s.c, c.Base, dx, dy, tile, shapeOpts{})
		if c.Follower >= 0 && style == "line" {
			s.drawMember(c.Follower, dx, dy, tile)
		}
		if c.Hit != nil && !s.burstTiles() {
			// The Apple II way: the ball's "HIT" frame replaces whatever was hit.
			s.gfx.drawShape(s.c, c.Hit.Shape, dx, dy, tile, shapeOpts{masked: true, altFrame: true})
			continue
		}
		if c.Party && style != "symbol" {
			s.drawParty(dx, dy, tile, style)
		} else if c.HasOverlay {
			s.gfx.drawShape(s.c, c.Overlay, dx, dy, tile, shapeOpts{masked: true, flip: c.Flip, altFrame: c.AltFrame})
		}
		if c.Hit != nil {
			s.drawBurst(dx, dy, tile, c.Hit.Frame)
		}
	}
	s.markActiveMember()
}

// drawParty draws the party on foot as its members: on the overworld at
// half size in a 2x2 grid in marching order, in a town the leader alone
// (the others follow in a line).
func (s *Screen) drawParty(dx, dy, tile int, style string) {
	if style == "line" {
		for m := range 4 {
			if s.world.MemberAlive(m) {
				s.drawMember(m, dx, dy, tile)
				break
			}
		}
		return
	}
	q := tile / 2
	for m := range 4 {
		if s.world.Party.MemberSlot(m) >= 0 {
			s.drawMember(m, dx+(m%2)*q, dy+(m/2)*q, q)
		}
	}
}

// drawMember draws one member's class figure: green if poisoned, nothing if dead or ashes.
func (s *Screen) drawMember(m, x, y, size int) {
	p := s.world.Member(m)
	if p.Status() == 'D' || p.Status() == 'A' {
		return
	}
	shape := game.MemberShape(s.world, p.ClassLetter())
	if p.Status() == 'P' {
		g := s.greenFigure(shape, size)
		s.c.Blit(g, g.Rect, image.Rect(x, y, x+size, y+size), true)
		return
	}
	s.gfx.drawShape(s.c, shape, x, y, size, shapeOpts{masked: true})
}

// greenFigure is a member figure with every pixel turned green, its shading kept as brightness.
func (s *Screen) greenFigure(shape, size int) *image.RGBA {
	key := [2]int{shape, size}
	if g := s.greenCache[key]; g != nil {
		return g
	}
	off := &Canvas{image.NewRGBA(image.Rect(0, 0, size, size))}
	s.gfx.drawShape(off, shape, 0, 0, size, shapeOpts{masked: true})
	d := off.Pix
	for i := 0; i < len(d); i += 4 {
		if d[i+3] == 0 {
			continue
		}
		bright := max(d[i], d[i+1], d[i+2])
		d[i], d[i+1], d[i+2] = 0, max(64, bright), 0
	}
	s.greenCache[key] = off.RGBA
	return off.RGBA
}

// drawBurst is a hit with the Standard tiles: a red burst growing over
// three frames on the 16-pixel grid of the era's tiles; frame 3 a ring.
func (s *Screen) drawBurst(dx, dy, tile, frame int) {
	gp := float64(tile) / 16
	radius := 6.8
	if frame >= 0 && frame < 4 {
		radius = []float64{0, 2.6, 4.6, 6.8}[frame]
	}
	inner := 0.0
	if frame == 3 {
		inner = 4.6
	} else if frame == 2 {
		inner = 3.0
	}
	for py := range 16 {
		for px := range 16 {
			d := math.Hypot(float64(px)-7.5, float64(py)-7.5)
			if d > radius || (frame == 3 && d <= inner) {
				continue
			}
			col := hexColour("#e02020")
			if d <= inner {
				col = hexColour("#a01818")
			} else if d > radius-1.2 {
				col = hexColour("#ff5040")
			}
			s.c.Fill(frect(float64(dx)+float64(px)*gp, float64(dy)+float64(py)*gp, gp, gp), col)
		}
	}
}

// FlashTiles briefly inverts the viewport.
func (s *Screen) FlashTiles() {
	s.invert(1, 1, 22, 22)
	s.Pause(100)
	s.viewDirty = true
	if s.viewCovered {
		s.invert(1, 1, 22, 22)
	}
}

// FlashSpell flashes a spell: with the Standard tiles a pulse on the
// caster's or target's square, else the whole view inverted twice.
func (s *Screen) FlashSpell(kind game.SpellFlash, member int, sound string) {
	if s.TileSetName != "Standard" || s.viewCovered {
		s.FlashTiles()
		s.Sound(sound)
		s.FlashTiles()
		return
	}
	r := s.spellFlashRect(kind, member)
	s.pulse = &r
	s.drawPulse()
	s.Sound(sound)
	s.Pause(100)
	s.pulse = nil
	s.viewDirty = true
}

func (s *Screen) drawPulse() {
	if p := s.pulse; p != nil {
		s.c.FillAlpha(s.cells(p.x, p.y, p.w, p.h), rgb(255, 255, 255), 0.5)
	}
}

// spellFlashRect is the cells a spell's flash covers.
func (s *Screen) spellFlashRect(kind game.SpellFlash, member int) rect {
	whole := rect{1, 1, 22, 22}
	if kind == game.FlashWindow {
		return whole
	}
	if c := s.world.Combat; c != nil {
		if member < 0 || member >= len(c.Members) || c.Members[member] == nil {
			return whole
		}
		p := c.Members[member]
		if p.X > 10 || p.Y > 10 {
			return whole
		}
		return rect{1 + p.X*2, 1 + p.Y*2, 2, 2}
	}
	if s.world.Party.Location() == game.LocationDungeon {
		return whole
	}
	return rect{1 + game.ViewCentre*2, 1 + game.ViewCentre*2, 2, 2}
}

// Frame is called from the window loop: it animates and repaints the
// viewport when the game goroutine is not drawing, and reports whether the
// canvas is safe to read (then the caller reads it and calls Release).
func (s *Screen) Frame() bool {
	if !s.mu.TryLock() {
		return false
	}
	s.frameLocked(time.Since(s.start))
	return true
}

// Release lets go of the lock Frame took.
func (s *Screen) Release() { s.mu.Unlock() }

// Canvas is the picture; read it only between Frame and Release.
func (s *Screen) Canvas() *image.RGBA { return s.c.RGBA }

func (s *Screen) frameLocked(t time.Duration) {
	// Paused with no menu to say so: PAUSED and no animation. Under the Pause
	// menu the frames run on, so the menu draws and answers the cursor.
	boxed := s.keyboard.Paused() && !s.pauseShown
	if boxed != (s.underPaused != nil) {
		if boxed {
			s.showPaused()
		} else {
			s.hidePaused()
		}
	}
	if s.underPaused != nil {
		return
	}
	if s.overlay != nil {
		s.overlay(t)
		return
	}
	if t-s.lastFrame >= animationTick {
		s.lastFrame = t
		s.gfx.tick()
		if !s.viewCovered {
			s.viewDirty = true
		}
	}
	if s.viewDirty && !s.viewCovered {
		s.paintViewport()
		s.drawPulse()
		s.viewDirty = false
		if s.menu != nil {
			drawMenu(s.c, s.gfx, s.cell, s.menu)
		}
	}
}

// showPaused puts a small black box reading PAUSED over the map window in play, or under the title's menu.
func (s *Screen) showPaused() {
	const text = "PAUSED"
	w, h := len(text)+4, 3
	x, y := (Columns-w)/2, 19
	if s.frameShown {
		x = (1+game.ViewSize*2)/2 - w/2 + 1
		y = (1+game.ViewSize*2)/2 - h/2 + 1
	}
	s.underPaused = s.c.Copy(s.cells(x, y, w, h))
	s.black(x, y, w, h)
	s.drawText(text, x+2, y+1, "")
}

func (s *Screen) hidePaused() {
	if s.underPaused != nil {
		s.c.Restore(s.underPaused)
		s.underPaused = nil
	}
}

// ShowImage shows a full-viewport picture.
func (s *Screen) ShowImage(name string) {
	s.viewCovered = true
	s.view = nil
	s.black(1, 1, 22, 22)
	if img := s.gfx.scenes[name]; img != nil {
		s.c.Blit(img, img.Bounds(), s.cells(1, 1, 22, 22), false)
	}
}

// paintAutoMap draws cells of the dungeon auto-map, at saying which cell
// of the level goes where: w x h of them from (i0, j0), from pixel
// (left, top), with the pieces Peer at gem uses. Secret doors are a wall
// with one pixel out of place. The party, at party, is its first living
// member's figure.
func (s *Screen) paintAutoMap(at func(i, j int) int, origin, size, pos, party [2]int) {
	cell, w := s.cell, s.world
	s.c.Fill(image.Rect(pos[0], pos[1], pos[0]+size[0]*cell, pos[1]+size[1]*cell), rgb(0, 0, 0))
	for row := range size[1] {
		for col := range size[0] {
			c := at(origin[0]+col, origin[1]+row)
			if c < 0 {
				continue
			}
			dx, dy := pos[0]+col*cell, pos[1]+row*cell
			value := w.GetXYDng(c&0x0f, c>>4)
			if value == game.DungeonCellSecretDoor {
				sd := s.secretDoor()
				s.c.Blit(sd, sd.Rect, image.Rect(dx, dy, dx+cell, dy+cell), true)
			} else {
				s.gfx.drawUIPiece(s.c, dungeonMapPieceColumn+dungeonMapPiece(value), 1, dx, dy, cell)
			}
		}
	}
	col, row := party[0]-origin[0], party[1]-origin[1]
	if leader := s.livingLeader(); leader >= 0 && col >= 0 && col < size[0] && row >= 0 && row < size[1] {
		s.drawMember(leader, pos[0]+col*cell, pos[1]+row*cell, cell)
	}
}

// livingLeader is the first member in marching order neither dead nor ashes, or -1.
func (s *Screen) livingLeader() int {
	for m := range 4 {
		if s.world.Party.MemberSlot(m) < 0 {
			continue
		}
		if st := s.world.Member(m).Status(); st != 'D' && st != 'A' {
			return m
		}
	}
	return -1
}

// chrome is the frame's two edge colours (light, dark), read from its
// straight piece, for the map's cursor.
func (s *Screen) chrome() [2]colour {
	if s.chromeColours != nil {
		return *s.chromeColours
	}
	size := s.gfx.uiSize
	off := &Canvas{image.NewRGBA(image.Rect(0, 0, size, size))}
	s.gfx.drawUIPiece(off, pieceHorizontal-1, 0, 0, 0, size)
	light, dark := rgb(255, 255, 255), rgb(0, 0, 0)
	most, least := -1.0, math.Inf(1)
	for y := range size {
		p := off.RGBAAt(size/2, y)
		if p.A < 128 {
			continue
		}
		lum := 0.299*float64(p.R) + 0.587*float64(p.G) + 0.114*float64(p.B)
		if y < size/2 && lum > most {
			most, light = lum, rgb(p.R, p.G, p.B)
		}
		if y >= size/2 && lum < least {
			least, dark = lum, rgb(p.R, p.G, p.B)
		}
	}
	s.chromeColours = &[2]colour{light, dark}
	return *s.chromeColours
}

// secretDoor is the wall piece with one Apple-scale pixel turned black.
func (s *Screen) secretDoor() *image.RGBA {
	if s.secretPiece != nil {
		return s.secretPiece
	}
	size := s.gfx.uiSize
	off := &Canvas{image.NewRGBA(image.Rect(0, 0, size, size))}
	s.gfx.drawUIPiece(off, dungeonMapPieceColumn+2, 1, 0, 0, size)
	dot := max(1, size/32)
	x, y := size*6/10, size*35/100
	off.Fill(image.Rect(x, y, x+dot, y+dot), rgb(0, 0, 0))
	s.secretPiece = off.RGBA
	return off.RGBA
}

// ShowMiniMap draws the whole map in miniature, the party's square blinking, until a key. (DrawMiniMap)
func (s *Screen) ShowMiniMap() {
	w, cell := s.world, s.cell
	s.viewCovered = true
	s.view = nil
	size := w.MapSize()
	mini := max(1, cell*22/size)
	left := 12*cell - mini*(size/2)
	top := left
	s.black(1, 1, 22, 22)
	for y := range size {
		for x := range size {
			s.gfx.drawShape(s.c, (w.GetXYVal(x, y)>>2)*2, left+x*mini, top+y*mini, mini, shapeOpts{})
		}
	}
	for bright := true; ; bright = !bright {
		col := rgb(0x44, 0x44, 0x44)
		if bright {
			col = rgb(255, 255, 255)
		}
		s.c.Fill(image.Rect(left+w.X*mini, top+w.Y*mini, left+(w.X+1)*mini, top+(w.Y+1)*mini), col)
		if _, ok := s.rawKey(150); ok {
			break
		}
	}
	s.RedrawMap()
}

// ShowMiniDungeon draws the level as a 16x16 map from the UI sheet's small pieces until a key. (DrawMiniDng)
func (s *Screen) ShowMiniDungeon() {
	w, cell := s.world, s.cell
	s.viewCovered = true
	s.view = nil
	s.black(1, 1, 22, 22)
	draw := func(xs, ys, piece int) {
		s.gfx.drawUIPiece(s.c, piece+dungeonMapPieceColumn, 1, (xs+4)*cell, (ys+4)*cell, cell)
	}
	under := 6
	for ys := range 16 {
		for xs := range 16 {
			piece := dungeonMapPiece(w.GetXYDng(xs, ys))
			draw(xs, ys, piece)
			if xs == w.X && ys == w.Y {
				under = piece
			}
		}
	}
	for shown := true; ; shown = !shown {
		if shown {
			draw(w.X, w.Y, 8)
		} else {
			draw(w.X, w.Y, under)
		}
		if _, ok := s.rawKey(160); ok {
			break
		}
	}
	s.Print("\n")
	s.RedrawMap()
}

// PlayEnding flashes colours over the map, then shows the closing text until a key.
func (s *Screen) PlayEnding() {
	s.viewCovered = true
	s.view = nil
	for range 21 {
		s.c.Fill(s.cells(1, 1, 22, 22), rgb(uint8(rand.IntN(256)), uint8(rand.IntN(256)), uint8(rand.IntN(256))))
		s.Sound(game.SoundHit)
		s.Pause(120)
	}
	s.black(1, 1, 22, 22)
	lines := []string{
		"   And so it came to",
		"pass  that  on  this",
		"day EXODUS,hell-born",
		"incarnate  of  evil,",
		"was vanquished  from",
		"SOSARIA.   What  now",
		"lies  ahead  in  the",
		"ULTIMA saga can only",
		"be pure speculation!",
		"Onward to ULTIMA IV!",
	}
	for i, line := range lines {
		s.drawText(line, 2, 3+i*2, "")
	}
	s.rawKey(0)
	s.RedrawMap()
}
