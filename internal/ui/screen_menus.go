package ui

import (
	"strings"

	"github.com/retro-ware/u3-go/internal/game"
)

// The screen's menus: the pop-up windows of controller mode, Settings and
// Pause with their lists, the level dials and the tile sets.

// runMenu shows a menu and runs it until the player picks an item (its
// index) or cancels (-1). Letter keys still pick the matching option, so a
// keyboard works in controller mode too. On the title screens each key is
// answered with a sound; over the game frame its menus stay quiet. An item
// with a card shows it over the party panel while highlighted; onCursor
// hears of each item the bar comes to.
func (s *Screen) runMenu(title string, options []game.MenuOption, columns int, place *game.MenuPlacement, cursor int, onCursor func(*game.MenuOption)) int {
	var visible []game.MenuOption
	for _, o := range options {
		if !o.Hidden {
			visible = append(visible, o)
		}
	}
	labels := make([]string, len(visible))
	var hints, hintColours, colours []string
	var disabled []bool
	anyHint, anyHC, anyDis, anyCol, cards := false, false, false, false, false
	for i, o := range visible {
		labels[i] = o.Label
		hints, hintColours, colours = append(hints, o.Hint), append(hintColours, o.HintColour), append(colours, o.Colour)
		disabled = append(disabled, o.Disabled)
		anyHint, anyHC, anyDis, anyCol = anyHint || o.Hint != "", anyHC || o.HintColour != "", anyDis || o.Disabled, anyCol || o.Colour != ""
		cards = cards || o.Card != nil
	}
	m := layoutMenu(title, labels, columns)
	if anyHint {
		m.Hints = hints
	}
	if anyHC {
		m.HintColours = hintColours
	}
	if anyDis {
		m.Disabled = disabled
	}
	if anyCol {
		m.Colours = colours
	}
	if place != nil {
		s.placeMenu(m, place)
	} else if cursor > 0 && cursor < len(m.Items) {
		m.Cursor = cursor
		if m.Cursor >= m.VisibleRows {
			m.Top = m.Cursor - m.VisibleRows + 1
		}
	}
	s.openMenu(m)
	feedback := !s.frameShown
	cards = cards && s.frameShown
	watch := func() {
		var o *game.MenuOption
		if m.Cursor >= 0 && m.Cursor < len(visible) {
			o = &visible[m.Cursor]
		}
		if cards {
			var card *game.Card
			if o != nil {
				card = o.Card
			}
			s.showCard(card)
			s.showMenuNow() // over the card, where a wide window reaches the panel
		}
		if onCursor != nil {
			onCursor(o)
		}
	}
	watch()
	defer func() {
		s.closeMenu()
		if cards {
			s.showCard(nil)
		}
	}()
	for {
		key, ok := s.readKey(0)
		if !ok {
			continue
		}
		switch act, index := applyMenuKey(m, options, visible, key); act {
		case actMove:
			if feedback {
				s.Sound(menuSounds.move)
			}
			s.showMenuNow()
			watch()
		case actChoose:
			if feedback {
				s.Sound(menuSounds.choose)
			}
			return index
		case actBack:
			if feedback {
				s.Sound(menuSounds.back)
			}
			return -1
		case actRefused:
			s.Sound(game.SoundError1)
		}
	}
}

// placeMenu sits a window at the given row, centred on the whole screen
// and as wide as its labels need. On a title screen it replaces the text
// at those rows; over the game frame it covers what is there.
func (s *Screen) placeMenu(m *MenuWindow, place *game.MenuPlacement) {
	longest := len(m.Title)
	for _, it := range m.Items {
		longest = max(longest, len(it))
	}
	if m.Columns == 1 {
		m.Width = min(Columns-2, longest+4)
	}
	m.Y = place.Row
	m.X = (Columns - m.Width) / 2
	extra := 0
	if m.Hints != nil {
		extra = hintRows
	}
	m.VisibleRows = max(1, min(m.VisibleRows, Rows-1-place.Row-2-extra))
	if place.Cursor >= 0 && place.Cursor < len(m.Items) {
		m.Cursor = place.Cursor
		if m.Cursor >= m.VisibleRows {
			m.Top = m.Cursor - m.VisibleRows + 1
		}
	}
	if !s.frameShown {
		s.black(1, place.Row, Columns-2, m.VisibleRows+2+extra)
	}
}

// openMenu opens a window: remember what is under it (the hint line runs
// across the whole screen under it, so that too), then draw it.
func (s *Screen) openMenu(m *MenuWindow) {
	x, w, h := m.X, m.Width, m.VisibleRows+2
	if m.Hints != nil {
		x, w, h = 1, Columns-2, h+hintRows
	}
	s.underMenu = s.c.Copy(s.cells(x, m.Y, w, min(h, Rows-m.Y)))
	s.menu = m
	s.showMenuNow()
}

// closeMenu closes the window and restores what it covered.
func (s *Screen) closeMenu() {
	s.menu = nil
	s.c.Restore(s.underMenu)
	s.underMenu = nil
	if !s.viewCovered {
		s.viewDirty = true
	}
}

// showMenuNow redraws the open window; over the live viewport the next
// frame draws it, over the map it is drawn now.
func (s *Screen) showMenuNow() {
	if s.menu == nil {
		return
	}
	if s.viewCovered {
		drawMenu(s.c, s.gfx, s.cell, s.menu)
	} else {
		s.viewDirty = true
	}
}

// ShowPause is the Pause menu: the game stops where it stands and the
// settings are shown, until Resume. The hold stops the timers and music.
func (s *Screen) ShowPause() string {
	if s.pauseShown {
		return ""
	}
	s.pauseShown = true
	release := game.HoldCombatMark(s.world.Combat, nil)
	s.keyboard.Pause(holdMenu)
	s.hidePaused()
	defer func() {
		s.keyboard.Resume(holdMenu)
		s.pauseShown = false
		release()
	}()
	return s.settingsMenu("Paused")
}

// ShowSettings is the title screen's Settings menu.
func (s *Screen) ShowSettings() { s.settingsMenu("Settings") }

func (s *Screen) titlePlace(title string, cursor int) *game.MenuPlacement {
	if s.frameShown {
		return nil
	}
	return &game.MenuPlacement{Row: 13, Title: title, Cursor: cursor}
}

func (s *Screen) settingsChanged() {
	if s.OnSettingsChange != nil {
		s.OnSettingsChange()
	}
}

// settingsMenu is Settings at the title or Pause in play (which leads with
// Resume, offers the cheats and can end the game). It returns "Q" when
// Quit and save was chosen.
func (s *Screen) settingsMenu(title string) string {
	w := s.world
	paused := title == "Paused"
	cursor := 0
	for {
		var options []game.MenuOption
		if paused {
			options = append(options, game.MenuOption{Key: "E", Label: "Resume"})
		}
		options = append(options,
			game.MenuOption{Key: "A", Label: "Auto combat: " + onOff(w.AutoCombat)},
			game.MenuOption{Key: "V", Label: "Visuals"},
			game.MenuOption{Key: "P", Label: "Gameplay"},
			game.MenuOption{Key: "U", Label: "Audio"},
			game.MenuOption{Key: "G", Label: "Tips"},
			game.MenuOption{Key: "H", Label: "Help"})
		if paused {
			options = append(options, game.MenuOption{Key: "C", Label: "Cheats"})
			if w.OnSurface() && w.Combat == nil {
				options = append(options, game.MenuOption{Key: "Q", Label: "Quit and save"})
			}
		}
		options = append(options, game.MenuOption{Key: "B", Label: "Back"})
		picked := s.runMenu(title, options, 1, s.titlePlace(title, cursor), cursor, nil)
		if picked < 0 {
			return ""
		}
		cursor = picked
		switch options[picked].Key {
		case "Q":
			return "Q"
		case "V":
			s.visualsMenu()
		case "A":
			w.AutoCombat = !w.AutoCombat
			if w.OnAutoCombatChange != nil {
				w.OnAutoCombatChange()
			}
		case "P":
			s.gameplayMenu()
		case "U":
			s.audioMenu()
		case "G":
			s.showTips()
		case "H":
			s.showHelp()
		case "C":
			if done := s.cheatMenu(); done != "" {
				s.UpdateStats(true)
				s.RedrawMap()
				s.printWrapped(done)
			}
		default:
			return "" // Resume, Back
		}
		s.settingsChanged()
	}
}

// settingsList is a list of settings, each line as it now stands, Back
// last; the one chosen is acted on in place and the list shown again.
func (s *Screen) settingsList(title string, lines func() []game.MenuOption, act func(key string, place *game.MenuPlacement)) {
	cursor := 0
	for {
		options := append(lines(), game.MenuOption{Key: "B", Label: "Back"})
		place := s.titlePlace(title, cursor)
		picked := s.runMenu(title, options, 1, place, cursor, nil)
		if picked < 0 || options[picked].Key == "B" {
			return
		}
		cursor = picked
		act(options[picked].Key, place)
		s.settingsChanged()
	}
}

func (s *Screen) visualsMenu() {
	s.settingsList("Visuals", func() []game.MenuOption {
		return []game.MenuOption{{Key: "T", Label: "Tiles: " + s.TileSetName}, {Key: "L", Label: "Scanlines: " + onOff(s.Scanlines)}}
	}, func(key string, _ *game.MenuPlacement) {
		if key == "T" {
			s.chooseTiles()
		} else if key == "L" {
			s.Scanlines = !s.Scanlines
		}
	})
}

func (s *Screen) audioMenu() {
	w := s.world
	s.settingsList("Audio", func() []game.MenuOption {
		fx := "Off"
		if w.SoundEnabled {
			fx = s.sounds.Set
		}
		return []game.MenuOption{
			{Key: "S", Label: "Sound FX: " + fx},
			{Key: "F", Label: "FX level: " + levelLabel(s.sounds.Level)},
			{Key: "M", Label: "Music: " + onOff(s.music.Enabled())},
			{Key: "U", Label: "Music level: " + levelLabel(s.music.Level())},
		}
	}, func(key string, place *game.MenuPlacement) {
		switch key {
		case "S": // Standard, Lairware, Off, round again
			if !w.SoundEnabled {
				w.SoundEnabled, s.sounds.Set = true, "Standard"
			} else if s.sounds.Set == "Standard" {
				s.sounds.Set = "Lairware"
			} else {
				w.SoundEnabled = false
			}
		case "M":
			s.music.SetEnabled(!s.music.Enabled())
		case "F":
			s.dialLevel("FX level", s.sounds.Level, place, func(n int) {
				s.sounds.Level = n
				s.Sound(menuSounds.move)
			})
		case "U":
			s.music.Hear(true)
			s.dialLevel("Music level", s.music.Level(), place, s.music.SetLevel)
			s.music.Hear(false)
		}
	})
}

func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// gameplayMenu: the input mode (offered to a player pressing real keys)
// and the rules the port lets the player soften.
func (s *Screen) gameplayMenu() {
	w := s.world
	s.settingsList("Gameplay", func() []game.MenuOption {
		var out []game.MenuOption
		if s.keyboard.LastSource() == sourceKeyboard {
			mode := "Keyboard"
			if s.InputMode == "controller" {
				mode = "Controller"
			}
			out = append(out, game.MenuOption{Key: "I", Label: "Input: " + mode})
		}
		return append(out,
			game.MenuOption{Key: "P", Label: "Poison kills: " + onOff(w.PoisonKills)},
			game.MenuOption{Key: "V", Label: "Starving: " + titleCase(string(w.Starvation))},
			game.MenuOption{Key: "X", Label: "Balanced XP: " + onOff(w.BalancedXP)},
			game.MenuOption{Key: "R", Label: "Timer: " + titleCase(string(w.Timer))})
	}, func(key string, _ *game.MenuPlacement) {
		switch key {
		case "I":
			if s.InputMode == "controller" {
				s.InputMode = "keyboard"
			} else {
				s.InputMode = "controller"
			}
			if s.OnModeChange != nil {
				s.OnModeChange()
			}
		case "P":
			w.PoisonKills = !w.PoisonKills
		case "V":
			w.Starvation = game.StarvationModes[(indexOf(game.StarvationModes, w.Starvation)+1)%len(game.StarvationModes)]
		case "X":
			w.BalancedXP = !w.BalancedXP
		case "R":
			w.Timer = game.TimerModes[(indexOf(game.TimerModes, w.Timer)+1)%len(game.TimerModes)]
		}
	})
}

func indexOf[T comparable](list []T, v T) int {
	for i, x := range list {
		if x == v {
			return i
		}
	}
	return -1
}

// dialLevel turns a level: up or right louder, down or left quieter, each
// step applied at once so it is heard. A or Enter keeps it; B or Escape
// puts back the level it started at.
func (s *Screen) dialLevel(title string, start int, place *game.MenuPlacement, apply func(int)) {
	value := start
	label := func() string { return padStart(levelLabel(value), 4) + "  up/down" }
	m := layoutMenu(title, []string{label()}, 1)
	if place != nil {
		p := *place
		p.Title = title
		s.placeMenu(m, &p)
	}
	m.Cursor = -1
	s.openMenu(m)
	defer s.closeMenu()
	for {
		key, ok := s.readKey(0)
		if !ok {
			continue
		}
		switch key {
		case game.KeyA, game.KeyEnter:
			return
		case game.KeyB, game.KeyEscape:
			if value != start {
				apply(start)
			}
			return
		}
		next := value
		if key == game.KeyUp || key == game.KeyRight {
			next = min(levels, value+1)
		} else if key == game.KeyDown || key == game.KeyLeft {
			next = max(0, value-1)
		}
		if next == value {
			continue
		}
		value = next
		apply(value)
		m.Items[0] = label()
		s.showMenuNow()
	}
}

// chooseTiles is the Tiles sub-menu: pick a set and switch to it.
func (s *Screen) chooseTiles() {
	var options []game.MenuOption
	for _, name := range TileSets {
		options = append(options, game.MenuOption{Label: name})
	}
	current := max(0, indexOf(TileSets, s.TileSetName))
	picked := s.runMenu("Tiles", options, 1, s.titlePlace("Tiles", current), current, nil)
	if picked >= 0 {
		s.loadTiles(TileSets[picked])
	}
}

// loadTiles switches to the named set, keeping the current one if it fails to load.
func (s *Screen) loadTiles(name string) {
	g, err := LoadGraphics(name, s.data)
	if err != nil {
		return
	}
	s.TileSetName = name
	s.setGraphics(g)
	if s.OnTileSet != nil {
		s.OnTileSet()
	}
}

// NextTileSet steps to the next tile set, round and round (Alt-Shift-6).
// It runs on the window's goroutine, so it takes the lock itself.
func (s *Screen) NextTileSet() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loadTiles(TileSets[(indexOf(TileSets, s.TileSetName)+1)%len(TileSets)])
	s.settingsChanged()
}
