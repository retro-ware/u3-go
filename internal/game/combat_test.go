package game

import (
	"slices"
	"strings"
	"testing"
)

func combatWorld(t *testing.T) (*World, *FakeIO) {
	w := newWorld(t, 42)
	for i := range w.Current.Tiles {
		w.Current.Tiles[i] = MapValueGrass
	}
	clear(w.Monsters().Bytes)
	w.X, w.Y = 32, 32
	return w, newFakeIO(w.Resources)
}

func fillArena(c *CombatState, shape int) {
	for i := range c.Tiles {
		c.Tiles[i] = byte(shape)
	}
}

func TestChooseArena(t *testing.T) {
	w, _ := combatWorld(t)
	if id, _ := ChooseArena(w, MapValueOrc>>1); id != BaseRes+4 {
		t.Errorf("grass arena %d", id)
	}
	w.PutXYVal(MapValueForest, 32, 32)
	if id, _ := ChooseArena(w, MapValueOrc>>1); id != BaseRes+3 {
		t.Errorf("forest arena %d", id)
	}
	// Boarding a pirate frigate means fighting thieves.
	if id, s := ChooseArena(w, 0x1e); id != BaseRes || s != 0x2e {
		t.Errorf("pirates %d %#x", id, s)
	}
}

func TestLoadArena(t *testing.T) {
	w, _ := combatWorld(t)
	c := LoadArena(w, BaseRes+4)
	if len(c.Tiles) != 121 || len(c.Monsters) != 8 || len(c.Members) != 4 {
		t.Fatalf("sizes %d %d %d", len(c.Tiles), len(c.Monsters), len(c.Members))
	}
	for _, m := range c.Members {
		if c.Tiles[m.Y*11+m.X] != ShapeGrass {
			t.Errorf("member on %#x", c.Tiles[m.Y*11+m.X])
		}
	}
}

func TestMemberShape(t *testing.T) {
	w, _ := combatWorld(t)
	for i, l := range "FCWTPBLIDAR" {
		if got := MemberShape(w, byte(l)); got != (68+i)*2 {
			t.Errorf("%c: %d", l, got)
		}
	}
	if got := MemberShape(w, '?'); got != 0x7e {
		t.Errorf("?: %#x", got)
	}
}

func TestMonsterName(t *testing.T) {
	w, _ := combatWorld(t)
	for _, c := range []struct {
		variant int
		plural  bool
		want    string
	}{{0, false, "Orc"}, {1, true, "Goblins"}, {2, false, "Troll"}} {
		if got := MonsterName(w, MapValueOrc>>1, c.variant, c.plural); got != c.want {
			t.Errorf("got %q want %q", got, c.want)
		}
	}
}

func TestPlanMonster(t *testing.T) {
	w, _ := combatWorld(t)
	c := LoadArena(w, BaseRes+4)
	fillArena(c, ShapeGrass)
	w.Combat = c
	w.Party.SetLocation(LocationCombat)
	for _, m := range c.Members {
		m.X, m.Y = 255, 255
	}
	c.Members[0].X, c.Members[0].Y = 5, 8
	c.Monsters[0].HP, c.Monsters[0].X, c.Monsters[0].Y = 10, 5, 2
	c.MonsterShape = MapValueOrc >> 1
	plan := PlanMonster(w, 0)
	if plan.Target != 0 || plan.NewX != 5 || plan.NewY != 3 || plan.Distance != 6 {
		t.Errorf("plan %+v", plan)
	}
	c.Monsters[0].Y = 7
	if d := PlanMonster(w, 0).Distance; d != 0 {
		t.Errorf("adjacent distance %d", d)
	}
}

func TestHandleMove(t *testing.T) {
	w, io := combatWorld(t)
	c := LoadArena(w, BaseRes+4)
	fillArena(c, ShapeGrass)
	w.Combat = c
	c.Members[0].X, c.Members[0].Y = 5, 5
	c.Tiles[4*11+5] = ShapeWall
	HandleMove(w, io, 0, 0, -1)
	if c.Members[0].Y != 5 || !strings.Contains(io.out(), "INVALID MOVE!") {
		t.Errorf("walked into a wall: y %d, %q", c.Members[0].Y, io.out())
	}
	HandleMove(w, io, 0, 1, 0)
	if c.Members[0].X != 6 || !slices.Contains(io.Sounds, "Step") {
		t.Errorf("did not step: x %d", c.Members[0].X)
	}
}

// strongParty makes every member strong with a sword.
func strongParty(w *World, hp int) {
	for m := range 4 {
		p := w.Member(m)
		if hp > 0 {
			p.SetHitPoints(hp)
		}
		p.Bytes[18], p.Bytes[19], p.Bytes[48], p.Bytes[54] = 99, 99, 6, 1
	}
}

// stepToward is the arrow key from a member toward a monster, as the TypeScript tests choose it.
func stepToward(me, target *Combatant) string {
	dx, dy := sign(target.X-me.X), sign(target.Y-me.Y)
	switch {
	case dx == 0 && dy < 0:
		return KeyUp
	case dx == 0:
		return KeyDown
	case dx < 0:
		return KeyLeft
	}
	return KeyRight
}

func firstAlive(c *CombatState) *Combatant {
	for _, m := range c.Monsters {
		if m.HP > 0 {
			return m
		}
	}
	return nil
}

func TestFightWonByAttacking(t *testing.T) {
	w, io := combatWorld(t)
	strongParty(w, 0)
	w.Party.SetLocation(LocationTown) // one monster only
	for i := range w.Current.Tiles {
		w.Current.Tiles[i] = MapValueFloor
	}
	expBefore := w.Member(0).Bytes[31]
	// Attack toward the nearest living monster if adjacent, otherwise step toward it.
	io.KeyProvider = func() string {
		c := w.Combat
		if c == nil {
			return " "
		}
		me := c.Members[c.ActiveMember]
		target := firstAlive(c)
		if target == nil {
			return " "
		}
		key := stepToward(me, target)
		if abs(target.X-me.X) <= 1 && abs(target.Y-me.Y) <= 1 && (target.X == me.X || target.Y == me.Y) {
			io.Keys = append(io.Keys, key)
			return "A"
		}
		return key
	}
	Combat(w, io, MapValueOrc>>1, 0)
	if w.Combat != nil || w.Party.Location() != LocationTown {
		t.Fatalf("fight did not end in town")
	}
	for _, s := range []string{"CONFLICT", "VICTORY", "KILLED! Exp.+"} {
		if !strings.Contains(io.out(), s) {
			t.Errorf("no %q", s)
		}
	}
	total := 0
	for m := range 4 {
		total += int(w.Member(m).Bytes[31])
	}
	if total <= int(expBefore) {
		t.Errorf("experience %d, was %d", total, expBefore)
	}
}

func TestWalkingIntoMonsterAttacks(t *testing.T) {
	w, io := combatWorld(t)
	strongParty(w, 400)
	w.Party.SetLocation(LocationTown)
	// Only ever step toward the monster; never press A.
	io.KeyProvider = func() string {
		c := w.Combat
		if c == nil {
			return " "
		}
		target := firstAlive(c)
		if target == nil {
			return " "
		}
		return stepToward(c.Members[c.ActiveMember], target)
	}
	Combat(w, io, MapValueOrc>>1, 0)
	for _, s := range []string{"VICTORY", "Sword attack", "KILLED! Exp.+"} {
		if !strings.Contains(io.out(), s) {
			t.Errorf("no %q", s)
		}
	}
}

func TestMonsterLeavesChest(t *testing.T) {
	w, io := combatWorld(t)
	m := w.Monsters()
	m.SetType(0, MapValueOrc)
	m.SetTileUnder(0, MapValueBrush)
	m.SetPosition(0, 33, 32)
	m.SetHP(0, 0xc0)
	w.PutXYVal(MapValueOrc, 33, 32)
	io.KeyProvider = func() string { return " " }
	// Kill the monsters as soon as the arena exists so the fight ends at once.
	io.OnRedraw = func() {
		if w.Combat != nil {
			for _, mon := range w.Combat.Monsters {
				mon.HP = 0
			}
		}
	}
	AttackMonster(w, io, 0)
	if m.Type(0) != 0 {
		t.Errorf("monster still in table")
	}
	if got := w.GetXYVal(33, 32); got != MapValueChest+2 {
		t.Errorf("left %#x, want chest on brush", got)
	}
	if !strings.Contains(io.out(), "VICTORY") {
		t.Errorf("no victory")
	}
}

func TestPartyWipe(t *testing.T) {
	w, io := combatWorld(t)
	kill := func() {
		for m := range w.Party.Size() {
			w.Member(m).SetStatus('D')
		}
	}
	kill()
	w.Party.SetGold(4321)
	loaded := 0
	w.LoadLastSave = func() bool {
		loaded++
		w.Party.SetGold(999) // stands in for the restored save
		for m := range w.Party.Size() {
			w.Member(m).SetStatus('G')
		}
		return true
	}
	io.Keys = []string{" ", "T"}
	CheckAllDead(w, io)
	if loaded != 1 || w.Party.Gold() != 999 || !w.Resurrecting {
		t.Fatalf("try again: loaded %d gold %d", loaded, w.Party.Gold())
	}
	// Fleeing keeps the world but strips the gear.
	kill()
	w.Resurrecting = false
	io.Keys = []string{" ", "F"}
	CheckAllDead(w, io)
	if loaded != 1 || w.Party.Gold() != 150*w.Party.Size() || w.Member(0).Status() != 'G' || w.X != 42 {
		t.Errorf("flee: loaded %d gold %d x %d", loaded, w.Party.Gold(), w.X)
	}
}

func TestPlaceRevived(t *testing.T) {
	w, io := combatWorld(t)
	c := LoadArena(w, BaseRes+4)
	fillArena(c, ShapeGrass)
	w.Combat = c
	me := c.Members[0]
	me.DiedAt = &Point{5, 5}
	me.X, me.Y = 255, 255
	w.Member(0).SetStatus('G')
	c.Monsters[0].X, c.Monsters[0].Y, c.Monsters[0].HP = 5, 5, 10 // the death square itself is taken
	PlaceRevived(w, io)
	if d := max(abs(me.X-5), abs(me.Y-5)); d != 1 {
		t.Errorf("placed %d away", d)
	}
	if io.Redraws == 0 {
		t.Errorf("no redraw")
	}
}

func TestThrowingDaggers(t *testing.T) {
	w, io := combatWorld(t)
	c := LoadArena(w, BaseRes+4)
	fillArena(c, ShapeGrass)
	w.Combat = c
	c.Members[0].X, c.Members[0].Y = 5, 9
	c.Monsters[0].X, c.Monsters[0].Y, c.Monsters[0].HP = 5, 5, 10
	p := w.Member(0)
	p.Bytes[48] = 1 // dagger readied, none spare in the bag
	w.Party.SetWeapons(1, 0)
	io.Keys = []string{KeyUp}
	CombatAttackForTest(w, io, 0)
	if !strings.Contains(io.out(), "Missed") {
		t.Errorf("lone dagger thrown: %q", io.out())
	}
	w.Party.SetWeapons(1, 1) // a spare: it is thrown, the one in hand stays
	io.Keys = []string{KeyUp}
	CombatAttackForTest(w, io, 0)
	if w.Party.Weapons(1) != 0 || p.Bytes[48] != 1 {
		t.Errorf("spare %d, in hand %d", w.Party.Weapons(1), p.Bytes[48])
	}
}

func TestShareExperience(t *testing.T) {
	w := newWorld(t)
	check := func(got, want []XPShare) {
		t.Helper()
		if !slices.Equal(got, want) {
			t.Errorf("got %v want %v", got, want)
		}
	}
	check(ShareExperience(w, 2, 3), []XPShare{{2, 1}, {0, 1}, {1, 1}}) // the fourth gets nothing this time
	check(ShareExperience(w, 0, 20), []XPShare{{0, 5}, {1, 5}, {2, 5}, {3, 5}})
	w.Member(3).SetStatus('D')
	check(ShareExperience(w, 0, 10), []XPShare{{0, 4}, {1, 3}, {2, 3}})
	w.BalancedXP = false
	check(ShareExperience(w, 1, 15), []XPShare{{1, 15}})
}
