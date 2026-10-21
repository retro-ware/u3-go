package game

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHoldCombatMarkGivesThePauseBack(t *testing.T) {
	c := &CombatState{MarkedAt: 1000, MarkedFor: 10000}
	now := 4000.0 // 3 s of the turn gone
	release := HoldCombatMark(c, func() float64 { return now })
	if c.MarkedFrozenAt != 4000 {
		t.Fatal(c.MarkedFrozenAt)
	}
	now = 24000 // 20 s paused
	release()
	if c.MarkedFrozenAt != 0 || now-c.MarkedAt != 3000 {
		t.Fatal(c.MarkedFrozenAt, now-c.MarkedAt)
	}
}

func TestHoldCombatMarkLeavesOthersAlone(t *testing.T) {
	five := func() float64 { return 5000 }
	steady := &CombatState{MarkedAt: 1000}
	HoldCombatMark(steady, five)()
	if steady.MarkedAt != 1000 || steady.MarkedFor != 0 || steady.MarkedFrozenAt != 0 {
		t.Fatal("steady changed")
	}
	frozen := &CombatState{MarkedAt: 1000, MarkedFor: 10000, MarkedFrozenAt: 2000}
	HoldCombatMark(frozen, five)()
	if frozen.MarkedFrozenAt != 2000 || frozen.MarkedAt != 1000 {
		t.Fatal("frozen changed")
	}
	HoldCombatMark(nil, func() float64 { return 0 })() // out of combat: nothing
}

// playField runs the field loop on keys then Q, reporting pauses and moves spent.
func playField(t *testing.T, keys ...string) (paused, moves int) {
	w := newWorld(t)
	io := newFakeIO(w.Resources)
	io.Keys = append(keys, "Q")
	before := w.Party.Moves()
	NewGame(w, io, GameOptions{}).Run()
	return io.Paused, w.Party.Moves() - before
}

func TestPauseFromTheFieldSpendsNoTurn(t *testing.T) {
	for _, k := range []string{KeyEscape, KeyPause} {
		if p, m := playField(t, k); p != 1 || m != 0 {
			t.Fatalf("%q: paused %d moves %d", k, p, m)
		}
	}
}

func TestPauseMenuQuitAndSave(t *testing.T) {
	w := newWorld(t)
	io := newFakeIO(w.Resources)
	io.PauseAnswer = "Q"
	io.Keys = []string{KeyPause}
	NewGame(w, io, GameOptions{}).Run()
	if !strings.Contains(io.out(), "Quit") {
		t.Fatal(io.out())
	}
}

func TestShowcaseSavesImport(t *testing.T) {
	files, _ := filepath.Glob("../../ref/ultima3/showcase/*.json")
	if len(files) == 0 {
		t.Skip("no showcase saves")
	}
	for _, f := range files {
		text, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		e, err := ParseExport(string(text))
		if err != nil {
			t.Fatal(f, err)
		}
		w := newWorld(t)
		if !Restore(w, *e.Save) || !w.Party.Formed() || w.Party.Size() != 4 {
			t.Fatal(f)
		}
	}
}
