package game

import (
	"slices"
	"strings"
	"testing"
)

// healParty is the default party, everyone without mana: 0 Tatiana (thief),
// 1 Roderic (ranger: both books), 2 Norric (cleric), 3 Ghiselle (wizard).
// Everyone starts at 100 of 100 hit points.
func healParty(t *testing.T) (*World, *FakeIO) {
	w := grassWorld(t)
	for m := range 4 {
		w.Member(m).SetMana(0)
	}
	return w, newFakeIO(w.Resources)
}

func hurt(w *World, m, gap int) { w.Member(m).SetHitPoints(w.Member(m).MaxHitPoints() - gap) }

func wantPlan(t *testing.T, got *HealPlan, want *HealPlan) {
	t.Helper()
	if (got == nil) != (want == nil) || got != nil && *got != *want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestHealPlanNobodyHurt(t *testing.T) {
	w, _ := healParty(t)
	w.Member(2).SetMana(60)
	wantPlan(t, HealPlanFor(w), nil)
}

func TestHealPlanNobodyCanPay(t *testing.T) {
	w, _ := healParty(t)
	hurt(w, 0, 50)
	w.Member(3).SetMana(99) // the wizard has mana, but no cleric spells
	w.Member(2).SetMana(9)  // the cleric is one short
	wantPlan(t, HealPlanFor(w), nil)
}

func TestHealPlanWorstWound(t *testing.T) {
	w, _ := healParty(t)
	w.Member(2).SetMana(60)
	hurt(w, 0, 80)
	hurt(w, 1, 15)
	hurt(w, 3, 5)
	wantPlan(t, HealPlanFor(w), &HealPlan{2, GreatHeal, 0})
	hurt(w, 0, 0)
	wantPlan(t, HealPlanFor(w), &HealPlan{2, Heal, 1})
}

func TestHealPlanFallsBack(t *testing.T) {
	w, _ := healParty(t)
	hurt(w, 0, 80)
	w.Member(2).SetMana(12) // Great heal costs 50, Heal 10
	wantPlan(t, HealPlanFor(w), &HealPlan{2, Heal, 0})
	hurt(w, 0, 5)
	w.Member(2).SetMana(55)
	w.Member(2).SetHitPoints(100)
	// A small wound with Heal affordable is a Heal, never a Great heal.
	if p := HealPlanFor(w); p == nil || p.Spell != Heal {
		t.Fatal(p)
	}
}

func TestHealPlanSkipsDeadAndUnhurtPoisoned(t *testing.T) {
	w, _ := healParty(t)
	w.Member(2).SetMana(60)
	w.Member(0).SetStatus('D')
	w.Member(0).SetHitPoints(0)
	w.Member(1).SetStatus('P')
	wantPlan(t, HealPlanFor(w), nil)
	hurt(w, 3, 30)
	if p := HealPlanFor(w); p == nil || p.Target != 3 {
		t.Fatal(p)
	}
}

func TestHealPlanRangerAndMostMana(t *testing.T) {
	w, _ := healParty(t)
	hurt(w, 0, 10)
	w.Member(1).SetMana(30) // the ranger
	wantPlan(t, HealPlanFor(w), &HealPlan{1, Heal, 0})
	w.Member(2).SetMana(40) // the cleric, now richer
	if p := HealPlanFor(w); p.Caster != 2 {
		t.Fatal(p)
	}
	w.Member(2).SetStatus('D') // a dead cleric cannot cast
	if p := HealPlanFor(w); p.Caster != 1 {
		t.Fatal(p)
	}
}

func TestHealPlanExample(t *testing.T) {
	w, _ := healParty(t)
	w.Member(2).SetMana(60)
	hurt(w, 0, 90)
	hurt(w, 1, 15)
	hurt(w, 3, 12)
	if p := HealPlanFor(w); p.Spell != GreatHeal {
		t.Fatal(p)
	}
	w.Member(2).SetMana(10) // cast it
	hurt(w, 0, 0)
	wantPlan(t, HealPlanFor(w), &HealPlan{2, Heal, 1})
	w.Member(2).SetMana(0)
	hurt(w, 1, 0)
	wantPlan(t, HealPlanFor(w), nil) // Ghiselle is still hurt, but nobody can pay
}

func TestQuickHeal(t *testing.T) {
	w, io := healParty(t)
	w.Member(2).SetMana(60)
	hurt(w, 0, 80)
	if !QuickHeal(w, io) || w.Member(2).Mana() != 10 || w.Member(0).HitPoints() < 40 {
		t.Fatal(w.Member(2).Mana(), w.Member(0).HitPoints())
	}
	if !strings.Contains(io.out(), "Great heal\non Tatiana") || !strings.Contains(io.out(), "SANCTU MANI") || w.LastSpell[2] != GreatHeal {
		t.Fatal(io.out())
	}
	w2, io2 := healParty(t)
	if QuickHeal(w2, io2) {
		t.Fatal("healed with no plan")
	}
}

func TestMenusLeadWithHeal(t *testing.T) {
	w, _ := healParty(t)
	first := func() MenuOption { return menuFor(w, ScopeField)[0] }
	if first().Key == QuickCastKey {
		t.Fatal("nobody hurt")
	}
	hurt(w, 0, 30)
	if first().Key == QuickCastKey {
		t.Fatal("no mana")
	}
	w.Member(2).SetMana(60)
	if first().Label != "Cast (Great heal)" {
		t.Fatal(first())
	}
	hurt(w, 0, 10)
	if first().Label != "Cast (Heal)" {
		t.Fatal(first())
	}
	// Ahead of the surroundings' own suggestion too.
	w.X, w.Y = int(w.Resources.Misc.LocationX[1]), int(w.Resources.Misc.LocationY[1])
	w.PutXYVal(MapValueTown, w.X, w.Y)
	m := menuFor(w, ScopeField)
	if m[0].Key != QuickCastKey || m[1].Key != "E" {
		t.Fatal(menuLabels(m))
	}
	if menuFor(w, ScopeDungeon)[0].Label != "Cast (Heal)" {
		t.Fatal("dungeon")
	}
}

func TestSafeChestShortcut(t *testing.T) {
	w, _ := healParty(t)
	labels := func() []string { return menuLabels(menuFor(w, ScopeField)) }
	under := func(l []string) bool {
		return slices.Index(l, "Cast (Safe chest)") == slices.Index(l, "Get chest")+1 && slices.Contains(l, "Get chest")
	}
	if slices.Contains(labels(), "Cast (Safe chest)") {
		t.Fatal("off a chest")
	}
	w.PutXYVal(MapValueChest+1, w.X, w.Y) // a chest left on grass
	if slices.Contains(labels(), "Cast (Safe chest)") {
		t.Fatal("nobody has the mana")
	}
	w.Member(2).SetMana(4)
	if SafeChestCaster(w) != -1 {
		t.Fatal("4 mana")
	}
	w.Member(2).SetMana(5)
	if SafeChestCaster(w) != 2 || !under(labels()) {
		t.Fatal(labels())
	}
	// The ranger casts cleric spells too; the caster with the most mana is chosen.
	w.Member(1).SetMana(9)
	if SafeChestCaster(w) != 1 {
		t.Fatal(SafeChestCaster(w))
	}
	// A hurt member keeps Cast (Heal) first; Safe chest stays under Get chest.
	hurt(w, 0, 10)
	w.Member(2).SetMana(20)
	if l := labels(); l[0] != "Cast (Heal)" || !under(l) {
		t.Fatal(l)
	}
	w.Member(1).SetMana(0)
	w.Member(2).SetMana(0)
	w.Member(3).SetMana(50) // the wizard alone cannot
	if SafeChestCaster(w) != -1 {
		t.Fatal(SafeChestCaster(w))
	}
}

func healDungeon(t *testing.T) (*World, *FakeIO) {
	w, io := healParty(t)
	w.EnterDungeon(MapIDFirstDungeon)
	w.Party.SetLocation(LocationDungeon)
	clear(w.Dungeon.Tiles)
	w.X, w.Y = 3, 3
	return w, io
}

func TestSafeChestInDungeon(t *testing.T) {
	w, _ := healDungeon(t)
	w.Dungeon.Torch = 5
	w.Member(2).SetMana(30)
	if slices.Contains(menuLabels(menuFor(w, ScopeDungeon)), "Cast (Safe chest)") {
		t.Fatal("no chest")
	}
	w.PutXYDng(DungeonCellChest, 3, 3)
	l := menuLabels(menuFor(w, ScopeDungeon))
	if slices.Index(l, "Cast (Safe chest)") != slices.Index(l, "Get chest")+1 {
		t.Fatal(l)
	}
}

// The test that casts Safe chest at once stubbed world.rng.range; Random is
// not swappable here, so it is not ported.

func TestQuickSafeChestNeedsChestAndCaster(t *testing.T) {
	w, io := healParty(t)
	w.Member(2).SetMana(30)
	if QuickSafeChest(w, io) {
		t.Fatal("off a chest")
	}
	w.PutXYVal(MapValueChest, w.X, w.Y)
	w.Member(2).SetMana(0)
	if QuickSafeChest(w, io) {
		t.Fatal("no caster")
	}
}

func wantLight(t *testing.T, got *LightPlan, want *LightPlan) {
	t.Helper()
	if (got == nil) != (want == nil) || got != nil && *got != *want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestLightPlanNone(t *testing.T) {
	w, _ := healParty(t)
	w.Member(3).SetMana(99)
	wantLight(t, LightPlanFor(w), nil)
	lit, _ := healDungeon(t)
	lit.Member(3).SetMana(99)
	lit.Dungeon.Torch = 5
	wantLight(t, LightPlanFor(lit), nil)
	poor, _ := healDungeon(t)
	poor.Member(3).SetMana(9)  // one short of Lorum
	poor.Member(2).SetMana(14) // one short of Luminae
	poor.Member(0).SetMana(99) // the thief has no book
	wantLight(t, LightPlanFor(poor), nil)
}

func TestLightPlanPreferences(t *testing.T) {
	w, _ := healDungeon(t)
	w.Member(3).SetMana(10)
	wantLight(t, LightPlanFor(w), &LightPlan{3, 2})
	w.Member(2).SetMana(15)
	wantLight(t, LightPlanFor(w), &LightPlan{2, 19})
	w.Member(3).SetMana(40) // Dag Lorum beats any Light
	wantLight(t, LightPlanFor(w), &LightPlan{3, 8})
	w.Member(2).SetMana(45) // Sominae: same tier, more mana
	wantLight(t, LightPlanFor(w), &LightPlan{2, 25})
	w.Member(3).SetMana(45) // a tie goes to party order
	wantLight(t, LightPlanFor(w), &LightPlan{2, 25})
	w.Member(1).SetMana(45) // the ranger takes the cheaper Dag Lorum
	wantLight(t, LightPlanFor(w), &LightPlan{1, 8})
}

func TestLightUnderIgnite(t *testing.T) {
	w, _ := healDungeon(t)
	w.Member(3).SetMana(40)
	under := func() bool {
		l := menuLabels(menuFor(w, ScopeDungeon))
		return slices.Contains(l, "Cast (Long light)") && slices.Index(l, "Cast (Long light)") == slices.Index(l, "Ignite torch")+1
	}
	if !under() {
		t.Fatal("not under Ignite")
	}
	w.Party.SetTorches(0) // Ignite torch greyed, the shortcut still under it
	if !under() {
		t.Fatal("not under greyed Ignite")
	}
	w.Dungeon.Torch = 3
	if slices.Contains(menuLabels(menuFor(w, ScopeDungeon)), "Cast (Long light)") {
		t.Fatal("lit")
	}
}

func TestQuickLight(t *testing.T) {
	w, io := healDungeon(t)
	if QuickLight(w, io) {
		t.Fatal("no caster")
	}
	w.Member(3).SetMana(50)
	if !QuickLight(w, io) || w.Dungeon.Torch != 250 || w.Member(3).Mana() != 10 || w.LastSpell[3] != 8 {
		t.Fatal(w.Dungeon.Torch, w.Member(3).Mana())
	}
}
