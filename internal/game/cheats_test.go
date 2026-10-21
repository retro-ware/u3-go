package game

import (
	"slices"
	"strings"
	"testing"
)

func cheat(key string) Cheat {
	return Cheats[slices.IndexFunc(Cheats, func(c Cheat) bool { return c.Key == key })]
}

func TestCheatsRestoreAndSupplies(t *testing.T) {
	w := newWorld(t)
	io := newFakeIO(w.Resources)
	w.Member(0).SetStatus('D')
	w.Member(1).SetStatus('P')
	w.Member(1).SetHitPoints(3)
	cheat("R").Apply(w, io)
	if w.Member(0).Status() != 'G' || w.Member(1).Status() != 'G' || w.Member(1).HitPoints() != w.Member(1).MaxHitPoints() {
		t.Fatal("restore")
	}
	gold, food := w.Party.Gold(), w.Party.Food()
	for _, k := range []string{"G", "F", "T", "M", "K"} {
		cheat(k).Apply(w, io)
	}
	if w.Party.Gold() != gold+100 || w.Party.Food() != food+100 || w.Party.Torches() != 5 || w.Party.Gems() != 10 || w.Party.Keys() != 5 {
		t.Fatal("supplies")
	}
}

func TestCheatAddWord(t *testing.T) {
	w := newWorld(t)
	io := newFakeIO(w.Resources)
	try := func(input, want string) {
		t.Helper()
		io.Inputs = []string{input}
		if got := cheat("W").Apply(w, io); got != want {
			t.Fatal(got)
		}
	}
	try("evocare", "Known now: EVOCARE. Other offers it from now on.")
	if !slices.Equal(KnownWords(w), []string{"EVOCARE"}) || !strings.Contains(io.out(), "Journal updated") {
		t.Fatal(KnownWords(w))
	}
	try("EVOCARE", "EVOCARE is known already.")
	try("SCREAM", "Other offers no word SCREAM.")
	try("", "No word added.")
	if !slices.Equal(KnownWords(w), []string{"EVOCARE"}) {
		t.Fatal(KnownWords(w))
	}
}

func TestCheatRaiseLevels(t *testing.T) {
	w := newWorld(t)
	io := newFakeIO(w.Resources)
	p := w.Member(0)
	level, max := p.Level(), p.MaxHitPoints()
	p.SetHitPoints(5)
	cheat("L").Apply(w, io)
	if p.Level() != level+1 || p.MaxHitPoints() != max+100 || p.HitPoints() != max+100 || w.Member(3).MaxHitPoints() != max+100 {
		t.Fatal("levels")
	}
}

func TestCheatGoHomeAndExitDungeon(t *testing.T) {
	w := newWorld(t)
	io := newFakeIO(w.Resources)
	w.ReturnX, w.ReturnY = 10, 11
	w.EnterMap(MapIDFirstTown)
	w.Party.SetLocation(LocationTown)
	cheat("H").Apply(w, io)
	if w.Party.Location() != LocationSosaria || w.X != 42 || w.Y != 20 {
		t.Fatal("home", w.X, w.Y)
	}
	w.EnterDungeon(MapIDFirstDungeon)
	w.Party.SetLocation(LocationDungeon)
	w.Dungeon.Exit = false
	if !cheat("X").Available(w) {
		t.Fatal("unavailable")
	}
	cheat("X").Apply(w, io)
	if !w.Dungeon.Exit {
		t.Fatal("no exit")
	}
	w.Party.SetLocation(LocationSosaria)
	if cheat("X").Available(w) {
		t.Fatal("available on surface")
	}
}
