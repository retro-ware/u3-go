package ui

import (
	"fmt"
	"image"
	"strings"
	"sync"
	"time"

	"github.com/retro-ware/u3-go/internal/game"
)

// The screen, and the implementation of game.IO (screen.ts).
//
// The screen is a grid of 40 x 24 cells. The layout is the Apple II one:
//
//	columns 0..23,  rows 0..23   border and the 11x11 map viewport (each
//	                             map tile is 2x2 cells, from cell 1,1)
//	columns 24..38, rows 1..11   four character boxes, two rows each,
//	                             separated by border lines at rows 3, 6, 9, 12
//	columns 24..39, rows 13..23  the scrolling message area; the prompt is
//	                             on row 23
//	row 0                        party gold, moon phases, party food; row 23 wind
//
// The border, text and stats are drawn onto the canvas when they change;
// the viewport is redrawn twelve times a second from a cached Viewport so
// terrain and creatures animate. In a dungeon it shows the first-person view.

// The screen's size in cells.
const (
	Columns = 40
	Rows    = 24
)

const (
	boxRows       = 2
	pageLeft      = 2
	pageTop       = 1
	boxPitch      = 3
	boxesBottom   = 12
	textLeft      = 24
	textRight     = 40 // exclusive
	textTop       = 13
	textBottom    = 24 // exclusive
	animationTick = time.Second / 12

	dungeonMapPieceColumn = 7 // the dungeon map's pieces, in row 1 of the UI sheet
	mapOverlaySize        = 5 // the auto-map's 5x5 overlay, under the fourth box
	mapOverlayLeft        = 35
	mapOverlayTop         = 13
	topDownReach          = 10 // the Top down view's cells each way from the party
)

var boxSeparators = []int{3, 6, 9, 12}

func onOff(on bool) string {
	if on {
		return "On"
	}
	return "Off"
}

// What each member's second line shows: hit points and mana, or, while the
// command menu's bar is on Ztats, Ready weapon or Wear armour, the
// experience, the weapon in hand or the armour worn.
const (
	lineHP     = "hp"
	lineXP     = "xp"
	lineWeapon = "weapon"
	lineArmour = "armour"
)

var panelLines = map[string]string{"Z": lineXP, "R": lineWeapon, "W": lineArmour}

// cardTones are a card line's colours: better green, worse red, by the way grey.
var cardTones = map[game.CardTone]string{game.CardToneBetter: "#40ff40", game.CardToneWorse: "#ff4040", game.CardToneDim: "#b0b0b0"}

// dungeonMapPiece is which of the nine map pieces draws a dungeon cell;
// piece 8 is the party marker.
func dungeonMapPiece(value int) int {
	switch value {
	case game.DungeonCellDoor:
		return 0
	case game.DungeonCellSecretDoor:
		return 1
	case game.DungeonCellWall:
		return 2
	case game.DungeonCellLadderUp | game.DungeonCellLadderDown:
		return 3
	case game.DungeonCellLadderUp:
		return 4
	case game.DungeonCellLadderDown:
		return 5
	case game.DungeonCellOpen:
		return 6
	}
	return 7
}

type titleText struct {
	x, y int
	text string
}

type foldBlock struct {
	texts  []string
	rows   []int
	counts []int
}

type rect struct{ x, y, w, h int }

// Screen draws the game and answers its IO calls.
type Screen struct {
	// mu is held by the game goroutine whenever it runs, and let go while it
	// waits; the frame loop takes it to animate and read the picture.
	mu sync.Mutex

	c    *Canvas
	cell int
	gfx  *Graphics
	data string // the import, for loading other tile sets

	keyboard *Keyboard
	sounds   *SoundPlayer
	music    *MusicPlayer
	world    *game.World

	view          *game.Viewport
	viewDirty     bool
	viewCovered   bool // a picture or page covers the viewport
	frameShown    bool // the in-game border is up (not a title screen)
	titleTexts    []titleText
	lastFrame     time.Duration
	overlayShown  bool
	secretPiece   *image.RGBA
	chromeColours *[2]colour
	dungeon       *dungeonRenderer
	pulse         *rect

	cursorX, cursorY int
	textRows         [][]byte
	prevTurn         *foldBlock
	thisTurn         foldBlock
	foldedThisTurn   int
	brokenThisTurn   bool
	linesSincePrompt int

	statsCache     [4]string
	selectedMember int
	highlighted    map[int]bool
	panelLine      string
	card           *game.Card

	// InputMode is "controller" (the default: d-pad and pop-up menus) or
	// "keyboard" (the Apple II letter commands).
	InputMode string
	// TileSetName is the set in use.
	TileSetName string
	// Scanlines lays CRT lines over the whole picture (drawn by the app at window resolution).
	Scanlines bool
	// OnModeChange is called when the input mode changes; OnSettingsChange
	// when anything in the Settings menu does.
	OnModeChange     func()
	OnSettingsChange func()
	// OnTileSet is called when the tile set changes (the app redraws its window title).
	OnTileSet func()

	underPaused *image.RGBA
	overlay     func(t time.Duration)
	pauseShown  bool
	menu        *MenuWindow
	underMenu   *image.RGBA
	macro       []string
	greenCache  map[[2]int]*image.RGBA
	start       time.Time
}

// NewScreen is a screen of cell-pixel cells over the world, drawing with gfx.
func NewScreen(cell int, gfx *Graphics, data string, k *Keyboard, sounds *SoundPlayer, music *MusicPlayer, w *game.World) *Screen {
	s := &Screen{
		c: NewCanvas(Columns*cell, Rows*cell), cell: cell, gfx: gfx, data: data,
		keyboard: k, sounds: sounds, music: music, world: w,
		viewDirty: true, cursorX: textLeft + 1, cursorY: Rows - 1,
		selectedMember: -1, highlighted: map[int]bool{}, panelLine: lineHP,
		InputMode: "controller", TileSetName: gfx.Name, greenCache: map[[2]int]*image.RGBA{}, start: time.Now(),
	}
	for range textBottom - textTop {
		s.textRows = append(s.textRows, []byte(strings.Repeat(" ", textRight-textLeft)))
	}
	s.dungeon = newDungeonRenderer(gfx)
	// A pause holds the music and should not eat into a combat turn's timer. A
	// window that loses focus pauses at once and asks for the Pause menu as
	// well: whatever waits for a key reads it and opens the menu.
	k.onPause = func(reason string) {
		music.Pause()
		if reason == holdFocus && !s.pauseShownNow() {
			k.Push(game.KeyPause, "")
		}
	}
	k.onResume = func(paused time.Duration) {
		music.Resume()
		go func() {
			s.mu.Lock()
			defer s.mu.Unlock()
			if c := s.world.Combat; c != nil && c.MarkedFrozenAt == 0 {
				c.MarkedAt += float64(paused) / float64(time.Millisecond)
			}
		}()
	}
	return s
}

// pauseShownNow reads pauseShown from outside the game goroutine.
func (s *Screen) pauseShownNow() bool {
	if s.mu.TryLock() {
		defer s.mu.Unlock()
		return s.pauseShown
	}
	return false
}

// UseController switches to controller mode, as a gamepad press does.
func (s *Screen) UseController() {
	if s.InputMode == "controller" {
		return
	}
	s.InputMode = "controller"
	if s.OnModeChange != nil {
		s.OnModeChange()
	}
}

// partyStyle is how the party on foot is drawn: "grid" the 2x2 of
// members, "line" the leader with the others following, "symbol" the
// Apple II's single figure.
func (s *Screen) partyStyle() string {
	switch s.TileSetName {
	case "Nintendo":
		return "line"
	case "Standard":
		if s.world.InTownOrCastle() {
			return "line"
		}
		return "grid"
	}
	return "symbol"
}

// burstTiles: Standard alone shows a red burst on a hit instead of the HIT tile.
func (s *Screen) burstTiles() bool { return s.TileSetName == "Standard" }

// ---------------------------------------------------------------------------
// Frames and title screen

func (s *Screen) piece(n, x, y int) {
	s.gfx.drawUIPiece(s.c, n-1, 0, x*s.cell, y*s.cell, s.cell)
}

func (s *Screen) cells(x, y, w, h int) image.Rectangle {
	return image.Rect(x*s.cell, y*s.cell, (x+w)*s.cell, (y+h)*s.cell)
}

func (s *Screen) black(x, y, w, h int) { s.c.Fill(s.cells(x, y, w, h), rgb(0, 0, 0)) }

// ShowGameFrame draws the in-game border. (DrawFrame(1))
func (s *Screen) ShowGameFrame() {
	s.viewCovered = false
	s.frameShown = true
	s.black(0, 0, Columns, Rows)
	for x := 1; x < 23; x++ {
		s.piece(pieceHorizontal, x, 0)
		s.piece(pieceHorizontal, x, 23)
	}
	for y := 1; y < 23; y++ {
		s.piece(pieceVertical, 0, y)
		s.piece(pieceVertical, 23, y)
	}
	s.piece(pieceTopLeft, 0, 0)
	s.piece(pieceTopRight, 39, 0)
	s.piece(pieceBottomLeft, 0, 23)
	s.piece(pieceTopTee, 23, 0)
	s.piece(pieceBottomRight, 23, 23)
	s.card = nil
	s.drawPanelFrame()
	for _, row := range s.textRows {
		for i := range row {
			row[i] = ' '
		}
	}
	s.prevTurn = nil
	s.thisTurn = foldBlock{}
	s.foldedThisTurn = 0
	s.redrawTextArea()
	s.ShowWind()
	s.ShowMoons()
	s.statsCache = [4]string{}
	s.UpdateStats(true)
	s.viewDirty = true
}

// ShowTitle draws the plain border with the Exodus picture. (DrawFrame(3), DrawExodusPict)
func (s *Screen) ShowTitle() {
	s.viewCovered = true
	s.frameShown = false
	s.titleTexts = nil
	s.view = nil
	s.black(0, 0, Columns, Rows)
	for x := 1; x < 39; x++ {
		s.piece(pieceHorizontal, x, 0)
		s.piece(pieceHorizontal, x, 23)
	}
	for y := 1; y < 23; y++ {
		s.piece(pieceVertical, 0, y)
		s.piece(pieceVertical, 39, y)
	}
	s.piece(pieceTopLeft, 0, 0)
	s.piece(pieceTopRight, 39, 0)
	s.piece(pieceBottomLeft, 0, 23)
	s.piece(pieceBottomRight, 39, 23)
	if exodus := s.gfx.scenes["Exodus"]; exodus != nil {
		c := float64(s.cell)
		s.c.Blit(exodus, exodus.Bounds(), frect(5.25*c, 1.25*c, 29.5*c, 8.375*c), false)
	}
}

// ClearBottom blacks out the lower title screen.
func (s *Screen) ClearBottom() {
	s.black(1, 10, 38, 13)
	kept := s.titleTexts[:0]
	for _, t := range s.titleTexts {
		if t.y < 10 {
			kept = append(kept, t)
		}
	}
	s.titleTexts = kept
}

// setGraphics switches to another tile set and redraws everything.
func (s *Screen) setGraphics(g *Graphics) {
	s.gfx = g
	s.greenCache = map[[2]int]*image.RGBA{}
	s.secretPiece = nil
	s.chromeColours = nil
	s.dungeon = newDungeonRenderer(g)
	s.redrawAll()
}

// drawPanelFrame draws the frame round the party panel, with the members' numbers.
func (s *Screen) drawPanelFrame() {
	for x := 24; x < 39; x++ {
		s.piece(pieceHorizontal, x, 0)
		for _, y := range boxSeparators {
			s.piece(pieceHorizontal, x, y)
		}
	}
	for y := 1; y < boxesBottom; y++ {
		s.piece(pieceVertical, 23, y)
		s.piece(pieceVertical, 39, y)
	}
	for _, y := range boxSeparators {
		s.piece(pieceLeftTee, 23, y)
	}
	for _, y := range boxSeparators[:3] {
		s.piece(pieceRightTee, 39, y)
	}
	s.piece(pieceBottomRight, 39, boxesBottom)
	for i := range 4 {
		s.piece(pieceCapLeft, 30, i*boxPitch)
		s.piece(pieceCapRight, 32, i*boxPitch)
		s.drawText(fmt.Sprint(i+1), 31, i*boxPitch, "")
	}
}

// showCard shows a card over the party panel while a list's item is
// highlighted: the four boxes made one, its title on the top border. nil
// puts the members back.
func (s *Screen) showCard(card *game.Card) {
	if !s.frameShown || (card == nil && s.card == nil) {
		return
	}
	s.card = card
	if card == nil {
		s.drawPanelFrame()
		s.UpdateStats(true)
		return
	}
	for _, y := range boxSeparators[:3] {
		s.black(24, y, game.CardWidth, 1)
		s.piece(pieceVertical, 23, y)
		s.piece(pieceVertical, 39, y)
	}
	for x := 24; x < 39; x++ {
		s.piece(pieceHorizontal, x, 0)
	}
	title := clipStr(card.Title, game.CardWidth)
	tx := 24 + (game.CardWidth-len(title))/2
	if len(title) <= game.CardWidth-2 {
		s.piece(pieceCapLeft, tx-1, 0)
		s.piece(pieceCapRight, tx+len(title), 0)
	}
	s.black(tx, 0, len(title), 1)
	s.drawText(title, tx, 0, "")
	s.black(24, 1, game.CardWidth, game.CardRows)
	for i, line := range card.Lines {
		if i >= game.CardRows {
			break
		}
		s.drawText(clipStr(line.Text, game.CardWidth), 24, 1+i, cardTones[line.Tone])
	}
}

// clipStr is s cut to n bytes.
func clipStr(s string, n int) string {
	if len(s) > n {
		return s[:max(0, n)]
	}
	return s
}

func padEnd(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}

func padStart(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return strings.Repeat(" ", n-len(s)) + s
}

func (s *Screen) setPanelLine(line string) {
	if line == s.panelLine {
		return
	}
	s.panelLine = line
	s.UpdateStats(true)
}

// redrawAll redraws the whole screen from what the Screen knows.
func (s *Screen) redrawAll() {
	if s.frameShown {
		rows := make([][]byte, len(s.textRows))
		for i, r := range s.textRows {
			rows[i] = append([]byte(nil), r...)
		}
		cx, cy := s.cursorX, s.cursorY
		s.ShowGameFrame()
		s.textRows = rows
		s.cursorX, s.cursorY = cx, cy
		s.redrawTextArea()
		for m := range s.highlighted {
			s.invert(24, m*boxPitch+1, 15, boxRows)
		}
	} else {
		texts := s.titleTexts
		s.ShowTitle()
		for _, t := range texts {
			s.drawText(t.text, t.x, t.y, "")
		}
		s.titleTexts = texts
	}
	s.showMenuNow()
}

// ---------------------------------------------------------------------------
// Text

// drawText draws a string with the bitmap font at cell (x, y), tinted by colour if given.
func (s *Screen) drawText(text string, x, y int, colour string) {
	if s.overlayShown && y >= mapOverlayTop && y < mapOverlayTop+mapOverlaySize && x+len(text) > mapOverlayLeft {
		keep := max(0, mapOverlayLeft-x)
		if keep == 0 {
			return
		}
		text = text[:keep]
	}
	cell := s.cell
	for i := range len(text) {
		s.black(x+i, y, 1, 1)
		s.gfx.drawGlyph(s.c, text[i], (x+i)*cell, y*cell, cell)
	}
	if colour != "" {
		s.c.Multiply(s.cells(x, y, len(text), 1), namedColour(colour))
	}
}

// TextAt draws text at a cell position. (UPrint)
func (s *Screen) TextAt(x, y int, text string) {
	s.drawText(text, x, y, "")
	if !s.frameShown {
		s.titleTexts = append(s.titleTexts, titleText{x, y, text})
	}
}

// CentreText draws text centred on the screen. (CenterMessage)
func (s *Screen) CentreText(y int, text string) { s.TextAt(20-len(text)/2, y, text) }

func (s *Screen) redrawTextArea() {
	for r, row := range s.textRows {
		s.drawText(string(row), textLeft, textTop+r, "")
	}
}

// scrollText scrolls the message area up a line. (UTextScroll)
func (s *Screen) scrollText() {
	s.textRows = append(s.textRows[1:], []byte(strings.Repeat(" ", textRight-textLeft)))
	if s.prevTurn != nil {
		for i := range s.prevTurn.rows {
			s.prevTurn.rows[i]--
		}
		if s.prevTurn.rows[0] < 0 {
			s.prevTurn = nil
		}
	}
	for i := range s.thisTurn.rows {
		s.thisTurn.rows[i]--
	}
	for len(s.thisTurn.rows) > 0 && s.thisTurn.rows[0] < 0 {
		s.thisTurn.rows = s.thisTurn.rows[1:]
		s.thisTurn.texts = s.thisTurn.texts[1:]
	}
	s.redrawTextArea()
	s.piece(pieceBottomRight, 23, 23)
}

// Print prints to the message area. (UPrintWin)
func (s *Screen) Print(text string) {
	for i := range len(text) {
		ch := text[i]
		if ch == '\n' {
			if !s.foldRepeat() {
				s.newline()
			}
			continue
		}
		if ch < ' ' || s.cursorX >= textRight {
			continue // the original simply ran off the edge
		}
		s.textRows[s.cursorY-textTop][s.cursorX-textLeft] = ch
		s.drawText(string(ch), s.cursorX, s.cursorY, "")
		s.cursorX++
	}
}

func (s *Screen) newline() {
	s.cursorX = textLeft
	s.cursorY++
	if s.cursorY >= textBottom {
		s.cursorY = textBottom - 1
		s.scrollText()
	}
}

// setRow writes a whole message-area row and draws it.
func (s *Screen) setRow(row int, text string) {
	padded := padEnd(clipStr(text, textRight-textLeft), textRight-textLeft)
	s.textRows[row] = []byte(padded)
	s.drawText(padded, textLeft, textTop+row, "")
}

func countLabel(text string, count int) string {
	if count > 1 && strings.TrimSpace(text) != "" {
		return fmt.Sprintf("%s (x%d)", text, count)
	}
	return text
}

// foldRepeat: a turn that prints exactly what the previous turn printed
// folds into it with counts ("North (x5)"). At the end of a line, if this
// turn has matched the previous block so far and this line matches its
// next line too, it is folded in and true returned. Otherwise the line is
// the turn's own; lines folded earlier this turn are unfolded first.
func (s *Screen) foldRepeat() bool {
	row := s.cursorY - textTop
	raw := strings.TrimRight(string(s.textRows[row]), " ")
	k := s.linesSincePrompt
	s.linesSincePrompt++
	prev := s.prevTurn
	contiguous := prev != nil && prev.rows[len(prev.rows)-1] == row-1
	if !s.brokenThisTurn && contiguous && k == s.foldedThisTurn && k < len(prev.texts) && prev.texts[k] == raw {
		prev.counts[k]++
		s.setRow(row, "")
		s.setRow(prev.rows[k], countLabel(prev.texts[k], prev.counts[k]))
		s.foldedThisTurn++
		s.cursorX = textLeft
		return true
	}
	if s.foldedThisTurn > 0 {
		s.unfold(&raw)
	}
	s.brokenThisTurn = true
	s.thisTurn.texts = append(s.thisTurn.texts, raw)
	s.thisTurn.rows = append(s.thisTurn.rows, s.cursorY-textTop)
	return false
}

// unfold takes back the lines folded this turn and prints them again ahead of pending.
func (s *Screen) unfold(pending *string) {
	prev := s.prevTurn
	lines := append([]string(nil), prev.texts[:s.foldedThisTurn]...)
	for i, text := range lines {
		prev.counts[i]--
		s.setRow(prev.rows[i], countLabel(text, prev.counts[i]))
	}
	s.foldedThisTurn = 0
	for _, text := range lines {
		s.setRow(s.cursorY-textTop, text)
		s.thisTurn.texts = append(s.thisTurn.texts, text)
		s.thisTurn.rows = append(s.thisTurn.rows, s.cursorY-textTop)
		s.newline()
	}
	if pending != nil {
		s.setRow(s.cursorY-textTop, *pending)
	}
}

// PrintMessage prints entry n of the Messages table. (UPrintMessage)
func (s *Screen) PrintMessage(n int) {
	if m := s.world.Resources.Strings["Messages"]; n >= 1 && n <= len(m) {
		s.Print(m[n-1])
		return
	}
	s.Print(fmt.Sprintf("[msg %d]", n))
}

// Prompt draws the command prompt. (DrawPrompt)
func (s *Screen) Prompt() {
	s.endTurn()
	s.piece(pieceBottomTee, 23, 23)
	s.piece(pieceCapLeft, 24, 23)
	row := s.textRows[textBottom-1-textTop]
	for i := range row {
		row[i] = ' '
	}
	for x := textLeft + 1; x < textRight; x++ {
		s.drawText(" ", x, textBottom-1, "")
	}
	s.cursorX = textLeft + 1
	s.cursorY = textBottom - 1
}

// endTurn: the block just printed becomes the one the next turn may fold into.
func (s *Screen) endTurn() {
	if prev := s.prevTurn; s.foldedThisTurn > 0 && prev != nil && s.foldedThisTurn < len(prev.texts) {
		s.unfold(nil)
	}
	if len(s.thisTurn.texts) > 0 {
		counts := make([]int, len(s.thisTurn.texts))
		for i := range counts {
			counts[i] = 1
		}
		s.prevTurn = &foldBlock{s.thisTurn.texts, s.thisTurn.rows, counts}
	}
	s.thisTurn = foldBlock{}
	s.foldedThisTurn = 0
	s.brokenThisTurn = false
	s.linesSincePrompt = 0
}

// printWrapped prints a sentence to the message area, wrapped to its columns.
func (s *Screen) printWrapped(text string) {
	s.Print("\n")
	for _, line := range wrapText(text, textRight-textLeft, textBottom-textTop-2) {
		s.Print(line + "\n")
	}
}

// ---------------------------------------------------------------------------
// Keys

// readKey reads a key, the scripted ones first, translating the
// controller stand-ins in controller mode. With timeoutMs > 0 it returns
// "", false when the time runs out.
func (s *Screen) readKey(timeoutMs int) (string, bool) {
	if len(s.macro) > 0 {
		k := s.macro[0]
		s.macro = s.macro[1:]
		return k, true
	}
	s.mu.Unlock()
	key, ok := s.keyboard.Next(timeoutMs)
	s.mu.Lock()
	if !ok {
		return "", false
	}
	if s.InputMode == "controller" {
		key = controllerKeyFor(key)
	}
	return key, true
}

// rawKey reads a key from the keyboard alone, untranslated (the blinking cursor's waits).
func (s *Screen) rawKey(timeoutMs int) (string, bool) {
	s.mu.Unlock()
	defer s.mu.Lock()
	return s.keyboard.Next(timeoutMs)
}

// WaitKey waits for a key. (WaitKeyMouse)
func (s *Screen) WaitKey() string {
	k, _ := s.readKey(0)
	return k
}

// WaitKeyOrTimeout waits for a key, or "" after ms.
func (s *Screen) WaitKeyOrTimeout(ms int) string {
	k, _ := s.readKey(max(1, ms))
	return k
}

// FlushKeys drops queued presses and scripted keys.
func (s *Screen) FlushKeys() {
	s.keyboard.Flush()
	s.macro = nil
}

// QueueKeys replaces the scripted keys.
func (s *Screen) QueueKeys(keys []string) { s.macro = append([]string(nil), keys...) }

// promptMode: scripted keys are Apple II keys, read keyboard-style whatever the mode.
func (s *Screen) promptMode() string {
	if len(s.macro) > 0 {
		return "keyboard"
	}
	return s.InputMode
}

// Sound plays an effect, if effects are on.
func (s *Screen) Sound(name string) {
	if s.world.SoundEnabled {
		s.sounds.Play(name)
	}
}

// Music selects the background track.
func (s *Screen) Music(track int) { s.music.Play(track) }

// Pause waits ms while animations run. (IdleUntil)
func (s *Screen) Pause(ms int) {
	if s.keyboard.headless {
		s.keyboard.scriptedPause()
		return
	}
	s.mu.Unlock()
	time.Sleep(time.Duration(ms) * time.Millisecond)
	s.mu.Lock()
}

// ---------------------------------------------------------------------------
// Stats boxes, wind, moons

// memberColour tells a member's state: green poisoned, light grey dead,
// dark grey ashes, blue when a raise is due, "" (white) otherwise.
func memberColour(p *game.PlayerRecord, due bool) string {
	switch p.Status() {
	case 'P':
		return "#40ff40"
	case 'D':
		return "#b0b0b0"
	case 'A':
		return "#606060"
	}
	if due {
		return "#60a0ff"
	}
	return ""
}

// UpdateStats redraws the character boxes that changed (ShowChars): the
// name, coloured by state with the Standard set (every other set prints
// the Apple II's status letter instead), and hit points and mana, or what
// the command menu's bar is about.
func (s *Screen) UpdateStats(force bool) {
	if s.card != nil {
		s.statsCache = [4]string{}
		s.ShowMoons()
		return
	}
	w := s.world
	for m := range 4 {
		slot := w.Party.MemberSlot(m)
		var p *game.PlayerRecord
		if slot >= 0 {
			p = w.Roster.Get(slot)
		}
		due := p != nil && game.LevelUpDue(p)
		selected := m == s.selectedMember
		key := ""
		if p != nil {
			b := p.Bytes
			key = fmt.Sprintf("%s|%c|%v|%d|%d|%d|%v|%s|%d|%d|%d|%d", p.Name(), p.Status(), due, p.HitPoints(), p.MaxHitPoints(),
				p.Mana(), selected, s.panelLine, b[30], b[31], b[48], b[40])
		}
		if !force && key == s.statsCache[m] {
			continue
		}
		s.statsCache[m] = key
		top := m*boxPitch + 1
		s.black(24, top, 15, boxRows)
		for _, row := range []int{top, top + 1} {
			if selected {
				s.black(23, row, 1, 1)
				s.drawText(">", 23, row, "")
				s.black(39, row, 1, 1)
				s.drawText("<", 39, row, "")
			} else {
				s.piece(pieceVertical, 23, row)
				s.piece(pieceVertical, 39, row)
			}
		}
		if p != nil {
			classic := s.TileSetName != "Standard"
			nameColour := ""
			if !classic {
				nameColour = memberColour(p, due)
			}
			s.drawText(p.Name(), 24, top, nameColour)
			if classic {
				s.drawText(string(game.StatusLetter(p)), 38, top, "")
			}
			gone := p.Status() == 'D' || p.Status() == 'A'
			goneColour := ""
			if gone && !classic {
				goneColour = nameColour
			}
			switch s.panelLine {
			case lineXP:
				xp := int(p.Bytes[30])*100 + int(p.Bytes[31])
				next := fmt.Sprintf("/%d", (int(p.Bytes[30])+1)*100)
				if xp >= 9899 {
					next = " MAX"
				}
				colour := goneColour
				if !classic && !gone && due {
					colour = memberColour(p, due)
				}
				s.drawText(fmt.Sprintf("XP %d%s", xp, next), 24, top+1, colour)
			case lineWeapon, lineArmour:
				weapon := s.panelLine == lineWeapon
				index := p.Bytes[40]
				if weapon {
					index = p.Bytes[48]
				}
				s.drawText(game.GearName(w, weapon, int(index)), 24, top+1, goneColour)
			default:
				hp, mx := p.HitPoints(), max(1, p.MaxHitPoints())
				colour := goneColour
				if !classic && !gone {
					if float64(hp) < float64(mx)/10 {
						colour = "#ff4040"
					} else if float64(hp) < float64(mx)/4 {
						colour = "#ffe040"
					}
				}
				s.drawText(fmt.Sprintf("%d/%d", hp, p.MaxHitPoints()), 24, top+1, colour)
				if game.HasMagic(w, p) {
					mana := fmt.Sprintf("M:%d", p.Mana())
					s.drawText(mana, 39-len(mana), top+1, goneColour)
				}
			}
		}
		if s.highlighted[m] {
			s.invert(24, top, 15, boxRows)
		}
	}
	s.ShowMoons()
}

// FlashMember briefly highlights a member's box. (InverseChar)
func (s *Screen) FlashMember(member int) {
	if s.card != nil {
		return
	}
	top := member*boxPitch + 1
	s.invert(24, top, 15, boxRows)
	s.Pause(100)
	s.invert(24, top, 15, boxRows)
}

// HighlightMember keeps a member's box highlighted. (InverseChnum)
func (s *Screen) HighlightMember(member int, on bool) {
	if s.highlighted[member] == on {
		return
	}
	if on {
		s.highlighted[member] = true
	} else {
		delete(s.highlighted, member)
	}
	if s.card == nil {
		s.invert(24, member*boxPitch+1, 15, boxRows)
	}
}

// ShowWind draws the wind on the bottom border, or the heading in a dungeon. (ShowWind, DngInfo)
func (s *Screen) ShowWind() {
	w := s.world
	mm := w.Resources.Strings["MoreMessages"]
	at := func(i int) string {
		if i >= 0 && i < len(mm) {
			return mm[i]
		}
		return ""
	}
	if w.Party.Location() == game.LocationDungeon {
		s.piece(pieceCapLeft, 6, 23)
		s.piece(pieceCapRight, 17, 23)
		s.drawText(padEnd(at(48)+at(49+w.Dungeon.Heading), 10), 7, 23, "") // "HEAD-" then North, -East, ...
		return
	}
	if w.Party.Location() == game.LocationCombat || !w.WindEnabled {
		for x := 6; x <= 17; x++ {
			s.piece(pieceHorizontal, x, 23)
		}
		return
	}
	s.piece(pieceCapLeft, 6, 23)
	s.piece(pieceCapRight, 17, 23)
	s.drawText(padEnd(at(42+w.WindDirection), 10), 7, 23, "") // CALM WINDS, NORTH WIND ...
}

// ShowMoons draws the status bar over the map: gold, the moons (or the
// dungeon level) and food. (DrawMoonGateStuff, DngInfo)
func (s *Screen) ShowMoons() {
	if !s.frameShown {
		return
	}
	w := s.world
	for x := 1; x <= 22; x++ {
		s.piece(pieceHorizontal, x, 0)
	}
	s.drawText(fmt.Sprintf("G:%d", w.Party.Gold()), 1, 0, "")
	if w.Party.Location() == game.LocationDungeon {
		s.drawText(padEnd(fmt.Sprintf("Lvl:%d", w.Dungeon.Level+1), 6), 9, 0, "")
	} else {
		s.piece(pieceCapLeft, 9, 0)
		s.gfx.drawMoon(s.c, 0, w.MoonPhase[0], 10*s.cell, 0, s.cell)
		s.gfx.drawMoon(s.c, 1, w.MoonPhase[1], 11*s.cell, 0, s.cell)
		s.piece(pieceCapRight, 12, 0)
	}
	food := fmt.Sprintf("F:%d", w.Party.Food())
	s.drawText(food, 23-len(food), 0, "")
}

func (s *Screen) invert(x, y, w, h int) { s.c.Invert(s.cells(x, y, w, h)) }

var _ game.IO = (*Screen)(nil)
