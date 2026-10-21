package game

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// The title screen and party organisation: MainMenu(), Organize(),
// CreateChar(), FormParty(), DisperseParty() and KillChar() of UltimaMain.c.
// The Apple II typed everything at prompts and the Mac used dialogs; this
// port uses menus on the title screen in every input mode. The records
// written are exactly what the Apple II wrote.

// 1-based indices into the MoreMessages table.
const (
	mmNotFormed              = 2
	mmFormTheParty           = 4
	mmFormed                 = 12
	mmDisperseTheParty       = 13
	mmNoOneThere             = 14
	mmDispersed              = 15
	mmCreate                 = 16
	mmCreated                = 21
	mmTerminate              = 22
	mmTerminated             = 25
	mmCopyright              = 26
	mmFromTheDepths          = 27
	mmHeComes                = 28
	mmOrganizeAParty         = 30
	mmJourneyOnwardOption    = 31
	mmOptions                = 32
	mmPartyOrganization      = 33
	mmCreateACharacter       = 35
	mmFormThePartyOption     = 36
	mmDisperseThePartyOption = 37
	mmTerminateACharacter    = 38
	mmMainMenu               = 39
)

// menuRow is where the menus sit on the title screen (rows below the picture).
const menuRow = 13

// TitleMenuLastRow is the last row the title menu's window may use: the copyright is below.
const TitleMenuLastRow = 21

func mm(w *World, n int) string {
	s := w.Resources.Strings["MoreMessages"]
	if n < 1 || n > len(s) {
		return ""
	}
	// The bitmap font only has ASCII; the copyright sign becomes "(C)".
	return strings.ReplaceAll(s[n-1], "©", "(C)")
}

// notice shows a line near the bottom of the title screen and waits for a key.
func notice(io IO, text string) {
	io.CentreText(21, text)
	io.WaitKey()
}

// Transfer exports and imports the game as text (the UI's clipboard and files).
type Transfer interface {
	Export() string            // the game as text, or "" when there is nothing to export
	Import(text string) string // replace the saved game; "" when it worked, else a short reason
	ClipboardWrite(text string) error
	ClipboardRead() (string, error)
	FileSave(text string) error
	FileLoad() (string, error) // "" when the picker is cancelled
}

// Update is a new version of the app, downloaded and waiting.
type Update interface {
	Ready() bool
	Apply()
	Label() string // the title menu's line for it; "" for "Update: restart"
}

// MenuOptions are the main menu's hooks; any may be nil.
type MenuOptions struct {
	Save     func(w *World) // after the roster or party changes in Organize
	Transfer Transfer
	Update   Update
	Quit     func() // leave the game, where the app can
}

// MainMenu mirrors MainMenu(). play runs the game and returns when the
// player quits; the menu then shows again, for ever.
func MainMenu(w *World, io IO, play func(), options MenuOptions) {
	for {
		io.ShowTitle()
		io.ClearBottom()
		io.CentreText(11, mm(w, mmFromTheDepths))
		io.CentreText(12, mm(w, mmHeComes))
		io.CentreText(22, mm(w, mmCopyright))

		entries := []MenuOption{
			{Key: "J", Label: mm(w, mmJourneyOnwardOption)},
			{Key: "O", Label: mm(w, mmOrganizeAParty)},
		}
		if options.Transfer != nil {
			if w.Party.Formed() {
				entries = append(entries, MenuOption{Key: "E", Label: "Export game"})
			}
			entries = append(entries, MenuOption{Key: "I", Label: "Import game"})
		}
		if options.Update != nil && options.Update.Ready() {
			label := options.Update.Label()
			if label == "" {
				label = "Update: restart"
			}
			entries = append(entries, MenuOption{Key: "U", Label: label})
		}
		entries = append(entries, MenuOption{Key: "S", Label: "Settings"})
		if options.Quit != nil {
			entries = append(entries, MenuOption{Key: "Q", Label: "Quit"})
		}
		// Row 15, or higher for a longer list, so the bottom border stays above the copyright.
		row := min(15, TitleMenuLastRow-len(entries)-1)
		switch key := io.ChooseFromList(entries, &MenuPlacement{Row: row, Title: mm(w, mmOptions)}); {
		case key == "Q" && options.Quit != nil:
			options.Quit() // the app goes; should it not, the menu is still here
		case key == "S":
			io.ShowSettings()
		case key == "E" && options.Transfer != nil:
			exportGameTo(io, options.Transfer)
		case key == "I" && options.Transfer != nil:
			importGameFrom(io, options.Transfer)
		case key == "U":
			if options.Update != nil {
				options.Update.Apply()
			}
		case key == "J":
			if !w.Party.Formed() {
				notice(io, mm(w, mmNotFormed))
				continue
			}
			if w.Party.RulesPending() && !ChooseRules(w, io) {
				continue
			}
			play()
		case key == "O":
			Organize(w, io)
			if options.Save != nil {
				options.Save(w)
			}
		}
	}
}

// exportGameTo exports the game as text, to the clipboard or a file.
func exportGameTo(io IO, t Transfer) {
	text := t.Export()
	if text == "" {
		notice(io, "Nothing to export")
		return
	}
	switch io.ChooseFromList([]MenuOption{{Key: "C", Label: "To the clipboard"}, {Key: "F", Label: "To a file"}}, &MenuPlacement{Row: 15, Title: "Export"}) {
	case "C":
		if t.ClipboardWrite(text) != nil {
			notice(io, "The clipboard is not available here")
		} else {
			notice(io, "Game copied to the clipboard")
		}
	case "F":
		if t.FileSave(text) != nil {
			notice(io, "Could not save a file here")
		} else {
			notice(io, "Game saved as a file")
		}
	}
}

// importGameFrom replaces the saved game with text from the clipboard or a file, after a warning.
func importGameFrom(io IO, t Transfer) {
	var text string
	switch io.ChooseFromList([]MenuOption{{Key: "C", Label: "From the clipboard"}, {Key: "F", Label: "From a file"}}, &MenuPlacement{Row: 15, Title: "Import"}) {
	case "C":
		s, err := t.ClipboardRead()
		if err != nil {
			notice(io, "The clipboard cannot be read here")
			return
		}
		if text = strings.TrimSpace(s); text == "" {
			notice(io, "The clipboard is empty")
			return
		}
	case "F":
		s, err := t.FileLoad()
		if err != nil {
			notice(io, "Could not read the file")
			return
		}
		if text = strings.TrimSpace(s); text == "" {
			return // cancelled
		}
	default:
		return
	}
	if io.ChooseFromList([]MenuOption{{Key: "Y", Label: "Replace the saved game"}, {Key: "N", Label: "Keep the saved game"}}, &MenuPlacement{Row: 15, Title: "Import?"}) != "Y" {
		return
	}
	if reason := t.Import(text); reason != "" {
		notice(io, "Not imported: "+reason)
	} else {
		notice(io, "Game imported")
	}
}

// ChooseRules is a new game's first question (this port): Modern (poison
// stops at one hit point, starvation at half, experience shared, timer
// slow), Classic (the Apple II's) or Story (Modern without starvation or
// timer). False if the player backs out.
func ChooseRules(w *World, io IO) bool {
	key := io.ChooseFromList([]MenuOption{
		{Key: "M", Label: "Modern (Recommended)"},
		{Key: "C", Label: "Classic (Hardcore)"},
		{Key: "S", Label: "Story (Relaxed)"},
	}, &MenuPlacement{Row: 15, Title: "Choose Thine Adventure!"})
	switch key {
	case "M":
		w.Starvation, w.Timer = StarvationMild, TimerSlow
	case "C":
		w.Starvation, w.Timer = StarvationClassic, TimerFast
	case "S":
		w.Starvation, w.Timer = StarvationNone, TimerOff
	default:
		return false
	}
	w.PoisonKills = key == "C"
	w.BalancedXP = key != "C"
	if w.OnRulesChange != nil {
		w.OnRulesChange()
	}
	w.Party.SetRulesPending(false)
	return true
}

// Organize mirrors Organize().
func Organize(w *World, io IO) {
	cursor := 0
	for {
		io.ShowTitle()
		io.ClearBottom()
		io.CentreText(11, mm(w, mmPartyOrganization))
		options := []MenuOption{
			{Key: "C", Label: mm(w, mmCreateACharacter)},
			{Key: "F", Label: mm(w, mmFormThePartyOption)},
			{Key: "D", Label: mm(w, mmDisperseThePartyOption)},
			{Key: "N", Label: "Clear notes"},
			{Key: "T", Label: mm(w, mmTerminateACharacter)},
			{Key: "M", Label: mm(w, mmMainMenu)},
		}
		key := io.ChooseFromList(options, &MenuPlacement{Row: menuRow, Title: mm(w, mmOptions), Cursor: cursor})
		cursor = max(0, slices.IndexFunc(options, func(o MenuOption) bool { return o.Key == key }))
		switch key {
		case "C":
			CreateCharacter(w, io)
		case "F":
			FormParty(w, io)
		case "D":
			DisperseParty(w, io)
		case "N":
			ClearNotes(w, io)
		case "T":
			TerminateCharacter(w, io)
		default:
			return
		}
	}
}

// careers is the career table as text.
func careers(w *World) string { return string(w.Resources.Misc.Career) }

// rosterLabel is one roster entry as a menu line: "12. Tatiana       Fighter    #1".
func rosterLabel(w *World, slot int, mark string) string {
	p := w.Roster.Get(slot)
	number := fmt.Sprintf("%3s", strconv.Itoa(slot+1)+".")
	if !p.Exists() {
		return number + " (empty)"
	}
	className := "?"
	if i := strings.IndexByte(careers(w), p.ClassLetter()); i >= 0 && i < len(w.Resources.Strings["Classes"]) {
		className = w.Resources.Strings["Classes"][i]
	}
	return fmt.Sprintf("%s %-13s %-11s%s", number, p.Name(), className, mark)
}

// rosterOptions is the roster entries matching include, keyed by entry number.
func rosterOptions(w *World, include func(*PlayerRecord) bool, mark func(slot int) string) []MenuOption {
	var out []MenuOption
	for slot := range RosterSize {
		if !include(w.Roster.Get(slot)) {
			continue
		}
		m := ""
		if mark != nil {
			m = mark(slot)
		}
		out = append(out, MenuOption{Key: strconv.Itoa(slot + 1), Label: rosterLabel(w, slot, m)})
	}
	return out
}

// pickEntry asks for a roster entry: the 0-based slot, or -1.
func pickEntry(w *World, io IO, title string, include func(*PlayerRecord) bool, none string) int {
	options := rosterOptions(w, include, nil)
	if len(options) == 0 {
		notice(io, none)
		return -1
	}
	key := io.ChooseFromList(options, &MenuPlacement{Row: menuRow, Title: title})
	if key == "" {
		return -1
	}
	n, _ := strconv.Atoi(key)
	return n - 1
}

// raceMax caps each attribute (strength, dexterity, intelligence, wisdom) by race.
var raceMax = map[byte][4]int{
	'H': {75, 75, 75, 75},
	'E': {75, 99, 50, 75},
	'D': {99, 75, 75, 50},
	'B': {75, 50, 99, 75},
	'F': {25, 99, 75, 99},
}

// classHint is a line on what a class can do, from the game's own tables.
func classHint(w *World, classIndex int) string {
	wa := w.Resources.Strings["WeaponsArmour"]
	weaponMax := int(w.Resources.Misc.WeaponUse[classIndex]) - 'A' - 1
	armourMax := int(w.Resources.Misc.ArmourUse[classIndex]) - 'A' - 1
	weapons, armour := "any weapon", "any armour"
	if weaponMax < 15 {
		weapons = "to " + strings.ToLower(wa[weaponMax])
	}
	if armourMax < 7 {
		armour = "to " + strings.ToLower(wa[16+armourMax])
	}
	cleric := slices.Contains([]int{1, 4, 7, 8, 10}, classIndex)
	wizard := slices.Contains([]int{2, 6, 9, 8, 10}, classIndex)
	magic := ""
	switch {
	case cleric && wizard:
		magic = "both magics, "
	case cleric:
		magic = "cleric magic, "
	case wizard:
		magic = "wizard magic, "
	}
	thief := ""
	if classIndex == 3 {
		thief = "steals, "
	}
	return magic + thief + weapons + ", " + armour
}

// CreateCharacter is the character creation screen; it writes the record CreateChar() wrote.
func CreateCharacter(w *World, io IO) {
	io.ClearBottom()
	io.CentreText(11, mm(w, mmCreate))
	slot := pickEntry(w, io, "Which entry?", func(p *PlayerRecord) bool { return !p.Exists() }, "The roster is full.")
	if slot < 0 {
		return
	}
	p := w.Roster.Get(slot)
	races := w.Resources.Strings["Races"]
	classes := w.Resources.Strings["Classes"]
	car := careers(w)
	var taken []string
	for i := range RosterSize {
		if w.Roster.Get(i).Exists() {
			taken = append(taken, w.Roster.Get(i).Name())
		}
	}

	// The line at the top of the screen grows as choices are made.
	var summary []string
	header := func() {
		io.ClearBottom()
		io.CentreText(11, strings.TrimSpace(fmt.Sprintf("Entry %d: %s", slot+1, strings.Join(summary, "  "))))
	}
	random := func() float64 { return float64(w.Rng.Range(0, 255)) / 256 }

	// Name: typed, or one from the stock list.
	var name string
	suggestion := RandomName(random, taken)
	cursor := 0
	for {
		header()
		key := io.ChooseFromList([]MenuOption{
			{Key: "T", Label: "Type a name"},
			{Key: "U", Label: `Use "` + suggestion + `"`},
			{Key: "A", Label: "Another random name"},
		}, &MenuPlacement{Row: menuRow, Title: "Name", Cursor: cursor})
		cursor = 2 // after "Another", stay on it
		if key == "" {
			return
		}
		if key == "A" {
			suggestion = RandomName(random, append(slices.Clone(taken), suggestion))
			continue
		}
		if key == "U" {
			name = suggestion
			break
		}
		io.TextAt(13, menuRow+1, "Name:")
		if name = strings.TrimSpace(io.InputTextAt(19, menuRow+1, 13, false)); name != "" {
			break
		}
	}
	summary = append(summary, name)

	header()
	sex := io.ChooseFromList([]MenuOption{{Key: "M", Label: "Male"}, {Key: "F", Label: "Female"}, {Key: "O", Label: "Other"}}, &MenuPlacement{Row: menuRow, Title: "Sex"})
	if sex == "" {
		return
	}
	summary = append(summary, map[string]string{"M": "Male", "F": "Female", "O": "Other"}[sex])

	header()
	var raceOptions []MenuOption
	for _, r := range races {
		m, ok := raceMax[r[0]]
		if !ok {
			m = [4]int{75, 75, 75, 75}
		}
		raceOptions = append(raceOptions, MenuOption{Key: r[:1], Label: r, Hint: fmt.Sprintf("Max STR %d DEX %d INT %d WIS %d", m[0], m[1], m[2], m[3])})
	}
	raceKey := io.ChooseFromList(raceOptions, &MenuPlacement{Row: menuRow, Title: "Race"})
	if raceKey == "" {
		return
	}
	race := slices.IndexFunc(races, func(r string) bool { return r[:1] == raceKey })
	summary = append(summary, races[race])

	header()
	var classOptions []MenuOption
	for i, c := range classes {
		classOptions = append(classOptions, MenuOption{Key: car[i : i+1], Label: c, Hint: classHint(w, i)})
	}
	classKey := io.ChooseFromList(classOptions, &MenuPlacement{Row: menuRow, Title: "Type"})
	if classKey == "" {
		return
	}
	career := strings.Index(car, classKey)
	summary = append(summary, classes[career])

	// Four attributes, 5..25 each, all 50 points spent.
	header()
	values := io.AllocatePoints([]string{"Strength", "Dexterity", "Intelligence", "Wisdom"}, 50, 5, 25, &MenuPlacement{Row: menuRow, Title: "50 points"})
	if values == nil {
		return
	}

	header()
	io.CentreText(menuRow-1, fmt.Sprintf("STR %d  DEX %d  INT %d  WIS %d", values[0], values[1], values[2], values[3]))
	if io.ChooseFromList([]MenuOption{{Key: "Y", Label: "Create this character"}, {Key: "N", Label: "Cancel"}}, &MenuPlacement{Row: menuRow + 1, Title: "O.K.?"}) != "Y" {
		return
	}

	b := p.Bytes
	clear(b)
	p.SetName(name)
	b[24] = sex[0]
	b[22] = races[race][0]
	b[23] = car[career]
	for i := range 4 {
		b[18+i] = byte(values[i])
	}
	p.SetStatus('G')
	b[27] = 100 // hit points
	b[29] = 100 // max hit points
	b[32] = 1   // food 150
	b[33] = 50
	b[36] = 150 // gold
	b[41] = 1   // cloth
	b[40] = 1   //   worn
	b[49] = 1   // dagger
	b[48] = 1   //   readied
	notice(io, mm(w, mmCreated))
}

// FormParty mirrors the Apple II FormParty(): up to four roster entries in
// marching order, from a list. Picking a chosen entry again removes it.
func FormParty(w *World, io IO) {
	io.ClearBottom()
	io.CentreText(11, mm(w, mmFormTheParty))
	if w.Party.Formed() {
		if io.ChooseFromList([]MenuOption{{Key: "Y", Label: "Disperse it and form a new one"}, {Key: "N", Label: "Keep the current party"}}, &MenuPlacement{Row: menuRow, Title: "A party is already formed"}) != "Y" {
			return
		}
		disperse(w)
	}

	var chosen []int
	cursor := 0
	for {
		io.ClearBottom()
		io.CentreText(11, mm(w, mmFormTheParty))
		order := func(slot int) string {
			if i := slices.Index(chosen, slot); i >= 0 {
				return " #" + strconv.Itoa(i+1)
			}
			return ""
		}
		options := rosterOptions(w, func(p *PlayerRecord) bool { return p.Exists() && !p.InParty() }, order)
		if len(options) == 0 {
			notice(io, "No characters to choose from.")
			return
		}
		done := "Cancel"
		if len(chosen) > 0 {
			done = fmt.Sprintf("Done (%d chosen)", len(chosen))
		}
		options = append(options, MenuOption{Key: "D", Label: done})
		key := io.ChooseFromList(options, &MenuPlacement{Row: menuRow, Title: "Choose up to four", Cursor: cursor})
		if key == "" {
			return
		}
		if key == "D" {
			if len(chosen) == 0 {
				return
			}
			break
		}
		n, _ := strconv.Atoi(key)
		slot := n - 1
		cursor = slices.IndexFunc(options, func(o MenuOption) bool { return o.Key == key })
		if at := slices.Index(chosen, slot); at >= 0 {
			chosen = slices.Delete(chosen, at, at+1)
		} else if len(chosen) < 4 {
			chosen = append(chosen, slot)
		}
	}

	clear(w.Party.Bytes)
	w.Party.SetSize(len(chosen))
	for i, slot := range chosen {
		w.Party.SetMemberRosterNumber(i, slot+1)
		w.Roster.Get(slot).SetInParty(true)
	}
	w.PoolPurses()
	w.PoolGear()
	w.PoolSupplies()
	w.Party.SetRulesPending(true) // asked now, and again at Journey onward if backed out of here
	w.Party.SetLocation(LocationSosaria)
	w.Party.SetShape(0x7e)
	w.Party.Bytes[5] = 0xff
	w.Party.SetSurfaceX(42)
	w.Party.SetSurfaceY(20)
	w.X, w.ReturnX = 42, 42
	w.Y, w.ReturnY = 20, 20
	w.ResetSosaria()
	w.MoonPhase = [2]int{4, 4}
	w.MoonTimer = [2]int{12, 4}
	notice(io, mm(w, mmFormed))
	ChooseRules(w, io)
}

// disperse gives the members their share of the pool, frees every entry and clears the party record.
func disperse(w *World) {
	w.SplitPool()
	w.SplitGear()
	w.SplitSupplies()
	for i := range RosterSize {
		w.Roster.Get(i).SetInParty(false)
	}
	clear(w.Party.Bytes)
}

// DisperseParty mirrors DisperseParty().
func DisperseParty(w *World, io IO) {
	io.ClearBottom()
	io.CentreText(11, mm(w, mmDisperseTheParty))
	if !w.Party.Formed() {
		for i := range RosterSize {
			w.Roster.Get(i).SetInParty(false)
		}
		notice(io, mm(w, mmNoOneThere))
		return
	}
	if io.ChooseFromList([]MenuOption{{Key: "Y", Label: "Yes, disperse the party"}, {Key: "N", Label: "No"}}, &MenuPlacement{Row: menuRow, Title: "Disperse the party?"}) != "Y" {
		return
	}
	disperse(w)
	notice(io, mm(w, mmDispersed))
}

// ClearNotes forgets what the party has seen and learnt - the maps, the
// journal, the moongates - after asking (this port).
func ClearNotes(w *World, io IO) {
	io.ClearBottom()
	io.CentreText(11, "CLEAR NOTES")
	if io.ChooseFromList([]MenuOption{{Key: "Y", Label: "Yes, clear map, journal, moongates"}, {Key: "N", Label: "No"}}, &MenuPlacement{Row: menuRow, Title: "Clear notes?"}) != "Y" {
		return
	}
	w.AutoMap.Clear()
	w.Journal = EmptyJournal()
	w.GatesKnown = nil
	notice(io, "(Notes cleared!)")
}

// TerminateCharacter mirrors KillChar(): erase a roster entry not in a party, after confirming.
func TerminateCharacter(w *World, io IO) {
	io.ClearBottom()
	io.CentreText(11, mm(w, mmTerminate))
	slot := pickEntry(w, io, "Terminate whom?", func(p *PlayerRecord) bool { return p.Exists() && !p.InParty() },
		"No one can be terminated (a party member must be dispersed first).")
	if slot < 0 {
		return
	}
	p := w.Roster.Get(slot)
	io.ClearBottom()
	io.CentreText(11, mm(w, mmTerminate))
	if io.ChooseFromList([]MenuOption{{Key: "N", Label: "No, keep them"}, {Key: "Y", Label: "Yes, terminate " + p.Name()}}, &MenuPlacement{Row: menuRow, Title: "Terminate " + p.Name() + "?"}) != "Y" {
		return
	}
	clear(p.Bytes)
	notice(io, mm(w, mmTerminated))
}
