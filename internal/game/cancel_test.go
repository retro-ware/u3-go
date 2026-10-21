package game

import (
	"slices"
	"strings"
	"testing"
)

const (
	cancelCleric = 2   // Norric, in the default party
	cancelHeal   = "C" // the third cleric spell
)

func TestCancelAtDirection(t *testing.T) {
	w := newWorld(t)
	io := newFakeIO(w.Resources)
	io.Keys = append(io.Keys, KeyEscape)
	Look(w, io)
	if !strings.Contains(io.out(), "Cancelled.") || !w.TakeCancelled() || w.TakeCancelled() {
		t.Fatal(io.out())
	}
	io.Keys = append(io.Keys, KeyUp)
	Look(w, io)
	if w.TakeCancelled() {
		t.Fatal("answered but cancelled")
	}
}

func TestCancelAtWhoOrList(t *testing.T) {
	w := newWorld(t)
	io := newFakeIO(w.Resources)
	weapon := w.Member(0).Bytes[48]
	io.Keys = append(io.Keys, "1", KeyEscape) // Tatiana, then no weapon
	ReadyWeapon(w, io, -1)
	if w.Member(0).Bytes[48] != weapon || strings.Contains(io.out(), "Not owned") || len(io.Sounds) != 0 || !w.TakeCancelled() {
		t.Fatal(io.out(), io.Sounds)
	}
	order := func() []int {
		var o []int
		for m := range 4 {
			o = append(o, w.Party.MemberRosterNumber(m))
		}
		return o
	}
	before := order()
	io.Keys = append(io.Keys, "1", KeyEscape) // first member, then no second
	ModifyOrder(w, io)
	if !slices.Equal(order(), before) || strings.Contains(io.out(), "Aborted") || !w.TakeCancelled() {
		t.Fatal(io.out())
	}
}

func TestCancelSpellRefunds(t *testing.T) {
	w := newWorld(t)
	io := newFakeIO(w.Resources)
	w.Member(cancelCleric).SetMana(50)
	io.Keys = append(io.Keys, cancelHeal, KeyEscape) // Heal, then nobody
	if Cast(w, io, cancelCleric) || w.Member(cancelCleric).Mana() != 50 {
		t.Fatal(w.Member(cancelCleric).Mana())
	}
	if !strings.Contains(io.out(), "Cancelled.") || strings.Contains(io.out(), "Failed") || !w.TakeCancelled() {
		t.Fatal(io.out())
	}
}

func TestCancelCostsNoMove(t *testing.T) {
	movesAfter := func(keys ...string) int {
		w := newWorld(t)
		io := newFakeIO(w.Resources)
		io.Keys = append(append(io.Keys, keys...), "Q") // Q ends the game loop
		before := w.Party.Moves()
		NewGame(w, io, GameOptions{}).Run()
		return w.Party.Moves() - before
	}
	if n := movesAfter("L", KeyEscape); n != 0 {
		t.Fatal(n)
	}
	if n := movesAfter("L", KeyUp); n <= 0 {
		t.Fatal(n)
	}
}

func TestCancelInDungeonPassesNoTurn(t *testing.T) {
	torchAfter := func(keys ...string) int {
		w := newWorld(t, 7)
		w.EnterDungeon(MapIDFirstDungeon)
		w.Party.SetLocation(LocationDungeon)
		for i := range 256 {
			w.Dungeon.Tiles[i] = DungeonCellWall
		}
		w.PutXYDng(DungeonCellLadderUp, 1, 1)
		w.X, w.Y = 1, 1
		w.Party.SetTorches(1)
		io := newFakeIO(w.Resources)
		io.Keys = append(append([]string{"I"}, keys...), "K") // light the torch, then climb out
		RunDungeon(w, io)
		return w.Dungeon.Torch
	}
	lit := torchAfter()
	if n := torchAfter("Z", KeyEscape); n != lit {
		t.Fatal(n, lit)
	}
	if n := torchAfter(" "); n != lit-1 {
		t.Fatal(n, lit)
	}
}

func TestCancelInCombatAsksAgain(t *testing.T) {
	w := newWorld(t, 42)
	for i := range w.Current.Tiles {
		w.Current.Tiles[i] = MapValueFloor
	}
	clear(w.Monsters().Bytes)
	w.Party.SetLocation(LocationTown) // one monster only
	io := newFakeIO(w.Resources)
	// Player 1: Attack then no direction, Ztats closed at once, Ready then no weapon, then Pass.
	io.Keys = append(io.Keys, "A", KeyEscape, "Z", KeyEscape, "R", KeyEscape, " ")
	// Everyone after passes, and the monster drops so the fight ends with the round.
	io.KeyProvider = func() string {
		if w.Combat != nil {
			for _, m := range w.Combat.Monsters {
				m.HP = 0
			}
		}
		return " "
	}
	Combat(w, io, MapValueOrc>>1, 0)
	out := io.out()
	first := out[:strings.Index(out, "PLAYER-2")]
	if strings.Count(first, "PLAYER-1") != 4 || strings.Count(first, "Cancelled.") != 2 || !strings.Contains(first, "Pass") {
		t.Fatal(first)
	}
}
