package game

import (
	"strings"
	"testing"
)

func TestParseAt(t *testing.T) {
	cases := map[string]*AtStart{
		"world:44,18":     {Kind: "world", X: 44, Y: 18},
		"ambrosia":        {Kind: "ambrosia", X: 32, Y: 54},
		"ambrosia:11,6":   {Kind: "ambrosia", X: 11, Y: 6},
		"place:yew":       {Kind: "place", Name: "yew"},
		"fight:Man-O-War": {Kind: "fight", Name: "Man-O-War"},
	}
	for in, want := range cases {
		if got := ParseAt(in); got == nil || *got != *want {
			t.Fatal(in, got)
		}
	}
	for _, junk := range []string{"", "town:3", "world:1"} {
		if ParseAt(junk) != nil {
			t.Fatal(junk)
		}
	}
}

func TestPickName(t *testing.T) {
	if Slug("Dungeon DardinÕs Pit") != "dungeondardinspit" {
		t.Fatal(Slug("Dungeon DardinÕs Pit"))
	}
	places, fights := StartNames(newWorld(t))
	if PickName(places, "lcb") != "LCB Towne" || PickName(places, "perinian") != "Dungeon Perinian Depths" ||
		PickName(places, "dardinspit") != "Dungeon DardinÕs Pit" || PickName(places, "m") != "" {
		t.Fatal("places")
	}
	for _, n := range places {
		if PickName(places, Slug(strings.TrimPrefix(n, "Dungeon "))) != n {
			t.Fatal(n)
		}
	}
	for _, n := range fights {
		if PickName(fights, Slug(n)) != n {
			t.Fatal(n)
		}
	}
	sets := []string{"Standard", "Apple II", "Commodore 64"}
	if TileSetAsked("commodore64", sets) != "Commodore 64" || TileSetAsked("pc", sets) != "" || TileSetAsked("", sets) != "" {
		t.Fatal("tile sets")
	}
}

func TestStartAtPlaces(t *testing.T) {
	for _, c := range []struct {
		name    string
		entered Entered
		id      int
	}{{"yew", EnteredTown, 404}, {"dawn", EnteredTown, 408}, {"lordbritish", EnteredCastle, 400}, {"perinian", EnteredDungeon, 416}} {
		w := newWorld(t)
		io := newFakeIO(w.Resources)
		if key, err := StartAt(w, AtStart{Kind: "place", Name: c.name}); key != "E" || err != nil {
			t.Fatal(c.name, key, err)
		}
		if got := Enter(w, io); got != c.entered || w.Current.ID != c.id {
			t.Fatal(c.name, got, w.Current.ID)
		}
	}
}

func TestStartAtFight(t *testing.T) {
	w := newWorld(t)
	clear(w.Monsters().Bytes)
	if key, err := StartAt(w, AtStart{Kind: "fight", Name: "orc"}); key != KeyRight || err != nil {
		t.Fatal(key, err)
	}
	b := Move(w, newFakeIO(w.Resources), MoveEast)
	if b == nil || b.Kind != BumpMonster || b.DX != 1 || b.DY != 0 {
		t.Fatalf("%+v", b)
	}
	if w.GetXYVal(w.X+1, w.Y) != MapValueOrc {
		t.Fatal("no orc")
	}
}

func TestStartAtAmbrosiaAndErrors(t *testing.T) {
	w := newWorld(t)
	if key, err := StartAt(w, AtStart{Kind: "ambrosia", X: 32, Y: 54}); key != "" || err != nil {
		t.Fatal(key, err)
	}
	if w.Party.Location() != LocationAmbrosia || w.X != 32 || w.Y != 54 {
		t.Fatal("not on Ambrosia")
	}
	if _, err := StartAt(newWorld(t), AtStart{Kind: "place", Name: "atlantis"}); err == nil || !strings.Contains(err.Error(), `no place "atlantis"`) {
		t.Fatal(err)
	}
	if _, err := StartAt(newWorld(t), AtStart{Kind: "fight", Name: "gazer"}); err == nil || !strings.Contains(err.Error(), `no monster "gazer"`) {
		t.Fatal(err)
	}
}
