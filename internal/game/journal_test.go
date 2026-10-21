package game

import (
	"encoding/json"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// inTown is a world standing in a town map on an empty floor with no townsfolk.
func inTown(t *testing.T, id int) (*World, *FakeIO) {
	w := newWorld(t, 99)
	locX := int(w.Resources.Misc.LocationX[id-400])
	w.ReturnX = locX
	w.Party.SetSurfaceX(locX)
	w.ReturnY = 19
	w.Party.SetSurfaceY(19)
	w.EnterMap(id)
	if id == MapIDLordBritishCastle {
		w.Party.SetLocation(LocationCastle)
	} else {
		w.Party.SetLocation(LocationTown)
	}
	for i := range w.Current.Tiles {
		w.Current.Tiles[i] = MapValueFloor
	}
	clear(w.Monsters().Bytes)
	w.X, w.Y = 20, 20
	return w, newFakeIO(w.Resources)
}

// personEast puts a townsperson with talk index n east of the party.
func personEast(w *World, n int) {
	m := w.Monsters()
	m.SetType(0, MapValueMerchant)
	m.SetTileUnder(0, MapValueFloor)
	m.SetPosition(0, 21, 20)
	m.SetHP(0, n) // the low nibble of the hp byte is the talk index
}

func section(t *testing.T, w *World, name string) JournalLine {
	t.Helper()
	for _, l := range JournalTop(w) {
		if l.Name == name {
			return l
		}
	}
	t.Fatalf("no %s", name)
	return JournalLine{}
}

func lineOf(t *testing.T, w *World, sec, name string) JournalLine {
	t.Helper()
	s := section(t, w, sec)
	if s.Open != nil {
		for _, l := range s.Open.Lines {
			if l.Name == name {
				return l
			}
		}
	}
	t.Fatalf("no %s in %s", name, sec)
	return JournalLine{}
}

func labels(t *testing.T, w *World, name string) []string {
	var out []string
	for _, l := range section(t, w, name).Open.Lines {
		out = append(out, strings.TrimSpace(l.Label))
	}
	return out
}

func everyLine(w *World) []JournalLine {
	var out []JournalLine
	var walk func([]JournalLine)
	walk = func(lines []JournalLine) {
		for _, l := range lines {
			out = append(out, l)
			if l.Open != nil {
				walk(l.Open.Lines)
			}
		}
	}
	walk(JournalTop(w))
	return out
}

func eq[T any](t *testing.T, got, want T) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v\nwant %#v", got, want)
	}
}

func openText(l JournalLine) []string {
	if l.Open == nil {
		return nil
	}
	return l.Open.Text
}

func mysteries(n int) []string { return slices.Repeat([]string{mysteryLabel}, n) }

func TestJournalOpens(t *testing.T) {
	w := newWorld(t)
	var top []string
	for _, l := range JournalTop(w) {
		top = append(top, l.Label)
	}
	eq(t, top, []string{"Main quest       0/9", "Towns           0/11", "Rumours          0/9", "Dungeons         0/7",
		"Mystery...", "Equipment        0/5", "Words            0/7"})
	eq(t, labels(t, w, "Main quest"), append(append([]string{"Lord British"}, mysteries(7)...), "Exodus"))
	eq(t, labels(t, w, "Words"), mysteries(7))
}

func TestJournalClueMakesStepKnown(t *testing.T) {
	w, io := inTown(t, MapIDFirstTown)
	personEast(w, 3)
	TalkTo(w, io, 0, 0)
	eq(t, w.Journal.Clues, []string{"LCB Towne#3"})
	if !strings.Contains(io.out(), "Journal updated") {
		t.Fatal("no update")
	}
	ex := lineOf(t, w, "Main quest", "Exotic arms")
	eq(t, ex.Label, "Exotic arms      0/2")
	eq(t, openText(ex), []string{"Britain: " + w.TownLine("LCB Towne", 3)})
	io.Output.Reset()
	personEast(w, 1)
	TalkTo(w, io, 0, 0)
	personEast(w, 3)
	TalkTo(w, io, 0, 0)
	eq(t, w.Journal.Clues, []string{"LCB Towne#3"})
	if strings.Contains(io.out(), "Journal updated") {
		t.Fatal("updated again")
	}
}

func TestJournalTownCountsClues(t *testing.T) {
	w, io := inTown(t, MapIDFirstTown)
	eq(t, lineOf(t, w, "Towns", "Britain").About, "Undiscovered.")
	m := w.Resources.Misc
	w.AutoMap.MarkSquares(MapIDSosaria, []Point{{int(m.LocationX[2]), int(m.LocationY[2])}})
	eq(t, lineOf(t, w, "Towns", "Britain").Label, "Britain         seen")
	w.AutoMap.MarkSquares(MapIDFirstTown, []Point{{20, 20}})
	eq(t, lineOf(t, w, "Towns", "Britain").Label, "Britain          0/4")
	personEast(w, 2)
	TalkTo(w, io, 0, 0)
	b := lineOf(t, w, "Towns", "Britain")
	eq(t, b.Label, "Britain          1/4")
	eq(t, b.About, "Entered. 1 of its 4 clues found.")
	eq(t, openText(b), []string{w.TownLine("LCB Towne", 2), mysteryLabel, mysteryLabel, mysteryLabel})
	eq(t, section(t, w, "Ambrosia").Label, "Ambrosia         0/4")
}

func lordBritishEast(w *World) {
	m := w.Monsters()
	m.SetType(0, MapValueLordBritish)
	m.SetTileUnder(0, MapValueFloor)
	m.SetPosition(0, 21, 20)
}

func TestJournalAudience(t *testing.T) {
	w, io := inTown(t, MapIDLordBritishCastle)
	lordBritishEast(w)
	w.PutXYVal(MapValueLordBritish, 21, 20)
	io.Keys = []string{"1"}
	TransactToward(w, io, 1, 0)
	if !w.Journal.LordBritish || !strings.Contains(io.out(), "Journal updated") {
		t.Fatal("audience")
	}
	eq(t, lineOf(t, w, "Main quest", "Lord British").Label, "Lord British    done")
	eq(t, lineOf(t, w, "Towns", "Lord British").Label, "Lord British     1/3")
	io.Output.Reset()
	io.Keys = []string{"1"}
	TransactToward(w, io, 1, 0)
	if strings.Contains(io.out(), "Journal updated") {
		t.Fatal("announced twice")
	}
}

func TestJournalSeekMarkOfKings(t *testing.T) {
	w, io := inTown(t, MapIDLordBritishCastle)
	lordBritishEast(w)
	p := w.Member(0)
	p.Bytes[30], p.Bytes[28], p.Bytes[29] = 6, 2, 0x58 // level 7, 600 max hit points
	io.Keys = []string{"1"}
	TransactToward(w, io, 1, 0)
	if !strings.Contains(io.out(), "SEEK YE") {
		t.Fatal(io.out())
	}
	k := lineOf(t, w, "Main quest", "Mark of Kings")
	eq(t, k.Label, "Mark of Kings    0/4")
	eq(t, openText(k), []string{"Lord British: SEEK YE, THE MARK OF KINGS!"})
}

func TestJournalStepKnownOnceNamed(t *testing.T) {
	w := newWorld(t)
	HearClue(w, "Moon#2")
	HearClue(w, "Yew#1")
	eq(t, labels(t, w, "Main quest")[2], mysteryLabel)
	HearClue(w, "East Montor#5")
	eq(t, openText(lineOf(t, w, "Main quest", "The four cards")), []string{
		"Moon: " + w.TownLine("Moon", 2), "Yew: " + w.TownLine("Yew", 1), "East Montor: " + w.TownLine("East Montor", 5)})
	for id, c := range Clues {
		for _, s := range c.Names {
			if !slices.Contains(c.Quest, s) {
				t.Errorf("%s names %s but does not bear on it", id, s)
			}
		}
	}
}

func TestJournalMarksCardsExotics(t *testing.T) {
	w := newWorld(t)
	w.Member(1).Bytes[14] |= 0x80
	k := lineOf(t, w, "Main quest", "Mark of Kings")
	eq(t, k.Label, "Mark of Kings    1/4")
	if !strings.Contains(k.About, "Held by "+w.Member(1).Name()+".") {
		t.Fatal(k.About)
	}
	for _, m := range []int{0, 2, 3} {
		w.Member(m).Bytes[14] |= 0x80
	}
	k = lineOf(t, w, "Main quest", "Mark of Kings")
	eq(t, k.Label, "Mark of Kings    4/4")
	if !strings.Contains(k.About, "Held by all.") {
		t.Fatal(k.About)
	}
	w.Member(0).Bytes[14] |= 0x01
	w.Member(2).Bytes[14] |= 0x04
	c := lineOf(t, w, "Main quest", "The four cards")
	eq(t, c.Label, "The four cards   2/4")
	if !strings.Contains(c.About, "Held: Love, Moons.") {
		t.Fatal(c.About)
	}
	w.AddGear(true, 15, 1)
	eq(t, lineOf(t, w, "Main quest", "Exotic arms").About, "Exotic weapons 1, armour 0, in the pack or in use.")
	w.AddGear(false, 7, 1)
	eq(t, lineOf(t, w, "Main quest", "Exotic arms").Label, "Exotic arms      2/2")
	eq(t, section(t, w, "Main quest").Label, "Main quest       2/9")
}

func TestJournalPrayInYew(t *testing.T) {
	w, io := inTown(t, 404)
	w.X, w.Y = 0x30, 0x30
	io.Keys, io.Inputs = []string{"1"}, []string{"PRAY"}
	OtherCommand(w, io, false)
	if !strings.Contains(io.out(), "EVOCARE") {
		t.Fatal(io.out())
	}
	eq(t, w.Journal.Words, []string{"EVOCARE"})
	eq(t, lineOf(t, w, "Towns", "Yew").Label, "Yew              1/3")
	eq(t, lineOf(t, w, "Main quest", "Silver Snake").About, "The mark: not held. The word: EVOCARE.")
	if !strings.Contains(lineOf(t, w, "Words", "EVOCARE").About, "Silver Snake") {
		t.Fatal("word about")
	}
	w.Journal = EmptyJournal()
	w.X = 20
	io.Keys, io.Inputs = []string{"1"}, []string{"PRAY"}
	OtherCommand(w, io, false)
	eq(t, w.Journal.Words, []string{})
}

func TestJournalWordsLearnt(t *testing.T) {
	w, io := inTown(t, 408)
	personEast(w, 2)
	TalkTo(w, io, 0, 0)
	eq(t, KnownWords(w), []string{"DIG"})
	HearClue(w, "Moon#3")
	eq(t, KnownWords(w), []string{"BRIBE", "DIG"})
	eq(t, labels(t, w, "Words"), []string{mysteryLabel, "BRIBE", mysteryLabel, mysteryLabel, mysteryLabel, "DIG", mysteryLabel})
	io.Keys, io.Inputs = []string{"1"}, []string{"SCREAM"}
	OtherCommand(w, io, false)
	eq(t, io.WordsOffered, []string{"BRIBE", "DIG"})
	if !strings.Contains(io.out(), "AIEEEEE") {
		t.Fatal(io.out())
	}
}

func TestJournalPaxum(t *testing.T) {
	w, io := inTown(t, 403)
	w.AutoMap.MarkSquares(403, []Point{{20, 20}})
	JournalCheck(w, io)
	io.Output.Reset()
	personEast(w, 5)
	TalkTo(w, io, 0, 0)
	if !strings.Contains(strings.Join(strings.Fields(io.out()), " "), "PEACE BE WITH YOU") {
		t.Fatal(io.out())
	}
	eq(t, KnownWords(w), []string{"PAXUM"})
	eq(t, lineOf(t, w, "Towns", "Moon").Label, "Moon             0/4")
	if !strings.Contains(lineOf(t, w, "Words", "PAXUM").About, "Pax: peace.") || !strings.Contains(io.out(), "Journal updated") {
		t.Fatal("paxum")
	}
}

func TestJournalWritingInClues(t *testing.T) {
	w := newWorld(t)
	if !ReadWriting(w, 415, 1) || ReadWriting(w, 415, 1) || ReadWriting(w, 415, 0) {
		t.Fatal("writing")
	}
	ReadWriting(w, 415, 2)
	eq(t, KnownWords(w), []string{"SEARCH", "INSERT"})
	eq(t, openText(lineOf(t, w, "Main quest", "The four cards")), []string{
		"Clues, level 2: <INSERT> CARDS INTO EXODUS!", "Clues, level 3: <SEARCH> THE SHRINES!"})
	eq(t, labels(t, w, "Main quest")[7], mysteryLabel)
}

func TestJournalDungeons(t *testing.T) {
	w := newWorld(t)
	m := w.Resources.Misc
	eq(t, labels(t, w, "Dungeons"), mysteries(7))
	HearClue(w, "Devil Guard#6")
	eq(t, lineOf(t, w, "Dungeons", "Fires of Hell").About, "Heard of, not yet found.")
	w.AutoMap.MarkSquares(MapIDSosaria, []Point{{int(m.LocationX[16]), int(m.LocationY[16])}})
	eq(t, lineOf(t, w, "Dungeons", "Perinian Depths").Label, "Perinian Depths seen")
	w.AutoMap.Mark(416, 0, 1, 1)
	d := lineOf(t, w, "Dungeons", "Perinian Depths")
	eq(t, d.Label, "Perinian Depths  0/2")
	eq(t, d.About, "Entered. What it holds for the quest is yet to be found.")
	RodFound(w, 416, 15)
	d = lineOf(t, w, "Dungeons", "Perinian Depths")
	eq(t, d.Label, "Perinian Depths  1/2")
	eq(t, d.About, "Entered. Found here: the Mark of Kings. More lies within.")
	RodFound(w, 416, 5)
	eq(t, lineOf(t, w, "Dungeons", "Perinian Depths").Label, "Perinian Depths  2/2")
	eq(t, section(t, w, "Dungeons").Label, "Dungeons         1/7")
	w.Journal.TimeLord = true
	eq(t, lineOf(t, w, "Dungeons", "Time Awaits").Label, "Time Awaits      1/1")
}

func TestJournalDawnMystery(t *testing.T) {
	w := newWorld(t)
	if !slices.Contains(labels(t, w, "Towns"), mysteryLabel) {
		t.Fatal("no mystery")
	}
	HearClue(w, "Lord British Castle#1")
	eq(t, lineOf(t, w, "Towns", "Dawn").About, "Heard of. It stands only while both moons are new.")
}

func TestJournalIsleTowns(t *testing.T) {
	w := newWorld(t)
	l := labels(t, w, "Towns")
	eq(t, l[len(l)-3:], mysteries(3))
	m := w.Resources.Misc
	w.AutoMap.MarkSquares(MapIDSosaria, []Point{{int(m.LocationX[10]), int(m.LocationY[10])}})
	eq(t, lineOf(t, w, "Towns", "Fawn").Label, "Fawn            seen")
	HearClue(w, "Radrion:3")
	eq(t, lineOf(t, w, "Towns", "Devil Guard").About, "Heard of, not yet found.")
	l = labels(t, w, "Towns")
	eq(t, l[len(l)-1], mysteryLabel)
	w.AutoMap.MarkSquares(411, []Point{{20, 20}})
	eq(t, lineOf(t, w, "Towns", "Death Gulch").Label, "Death Gulch      0/2")
}

func TestJournalTownCluesAreSaid(t *testing.T) {
	w := newWorld(t)
	for id := range Clues {
		town, n, ok := strings.Cut(id, "#")
		if !ok {
			continue
		}
		mapID := -1
		for k, name := range w.Resources.MapNames {
			if name == town {
				mapID = k
			}
		}
		if mapID < 0 {
			t.Fatalf("%s: no map", id)
		}
		w.EnterMap(mapID)
		mon := w.Monsters()
		num, _ := strconv.Atoi(n)
		found := false
		for i := range 32 {
			if mon.Type(i) != 0 && mon.Type(i) != MapValueLordBritish && mon.TalkIndex(i) == num {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: nobody says it", id)
		}
	}
}

func TestJournalEquipment(t *testing.T) {
	w, io := inTown(t, 407)
	eq(t, labels(t, w, "Equipment"), []string{"Supplies", "Magic Weapons", "Magic Armour", "Ships", mysteryLabel})
	eq(t, lineOf(t, w, "Equipment", "Supplies").About, "Find a guild to get adventuring supplies.")
	io.Keys = []string{"N"}
	Shop(w, io, 5, 0)
	eq(t, lineOf(t, w, "Equipment", "Supplies").Label, "Supplies        done")
	io.Keys = []string{"N", "B", KeyEscape}
	Shop(w, io, 3, 0)
	if w.Journal.MagicWeapons {
		t.Fatal("Grey sells magic")
	}
	w.Party.SetSurfaceX(37) // Dawn
	io.Keys = []string{"N", "B", KeyEscape}
	Shop(w, io, 3, 0)
	io.Keys = []string{"N", "B", KeyEscape}
	Shop(w, io, 4, 0)
	eq(t, labels(t, w, "Equipment"), []string{"Supplies        done", "Magic Weapons   done", "Magic Armour    done", "Ships", mysteryLabel})
	eq(t, section(t, w, "Equipment").Label, "Equipment        3/5")
}

func TestJournalBoardShip(t *testing.T) {
	w := newWorld(t)
	io := newFakeIO(w.Resources)
	JournalCheck(w, io)
	w.PutXYVal(MapValueFrigate, w.X, w.Y)
	Board(w, io)
	eq(t, lineOf(t, w, "Equipment", "Ships").Label, "Ships           done")
	if !strings.Contains(io.out(), "Journal updated") {
		t.Fatal(io.out())
	}
}

func TestJournalMoongates(t *testing.T) {
	w := newWorld(t)
	l := labels(t, w, "Equipment")
	eq(t, l[len(l)-1], mysteryLabel)
	HearClue(w, "Pub:3")
	eq(t, lineOf(t, w, "Equipment", "Moongates").Label, "Moongates        0/8")
	w.GatesKnown = append(w.GatesKnown, 7)
	g := lineOf(t, w, "Equipment", "Moongates")
	eq(t, g.Label, "Moongates        1/8")
	if !strings.Contains(g.About, "Come out of 1 of 8.") || !strings.Contains(g.Hint, "Time Awaits") {
		t.Fatal(g)
	}
}

func TestJournalPubAndRadrion(t *testing.T) {
	w, io := inTown(t, MapIDFirstTown)
	w.Party.SetGold(1000)
	io.Inputs, io.Keys = []string{"90"}, []string{"N"}
	Shop(w, io, 0, 0)
	eq(t, openText(lineOf(t, w, "Main quest", "Cards' order")), []string{"A pub: Seek ye out the Lord of Time, and the one way is a sure find!"})
	io.Keys = []string{"3", "N"}
	Shop(w, io, 6, 0)
	if !slices.Contains(w.Journal.Clues, "Radrion:3") {
		t.Fatal("verse 3")
	}
	eq(t, lineOf(t, w, "Towns", "Devil Guard").About, "Heard of, not yet found.")
	eq(t, labels(t, w, "Main quest")[4], mysteryLabel)
	io.Keys = []string{"2", "N"}
	Shop(w, io, 6, 0)
	eq(t, openText(lineOf(t, w, "Main quest", "Mark of Fire")), []string{
		"Radrion: Learn their use in Devil Guard, or death you'll surely bring.",
		"Radrion: Of marks I say there are but 4 of fire, force, snake & king."})
}

func TestJournalRumours(t *testing.T) {
	w, io := inTown(t, MapIDFirstTown)
	w.Party.SetGold(1000)
	eq(t, labels(t, w, "Rumours"), []string{"Pub rumours      0/9", mysteryLabel})
	io.Inputs, io.Keys = []string{"20"}, []string{"N"}
	Shop(w, io, 0, 0)
	p := lineOf(t, w, "Rumours", "Pub rumours")
	eq(t, p.Label, "Pub rumours      1/9")
	eq(t, openText(p)[:3], []string{mysteryLabel, "Dawn, the city of myths & magic!", mysteryLabel})
	io.Keys = []string{"0", "N"}
	Shop(w, io, 6, 0)
	r := lineOf(t, w, "Rumours", "Radrion")
	eq(t, r.Label, "Radrion          0/9")
	eq(t, openText(r)[0], "And so the sage said unto thee 'If thou can solve my rhyme,")
	eq(t, section(t, w, "Rumours").Label, "Rumours         1/18")
}

func TestJournalShrines(t *testing.T) {
	w := newWorld(t)
	w.Journal.Ambrosia = true
	eq(t, labels(t, w, "Ambrosia"), []string{"Strength", "Dexterity", "Intelligence", "Wisdom"})
	w.Member(3).Bytes[14] |= 0x08
	eq(t, lineOf(t, w, "Ambrosia", "Wisdom").Label, "Wisdom         Death")
	eq(t, section(t, w, "Ambrosia").Label, "Ambrosia         1/4")
}

func TestJournalCheckOnlyOnChange(t *testing.T) {
	w := newWorld(t)
	io := newFakeIO(w.Resources)
	JournalCheck(w, io)
	io.Output.Reset()
	if JournalCheck(w, io) {
		t.Fatal("unchanged")
	}
	w.AutoMap.MarkSquares(MapIDSosaria, []Point{{46, 19}})
	if JournalCheck(w, io) {
		t.Fatal("a place seen")
	}
	HearClue(w, "TimeLord")
	if !JournalCheck(w, io) {
		t.Fatal("clue")
	}
	w.Member(0).Bytes[14] |= 0x20
	if !JournalCheck(w, io) {
		t.Fatal("mark")
	}
	eq(t, strings.Count(io.out(), "Journal updated"), 2)
}

func TestJournalEveryLineFits(t *testing.T) {
	w := newWorld(t)
	for id := range Clues {
		if town, n, ok := strings.Cut(id, "#"); ok {
			num, _ := strconv.Atoi(n)
			if w.TownLine(town, num) == "" {
				t.Errorf("%s: no line", id)
			}
		}
	}
	for id := range Clues {
		HearClue(w, id)
	}
	j := &w.Journal
	j.LordBritish, j.Ambrosia, j.TimeLord = true, true, true
	j.Guild, j.MagicWeapons, j.MagicArmour, j.Ship, j.Radrion = true, true, true, true, true
	for id := 412; id <= 418; id++ {
		w.AutoMap.Mark(id, 0, 1, 1)
	}
	for m := range 4 {
		w.Member(m).Bytes[14] = 0xff
	}
	lines := everyLine(w)
	var all []string
	for _, wd := range Words {
		all = append(all, wd.Word)
	}
	eq(t, KnownWords(w), all)
	for _, l := range lines {
		if l.Label == mysteryLabel {
			t.Errorf("a mystery: %s", l.Hint)
		}
		if len(l.Label) > 20 {
			t.Errorf("label too long: %q", l.Label)
		}
		if len(JournalCard(l).Lines) > CardRows {
			t.Errorf("card too long: %s", l.Name)
		}
	}
	// ponytail: the hint's fit in the message area (ui/menus wrapText) is the UI's test.
}

func TestJournalSaved(t *testing.T) {
	a := newWorld(t)
	a.Journal.LordBritish = true
	HearClue(a, "Moon#3")
	RodFound(a, 416, 15)
	JournalCheck(a, newFakeIO(a.Resources))
	data := savedCopy(t, a)
	eq(t, data.Version, SaveVersion)
	b := NewWorld(a.Resources, 0)
	if !Restore(b, data) {
		t.Fatal("restore")
	}
	eq(t, b.Journal, a.Journal)

	old := data
	old.Version, old.Journal = 4, nil
	c := NewWorld(a.Resources, 0)
	if !Restore(c, old) {
		t.Fatal("old")
	}
	eq(t, c.Journal, EmptyJournal())

	odd := data
	odd.Journal = json.RawMessage(`{"lordBritish":"yes","clues":[1,"Moon#6"],"words":[],"seen":null}`)
	d := NewWorld(a.Resources, 0)
	if !Restore(d, odd) {
		t.Fatal("odd")
	}
	want := EmptyJournal()
	want.Clues = []string{"Moon#6"}
	eq(t, d.Journal, want)
}

func TestJournalUpgraded(t *testing.T) {
	a := newWorld(t)
	a.Member(0).Bytes[14] |= 0x80
	data := savedCopy(t, a)
	data.Journal = json.RawMessage(`{"lordBritish":true,"ambrosia":false,"timeLord":false,"wordKnown":true,"serpentParted":false,"clues":["Yew:pray","Dawn#2"],"seen":["king:done:0","kings:open:0"]}`)
	b := NewWorld(a.Resources, 0)
	b.AutoMap.Mark(416, 0, 15, 11)
	b.AutoMap.Mark(413, 7, 5, 14)
	if !Restore(b, data) {
		t.Fatal("restore")
	}
	words := slices.Clone(b.Journal.Words)
	sort.Strings(words)
	eq(t, words, []string{"DIG", "EVOCARE"})
	eq(t, b.Journal.Goals, []string{"416:kings"})
	if JournalCheck(b, newFakeIO(b.Resources)) {
		t.Fatal("announced")
	}
}

func TestJournalNewGameAfresh(t *testing.T) {
	w := newWorld(t)
	w.Journal.LordBritish = true
	w.NewGame()
	eq(t, w.Journal, EmptyJournal())
}
