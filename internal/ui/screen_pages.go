package ui

import (
	"fmt"
	"image"
	"math"
	"time"

	"github.com/retro-ware/u3-go/internal/game"
	"github.com/retro-ware/u3-go/internal/res"
)

// Pages over the map: Help, the Journal, Tips, the world map, the cheats,
// and the title screens' list and point-sharing menus.

// openPage covers the map area for a page of text, boxed as a menu window
// is. Over the game frame the box is the map's own frame; on a title
// screen the box is drawn first. It returns whether the viewport was
// already covered, for closePage.
func (s *Screen) openPage() bool {
	was := s.viewCovered
	s.viewCovered = true
	if !s.frameShown {
		s.black(1, 1, Columns-2, Rows-2)
		for y := 1; y < 23; y++ {
			s.piece(pieceVertical, 23, y)
		}
		s.piece(pieceTopTee, 23, 0)
		s.piece(pieceBottomTee, 23, 23)
	}
	return was
}

func (s *Screen) closePage(was bool) {
	s.viewCovered = was
	s.redrawAll()
}

// drawPage draws a page: the title in the top edge, up to pageRows lines,
// the cursor line inverted, a note row and footer rows.
func (s *Screen) drawPage(title string, lines, footer []string, note string, cursor int) {
	s.black(1, 0, 22, 23)
	for x := 1; x <= 22; x++ {
		s.piece(pieceHorizontal, x, 0)
	}
	t := clipStr(title, pageWidth)
	tx := 1 + (22-len(t))/2
	s.black(tx, 0, len(t), 1)
	s.drawText(t, tx, 0, "")
	for i, line := range lines {
		if i >= pageRows {
			break
		}
		s.drawText(clipStr(line, pageWidth), pageLeft, pageTop+i, "")
	}
	if cursor >= 0 && cursor < min(len(lines), pageRows) {
		s.invert(pageLeft-1, pageTop+cursor, pageWidth+2, 1)
	}
	if note != "" {
		s.drawText(clipStr(note, pageWidth), pageLeft, pageTop+pageRows, "")
	}
	for i, line := range footer {
		s.drawText(clipStr(line, pageWidth), pageLeft, pageTop+pageRows+1+i, "")
	}
}

// showHelp shows the help pages for the input mode: any key turns the page, Escape or B closes.
func (s *Screen) showHelp() {
	pages := helpPages(s.InputMode, s.TileSetName)
	footer := []string{"Any key next  Esc"}
	if s.InputMode == "controller" {
		footer = []string{"A next  B close"}
	}
	was := s.openPage()
	defer s.closePage(was)
	for _, p := range pages {
		s.drawPage(p.Title, p.Lines, footer, "", -1)
		if k := s.WaitKey(); k == game.KeyEscape || k == game.KeyB {
			return
		}
	}
}

type journalLevel struct {
	title    string
	lines    []game.JournalLine
	text     []string
	from     *game.JournalLine
	selected int
	top      int
}

// ShowJournal is the quest journal, one page over the map: its lines each
// opening to its own. Up and Down move, A (Enter, Right) opens, B
// (Backspace, Left) comes back and on the top closes, Escape closes; Y (H)
// prints the line's hint. The highlighted line's card stands over the panel.
func (s *Screen) ShowJournal() {
	controller := s.InputMode == "controller"
	was := s.openPage()
	defer func() {
		s.showCard(nil)
		game.JournalSnapshot(s.world)
		s.closePage(was)
	}()
	stack := []*journalLevel{{title: "Journal", lines: game.JournalTop(s.world)}}
	for {
		level := stack[len(stack)-1]
		line := level.from
		if level.lines != nil {
			line = &level.lines[level.selected]
		}
		var rows []string
		cursor := -1
		if level.lines != nil {
			for _, l := range level.lines {
				rows = append(rows, l.Label)
			}
			level.top = min(level.top, level.selected)
			if level.selected >= level.top+pageRows {
				level.top = level.selected - pageRows + 1
			}
			cursor = level.selected - level.top
		} else {
			for i, p := range level.text {
				if i > 0 {
					rows = append(rows, "")
				}
				rows = append(rows, wrapText(p, pageWidth, 12)...)
			}
			level.top = max(0, min(len(rows)-pageRows, level.top))
		}
		opens := line != nil && line.Open != nil && level.lines != nil
		footer := []string{"H hint", ""}
		if controller {
			footer[0] = "Y hint"
		}
		note := ""
		if level.lines == nil && len(rows) > pageRows {
			note = "Up/Down: scroll"
		}
		s.drawPage(level.title, rows[min(level.top, len(rows)):], footer, note, cursor)
		var card *game.Card
		if line != nil {
			card = game.JournalCard(*line)
		}
		s.showCard(card)
		key := s.WaitKey()
		switch {
		case key == game.KeyEscape || key == "j" || key == "J":
			return
		case key == game.KeyB || key == game.KeyBackspace || key == game.KeyLeft:
			if len(stack) > 1 {
				stack = stack[:len(stack)-1]
			} else if key != game.KeyLeft {
				return
			}
		case key == game.KeyUp || key == game.KeyDown:
			step := 1
			if key == game.KeyUp {
				step = -1
			}
			if level.lines != nil {
				level.selected = (level.selected + step + len(level.lines)) % len(level.lines)
			} else {
				level.top += step
			}
		case key == game.KeyA || key == game.KeyEnter || key == game.KeyRight:
			if opens {
				o := line.Open
				stack = append(stack, &journalLevel{title: o.Title, lines: o.Lines, text: o.Text, from: line})
			}
		case line != nil && (key == game.KeyY || key == "h" || key == "H"):
			s.printWrapped(line.Hint)
		}
	}
}

// showTips lists the tips; A opens one, B or Escape closes.
func (s *Screen) showTips() {
	was := s.openPage()
	defer s.closePage(was)
	topics := make([]string, len(game.Tips))
	for i, t := range game.Tips {
		topics[i] = t.Topic
	}
	selected := 0
	for {
		s.drawPage("Tips", topics, nil, "", selected)
		switch key := s.WaitKey(); key {
		case game.KeyEscape, game.KeyB, game.KeyBackspace:
			return
		case game.KeyUp:
			selected = (selected + len(topics) - 1) % len(topics)
		case game.KeyDown:
			selected = (selected + 1) % len(topics)
		case game.KeyA, game.KeyEnter, game.KeyRight:
			if !s.readTip(game.Tips[selected]) {
				return
			}
		}
	}
}

// readTip shows a topic a page at a time; false where Escape closed Tips altogether.
func (s *Screen) readTip(tip game.Tip) bool {
	pages := game.TipPages(tip, pageWidth, pageRows, wrapText)
	for index := 0; index < len(pages); {
		s.drawPage(tip.Topic, pages[index], nil, "", -1)
		switch s.WaitKey() {
		case game.KeyEscape:
			return false
		case game.KeyB, game.KeyBackspace, game.KeyLeft:
			return true
		case game.KeyA, game.KeyEnter, game.KeyRight:
			index++
		}
	}
	return true
}

// ShowMap shows the map of what the party has seen over the whole screen
// until closed; no turn passes. The d-pad moves between the marks, A opens
// the map of a town or castle been in and B comes back, Y comes back to the
// party, X turns to the next map, and B on the first page, Escape or # closes.
func (s *Screen) ShowMap() {
	w := s.world
	controller := s.InputMode == "controller"
	was := s.openPage()
	pages := game.MapPages(w)
	var back []game.MapPage
	turned, selected := 0, 0
	var page game.MapPage
	var marks []game.MapMark
	var art *image.RGBA
	show := func(next game.MapPage) {
		page = next
		marks = game.MarksOn(w, page)
		selected = 0
		art = nil
		if !page.Dungeon {
			art = s.mapArt(page.ID)
		}
	}
	show(pages[0])
	s.overlay = func(t time.Duration) {
		s.paintWorldMap(page, art, marks, selected, len(back) > 0, len(pages) > 1, controller, t)
	}
	defer func() {
		s.overlay = nil
		s.closePage(was)
	}()
	for {
		key := s.WaitKey()
		letter := ""
		if len(key) == 1 && key[0] >= 'A' {
			letter = string(key[0] | 0x20)
		}
		switch {
		case key == game.KeyEscape || key == game.KeyPause || key == "#":
			return
		case key == game.KeyB || key == game.KeyBackspace:
			if len(back) == 0 {
				if key == game.KeyB {
					return
				}
				continue
			}
			show(back[len(back)-1])
			back = back[:len(back)-1]
		case isDirection(key):
			dir := map[string]game.MarkDirection{game.KeyUp: game.DirUp, game.KeyDown: game.DirDown, game.KeyLeft: game.DirLeft, game.KeyRight: game.DirRight}[key]
			side := 16
			if !page.Dungeon {
				side = game.MapSide
			}
			wraps := !page.Dungeon && (page.ID == res.Sosaria || page.ID == res.Ambrosia)
			selected = game.StepMark(marks, selected, dir, side, wraps)
		case key == game.KeyY || key == game.KeySpace || letter == "y":
			selected = 0
			for i, m := range marks {
				if m.Kind == game.MarkParty || m.Party {
					selected = i
					break
				}
			}
		case key == game.KeyX || letter == "x":
			back = nil
			turned = (turned + 1) % len(pages)
			show(pages[turned])
		case key == game.KeyA || key == game.KeyEnter:
			if selected < len(marks) && marks[selected].Opens != 0 {
				back = append(back, page)
				show(game.MapPage{ID: marks[selected].Opens})
			}
		}
	}
}

// mapArt is a map page's picture: what the party has seen, square by
// square in the tile set's tiles, over Sosaria's chart or the dark.
func (s *Screen) mapArt(id int) *image.RGBA {
	w := s.world
	sq := 22 * s.cell / game.MapSide
	state := w.Current
	if id == res.Sosaria {
		state = w.Surface
	} else if id != w.Current.ID {
		state = w.LoadMapState(id)
	}
	var art *Canvas
	if id == res.Sosaria {
		raw := w.Resources.Maps[id]
		art = drawChart(float64(sq), func(x, y int) bool {
			i := 1 + y*game.MapSide + x
			return i < len(raw) && game.IsLandValue(int(raw[i]))
		})
	} else {
		art = NewCanvas(sq*game.MapSide, sq*game.MapSide)
	}
	for y := range game.MapSide {
		for x := range game.MapSide {
			if !w.AutoMap.SquareSeen(id, x, y) {
				continue
			}
			value := int(state.Tiles[y*game.MapSide+x])
			if someone := state.Monsters.At(x, y); someone >= 0 {
				value = state.Monsters.TileUnder(someone)
			}
			s.gfx.drawShape(art, (value>>2)*2, x*sq, y*sq, sq, shapeOpts{})
		}
	}
	return art.RGBA
}

// paintWorldMap paints one frame of the map: the page, its marks and the
// cursor, and the legend beside it.
func (s *Screen) paintWorldMap(page game.MapPage, art *image.RGBA, marks []game.MapMark, selected int, canBack, more, controller bool, t time.Duration) {
	cell, w := s.cell, s.world
	s.black(1, 1, 22, 22)
	var level *game.WholeLevel
	sq := float64(22*cell) / game.MapSide
	left, top := float64(cell), float64(cell)
	if page.Dungeon {
		l := game.WholeLevelOf(w)
		level = &l
		sq = float64(cell)
		left = float64(cell + (22-l.Cols)*cell/2)
		top = float64(cell + (22-l.Rows)*cell/2)
		s.paintAutoMap(l.At, [2]int{0, 0}, [2]int{l.Cols, l.Rows}, [2]int{int(left), int(top)}, l.Party)
	} else if art != nil {
		s.c.Blit(art, art.Bounds(), s.cells(1, 1, 22, 22), false)
	}
	corner := func(m game.MapMark) (float64, float64) {
		x, y := m.X, m.Y
		if level != nil && m.Kind == game.MarkParty {
			x, y = level.Party[0], level.Party[1]
		}
		return left + float64(x)*sq, top + float64(y)*sq
	}
	for _, m := range marks {
		if m.Unseen {
			x, y := corner(m)
			drawPlaceSign(s.c, x+sq/2, y+sq/2, sq, m.Kind == game.MarkCastle)
		}
	}
	for _, m := range marks {
		if m.Kind == game.MarkMoongate {
			x, y := corner(m)
			s.drawMoonMark(x+sq/2, y+sq/2, sq*0.9, m.Phase)
		}
	}
	// The party, blinking.
	for _, m := range marks {
		if m.Kind != game.MarkParty && !m.Party {
			continue
		}
		if (t/(350*time.Millisecond))%2 == 0 {
			x, y := corner(m)
			lw := math.Max(2, sq/5)
			s.strokeRect(x-1, y-1, sq+2, sq+2, lw, rgb(255, 255, 255))
		}
		break
	}
	// The cursor: brackets round the mark it is on, three squares across, in
	// the frame's two edge colours, dark outside, light inside. Where the
	// party is all there is to mark there is no cursor.
	var chosen *game.MapMark
	if selected < len(marks) {
		chosen = &marks[selected]
	}
	places := len(marks) > 1
	if chosen != nil && places {
		x, y := corner(*chosen)
		r := sq * 1.5
		cx, cy := x+sq/2, y+sq/2
		band := max(1, int(math.Round(float64(2*cell)/32)))
		ch := s.chrome()
		inward := []colour{ch[1], ch[0]} // dark, light
		th := band * len(inward)
		arm := max(th*2, int(math.Round(r*0.7)))
		l, tp := int(math.Round(cx-r)), int(math.Round(cy-r))
		rt, bt := int(math.Round(cx+r))-th, int(math.Round(cy+r))-th
		fill := func(c colour, fx, fy, fw, fh int) { s.c.Fill(image.Rect(fx, fy, fx+fw, fy+fh), c) }
		reversed := []colour{inward[1], inward[0]}
		for _, p := range [][2]int{{l, tp}, {rt, tp}, {l, bt}, {rt, bt}} {
			sx, sy := p[0], p[1]
			atLeft, atTop := sx == l, sy == tp
			across, down := inward, inward
			if !atTop {
				across = reversed
			}
			if !atLeft {
				down = reversed
			}
			ax, ay := sx+th-arm, sy+th-arm
			if atLeft {
				ax = sx + th
			}
			if atTop {
				ay = sy + th
			}
			for k := range inward {
				fill(across[k], ax, sy+k*band, arm-th, band)
				fill(down[k], sx+k*band, ay, band, arm-th)
			}
			for dy := range th {
				for dx := range th {
					fromSide, fromEnd := dx, dy
					if !atLeft {
						fromSide = th - 1 - dx
					}
					if !atTop {
						fromEnd = th - 1 - dy
					}
					bands, at := down, dx
					if fromEnd < fromSide {
						bands, at = across, dy
					}
					fill(bands[at/band], sx+dx, sy+dy, 1, 1)
				}
			}
		}
	}
	// The legend, in one box beside the map's.
	const lx, width = 24, 15
	s.black(lx, 1, width+1, 23)
	for x := lx; x < 39; x++ {
		s.piece(pieceHorizontal, x, 0)
		s.piece(pieceHorizontal, x, 23)
	}
	for y := 1; y < 23; y++ {
		s.piece(pieceVertical, 23, y)
		s.piece(pieceVertical, 39, y)
	}
	s.piece(pieceTopTee, 23, 0)
	s.piece(pieceBottomTee, 23, 23)
	s.piece(pieceTopRight, 39, 0)
	s.piece(pieceBottomRight, 39, 23)
	const grey = "#b0b0b0"
	for i, line := range wrapText(game.PageTitle(w, page), width, 2) {
		s.drawText(line, lx+(width-len(line))/2, 1+i, "")
	}
	row := 4
	if chosen != nil {
		for _, line := range wrapText(chosen.Label, width, 3) {
			s.drawText(line, lx, row, "")
			row++
		}
		for _, line := range wrapText(chosen.Detail, width, 2) {
			s.drawText(line, lx, row, grey)
			row++
		}
		if chosen.Kind == game.MarkMoongate {
			s.drawMoonMark(float64((lx+1)*cell), (float64(row)+0.6)*float64(cell), float64(cell)*0.55, chosen.Phase)
		}
	}
	opens := chosen != nil && chosen.Opens != 0
	pick := func(on bool, text string) string {
		if on {
			return text
		}
		return ""
	}
	var keys []string
	if controller {
		keys = []string{pick(places, "D-pad: places"), pick(opens, "A: its map"), pick(places, "Y: the party"), pick(more, "X: next map"), "B: close"}
		if canBack {
			keys[4] = "B: back"
		}
	} else {
		keys = []string{pick(places, "Arrows: places"), pick(opens, "Enter: its map"), pick(places, "Space: party"), pick(more, "X: next map"), "Esc: close"}
		if canBack {
			keys[4] = "Bksp: back"
		}
	}
	var shown []string
	for _, k := range keys {
		if k != "" {
			shown = append(shown, k)
		}
	}
	for i, line := range shown {
		s.drawText(line, lx, 23-len(shown)+i, grey)
	}
}

// strokeRect outlines a rectangle lw pixels wide, the line centred on its edge.
func (s *Screen) strokeRect(x, y, w, h, lw float64, col colour) {
	half := lw / 2
	s.c.Fill(frect(x-half, y-half, w+lw, lw), col)
	s.c.Fill(frect(x-half, y+h-half, w+lw, lw), col)
	s.c.Fill(frect(x-half, y-half, lw, h+lw), col)
	s.c.Fill(frect(x+w-half, y-half, lw, h+lw), col)
}

// drawMoonMark draws the moon (Trammel's phase) that opens a gate, in a round frame centred on (x, y).
func (s *Screen) drawMoonMark(x, y, radius float64, phase int) {
	d := max(1, int(math.Round(radius*2)))
	tmp := &Canvas{image.NewRGBA(image.Rect(0, 0, d, d))}
	s.gfx.drawMoon(tmp, 0, phase, 0, 0, d)
	r2 := radius * radius
	for py := range d {
		for px := range d {
			if dx, dy := float64(px)+0.5-radius, float64(py)+0.5-radius; dx*dx+dy*dy > r2 {
				tmp.SetRGBA(px, py, colour{})
			}
		}
	}
	ox, oy := int(math.Round(x-radius)), int(math.Round(y-radius))
	s.c.Blit(tmp, tmp.Rect, image.Rect(ox, oy, ox+d, oy+d), true)
	s.c.FillPolygon(ring(x, y, radius, math.Max(2, float64(s.cell)/12)), hexColour("#f0e0a0"))
}

// cheatMenu is the cheats, from the Pause menu: a confirmation line, or "" when nothing was done.
func (s *Screen) cheatMenu() string {
	var cheats []game.Cheat
	var options []game.MenuOption
	for _, c := range game.Cheats {
		if c.Available(s.world) {
			cheats = append(cheats, c)
			options = append(options, game.MenuOption{Key: c.Key, Label: c.Label})
		}
	}
	options = append(options, game.MenuOption{Key: "B", Label: "Back"})
	picked := s.runMenu("Cheats", options, 1, &game.MenuPlacement{Row: 8, Title: "Cheats"}, 0, nil)
	if picked < 0 || picked >= len(cheats) {
		return ""
	}
	return cheats[picked].Apply(s.world, s)
}

// ChooseFromList picks from a list shown as a menu in every input mode.
func (s *Screen) ChooseFromList(options []game.MenuOption, place *game.MenuPlacement) string {
	title := "Choose"
	if place != nil {
		title = place.Title
	}
	if picked := s.runMenu(title, options, 1, place, 0, nil); picked >= 0 {
		return options[picked].Key
	}
	return ""
}

// AllocatePoints shares total points among the labels, Left and Right
// moving points; A on O.K. accepts once all are spent.
func (s *Screen) AllocatePoints(labels []string, total, lo, hi int, place *game.MenuPlacement) []int {
	n := len(labels)
	values := make([]int, n)
	width := 0
	for i, l := range labels {
		values[i] = total / n
		if i < total%n {
			values[i]++
		}
		width = max(width, len(l))
	}
	left := func() int {
		sum := 0
		for _, v := range values {
			sum += v
		}
		return total - sum
	}
	items := func() []string {
		var out []string
		for i, l := range labels {
			out = append(out, fmt.Sprintf("%s  < %2d >", padEnd(l, width), values[i]))
		}
		return append(out, "O.K.")
	}
	hints := func() []string {
		hint := "All points spent"
		if left() > 0 {
			hint = fmt.Sprintf("%d points left to spend", left())
		}
		out := make([]string, n+1)
		for i := range out {
			out[i] = hint
		}
		if left() == 0 {
			out[n] = "Press A or Enter to accept"
		}
		return out
	}
	m := layoutMenu(place.Title, items(), 1)
	m.Hints = hints()
	s.placeMenu(m, place)
	s.openMenu(m)
	defer s.closeMenu()
	for {
		key, ok := s.readKey(0)
		if !ok {
			continue
		}
		switch {
		case key == game.KeyLeft || key == game.KeyRight:
			if m.Cursor < n {
				delta := 1
				if key == game.KeyLeft {
					delta = -1
				}
				if delta > 0 && left() <= 0 {
					s.Sound(game.SoundError1) // the pool can be spent, never overspent
				} else {
					values[m.Cursor] = max(lo, min(hi, values[m.Cursor]+delta))
				}
				m.Items, m.Hints = items(), hints()
				s.showMenuNow()
			}
		case moveCursor(m, key):
			s.showMenuNow()
		case key == game.KeyA || key == game.KeyEnter:
			if m.Cursor < n {
				m.Cursor++
				s.showMenuNow()
			} else if left() == 0 {
				return values
			}
		case key == game.KeyB || key == game.KeyEscape:
			return nil
		}
	}
}
