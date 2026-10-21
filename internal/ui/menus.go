package ui

import (
	"image"
	"image/color"
	"strings"
	"time"

	"github.com/retro-ware/u3-go/internal/game"
)

// Controller mode (menus.ts): pop-up menu windows drawn over the map, the
// command lists they show (in the spirit of the NES Ultima III, where a
// button opened a command window), the keyboard stand-ins for the four
// buttons, and a gamepad reader that turns button presses into keys.
// Nothing here knows about the game; the Screen decides when to show a
// menu and what to do with the answer.

// controllerKeyFor is how keyboard keys stand in for the controller in
// controller mode. Escape is not among them: it pauses, as the pad's
// system buttons do, and a menu takes it as backing out anyway.
func controllerKeyFor(key string) string {
	switch key {
	case "w", "W":
		return game.KeyUp
	case "s", "S":
		return game.KeyDown
	case "a", "A":
		return game.KeyLeft
	case "d", "D":
		return game.KeyRight
	case game.KeyEnter, "z", "Z":
		return game.KeyA
	case "x", "X", "b", "B":
		return game.KeyB
	case "c", "C":
		return game.KeyX
	case "v", "V", "y", "Y":
		return game.KeyY
	}
	return key
}

var directionKeys = []string{game.KeyUp, game.KeyDown, game.KeyLeft, game.KeyRight}

func isDirection(key string) bool {
	return key == game.KeyUp || key == game.KeyDown || key == game.KeyLeft || key == game.KeyRight
}

// CommandMenus are the command menus for each scope. Pass is not among
// them (B passes), nor Pause (the system buttons and Escape open it), nor
// what it holds: Quit and save and Auto combat are in the Pause menu.
var CommandMenus = map[game.CommandScope][]game.MenuOption{
	game.ScopeField: {
		{Key: "A", Label: "Attack"},
		{Key: "T", Label: "Transact (talk)"},
		{Key: "L", Label: "Look"},
		{Key: "E", Label: "Enter"},
		{Key: "B", Label: "Board"},
		{Key: "X", Label: "X-it craft"},
		{Key: "F", Label: "Fire cannons"},
		{Key: "G", Label: "Get chest"},
		{Key: "U", Label: "Unlock door"},
		{Key: "S", Label: "Steal"},
		{Key: "C", Label: "Cast spell"},
		{Key: "Z", Label: "Ztats"},
		{Key: "R", Label: "Ready weapon"},
		{Key: "W", Label: "Wear armour"},
		{Key: "M", Label: "Modify order"},
		{Key: "N", Label: "Negate time"},
		{Key: "P", Label: "Peer at gem"},
		{Key: "O", Label: "Other command"},
		{Key: "J", Label: "Journal"},
		{Key: game.ViewMapKey, Label: "View map"},
	},
	game.ScopeCombat: {
		{Key: "A", Label: "Attack"},
		{Key: "C", Label: "Cast spell"},
		{Key: "N", Label: "Negate time"},
		{Key: "R", Label: "Ready weapon"},
		{Key: "Z", Label: "Ztats"},
	},
	game.ScopeDungeon: {
		{Key: "I", Label: "Ignite torch"},
		{Key: "L", Label: "Change view"},
		{Key: "K", Label: "Klimb ladder"},
		{Key: "D", Label: "Descend ladder"},
		{Key: "G", Label: "Get chest"},
		{Key: "C", Label: "Cast spell"},
		{Key: "Z", Label: "Ztats"},
		{Key: "P", Label: "Peer at gem"},
		{Key: "R", Label: "Ready weapon"},
		{Key: "W", Label: "Wear armour"},
		{Key: "M", Label: "Modify order"},
		{Key: "N", Label: "Negate time"},
		{Key: "O", Label: "Other command"},
		{Key: "J", Label: "Journal"},
		{Key: game.ViewMapKey, Label: "View map"},
	},
}

// buttonShortcuts are what B, X and Y do outside a menu, per scope.
var buttonShortcuts = map[game.CommandScope][3]string{
	game.ScopeField:   {" ", "Z", "L"},
	game.ScopeCombat:  {" ", "Z", "A"},
	game.ScopeDungeon: {" ", "Z", "I"},
}

// The on-screen keyboard for typing names and words with a controller.
var onScreenKeyboard = []string{"ABCDEFGHIJKLM", "NOPQRSTUVWXYZ", "abcdefghijklm", "nopqrstuvwxyz", "0123456789 '-"}

// Its delete and done keys, drawn as DEL and DONE.
const (
	oskDelete = "<"
	oskDone   = ">"
)

// oskKeys are every key of the on-screen keyboard, in order.
var oskKeys = func() []string {
	var out []string
	for _, r := range strings.Join(onScreenKeyboard, "") {
		out = append(out, string(r))
	}
	return append(out, oskDelete, oskDone)
}()

// MenuWindow is a menu window's state, kept by the Screen and drawn every frame.
type MenuWindow struct {
	Title       string
	Items       []string
	Cursor      int
	Top         int // first visible item row when the list scrolls
	X, Y        int // cells
	Width       int
	VisibleRows int
	Columns     int // for grid menus (the on-screen keyboard): items per row

	Hints       []string // optional one-line hint per item, drawn under the window
	HintColours []string
	Disabled    []bool // items drawn grey and refused
	Colours     []string
	// SpreadLastRow lays a grid's short last row across the window, its
	// first item at the left and its last at the right (DEL and DONE).
	SpreadLastRow bool
}

const (
	maxMenuRows   = 12
	screenColumns = 40
	hintRows      = 2 // rows kept under a menu window for its hint
)

// layoutMenu lays out a menu window over the map area (cells 1..22). A
// grid may be wider than the map area; it is then centred on the screen.
func layoutMenu(title string, items []string, columns int) *MenuWindow {
	if columns < 1 {
		columns = 1
	}
	longest := 0
	for _, s := range items {
		longest = max(longest, len(s))
	}
	rows := (len(items) + columns - 1) / columns
	visible := min(rows, maxMenuRows)
	var width int
	if columns > 1 {
		width = min(screenColumns-2, max(len(title)+4, columns*(longest+1)+3))
	} else {
		width = min(22, max(8, max(len(title), longest)+4))
	}
	height := visible + 2
	centre := 12.0
	if width > 22 {
		centre = screenColumns / 2
	}
	return &MenuWindow{
		Title: title, Items: items, Columns: columns, Width: width, VisibleRows: visible,
		X: max(1, int(centre-float64(width)/2)),
		Y: max(1, int(12-float64(height)/2)),
	}
}

// layoutKeyboard is the on-screen keyboard's window.
func layoutKeyboard(title string) *MenuWindow {
	m := layoutMenu(title, oskKeys, len(onScreenKeyboard[0]))
	m.Items = append([]string(nil), oskKeys...)
	m.Items[len(m.Items)-2], m.Items[len(m.Items)-1] = "DEL", "DONE"
	m.SpreadLastRow = true
	return m
}

// spreadRow is where a spread last row starts, or -1.
func spreadRow(m *MenuWindow) int {
	n, c := len(m.Items), m.Columns
	if m.SpreadLastRow && c > 1 && n%c != 0 {
		return n / c * c
	}
	return -1
}

func at(s []string, i int) string {
	if i < len(s) {
		return s[i]
	}
	return ""
}

// drawMenu draws a menu window with the bitmap font and the border pieces.
func drawMenu(c *Canvas, g *Graphics, cell int, m *MenuWindow) {
	x, y, width := m.X, m.Y, m.Width
	height := m.VisibleRows + 2
	black := rgb(0, 0, 0)
	c.Fill(image.Rect(x*cell, y*cell, (x+width)*cell, (y+height)*cell), black)
	piece := func(n, px, py int) { g.drawUIPiece(c, n-1, 0, px*cell, py*cell, cell) }
	for i := 1; i < width-1; i++ {
		piece(pieceHorizontal, x+i, y)
		piece(pieceHorizontal, x+i, y+height-1)
	}
	for j := 1; j < height-1; j++ {
		piece(pieceVertical, x, y+j)
		piece(pieceVertical, x+width-1, y+j)
	}
	piece(pieceTopLeft, x, y)
	piece(pieceTopRight, x+width-1, y)
	piece(pieceBottomLeft, x, y+height-1)
	piece(pieceBottomRight, x+width-1, y+height-1)

	text := func(s string, tx, ty int) {
		for i := range len(s) {
			g.drawGlyph(c, s[i], (tx+i)*cell, ty*cell, cell)
		}
	}
	title := m.Title
	if len(title) > width-4 {
		title = title[:max(0, width-4)]
	}
	tx := x + (width-len(title))/2
	c.Fill(image.Rect(tx*cell, y*cell, (tx+len(title))*cell, (y+1)*cell), black)
	text(title, tx, y)

	colWidth := (width - 3) / m.Columns
	spread := spreadRow(m)
	for row := range m.VisibleRows {
		for col := range m.Columns {
			index := (m.Top+row)*m.Columns + col
			if index >= len(m.Items) {
				continue
			}
			px := x + 2 + col*colWidth
			py := y + 1 + row
			limit := width - 4
			if m.Columns > 1 {
				limit = colWidth - 1
			}
			label := m.Items[index]
			if len(label) > limit {
				label = label[:max(0, limit)]
			}
			if spread >= 0 && index >= spread {
				label = m.Items[index]
				if index == len(m.Items)-1 && index > spread {
					px = x + 2 + (m.Columns-1)*colWidth + 1 - len(label)
				}
			}
			text(label, px, py)
			if index == m.Cursor {
				c.Invert(image.Rect((px-1)*cell, py*cell, (px+len(label))*cell, (py+1)*cell))
			}
			tint := at(m.Colours, index)
			if index < len(m.Disabled) && m.Disabled[index] {
				tint = "#707070"
			}
			if tint != "" {
				left := px
				if index == m.Cursor {
					left--
				}
				c.Multiply(image.Rect(left*cell, py*cell, (px+len(label))*cell, (py+1)*cell), namedColour(tint))
			}
		}
	}
	if m.Top > 0 {
		text("^", x+width-2, y+1)
	}
	if (m.Top+m.VisibleRows)*m.Columns < len(m.Items) {
		text("v", x+width-2, y+height-2)
	}
	if m.Hints != nil {
		c.Fill(image.Rect(cell, (y+height)*cell, 39*cell, (y+height+hintRows)*cell), black)
		if hint := at(m.Hints, m.Cursor); hint != "" {
			tint := at(m.HintColours, m.Cursor)
			for i, line := range wrapText(hint, 38, hintRows) {
				lx := max(1, 20-len(line)/2)
				text(line, lx, y+height+i)
				if tint != "" {
					c.Multiply(image.Rect(lx*cell, (y+height+i)*cell, (lx+len(line))*cell, (y+height+i+1)*cell), namedColour(tint))
				}
			}
		}
	}
}

type colour = color.RGBA

// Border pieces in row 0 of the UI sheet, numbered from 1 as DrawFramePiece numbered them.
const (
	pieceTopLeft     = 1
	pieceTopTee      = 2
	pieceTopRight    = 3
	pieceLeftTee     = 4
	pieceRightTee    = 6
	pieceBottomLeft  = 7
	pieceBottomTee   = 8
	pieceBottomRight = 9
	pieceHorizontal  = 10
	pieceVertical    = 11
	pieceCapLeft     = 12
	pieceCapRight    = 13
)

// namedColour reads the CSS-style colours the game names, or #rgb.
func namedColour(s string) colour {
	if c, ok := cssColours[strings.ToLower(s)]; ok {
		return hexColour(c)
	}
	return hexColour(s)
}

var cssColours = map[string]string{
	"red": "#ff0000", "green": "#008000", "lime": "#00ff00", "blue": "#0000ff", "yellow": "#ffff00",
	"cyan": "#00ffff", "magenta": "#ff00ff", "white": "#ffffff", "grey": "#808080", "gray": "#808080",
	"orange": "#ffa500", "gold": "#ffd700", "silver": "#c0c0c0", "purple": "#800080", "black": "#000000",
}

// wrapText breaks text into at most rows lines of width characters, at spaces where possible.
func wrapText(s string, width, rows int) []string {
	var lines []string
	rest := strings.TrimSpace(s)
	for rest != "" && len(lines) < rows {
		if len(rest) <= width {
			lines = append(lines, rest)
			break
		}
		cut := strings.LastIndexByte(rest[:width+1], ' ')
		if cut <= 0 {
			cut = width
		}
		lines = append(lines, strings.TrimRight(rest[:cut], " "))
		rest = strings.TrimLeft(rest[cut:], " ")
	}
	return lines
}

// moveCursor moves the cursor for a d-pad key and keeps it visible; false if the key is no movement.
func moveCursor(m *MenuWindow, key string) bool {
	n, c := len(m.Items), m.Columns
	scroll := func() {
		row := m.Cursor / c
		if row < m.Top {
			m.Top = row
		}
		if row >= m.Top+m.VisibleRows {
			m.Top = row - m.VisibleRows + 1
		}
	}
	if spread := spreadRow(m); spread >= 0 && isDirection(key) {
		inLast := m.Cursor >= spread
		colOf := func(i int) int {
			switch {
			case i < spread:
				return i % c
			case i == spread:
				return 0
			}
			return c - 1
		}
		nearer := func(col int) int {
			if float64(col) < float64(c)/2 {
				return spread
			}
			return n - 1
		}
		next := -1
		switch key {
		case game.KeyLeft, game.KeyRight:
			if inLast {
				next = spread
				if m.Cursor == spread {
					next = n - 1
				}
			}
		case game.KeyUp:
			if inLast {
				next = spread - c + colOf(m.Cursor)
			} else if m.Cursor < c {
				next = nearer(m.Cursor)
			}
		default:
			if inLast {
				next = colOf(m.Cursor)
			} else if m.Cursor+c >= spread {
				next = nearer(m.Cursor % c)
			}
		}
		if next >= 0 {
			m.Cursor = next
			scroll()
			return true
		}
	}
	var next int
	switch key {
	case game.KeyUp:
		next = m.Cursor - c
		if next < 0 {
			next = m.Cursor + (n-1-m.Cursor)/c*c // wrap to the bottom of the column
		}
	case game.KeyDown:
		next = m.Cursor + c
		if next >= n {
			if m.Cursor/c < (n-1)/c {
				next = n - 1 // into a short last row
			} else {
				next = m.Cursor % c
			}
		}
	case game.KeyLeft:
		next = m.Cursor
		if c > 1 {
			if m.Cursor%c == 0 {
				next = m.Cursor + c - 1
			} else {
				next = m.Cursor - 1
			}
		}
	case game.KeyRight:
		next = m.Cursor
		if c > 1 {
			if m.Cursor%c == c-1 {
				next = m.Cursor - c + 1
			} else {
				next = m.Cursor + 1
			}
		}
	default:
		return false
	}
	m.Cursor = max(0, min(n-1, next))
	scroll()
	return true
}

// Menu sounds: a footstep as the cursor moves, the attack for a choice, a swing on backing out.
var menuSounds = struct{ move, choose, back string }{game.SoundStep, game.SoundAttack, game.SoundSwish[0]}

// Menu actions: what a key did in a menu window.
const (
	actNone = iota
	actMove
	actChoose
	actRefused
	actBack
)

// applyMenuKey applies a key to a menu window: a direction moves, A or
// Enter chooses, B, Escape or Pause backs out, and a letter chooses the
// option with that key, shown or hidden. options is the whole list and
// visible the options the window shows. It returns the action and, for a
// choice, the index in options.
func applyMenuKey(m *MenuWindow, options, visible []game.MenuOption, key string) (int, int) {
	before := m.Cursor
	if moveCursor(m, key) {
		if m.Cursor == before {
			return actNone, 0
		}
		return actMove, 0
	}
	if key == game.KeyA || key == game.KeyEnter {
		if m.Cursor >= len(visible) {
			return actBack, 0
		}
		o := visible[m.Cursor]
		if o.Disabled {
			return actRefused, 0
		}
		for i := range options {
			if options[i].Key == o.Key && options[i].Label == o.Label {
				return actChoose, i
			}
		}
		return actBack, 0
	}
	if key == game.KeyB || key == game.KeyEscape || key == game.KeyPause {
		return actBack, 0
	}
	upper := strings.ToUpper(key)
	for i, o := range options {
		if o.Key == upper {
			if o.Disabled {
				return actRefused, 0
			}
			return actChoose, i
		}
	}
	return actNone, 0
}

// A held direction repeats, as a held key does: after this long, then at this rate.
const (
	gamepadRepeatFirst = 250 * time.Millisecond
	gamepadRepeat      = 120 * time.Millisecond
)

// PadState is one gamepad's buttons, in the standard layout.
type PadState struct {
	A, B, X, Y, Pause     bool
	Up, Down, Left, Right bool
}

// GamepadReader turns pad states into keys: a press is one key, a held
// direction repeats while the game waits, so walking flows at the game's
// pace and nothing piles up; the buttons never repeat.
type GamepadReader struct {
	held map[string]*struct{ since, last time.Duration }
}

// Poll takes each pad's state at time now and pushes new presses to k.
func (r *GamepadReader) Poll(now time.Duration, pads map[int]PadState, k *Keyboard, onActivity func()) {
	if r.held == nil {
		r.held = map[string]*struct{ since, last time.Duration }{}
	}
	for id, p := range pads {
		for _, b := range []struct {
			key  string
			down bool
		}{{game.KeyA, p.A}, {game.KeyB, p.B}, {game.KeyX, p.X}, {game.KeyY, p.Y}, {game.KeyPause, p.Pause},
			{game.KeyUp, p.Up}, {game.KeyDown, p.Down}, {game.KeyLeft, p.Left}, {game.KeyRight, p.Right}} {
			name := string(rune('0'+id)) + b.key
			was := r.held[name]
			if !b.down {
				delete(r.held, name)
				continue
			}
			if was == nil {
				r.held[name] = &struct{ since, last time.Duration }{now, now}
				k.Push(b.key, sourceGamepad)
				if onActivity != nil {
					onActivity()
				}
				continue
			}
			if isDirection(b.key) && now-was.since >= gamepadRepeatFirst && now-was.last >= gamepadRepeat && k.Waiting() {
				was.last = now
				k.Push(b.key, sourceGamepad)
			}
		}
	}
}
