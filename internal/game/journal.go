package game

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// The quest journal (the TypeScript port's; the Apple II had none), after the
// Ultima V port's: what the quest asks for, and the places it sends the
// party, each with how far it has got. It records what the world has told
// the party and what the party has done. The top is seven lines - Main
// quest, Towns, Rumours, Dungeons, Ambrosia, Equipment, Words - each opening
// to more. Every line has a card (what the party knows of it) and a hint,
// printed to the message area only when asked for (sixteen columns, nine
// lines). The whole state lives in the save file.

// JournalState is what the journal records as it happens. The JSON names are
// the TypeScript save's.
type JournalState struct {
	LordBritish   bool     `json:"lordBritish"`
	Ambrosia      bool     `json:"ambrosia"`
	TimeLord      bool     `json:"timeLord"`
	SerpentParted bool     `json:"serpentParted"`
	Radrion       bool     `json:"radrion"`      // Radrion met: the party has been to an oracle's counter
	Guild         bool     `json:"guild"`        // equipment found: a guild visited ...
	MagicWeapons  bool     `json:"magicWeapons"` // ... Dawn's weapon shop ...
	MagicArmour   bool     `json:"magicArmour"`  // ... and armour shop visited ...
	Ship          bool     `json:"ship"`         // ... a ship boarded
	Clues         []string `json:"clues"`        // clue ids heard (Clues), e.g. "Moon#6" or "Yew:pray"
	Words         []string `json:"words"`        // the words Other answers to that have been heard said
	Goals         []string `json:"goals"`        // dungeon goals found, "<dungeon map id>:<mark>"
	Seen          []string `json:"seen"`         // what the journal showed when last noted (JournalSnapshot)
}

// EmptyJournal is a new game's journal.
func EmptyJournal() JournalState {
	return JournalState{Clues: []string{}, Words: []string{}, Goals: []string{}, Seen: []string{}}
}

// JournalFlags are the journal's flags, saved, restored and compared alike.
var JournalFlags = []string{"lordBritish", "ambrosia", "timeLord", "serpentParted", "radrion", "guild", "magicWeapons", "magicArmour", "ship"}

// Flag is a journal flag by its JournalFlags name.
func (j *JournalState) Flag(name string) *bool {
	switch name {
	case "lordBritish":
		return &j.LordBritish
	case "ambrosia":
		return &j.Ambrosia
	case "timeLord":
		return &j.TimeLord
	case "serpentParted":
		return &j.SerpentParted
	case "radrion":
		return &j.Radrion
	case "guild":
		return &j.Guild
	case "magicWeapons":
		return &j.MagicWeapons
	case "magicArmour":
		return &j.MagicArmour
	case "ship":
		return &j.Ship
	}
	return nil
}

// A main-quest step.
type step string

// Towns kept a mystery till seen, entered or told of: Dawn, and the three on isles.
const (
	jDawn       = 408
	jDevilGuard = 409
	jFawn       = 410
	jDeathGulch = 411
)

var hiddenTowns = []int{jDawn, jDevilGuard, jFawn, jDeathGulch}

// Clue is a line the party may hear that bears on the quest, and what
// hearing it does: the Other word it teaches, the steps it bears on, those it
// names (makes known), and what it reveals ("ambrosia", "moongates", or a
// hidden town or dungeon by its map id as text). A townsperson's id is
// "<town name in the files>#<speaker>", a rumour "Pub:<n>", a verse
// "Radrion:<n>", a dungeon's writing "<map id>:<level>".
type Clue struct {
	Word    string
	Quest   []step
	Names   []step
	Reveals []string
	From    string // who says it, where it is not a townsperson's line
	Text    string // and what
}

func ids(n ...int) []string {
	var out []string
	for _, i := range n {
		out = append(out, strconv.Itoa(i))
	}
	return out
}

func steps(s ...step) []step { return s }

// Clues are all the clues by id.
var Clues = map[string]Clue{
	"LB:mark":               {Quest: steps("kings"), Names: steps("kings"), From: "Lord British", Text: "SEEK YE, THE MARK OF KINGS!"},
	"Lord British Castle#1": {Reveals: ids(jDawn)},
	"Lord British Castle#6": {Quest: steps("cards", "order"), Names: steps("cards")},
	"LCB Towne#2":           {Reveals: []string{"ambrosia"}},
	"LCB Towne#3":           {Quest: steps("exotics"), Names: steps("exotics")},
	"LCB Towne#4":           {Quest: steps("snake"), Names: steps("snake")},
	"LCB Towne#5":           {Reveals: ids(jDawn)},
	"Moon#2":                {Quest: steps("cards")},
	"Moon#3":                {Word: "BRIBE"},
	// "Peace be with you": the word that calms, though the line never says it.
	"Moon#5":        {Word: "PAXUM"},
	"Moon#6":        {Reveals: []string{"ambrosia"}},
	"Moon#7":        {Reveals: ids(jDawn)},
	"Yew#1":         {Word: "SEARCH", Quest: steps("cards")},
	"Yew#2":         {},
	"Yew:pray":      {Word: "EVOCARE", Quest: steps("snake"), From: "The circle of light", Text: "YELL 'EVOCARE'"},
	"East Montor#1": {Word: "INSERT", Quest: steps("cards", "order"), Names: steps("cards")},
	"East Montor#2": {Quest: steps("cards", "order"), Names: steps("cards")},
	"East Montor#5": {Quest: steps("cards"), Names: steps("cards")},
	"East Montor#6": {Quest: steps("kings")},
	"East Montor#7": {Quest: steps("exotics"), Names: steps("exotics")},
	"West Montor#5": {Word: "SEARCH", Quest: steps("cards")},
	"West Montor#6": {Word: "SEARCH", Quest: steps("cards"), Names: steps("cards")},
	"Grey#3":        {Quest: steps("exotics"), Names: steps("exotics")},
	"Grey#4":        {Quest: steps("exotics"), Names: steps("exotics"), Reveals: ids(jDawn)},
	"Grey#5":        {Quest: steps("exotics"), Names: steps("exotics")},
	"Dawn#2":        {Word: "DIG", Quest: steps("exotics")},
	"Dawn#3":        {Word: "DIG", Quest: steps("exotics"), Names: steps("exotics")},
	"Dawn#4":        {Word: "DIG"},
	"Dawn#6":        {},
	"Dawn#7":        {Reveals: []string{"ambrosia"}},
	"Devil Guard#1": {Quest: steps("kings")},
	"Devil Guard#2": {Quest: steps("kings")},
	"Devil Guard#3": {Quest: steps("kings"), Names: steps("kings")},
	"Devil Guard#4": {Quest: steps("snake"), Names: steps("snake")},
	"Devil Guard#5": {Quest: steps("kings", "fire", "force", "snake")},
	"Devil Guard#6": {Quest: steps("fire"), Reveals: ids(413)}, // the Fires of Hell, the only dungeon a line names
	"Devil Guard#7": {Quest: steps("kings")},
	"Fawn#1":        {Word: "PRAY", Quest: steps("snake")},
	"Fawn#2":        {Word: "PRAY", Quest: steps("snake")},
	"Fawn#3":        {Quest: steps("snake"), Names: steps("snake")},
	"Fawn#4":        {Quest: steps("snake")},
	"Death Gulch#2": {Word: "BRIBE"},
	"Death Gulch#3": {Quest: steps("order", "exodus")},
	"TimeLord":      {Quest: steps("order", "exodus"), From: "The Time Lord", Text: "THE ONE WAY IS LOVE, SOL, MOONS & DEATH, ALL ELSE FAILS."},
	// The pubs' rumours (shops.go pub); #0, "Thank you, kindly!", is no rumour.
	"Pub:1": {Reveals: []string{"ambrosia"}},
	"Pub:2": {Reveals: ids(jDawn)},
	"Pub:3": {Reveals: []string{"moongates", strconv.Itoa(jDawn)}},
	"Pub:4": {},
	"Pub:5": {Reveals: []string{"ambrosia"}},
	"Pub:6": {Reveals: []string{"ambrosia"}, Quest: steps("cards")},
	"Pub:7": {},
	"Pub:8": {},
	"Pub:9": {Quest: steps("order"), Names: steps("order")},
	// Radrion's rhyme, a verse for each hundred gold offered (shops.go oracle).
	"Radrion:0": {},
	"Radrion:1": {Quest: steps("kings", "cards"), Names: steps("cards")},
	"Radrion:2": {Quest: steps("kings", "fire", "force", "snake"), Names: steps("kings", "fire", "force", "snake")},
	"Radrion:3": {Quest: steps("kings", "fire", "force", "snake"), Reveals: ids(jDevilGuard)},
	"Radrion:4": {Quest: steps("cards")},
	"Radrion:5": {Quest: steps("cards")},
	"Radrion:6": {Quest: steps("cards"), Names: steps("cards")},
	"Radrion:7": {Quest: steps("cards", "order")},
	"Radrion:8": {Quest: steps("order")},
	"Radrion:9": {Quest: steps("order"), Names: steps("order")},
	// The misty writing on a dungeon's level.
	"415:2": {Word: "INSERT", Quest: steps("cards", "order"), Names: steps("cards")},
	"415:3": {Word: "SEARCH", Quest: steps("cards")},
}

// audience is Lord British's audience, counted among his castle's clues.
const audience = "LB:audience"

var questSteps = []struct {
	id         step
	name, hint string
}{
	{"king", "Lord British", "Lord British sits in his castle north of Britain's town. Walk into him."},
	{"kings", "Mark of Kings", "Level 1 of the Perinian Depths, north-east of Britain: step on the rod. Light: Lorum, Luminae, or torches from a guild."},
	{"cards", "The four cards", "In Ambrosia's four shrines, Other and say SEARCH. Two lie past locked doors: bring keys and take a pirate's ship."},
	{"exotics", "Exotic arms", "Other, DIG once per member. Weapons: the islet east of Fawn, its east square. Armour: the islet east of Grey, its west square."},
	{"fire", "Mark of Fire", "Level 8 of the Perinian Depths, the Fires of Hell or Morinia. Each member needs it to cross lava."},
	{"force", "Mark of Force", "Level 8 of Doom or the Fires of Hell. Each member, for the force fields in Exodus' castle."},
	{"snake", "Silver Snake", "Mark of Snake: level 8 of Clues. PRAY in Yew's circle of light for the word. Yell it from a ship just south of the serpent."},
	{"order", "Cards' order", "The Time Lord waits on level 8 of Time Awaits, beside Death Gulch. He names the order of the cards."},
	{"exodus", "Exodus", "Exodus' castle: the south-west isle, past the serpent. Only exotics bite. INSERT the cards left to right in the Time Lord's order."},
}

type journalTown struct {
	id    int
	name  string
	clues []string
	hint  string
}

var journalTowns = []journalTown{
	{MapIDLordBritishCastle, "Lord British", []string{audience, "Lord British Castle#1", "Lord British Castle#6"}, "Lord British's castle, where the party starts. Walk into the king to speak with him."},
	{402, "Britain", []string{"LCB Towne#2", "LCB Towne#3", "LCB Towne#4", "LCB Towne#5"}, "Beside Lord British's castle, where the party starts."},
	{403, "Moon", []string{"Moon#2", "Moon#3", "Moon#6", "Moon#7"}, "In the far north-west. On foot."},
	{404, "Yew", []string{"Yew#1", "Yew#2", "Yew:pray"}, "West of Britain, on foot. Its circle of light: the ring of fire in the woods south-east of the town. PRAY at its centre."},
	{405, "East Montor", []string{"East Montor#1", "East Montor#2", "East Montor#5", "East Montor#6", "East Montor#7"}, "In the far south, beside West Montor. On foot."},
	{406, "West Montor", []string{"West Montor#5", "West Montor#6"}, "In the far south, beside East Montor. On foot."},
	{407, "Grey", []string{"Grey#3", "Grey#4", "Grey#5"}, "In the south-west. On foot."},
	{jDawn, "Dawn", []string{"Dawn#2", "Dawn#3", "Dawn#4", "Dawn#6", "Dawn#7"}, "In the forest west of the Montors, 8 west and 35 south of Lord British's castle. It stands only while both moons are new."},
	{jDevilGuard, "Devil Guard", []string{"Devil Guard#1", "Devil Guard#2", "Devil Guard#3", "Devil Guard#4", "Devil Guard#5", "Devil Guard#6", "Devil Guard#7"}, "An isle in the west: a ship is needed. Take one from pirates."},
	{jFawn, "Fawn", []string{"Fawn#1", "Fawn#2", "Fawn#3", "Fawn#4"}, "A small isle in the far north: a ship is needed. Take one from pirates."},
	{jDeathGulch, "Death Gulch", []string{"Death Gulch#2", "Death Gulch#3"}, "An isle in the east: a ship is needed. Take one from pirates."},
}

// rod is a mark's rod in a dungeon level's cells (DungeonCellMark).
const rod = 0x05

// RodMarks is the mark a rod gives, by its column (x & 3), as dungeon.go sets it.
var RodMarks = []string{"force", "fire", "snake", "kings"}

var goalNames = map[string]string{
	"force": "the Mark of Force", "fire": "the Mark of Fire", "snake": "the Mark of Snake",
	"kings": "the Mark of Kings", "timeLord": "the Time Lord",
}

type journalDungeon struct {
	id    int
	name  string
	goals []string
	hint  string
}

// The dungeons in the order the quest tends to want them, with what each is there for.
var journalDungeons = []journalDungeon{
	{416, "Perinian Depths", []string{"kings", "fire"}, "North-east of Britain, on foot. Level 1: the Mark of Kings. Level 8: the Mark of Fire."},
	{418, "Dardin's Pit", []string{"kings"}, "North of Britain, on foot. Level 2: the Mark of Kings."},
	{417, "Morinia", []string{"fire"}, "The Mines of Morinia, in the west. On foot. Level 8: the Mark of Fire."},
	{413, "Fires of Hell", []string{"fire", "force"}, "South of Britain, on foot. Level 8: the Marks of Fire and Force."},
	{412, "Doom", []string{"force"}, "In the far south-west. On foot. Level 8: the Mark of Force."},
	{415, "Clues", []string{"snake"}, "An isle in the east: a ship is needed. Level 8: the Mark of Snake."},
	{414, "Time Awaits", []string{"timeLord"}, "Ringed by mountains beside Death Gulch: the moongate at its door is the only way in. Level 8: the Time Lord."},
}

// The cards, by the shrine's column (x & 3) and the bit SEARCH sets.
var cardNames = []string{"Love", "Sol", "Moons", "Death"}

// Word is a word Other answers to that someone says: what it does, and where it is learnt.
type Word struct{ Word, About, Hint string }

// Words are those words in the order Other offers them. SCREAM and PISSOFF
// nobody says, so they are never offered.
var Words = []Word{
	{"SEARCH", "Standing in one of Ambrosia's shrines, finds its card.", "Taught in Yew and West Montor."},
	{"BRIBE", "Pays a guard 100 gold to leave his post.", "Taught in Moon and Death Gulch."},
	{"PRAY", "In the right place, a prayer is answered with a word.", "Taught in Fawn. The place: Yew's circle of light."},
	{"EVOCARE", "Yelled beside the Silver Snake by one with the Mark of Snake: the serpent parts.", "Other, PRAY at the centre of Yew's circle of light."},
	{"INSERT", "In Exodus' castle, sets a card in its slot.", "Taught in East Montor."},
	{"DIG", "Digs where something lies buried.", "Taught in Dawn, which stands only while both moons are new."},
	{"PAXUM", "In a town or castle, calms those nearby who have turned on the party. Pax: peace.", "Learnt in Moon, from one who wishes the party peace."},
}

// JournalLine is a line of the journal.
type JournalLine struct {
	Label string          // its name at the left and where it stands at the right
	Name  string          // its name alone, for its card's title
	About string          // what the party knows of it, for its card
	Hint  string          // printed to the message area when asked for
	Open  *JournalSection // what it opens to, or nil
}

// JournalSection is what a line opens to: more lines, or paragraphs.
type JournalSection struct {
	Title string
	Lines []JournalLine
	Text  []string
}

const (
	journalWidth = 20 // the journal's page width
	mysteryLabel = "Mystery..."
)

// row is name at the left, right at the right, in the page's width.
func row(name, right string) string {
	if right == "" {
		return name[:min(len(name), journalWidth)]
	}
	n := journalWidth - len(right) - 1
	if len(name) > n {
		name = name[:n]
	}
	return name + strings.Repeat(" ", max(0, journalWidth-len(right)-len(name))) + right
}

// mystery is a line not yet known of; its hint is still there for the asking.
func mystery(hint, about string) JournalLine {
	if about == "" {
		about = "Not yet heard of."
	}
	return JournalLine{Label: mysteryLabel, Name: mysteryLabel, About: about, Hint: hint}
}

// holders is the members with marks bit, by name.
func holders(w *World, bit int) []string {
	var out []string
	for _, m := range w.memberIndices() {
		if w.Member(m).Marks()&bit != 0 {
			out = append(out, w.Member(m).Name())
		}
	}
	return out
}

func heldBy(w *World, bit int) string {
	who := holders(w, bit)
	if len(who) == 0 {
		return "Held by no one."
	}
	if len(who) == len(w.memberIndices()) && len(who) > 1 {
		return "Held by all."
	}
	return "Held by " + strings.Join(who, ", ") + "."
}

var markBits = map[string]int{"force": 0x10, "fire": 0x20, "snake": 0x40, "kings": 0x80}

func heard(w *World, id string) bool {
	if id == audience {
		return w.Journal.LordBritish
	}
	return slices.Contains(w.Journal.Clues, id)
}

var (
	toldRe    = regexp.MustCompile(`^(Pub|Radrion):(\d)$`)
	writingRe = regexp.MustCompile(`^(\d+):(\d+)$`)
	spacesRe  = regexp.MustCompile(`\s+`)
)

// clueText is a clue's words as spoken.
func clueText(w *World, id string) string {
	if c, ok := Clues[id]; ok && c.Text != "" {
		return c.Text
	}
	if m := toldRe.FindStringSubmatch(id); m != nil {
		n, _ := strconv.Atoi(m[2])
		if s := w.Resources.Strings[m[1]]; n < len(s) {
			return strings.TrimSpace(spacesRe.ReplaceAllString(s[n], " "))
		}
		return ""
	}
	if m := writingRe.FindStringSubmatch(id); m != nil {
		d, _ := strconv.Atoi(m[1])
		n, _ := strconv.Atoi(m[2])
		if talk, ok := w.Resources.Talk[d]; ok {
			return strings.TrimSpace(strings.ReplaceAll(Speech(talk, n), "\n", " "))
		}
		return ""
	}
	town, n, _ := strings.Cut(id, "#")
	speaker, _ := strconv.Atoi(n)
	return w.TownLine(town, speaker)
}

// clueFrom is who said a clue, for a step's list of them.
func clueFrom(w *World, id string) string {
	if c, ok := Clues[id]; ok && c.From != "" {
		return c.From
	}
	if strings.HasPrefix(id, "Pub:") {
		return "A pub"
	}
	if strings.HasPrefix(id, "Radrion:") {
		return "Radrion"
	}
	if m := writingRe.FindStringSubmatch(id); m != nil {
		d, _ := strconv.Atoi(m[1])
		name := "A dungeon"
		for _, jd := range journalDungeons {
			if jd.id == d {
				name = jd.name
			}
		}
		return name + ", level " + m[2]
	}
	town, _, _ := strings.Cut(id, "#")
	for _, t := range journalTowns {
		if w.Resources.MapNames[t.id] == town {
			return t.name
		}
	}
	return town
}

func heardOf(w *World, test func(Clue) bool) bool {
	for _, id := range w.Journal.Clues {
		if c, ok := Clues[id]; ok && test(c) {
			return true
		}
	}
	return false
}

func reveals(what string) func(Clue) bool {
	return func(c Clue) bool { return slices.Contains(c.Reveals, what) }
}

type stepStatus struct {
	done         bool
	right, about string
	started      bool
}

// stepState is how a main-quest step stands.
func stepState(w *World, id step) stepStatus {
	j := &w.Journal
	members := len(w.memberIndices())
	marks := func(bit int, why string) stepStatus {
		n := len(holders(w, bit))
		right := ""
		if members > 0 {
			right = strconv.Itoa(n) + "/" + strconv.Itoa(members)
		}
		return stepStatus{members > 0 && n == members, right, heldBy(w, bit) + " " + why, n > 0}
	}
	switch id {
	case "king":
		if j.LordBritish {
			return stepStatus{true, "done", "Audience had. He raises a level for any with the experience.", true}
		}
		return stepStatus{false, "", "He awaits in his castle, beside Britain's town.", false}
	case "kings":
		return marks(0x80, "Lord British raises no one past level 5 without it.")
	case "cards":
		union := 0
		for _, m := range w.memberIndices() {
			union |= w.Member(m).Marks() & 0x0f
		}
		var held []string
		for i, c := range cardNames {
			if union&(1<<i) != 0 {
				held = append(held, c)
			}
		}
		about := "None held yet."
		if len(held) > 0 {
			about = "Held: " + strings.Join(held, ", ") + "."
		}
		return stepStatus{len(held) == 4, strconv.Itoa(len(held)) + "/4", about + " All four are set in Exodus' castle.", len(held) > 0}
	case "exotics":
		weapons, armour := w.Party.Weapons(15), w.Party.Armour(7)
		for _, m := range w.memberIndices() {
			if w.Member(m).Bytes[48] == 15 {
				weapons++
			}
			if w.Member(m).Bytes[40] == 7 {
				armour++
			}
		}
		kinds := 0
		if weapons > 0 {
			kinds++
		}
		if armour > 0 {
			kinds++
		}
		return stepStatus{kinds == 2, strconv.Itoa(kinds) + "/2",
			"Exotic weapons " + strconv.Itoa(weapons) + ", armour " + strconv.Itoa(armour) + ", in the pack or in use.", kinds > 0}
	case "fire":
		return marks(0x20, "Each member needs it to cross lava.")
	case "force":
		return marks(0x10, "Each member needs it for the force fields in Exodus' castle.")
	case "snake":
		who := holders(w, 0x40)
		word := slices.Contains(j.Words, "EVOCARE")
		if j.SerpentParted {
			return stepStatus{true, "done", "Passed: the serpent parted at the word.", true}
		}
		mark, wd := "not held", "not known"
		parts := 0
		if len(who) > 0 {
			mark = strings.Join(who, ", ")
			parts++
		}
		if word {
			wd = "EVOCARE"
			parts++
		}
		return stepStatus{false, strconv.Itoa(parts) + "/3", "The mark: " + mark + ". The word: " + wd + ".", parts > 0}
	case "order":
		if j.TimeLord {
			return stepStatus{true, "done", "The Time Lord: LOVE, SOL, MOONS & DEATH, all else fails.", true}
		}
		return stepStatus{false, "", "Four cards, four slots, and one order. Not yet known.", false}
	case "exodus":
		if w.Party.ExodusDestroyed() {
			return stepStatus{true, "done", "Destroyed. Sosaria is saved.", true}
		}
		return stepStatus{false, "", "Exodus waits in its castle, on an isle in the south-west.", false}
	}
	return stepStatus{}
}

// mainQuest is the main quest's lines.
func mainQuest(w *World) []JournalLine {
	var out []JournalLine
	for _, s := range questSteps {
		state := stepState(w, s.id)
		var told []string
		named := false
		for _, id := range w.Journal.Clues {
			if slices.Contains(Clues[id].Quest, s.id) {
				told = append(told, id)
			}
			if slices.Contains(Clues[id].Names, s.id) {
				named = true
			}
		}
		// Known once a line names it, or the party has made a start on it.
		known := s.id == "king" || s.id == "exodus" || state.started || state.done || named
		if !known {
			out = append(out, mystery(s.hint, ""))
			continue
		}
		line := JournalLine{Label: row(s.name, state.right), Name: s.name, About: state.about, Hint: s.hint}
		if len(told) > 0 {
			var text []string
			for _, id := range told {
				text = append(text, clueFrom(w, id)+": "+clueText(w, id))
			}
			line.Open = &JournalSection{Title: s.name, Text: text}
		}
		out = append(out, line)
	}
	return out
}

// seenOnSosaria says whether a place's square on Sosaria has been seen.
func seenOnSosaria(w *World, id int) bool {
	m := w.Resources.Misc
	return w.AutoMap.SquareSeen(MapIDSosaria, int(m.LocationX[id-400]), int(m.LocationY[id-400]))
}

func townCount(w *World, t journalTown) (n, all int) {
	for _, id := range t.clues {
		if heard(w, id) {
			n++
		}
	}
	return n, len(t.clues)
}

func journalTownLine(w *World, t journalTown) JournalLine {
	n, all := townCount(w, t)
	entered := BeenIn(w, t.id) || n > 0
	seen := seenOnSosaria(w, t.id)
	// Dawn stands only while both moons are new: seen is seen standing, now.
	if t.id == jDawn {
		m := w.Resources.Misc
		seen = seen && w.Surface.Tiles[int(m.LocationY[jDawn-400])*64+int(m.LocationX[jDawn-400])] == MapValueTown
	}
	hidden := slices.Contains(hiddenTowns, t.id)
	if hidden && !entered && !seen && !heardOf(w, reveals(strconv.Itoa(t.id))) {
		return mystery(t.hint, "")
	}
	if !entered {
		right, about := "", "Undiscovered."
		switch {
		case seen:
			right, about = "seen", "Found, not yet entered."
		case t.id == jDawn:
			about = "Heard of. It stands only while both moons are new."
		case hidden:
			about = "Heard of, not yet found."
		}
		return JournalLine{Label: row(t.name, right), Name: t.name, About: about, Hint: t.hint}
	}
	var text []string
	for _, id := range t.clues {
		switch {
		case !heard(w, id):
			text = append(text, mysteryLabel)
		case id == audience:
			text = append(text, "An audience with Lord British.")
		case id == "Yew:pray":
			text = append(text, "In the circle of light: "+clueText(w, id))
		default:
			text = append(text, clueText(w, id))
		}
	}
	about := "Entered. " + strconv.Itoa(n) + " of its " + strconv.Itoa(all) + " clues found."
	if n == all {
		about = "Entered. All its clues found."
	}
	return JournalLine{Label: row(t.name, strconv.Itoa(n)+"/"+strconv.Itoa(all)), Name: t.name, About: about, Hint: t.hint,
		Open: &JournalSection{Title: t.name, Text: text}}
}

func goalFound(w *World, dungeon int, goal string) bool {
	if goal == "timeLord" {
		return w.Journal.TimeLord
	}
	return slices.Contains(w.Journal.Goals, strconv.Itoa(dungeon)+":"+goal)
}

func journalDungeonLine(w *World, d journalDungeon) JournalLine {
	var found []string
	for _, g := range d.goals {
		if goalFound(w, d.id, g) {
			found = append(found, g)
		}
	}
	entered := BeenIn(w, d.id) || len(found) > 0
	seen := seenOnSosaria(w, d.id)
	if !entered && !seen && !heardOf(w, reveals(strconv.Itoa(d.id))) {
		return mystery(d.hint, "")
	}
	line := JournalLine{Name: d.name, Hint: d.hint}
	if !entered {
		line.Label, line.About = row(d.name, ""), "Heard of, not yet found."
		if seen {
			line.Label, line.About = row(d.name, "seen"), "Found, not yet entered."
		}
		return line
	}
	var what []string
	for _, g := range found {
		what = append(what, goalNames[g])
	}
	line.About = "Entered. What it holds for the quest is yet to be found."
	if len(found) > 0 {
		line.About = "Entered. Found here: " + strings.Join(what, " and ") + "."
		if len(found) != len(d.goals) {
			line.About += " More lies within."
		}
	}
	line.Label = row(d.name, strconv.Itoa(len(found))+"/"+strconv.Itoa(len(d.goals)))
	return line
}

// shrineSquares is Ambrosia's shrines, one to a card (x & 3), each with its squares.
func shrineSquares(w *World) [4][]Point {
	var out [4][]Point
	for _, p := range Shrines(w) {
		out[p.X&3] = append(out[p.X&3], p)
	}
	return out
}

func ambrosiaLines(w *World) []JournalLine {
	stats := w.Resources.Strings["Messages"]
	var out []JournalLine
	for i, squares := range shrineSquares(w) {
		stat := ""
		if 173+i < len(stats) {
			stat = strings.TrimSpace(stats[173+i])
		}
		name := ""
		if stat != "" {
			name = strings.ToUpper(stat[:1]) + strings.ToLower(stat[1:])
		}
		seen, entered := false, false
		for _, p := range squares {
			seen = seen || w.AutoMap.SquareSeen(MapIDAmbrosia, p.X, p.Y)
			entered = entered || w.AutoMap.Visited("shrine:"+strconv.Itoa(p.X)+","+strconv.Itoa(p.Y))
		}
		who := holders(w, 1<<i)
		hint := "Stand in the shrine: Other, SEARCH for the Card of " + cardNames[i] + ". Offer gold there to raise " + name + "."
		about := "Undiscovered."
		if entered {
			card := "Its card is not yet found."
			if len(who) > 0 {
				card = "The Card of " + cardNames[i] + ": held by " + strings.Join(who, ", ") + "."
			}
			about = "Entered. Gold offered here raises " + name + ". " + card
		} else if seen {
			about = "Found, not yet entered."
		}
		right := ""
		if len(who) > 0 {
			right = cardNames[i]
		} else if seen || entered {
			right = "seen"
		}
		out = append(out, JournalLine{Label: row(name, right), Name: name, About: about, Hint: hint})
	}
	return out
}

const (
	pubsHint    = "Pubs: Britain, Moon, both Montors, Grey, Dawn, Devil Guard and Fawn. Pay 10 gold more for each rumour further on, up to 90."
	oraclesHint = "Radrion's counter: Lord British's castle, Britain and Dawn. Offer nothing for the opening verse, then a hundred gold for each."
)

// rumours is what the pubs and Radrion tell, for gold.
func rumours(w *World) (lines []JournalLine, heardN, all int) {
	told := func(id string) bool { return slices.Contains(w.Journal.Clues, id) }
	said := func(prefix string, from int) []string {
		var out []string
		for n := from; n <= 9; n++ {
			id := prefix + strconv.Itoa(n)
			if told(id) {
				out = append(out, clueText(w, id))
			} else {
				out = append(out, mysteryLabel)
			}
		}
		return out
	}
	pubs, verses := 0, 0
	for n := 1; n <= 9; n++ {
		if told("Pub:" + strconv.Itoa(n)) {
			pubs++
		}
		if told("Radrion:" + strconv.Itoa(n)) {
			verses++
		}
	}
	lines = []JournalLine{{
		Label: row("Pub rumours", strconv.Itoa(pubs)+"/9"), Name: "Pub rumours",
		About: "Every pub tells the same nine, picked by what the drink costs. " + strconv.Itoa(pubs) + " of 9 heard.",
		Hint:  pubsHint, Open: &JournalSection{Title: "Pub rumours", Text: said("Pub:", 1)},
	}}
	if w.Journal.Radrion {
		lines = append(lines, JournalLine{
			Label: row("Radrion", strconv.Itoa(verses)+"/9"), Name: "Radrion",
			About: "The Prophet of Life sells his rhyme a verse at a time, by the hundred gold offered. " + strconv.Itoa(verses) + " of 9 heard.",
			Hint:  oraclesHint, Open: &JournalSection{Title: "Radrion", Text: said("Radrion:", 0)},
		})
		return lines, pubs + verses, 18
	}
	return append(lines, mystery(oraclesHint, "")), pubs, 9
}

const (
	guildsHint    = "Guilds sell keys, torches, powders and gems: in Grey, a walk south-west; Devil Guard and Fawn, on isles; and Dawn."
	dawnShopsHint = "One town's shops sell them: Dawn, in the forest west of the Montors, standing only while both moons are new."
	moongatesHint = "Left moon: which gate stands. Right moon: where it leads. For Time Awaits, step in with the right moon one phase short of new."
)

// equipment is what the party can go and get, and the moongates.
func equipment(w *World) (lines []JournalLine, done int) {
	j := &w.Journal
	line := func(name string, found bool, before, after, hint string) JournalLine {
		if found {
			done++
			return JournalLine{Label: row(name, "done"), Name: name, About: after, Hint: hint}
		}
		return JournalLine{Label: row(name, ""), Name: name, About: before, Hint: hint}
	}
	gates := len(w.GatesKnown)
	if gates == 8 {
		done++
	}
	moongates := mystery(moongatesHint, "")
	if gates > 0 || heardOf(w, reveals("moongates")) {
		moongates = JournalLine{Label: row("Moongates", strconv.Itoa(gates)+"/8"), Name: "Moongates",
			About: "Come out of " + strconv.Itoa(gates) + " of 8. One gate stands at a time, by the left moon; step in, and the right moon picks where you come out.",
			Hint:  moongatesHint}
	}
	lines = []JournalLine{
		line("Supplies", j.Guild, "Find a guild to get adventuring supplies.", "Found a guild: keys, torches, powders and gems.", guildsHint),
		line("Magic Weapons", j.MagicWeapons, "Find the shop that sells magic weapons.", "Found, in Dawn: the +2 and +4 weapons. Ordinary shops stop at the 2-H sword.", dawnShopsHint),
		line("Magic Armour", j.MagicArmour, "Find the shop that sells magic armour.", "Found, in Dawn: +2 chain and +2 plate. Ordinary shops stop at plate.", dawnShopsHint),
		line("Ships", j.Ship, "Acquire a ship...", "Boarded. A ship reaches the isles, and through the whirlpool, Ambrosia.", "No one sells ships. Attack a pirate ship from the shore, then board (B) the frigate it leaves."),
		moongates,
	}
	return lines, done
}

// JournalTop is the top of the journal, each line with its count and opening to its lines.
func JournalTop(w *World) []JournalLine {
	j := &w.Journal
	qs := mainQuest(w)
	stepsDone := 0
	for _, s := range questSteps {
		if stepState(w, s.id).done {
			stepsDone++
		}
	}
	var towns []JournalLine
	townsDone := 0
	for _, t := range journalTowns {
		towns = append(towns, journalTownLine(w, t))
		if n, all := townCount(w, t); n == all {
			townsDone++
		}
	}
	var dungeons []JournalLine
	dungeonsDone := 0
	for _, d := range journalDungeons {
		dungeons = append(dungeons, journalDungeonLine(w, d))
		all := true
		for _, g := range d.goals {
			all = all && goalFound(w, d.id, g)
		}
		if all {
			dungeonsDone++
		}
	}
	toldLines, toldN, toldAll := rumours(w)
	ambrosiaHint := "Sail a frigate into the whirlpool; the ship is lost. Home from Ambrosia is a pirate's ship into the lake's whirlpool."
	ambrosiaKnown := j.Ambrosia || w.Party.Location() == LocationAmbrosia || w.AutoMap.HasMap(MapIDAmbrosia) || heardOf(w, reveals("ambrosia"))
	shrineLines := ambrosiaLines(w)
	cards := 0
	for i := range shrineLines {
		if len(holders(w, 1<<i)) > 0 {
			cards++
		}
	}
	var words []JournalLine
	wordsKnown := 0
	for _, wd := range Words {
		if slices.Contains(j.Words, wd.Word) {
			words = append(words, JournalLine{Label: wd.Word, Name: wd.Word, About: wd.About, Hint: wd.Hint})
			wordsKnown++
		} else {
			words = append(words, mystery(wd.Hint, "Not yet heard said."))
		}
	}
	reached := j.Ambrosia || w.AutoMap.HasMap(MapIDAmbrosia)
	gear, gearDone := equipment(w)
	frac := func(a, b int) string { return strconv.Itoa(a) + "/" + strconv.Itoa(b) }
	ambrosia := mystery(ambrosiaHint, "")
	if ambrosiaKnown {
		about := "The lost land. Heard of, not yet reached."
		if reached {
			about = "The lost land. " + strconv.Itoa(cards) + " of its four shrines' cards held."
		}
		ambrosia = JournalLine{Label: row("Ambrosia", frac(cards, 4)), Name: "Ambrosia", About: about, Hint: ambrosiaHint,
			Open: &JournalSection{Title: "Ambrosia", Lines: shrineLines}}
	}
	return []JournalLine{
		{Label: row("Main quest", frac(stepsDone, len(questSteps))), Name: "Main quest",
			About: "All that stands between you and Exodus. After the king, in any order.",
			Hint:  "Talk to everyone: the townsfolk tell what the quest needs. Each step has its own hint.",
			Open:  &JournalSection{Title: "Main quest", Lines: qs}},
		{Label: row("Towns", frac(townsDone, len(journalTowns))), Name: "Towns",
			About: "Lord British's castle and the ten towns, and the clues to be heard in each.",
			Hint:  "Walk into townsfolk to talk. The isles need a ship: take one from pirates.",
			Open:  &JournalSection{Title: "Towns", Lines: towns}},
		{Label: row("Rumours", frac(toldN, toldAll)), Name: "Rumours",
			About: "What the pubs, and an oracle, will tell for gold.",
			Hint:  "Buy a drink in a pub, or make an offering at an oracle: the more gold, the further the tale goes.",
			Open:  &JournalSection{Title: "Rumours", Lines: toldLines}},
		{Label: row("Dungeons", frac(dungeonsDone, len(journalDungeons))), Name: "Dungeons",
			About: "Seven dungeons, each holding something the quest needs.",
			Hint:  "Bring light: torches, or the spells Lorum and Luminae. What the quest needs is mostly on level 8.",
			Open:  &JournalSection{Title: "Dungeons", Lines: dungeons}},
		ambrosia,
		{Label: row("Equipment", frac(gearDone, len(gear))), Name: "Equipment",
			About: "What the party can go and get, beyond what it sets out with.",
			Hint:  "Each line has its own hint.",
			Open:  &JournalSection{Title: "Equipment", Lines: gear}},
		{Label: row("Words", frac(wordsKnown, len(words))), Name: "Words",
			About: "Words to say with Other (O), learnt from those who say them.",
			Hint:  "Other: choose who speaks, then the word. On a controller, the words learnt are offered.",
			Open:  &JournalSection{Title: "Words", Lines: words}},
	}
}

func textCard(title, about string) *Card {
	c := &Card{Title: title}
	for _, t := range CardWrap(about) {
		c.Lines = append(c.Lines, CardLine{Text: t})
	}
	return c
}

// JournalCard is a line's card: its name, and what the party knows of it.
func JournalCard(line JournalLine) *Card { return textCard(line.Name, line.About) }

// WordCard is a word's card, for Other's list on a controller; nil for no word Other offers.
func WordCard(word string) *Card {
	for _, w := range Words {
		if w.Word == word {
			return textCard(w.Word, w.About)
		}
	}
	return nil
}

// KnownWords is the words learnt, in Other's order: what a controller's Other offers.
func KnownWords(w *World) []string {
	var out []string
	for _, wd := range Words {
		if slices.Contains(w.Journal.Words, wd.Word) {
			out = append(out, wd.Word)
		}
	}
	return out
}

// LearnWord makes one of Words known without its being heard (Cheats' Add
// word): "added", "known" already, or "unknown" for no word Other offers.
func LearnWord(w *World, typed string) string {
	word := strings.ToUpper(strings.TrimSpace(typed))
	if !slices.ContainsFunc(Words, func(x Word) bool { return x.Word == word }) {
		return "unknown"
	}
	if slices.Contains(w.Journal.Words, word) {
		return "known"
	}
	w.Journal.Words = append(w.Journal.Words, word)
	return "added"
}

// HearClue records a clue heard, and the word it teaches; true if it was new.
func HearClue(w *World, id string) bool {
	c, ok := Clues[id]
	if !ok || slices.Contains(w.Journal.Clues, id) {
		return false
	}
	w.Journal.Clues = append(w.Journal.Clues, id)
	if c.Word != "" && !slices.Contains(w.Journal.Words, c.Word) {
		w.Journal.Words = append(w.Journal.Words, c.Word)
	}
	return true
}

// HearTownLine records a townsperson's line; true if it was a new clue.
func HearTownLine(w *World, town string, speaker int) bool {
	return HearClue(w, town+"#"+strconv.Itoa(speaker))
}

// ReadWriting records the misty writing on a dungeon's level (0-based); true if new.
func ReadWriting(w *World, dungeon, level int) bool {
	return HearClue(w, strconv.Itoa(dungeon)+":"+strconv.Itoa(level+1))
}

// RodFound records a mark's rod stood on in a dungeon; its column (x & 3) says which mark.
func RodFound(w *World, dungeon, x int) {
	id := strconv.Itoa(dungeon) + ":" + RodMarks[x&3]
	if !slices.Contains(w.Journal.Goals, id) {
		w.Journal.Goals = append(w.Journal.Goals, id)
	}
}

// JournalCheck runs after anything that may move the journal on: on a
// change it notes it and prints "Journal updated".
func JournalCheck(w *World, io IO) bool {
	if !JournalSnapshot(w) {
		return false
	}
	io.Print("Journal updated\n")
	return true
}

// journalSnapshotList is what the journal shows that comes of what happens, as a list to compare.
func journalSnapshotList(w *World) []string {
	j := &w.Journal
	var out []string
	for _, f := range JournalFlags {
		if *j.Flag(f) {
			out = append(out, f)
		}
	}
	for _, list := range [][]string{j.Clues, j.Words, j.Goals} {
		s := slices.Clone(list)
		slices.Sort(s)
		out = append(out, s...)
	}
	var marks []string
	for _, m := range w.memberIndices() {
		marks = append(marks, strconv.Itoa(w.Member(m).Marks()))
	}
	out = append(out, "marks:"+strings.Join(marks, ","))
	for i, l := range mainQuest(w) {
		state := "?"
		if l.Label != mysteryLabel {
			state = stepState(w, questSteps[i].id).right
		}
		out = append(out, string(questSteps[i].id)+":"+state)
	}
	return out
}

// JournalSnapshot notes what the journal shows as seen; true if it differed from the last note.
func JournalSnapshot(w *World) bool {
	now := journalSnapshotList(w)
	if slices.Equal(now, w.Journal.Seen) {
		return false
	}
	w.Journal.Seen = now
	return true
}

// UpgradeJournal fills in a journal saved before words and goals were
// recorded: the words from the clues heard, each dungeon's goals from the
// marks held whose rod the auto-map has seen. Then it notes what it shows,
// so loading it announces nothing.
func UpgradeJournal(w *World) {
	j := &w.Journal
	for _, id := range j.Clues {
		if word := Clues[id].Word; word != "" && !slices.Contains(j.Words, word) {
			j.Words = append(j.Words, word)
		}
	}
	held := 0
	for _, m := range w.memberIndices() {
		held |= w.Member(m).Marks()
	}
	for _, d := range journalDungeons {
		raw, ok := w.Resources.Maps[d.id]
		if !ok {
			continue
		}
		for level := range 8 {
			for i := range 256 {
				if raw[level*256+i] != rod {
					continue
				}
				x, y := i&15, i>>4
				if held&markBits[RodMarks[x&3]] != 0 && w.AutoMap.IsSeen(d.id, level, x, y) {
					RodFound(w, d.id, x)
				}
			}
		}
	}
	JournalSnapshot(w)
}
