package ui

import (
	"fmt"
	"strings"

	"github.com/retro-ware/u3-go/internal/game"
)

// The screen's prompts: commands, "Who?", directions, options and text.
// In keyboard mode each reads a key as the Apple II did; in controller
// mode it shows a menu.

// WaitCommand reads the next top-level command, "" when the time runs out.
// In controller mode B, X and Y are shortcuts and A opens the command menu.
func (s *Screen) WaitCommand(scope game.CommandScope, timeoutMs int) string {
	s.setPanelLine(lineHP)
	left := timeoutMs
	for {
		asked := s.keyboard.ActiveTime()
		key, ok := s.readKey(left)
		if !ok {
			if timeoutMs > 0 {
				return ""
			}
			continue
		}
		if s.promptMode() == "keyboard" || isDirection(key) {
			return key
		}
		sc := buttonShortcuts[scope]
		switch key {
		case game.KeyB:
			return sc[0]
		case game.KeyX:
			return sc[1]
		case game.KeyY:
			return sc[2]
		case game.KeyA:
		default:
			return key
		}
		if timeoutMs > 0 {
			left = max(1, left-(s.keyboard.ActiveTime()-asked))
		}
		// With the menu open the turn cannot pass by itself, so the outline's fade stops.
		release := game.HoldCombatMark(s.world.Combat, nil)
		options := game.CommandMenu(s.world, scope, CommandMenus[scope])
		picked := s.runMenu("Command", options, 1, nil, 0, func(o *game.MenuOption) {
			line := lineHP
			if o != nil && panelLines[o.Key] != "" {
				line = panelLines[o.Key]
			}
			s.setPanelLine(line)
		})
		if picked >= 0 {
			return options[picked].Key
		}
		s.setPanelLine(lineHP)
		release()
	}
}

// ChooseMember answers "Who?" in the stats boxes: Up and Down move a pair
// of arrows among the members offered, Enter or A takes one, Escape or B
// cancels (0), and a digit still answers at once. The answer is echoed.
func (s *Screen) ChooseMember(only []int) int {
	var offered []int
	for m := range 4 {
		if s.world.Party.MemberSlot(m) >= 0 && (only == nil || indexOf(only, m) >= 0) {
			offered = append(offered, m)
		}
	}
	if !s.frameShown {
		key := strings.ToUpper(s.WaitKey())
		if len(key) == 1 && key[0] >= ' ' {
			s.Print(key)
		} else {
			s.Print(" ")
		}
		s.Print("\n")
		if key == "" {
			return 0
		}
		return int(key[0]) - '0'
	}
	label := "Who? Esc Cancel"
	if s.promptMode() == "controller" {
		label = "Who? B: Cancel"
	}
	s.borderPrompt(label)
	at, n := 0, 0
loop:
	for {
		s.selectedMember = -1
		if at < len(offered) {
			s.selectedMember = offered[at]
		}
		s.UpdateStats(false)
		key := s.WaitKey()
		switch {
		case key == game.KeyEscape || key == game.KeyB:
			break loop
		case len(key) == 1 && key >= "1" && key <= "4":
			n = int(key[0] - '0')
			break loop
		case len(offered) == 0:
		case key == game.KeyUp:
			at = (at + len(offered) - 1) % len(offered)
		case key == game.KeyDown:
			at = (at + 1) % len(offered)
		case key == game.KeyEnter || key == game.KeyA:
			n = offered[at] + 1
			break loop
		}
	}
	s.selectedMember = -1
	s.UpdateStats(false)
	s.restoreBorder()
	if n > 0 {
		s.Print(fmt.Sprintf("%d\n", n))
	} else {
		s.Print(" \n")
	}
	return n
}

// borderPrompt centres a one-line prompt on the map's bottom border.
func (s *Screen) borderPrompt(label string) {
	for x := 1; x <= 22; x++ {
		s.piece(pieceHorizontal, x, 23)
	}
	text := clipStr(label, 22)
	x := 1 + (22-len(text))/2
	s.black(x, 23, len(text), 1)
	s.drawText(text, x, 23, "")
}

func (s *Screen) restoreBorder() {
	for x := 1; x <= 22; x++ {
		s.piece(pieceHorizontal, x, 23)
	}
	s.ShowWind()
}

// ChooseDirection answers "Direction?" on the map's bottom border.
func (s *Screen) ChooseDirection(allowNone, allowDiagonal bool) string {
	accepted := append([]string(nil), directionKeys...)
	if allowDiagonal {
		accepted = append(accepted, "1", "2", "3", "4", "6", "7", "8", "9")
	} else {
		accepted = append(accepted, "2", "4", "6", "8")
	}
	label := "Direction? Esc Cancel"
	if s.promptMode() == "controller" {
		label = "Direction? B: Cancel"
	}
	s.borderPrompt(label)
	defer s.restoreBorder()
	for {
		key := s.WaitKey()
		switch {
		case indexOf(accepted, key) >= 0:
			return key
		case allowNone && (key == game.KeySpace || key == game.KeyA):
			return game.KeySpace
		case key == game.KeyB || key == game.KeyEscape:
			return ""
		}
	}
}

// ChooseOption picks one option: a letter in keyboard mode, a menu in controller mode.
func (s *Screen) ChooseOption(options []game.MenuOption, echo game.Echo, place *game.MenuPlacement) string {
	key := ""
	for {
		if s.promptMode() == "controller" {
			title := "Choose"
			if place != nil {
				title = place.Title
			}
			if picked := s.runMenu(title, options, 1, place, 0, nil); picked >= 0 {
				key = options[picked].Key
			}
			break
		}
		k := strings.ToUpper(s.WaitKey())
		if k == game.KeyEscape {
			break
		}
		i := indexOf(optionKeys(options), k)
		if i >= 0 && options[i].Disabled {
			s.Sound(game.SoundError1) // a spell the caster cannot afford
			continue
		}
		if i >= 0 {
			key = k
			break
		}
	}
	if echo != game.EchoNone {
		if key == "" {
			s.Print(" ")
		} else {
			s.Print(key)
		}
	}
	if echo == game.EchoLine {
		s.Print("\n")
	}
	return key
}

func optionKeys(options []game.MenuOption) []string {
	out := make([]string, len(options))
	for i, o := range options {
		out[i] = o.Key
	}
	return out
}

// readLine reads a line with a blinking cursor (UInputText): the cursor
// frames are in row 1 of the UI sheet, and the text is echoed in place.
func (s *Screen) readLine(x, y, maxChars int, numbersOnly bool, echo func(string)) string {
	text, frame := "", 0
	for {
		echo(text)
		cx := x + len(text)
		var key string
		for ok := false; !ok; {
			frame = (frame + 1) % 7
			if cx < Columns {
				s.gfx.drawUIPiece(s.c, frame, 1, cx*s.cell, y*s.cell, s.cell)
			}
			key, ok = s.rawKey(120)
		}
		if cx < Columns {
			s.drawText(" ", cx, y, "")
		}
		switch {
		case key == game.KeyEnter || key == game.KeyEscape:
			return text
		case key == game.KeyBackspace:
			if text != "" {
				text = text[:len(text)-1]
			}
		case len(key) != 1 || key[0] < ' ' || key[0] > '~':
		case numbersOnly && (key < "0" || key > "9"):
		case len(text) < maxChars:
			text += key
		}
	}
}

// InputText reads a line of text at the cursor, echoing it.
func (s *Screen) InputText(maxChars int, numbersOnly bool, words []string) string {
	x, y := s.cursorX, s.cursorY
	var result string
	if s.InputMode == "controller" {
		result = s.controllerInput(maxChars, numbersOnly, words, -1)
	} else {
		result = s.readLine(x, y, maxChars, numbersOnly, func(t string) { s.drawText(clipStr(t+"   ", maxChars+1), x, y, "") })
	}
	s.cursorX = x
	s.Print(result)
	return result
}

// InputTextAt reads a line of text at a cell position.
func (s *Screen) InputTextAt(x, y, maxChars int, numbersOnly bool) string {
	echo := func(t string) { s.drawText(padEnd(t+" ", maxChars+1), x, y, "") }
	if s.InputMode == "controller" {
		// The keyboard's window opens on the row above the prompt, over it.
		result := s.controllerInput(maxChars, numbersOnly, nil, y-1)
		echo(result)
		return result
	}
	return s.readLine(x, y, maxChars, numbersOnly, echo)
}

// controllerInput is text entry by controller: a spinner for numbers
// (up/down by one, left/right by ten), a word list when one is offered,
// otherwise an on-screen keyboard, at row when >= 0.
func (s *Screen) controllerInput(maxChars int, numbersOnly bool, words []string, row int) string {
	if numbersOnly {
		top := 1
		for range maxChars {
			top *= 10
		}
		top--
		value := 0
		label := func() string { return padStart(fmt.Sprint(value), maxChars) + "  up/down" }
		m := layoutMenu("Amount", []string{label()}, 1)
		m.Cursor = -1
		s.openMenu(m)
		defer s.closeMenu()
		for {
			key, _ := s.readKey(0)
			switch {
			case key == game.KeyUp:
				value = min(top, value+1)
			case key == game.KeyDown:
				value = max(0, value-1)
			case key == game.KeyRight:
				value = min(top, value+10)
			case key == game.KeyLeft:
				value = max(0, value-10)
			case key == game.KeyA:
				return fmt.Sprint(value)
			case key == game.KeyB:
				return ""
			case len(key) == 1 && key >= "0" && key <= "9":
				value = min(top, value*10+int(key[0]-'0'))
			}
			m.Items[0] = label()
			s.showMenuNow()
		}
	}
	if words != nil {
		var options []game.MenuOption
		for _, w := range words {
			options = append(options, game.MenuOption{Key: w[:1], Label: w, Card: game.WordCard(w)})
		}
		if len(options) == 0 {
			var lines []game.CardLine
			for _, t := range game.CardWrap("None learnt yet. Townsfolk say them: walk into everyone.") {
				lines = append(lines, game.CardLine{Text: t})
			}
			options = []game.MenuOption{{Key: "-", Label: "None known", Disabled: true, Card: &game.Card{Title: "Words", Lines: lines}}}
		}
		picked := s.runMenu("Word", options, 1, nil, 0, nil)
		if picked < 0 || picked >= len(words) {
			return ""
		}
		return words[picked]
	}
	// The on-screen keyboard, made again for each key, the cursor kept.
	text, cursor := "", 0
	for {
		title := text
		if title == "" {
			title = "Type"
		}
		m := layoutKeyboard(title)
		if row >= 0 {
			s.placeMenu(m, &game.MenuPlacement{Row: row, Title: m.Title, Cursor: -1})
		}
		m.Cursor = cursor
		s.openMenu(m)
		picked := -1
	read:
		for {
			key, _ := s.readKey(0)
			switch {
			case moveCursor(m, key):
				s.showMenuNow()
			case key == game.KeyA:
				picked = m.Cursor
				break read
			case key == game.KeyB:
				picked = indexOf(oskKeys, oskDelete)
				break read
			case key == game.KeyX || key == game.KeyY:
				picked = indexOf(oskKeys, oskDone)
				break read
			case len(key) == 1 && key >= " ":
				text = clipStr(text+key, maxChars)
				break read
			}
		}
		cursor = m.Cursor
		s.closeMenu()
		if picked < 0 {
			continue
		}
		switch k := oskKeys[picked]; k {
		case oskDone:
			return text
		case oskDelete:
			if text == "" {
				return ""
			}
			text = text[:len(text)-1]
		default:
			if len(text) < maxChars {
				text += k
			}
		}
	}
}
