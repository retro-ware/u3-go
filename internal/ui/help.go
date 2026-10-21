package ui

import "fmt"

// The in-game help pages (help.ts). They show in a box over the map area:
// each has a title, drawn in the box's top edge, and up to pageRows lines
// of at most pageWidth characters. Which pages show depends on the input
// mode.

// HelpPage is one page of help.
type HelpPage struct {
	Title string
	Lines []string
}

// The page box over the map: how many lines fit, and how wide each is.
const (
	pageRows  = 19
	pageWidth = 20
)

// Build is the build this binary came from, set with -ldflags "-X ...ui.Build=...".
var Build = "dev"

// partyStatus is how a member's state is shown: the Standard set colours
// the name, every other set draws the Apple II's status letter after it.
func partyStatus(tileSet string) []string {
	if tileSet == "Standard" {
		return []string{"The name's colour:", "Green: poisoned", "Grey: dead", "Dark grey: ashes", "Blue: level up from", "  Lord British"}
	}
	return []string{"A letter by the name", "G: good", "L: level up from", "  Lord British", "P: poisoned", "D: dead", "A: ashes"}
}

var keyboardPages = []HelpPage{
	{"Keys 1/5", []string{
		"Arrows: walk; move", "  in combat", "Space: pass a turn", "Escape: pause menu", "J: journal",
		"#: view the map", "!: heal the party", "@: safe chest", "$: light the way", "",
		"A \"Who?\" prompt", "takes 1-4, or", "Up/Down and Enter.", "A direction is an", "arrow key.",
	}},
	{"Keys 2/5", []string{
		"A: attack", "B: board horse/ship", "C: cast a spell", "E: enter a place", "F: fire cannons",
		"G: get a chest", "I: ignite a torch", "K: klimb a ladder", "L: look", "M: marching order",
		"N: negate time", "O: other, say a word", "P: peer at a gem",
	}},
	{"Keys 3/5", []string{
		"Q: quit and save", "  (surface only)", "R: ready a weapon", "S: steal", "T: transact: talk,",
		"  shop, the king", "U: unlock a door", "V: volume", "W: wear armour", "X: exit horse/ship",
		"Y: yell (same as O)", "Z: ztats", "",
		"Alt +/-: music vol", "Alt V: music on/off", "F11: full screen",
	}},
	{"Keys 4/5", []string{
		"COMBAT", "Arrows: move; into", "  a monster attacks", "A: attack, then a", "  direction",
		"C, N, R, Z: as above", "B or Escape: take", "  the fight back", "  from auto combat", "",
		"DUNGEONS", "Up/Down: advance,", "  retreat", "Left/Right: turn", "I, K, D: ignite,",
		"  klimb, descend", "L: change view:", "  first person,", "  + map, top down",
	}},
}

var controllerPages = []HelpPage{
	{"Controller 1/2", []string{
		"D-pad: move, menus", "A: menu, and choose", "B: cancel, or pass", "X: ztats", "Y: look; attack in",
		"  combat; ignite in", "  dungeons", "View or Menu: pause", "",
		"Keys for the pad:", "WASD/arrows: d-pad", "Enter or Z: A", "X or B: B", "C: X    V or Y: Y",
		"Escape: pause", "", "The pause menu holds", "the settings, auto", "combat and quit.",
	}},
}

// helpPages are the pages for an input mode ("keyboard" or "controller"),
// the last written for the tile set in use.
func helpPages(mode, tileSet string) []HelpPage {
	pages := keyboardPages
	saving := []string{"Q saves and resumes", "next visit."}
	if mode == "controller" {
		pages = controllerPages
		saving = []string{"Quit (Pause) saves;", "resumes next visit."}
	}
	n := len(pages) + 1
	lines := append(append(append(partyStatus(tileSet), ""), saving...), "Build "+Build)
	return append(append([]HelpPage(nil), pages...), HelpPage{fmt.Sprintf("Status %d/%d", n, n), lines})
}
