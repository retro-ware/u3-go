package game

import (
	"slices"
	"strings"
	"testing"
)

// acArena is a flat grass arena with member 0 at (5, 9) and no monsters;
// diagonals allowed unless classic.
func acArena(t *testing.T, monsterShape int, classic bool) (*World, *CombatState) {
	w := newWorld(t, 7)
	w.SetDiagonalMoves(!classic)
	for i := range w.Current.Tiles {
		w.Current.Tiles[i] = MapValueGrass
	}
	clear(w.Monsters().Bytes)
	c := LoadArena(w, BaseRes+4)
	fillArena(c, ShapeGrass)
	c.MonsterShape = monsterShape
	for _, m := range c.Monsters {
		m.HP = 0
	}
	for _, m := range c.Members {
		m.X, m.Y = missing, missing
	}
	c.Members[0].X, c.Members[0].Y = 5, 9
	w.Combat = c
	return w, c
}

func acMonster(c *CombatState, slot, x, y int) {
	c.Monsters[slot].X, c.Monsters[slot].Y, c.Monsters[slot].HP = x, y, 20
}

// acFighter makes member 0 a plain fighter with no magic and the given weapon.
func acFighter(w *World, weapon int) {
	p := w.Member(0)
	p.Bytes[23] = 'F'
	p.Bytes[48] = byte(weapon)
	p.SetMana(0)
	p.SetHitPoints(150)
}

func wantKeys(t *testing.T, got []string, want ...string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestAutoCombatPlanning(t *testing.T) {
	orc := MapValueOrc >> 1
	t.Run("attacks adjacent", func(t *testing.T) {
		w, c := acArena(t, orc, false)
		acFighter(w, 2)
		acMonster(c, 0, 5, 8)
		wantKeys(t, AutoCombatKeys(w, 0), "A", "8")
	})
	t.Run("walks toward nearest", func(t *testing.T) {
		w, c := acArena(t, orc, false)
		acFighter(w, 2)
		acMonster(c, 0, 5, 2)
		wantKeys(t, AutoCombatKeys(w, 0), "8")
	})
	t.Run("runs when nearly dead", func(t *testing.T) {
		w, c := acArena(t, orc, false)
		acFighter(w, 2)
		w.Member(0).SetHitPoints(20)
		acMonster(c, 0, 5, 8)
		if k := AutoCombatKeys(w, 0); k[0] != "2" { // south is the first safe square tried
			t.Errorf("got %q", k)
		}
	})
	t.Run("fires a bow in line", func(t *testing.T) {
		w, c := acArena(t, orc, false)
		acFighter(w, 5)
		acMonster(c, 0, 9, 5) // on the diagonal, up and to the right
		wantKeys(t, AutoCombatKeys(w, 0), "A", "9")
	})
	t.Run("Repond then Mittar", func(t *testing.T) {
		w, c := acArena(t, orc, false)
		p := w.Member(0)
		p.Bytes[23], p.Bytes[48] = 'W', 0
		p.SetMana(30)
		p.SetHitPoints(150)
		acMonster(c, 0, 5, 2)
		wantKeys(t, AutoCombatKeys(w, 0), "C", "A")
		w.SpellFlags.Repond = true
		wantKeys(t, AutoCombatKeys(w, 0), "C", "B", "8")
	})
	t.Run("multi-class asks which list", func(t *testing.T) {
		w, c := acArena(t, orc, false)
		p := w.Member(0)
		p.Bytes[23] = 'D' // Druid
		p.SetMana(30)
		p.SetHitPoints(150)
		acMonster(c, 0, 5, 2)
		wantKeys(t, AutoCombatKeys(w, 0), "C", "W", "A")
	})
	t.Run("Sanctu on the weakest", func(t *testing.T) {
		w, c := acArena(t, MapValueSkeleton>>1, false)
		p := w.Member(0)
		p.Bytes[23] = 'C'
		p.SetMana(30)
		p.SetHitPoints(150)
		w.SpellFlags.Pontori = true
		w.Member(1).SetHitPoints(40)
		acMonster(c, 0, 5, 2)
		wantKeys(t, AutoCombatKeys(w, 0), "C", "C", "2")
	})
	t.Run("no diagonals in classic mode", func(t *testing.T) {
		w, c := acArena(t, orc, true)
		acFighter(w, 2)
		acMonster(c, 0, 6, 8) // diagonally adjacent: a step, not an attack
		if slices.Contains(AutoCombatKeys(w, 0), "A") {
			t.Errorf("attacked diagonally")
		}
		acFighter(w, 5)
		// The bow needs a row or column: a step east puts the orc straight north.
		wantKeys(t, AutoCombatKeys(w, 0), "6")
		c.Monsters[0].X, c.Monsters[0].Y = 5, 3
		wantKeys(t, AutoCombatKeys(w, 0), "A", "8")
	})
	t.Run("retreats diagonally only when allowed", func(t *testing.T) {
		w, c := acArena(t, orc, false)
		acFighter(w, 2)
		w.Member(0).SetHitPoints(20)
		acMonster(c, 0, 5, 8)
		acMonster(c, 1, 6, 10)
		wantKeys(t, AutoCombatKeys(w, 0), "1")
		// Without diagonals there is nowhere safe, so the member fights back.
		w.SetDiagonalMoves(false)
		wantKeys(t, AutoCombatKeys(w, 0), "A", "8")
	})
	t.Run("passes when boxed in", func(t *testing.T) {
		w, c := acArena(t, orc, false)
		acFighter(w, 2)
		c.Members[0].X, c.Members[0].Y = 0, 0
		for x := range 11 {
			c.Tiles[11+x] = ShapeWater
		}
		c.Tiles[1] = ShapeWater
		acMonster(c, 0, 10, 10)
		wantKeys(t, AutoCombatKeys(w, 0), KeySpace)
	})
}

func acFightWorld(t *testing.T) *World {
	w := newWorld(t, 42)
	for i := range w.Current.Tiles {
		w.Current.Tiles[i] = MapValueGrass
	}
	clear(w.Monsters().Bytes)
	w.X, w.Y = 32, 32
	return w
}

func TestAutoCombatWholeBattle(t *testing.T) {
	w := acFightWorld(t)
	for m := range w.Party.Size() {
		p := w.Member(m)
		p.SetHitPoints(400)
		p.Bytes[48] = 2 // maces for everyone
	}
	w.AutoCombat = true
	io := newFakeIO(w.Resources)
	Combat(w, io, MapValueOrc>>1, 0)
	if w.Combat != nil || !strings.Contains(io.out(), "VICTORY") || len(io.Macro) != 0 {
		t.Errorf("combat %v macro %q", w.Combat, io.Macro)
	}
}

func TestAutoCombatEscape(t *testing.T) {
	w := acFightWorld(t)
	for m := range w.Party.Size() {
		w.Member(m).SetHitPoints(400)
	}
	w.AutoCombat = true
	changes := 0
	w.OnAutoCombatChange = func() { changes++ }
	io := newFakeIO(w.Resources)
	io.Keys = []string{KeyEscape}
	// After the interruption every turn passes until the fight is over.
	io.KeyProvider = func() string { return " " }
	Combat(w, io, MapValueOrc>>1, 0)
	if w.AutoCombat || changes != 1 || !strings.Contains(io.out(), "Manual combat") {
		t.Errorf("auto %v changes %d", w.AutoCombat, changes)
	}
}
