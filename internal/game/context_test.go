package game

import (
	"slices"
	"strings"
	"testing"
)

// testCommandMenus is the UI's COMMAND_MENUS (ui/menus.ts), the template the
// context menus are built from.
var testCommandMenus = map[CommandScope][]MenuOption{
	ScopeField: {
		{Key: "A", Label: "Attack"}, {Key: "T", Label: "Transact (talk)"}, {Key: "L", Label: "Look"},
		{Key: "E", Label: "Enter"}, {Key: "B", Label: "Board"}, {Key: "X", Label: "X-it craft"},
		{Key: "F", Label: "Fire cannons"}, {Key: "G", Label: "Get chest"}, {Key: "U", Label: "Unlock door"},
		{Key: "S", Label: "Steal"}, {Key: "C", Label: "Cast spell"}, {Key: "Z", Label: "Ztats"},
		{Key: "R", Label: "Ready weapon"}, {Key: "W", Label: "Wear armour"}, {Key: "M", Label: "Modify order"},
		{Key: "N", Label: "Negate time"}, {Key: "P", Label: "Peer at gem"}, {Key: "O", Label: "Other command"},
		{Key: "J", Label: "Journal"}, {Key: ViewMapKey, Label: "View map"},
	},
	ScopeCombat: {
		{Key: "A", Label: "Attack"}, {Key: "C", Label: "Cast spell"}, {Key: "N", Label: "Negate time"},
		{Key: "R", Label: "Ready weapon"}, {Key: "Z", Label: "Ztats"},
	},
	ScopeDungeon: {
		{Key: "I", Label: "Ignite torch"}, {Key: "L", Label: "Change view"}, {Key: "K", Label: "Klimb ladder"},
		{Key: "D", Label: "Descend ladder"}, {Key: "G", Label: "Get chest"}, {Key: "C", Label: "Cast spell"},
		{Key: "Z", Label: "Ztats"}, {Key: "P", Label: "Peer at gem"}, {Key: "R", Label: "Ready weapon"},
		{Key: "W", Label: "Wear armour"}, {Key: "M", Label: "Modify order"}, {Key: "N", Label: "Negate time"},
		{Key: "O", Label: "Other command"}, {Key: "J", Label: "Journal"}, {Key: ViewMapKey, Label: "View map"},
	},
}

func menuFor(w *World, scope CommandScope) []MenuOption {
	return CommandMenu(w, scope, testCommandMenus[scope])
}

func findKey(menu []MenuOption, key string) *MenuOption {
	if i := slices.IndexFunc(menu, func(o MenuOption) bool { return o.Key == key }); i >= 0 {
		return &menu[i]
	}
	return nil
}

func menuLabels(menu []MenuOption) []string {
	var out []string
	for _, o := range menu {
		out = append(out, o.Label)
	}
	return out
}

func grassWorld(t *testing.T) *World {
	w := newWorld(t)
	for i := range w.Current.Tiles {
		w.Current.Tiles[i] = MapValueGrass
	}
	clear(w.Monsters().Bytes)
	return w
}

func wantStrings(t *testing.T, got []string, want ...string) {
	t.Helper()
	if len(want) == 0 && len(got) == 0 {
		return
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSuggestEnterAndBoard(t *testing.T) {
	w := grassWorld(t)
	wantStrings(t, SuggestedCommands(w, ScopeField))
	w.PutXYVal(MapValueTown, w.X, w.Y)
	wantStrings(t, SuggestedCommands(w, ScopeField), "E")
	w.PutXYVal(MapValueHorse, w.X, w.Y)
	wantStrings(t, SuggestedCommands(w, ScopeField), "B")
	w.Party.SetShape(PartyShapeHorse)
	wantStrings(t, SuggestedCommands(w, ScopeField), "X")
}

func TestSuggestGetOnChest(t *testing.T) {
	w := grassWorld(t)
	w.PutXYVal(MapValueChest+2, w.X, w.Y) // a chest dropped on forest
	wantStrings(t, SuggestedCommands(w, ScopeField), "G")
}

func TestSuggestAttackAndFire(t *testing.T) {
	w := grassWorld(t)
	m := w.Monsters()
	m.SetType(0, MapValueOrc)
	m.SetPosition(0, w.X+1, w.Y)
	m.SetHP(0, 0x80)
	wantStrings(t, SuggestedCommands(w, ScopeField), "A")
	w.Party.SetShape(PartyShapeFrigate)
	wantStrings(t, SuggestedCommands(w, ScopeField), "X", "A", "F")
}

func TestSuggestTransactAndUnlock(t *testing.T) {
	w := newWorld(t)
	w.EnterMap(MapIDFirstTown)
	w.Party.SetLocation(LocationTown)
	for i := range w.Current.Tiles {
		w.Current.Tiles[i] = MapValueFloor
	}
	clear(w.Monsters().Bytes)
	w.X, w.Y = 20, 20
	w.PutXYVal(MapValueLetterA, 21, 20)
	w.PutXYVal(MapValueMerchant, 22, 20)
	w.PutXYVal(MapValueLetterI, 19, 20)
	wantStrings(t, SuggestedCommands(w, ScopeField), "T", "U")
}

func TestSuggestDungeon(t *testing.T) {
	w := newWorld(t)
	clear(w.Dungeon.Tiles)
	w.X, w.Y = 3, 3
	w.Dungeon.Level, w.Dungeon.Torch = 0, 5
	w.PutXYDng(DungeonCellLadderDown|DungeonCellChest, 3, 3)
	wantStrings(t, SuggestedCommands(w, ScopeDungeon), "D", "G")
	w.Dungeon.Torch = 0
	w.Party.SetTorches(2)
	wantStrings(t, SuggestedCommands(w, ScopeDungeon), "D", "G", "I")
}

func TestSuggestCombatAttack(t *testing.T) {
	w := newWorld(t)
	c := LoadArena(w, BaseRes+4)
	w.Combat = c
	c.ActiveMember = 0
	c.Members[0].X, c.Members[0].Y = 5, 5
	for _, m := range c.Monsters {
		m.HP = 0
	}
	wantStrings(t, SuggestedCommands(w, ScopeCombat))
	c.Monsters[0].X, c.Monsters[0].Y, c.Monsters[0].HP = 6, 4, 10
	wantStrings(t, SuggestedCommands(w, ScopeCombat)) // diagonal: not without diagonal moves
	w.SetDiagonalMoves(true)
	wantStrings(t, SuggestedCommands(w, ScopeCombat), "A")
}

func TestPrioritiseKeepsAll(t *testing.T) {
	field := testCommandMenus[ScopeField]
	menu := Prioritise(field, []string{"B", "E"})
	if menu[0].Key != "B" || menu[1].Key != "E" || len(menu) != len(field) {
		t.Fatal(menuLabels(menu))
	}
	seen := map[string]bool{}
	for _, o := range menu {
		seen[o.Key] = true
	}
	if len(seen) != len(field) {
		t.Fatal("duplicates")
	}
}

func TestAvailabilityHidesAndGreys(t *testing.T) {
	w := grassWorld(t)
	field := menuFor(w, ScopeField)
	// Plain grass, nothing around, nobody carries gems or powders.
	for _, k := range []string{"A", "T", "E", "B", "X", "F", "G", "U", "S", " ", "Q"} {
		if findKey(field, k) != nil {
			t.Fatal("shown", k)
		}
	}
	if !findKey(field, "P").Disabled || !findKey(field, "N").Disabled || findKey(field, "C").Disabled {
		t.Fatal("P/N/C")
	}
	w.Member(2).SetStatus('D')
	w.Member(3).SetStatus('D') // the ranger still casts
	if findKey(menuFor(w, ScopeField), "C").Disabled {
		t.Fatal("C disabled")
	}
	w.Member(1).SetStatus('D')
	if !findKey(menuFor(w, ScopeField), "C").Disabled {
		t.Fatal("C enabled for the thief alone")
	}
	// A gem makes Peer usable; a horse underfoot makes Board appear, first.
	w.Party.SetGems(1)
	w.PutXYVal(MapValueHorse, w.X, w.Y)
	again := menuFor(w, ScopeField)
	if findKey(again, "P").Disabled || findKey(again, "B") == nil || again[0].Key != "B" {
		t.Fatal(menuLabels(again))
	}
}

func TestUnlockFirstBesideAmbrosiaDoor(t *testing.T) {
	w := newWorld(t)
	w.EnterMap(MapIDAmbrosia)
	w.Party.SetLocation(LocationAmbrosia)
	clear(w.Monsters().Bytes)
	w.X, w.Y = 11, 6 // east of the door at (10, 6)
	w.Party.SetKeys(2)
	if m := menuFor(w, ScopeField); m[0].Key != "U" || m[0].Disabled {
		t.Fatal(menuLabels(m))
	}
	w.Party.SetKeys(0)
	if !findKey(menuFor(w, ScopeField), "U").Disabled {
		t.Fatal("U enabled")
	}
	w.X = 12
	if findKey(menuFor(w, ScopeField), "U") != nil {
		t.Fatal("U shown")
	}
}

func TestDungeonLaddersAndTorch(t *testing.T) {
	w := newWorld(t)
	clear(w.Dungeon.Tiles)
	w.X, w.Y = 3, 3
	w.Dungeon.Level = 0
	if findKey(menuFor(w, ScopeDungeon), "K") != nil || !findKey(menuFor(w, ScopeDungeon), "I").Disabled {
		t.Fatal("start")
	}
	w.PutXYDng(DungeonCellLadderUp, 3, 3)
	w.Party.SetTorches(1)
	if findKey(menuFor(w, ScopeDungeon), "K") == nil || findKey(menuFor(w, ScopeDungeon), "I").Disabled {
		t.Fatal("after")
	}
}

// ctxFight is a fight on member's turn against foes of shape (0 for none
// set); with inCombat the party's location is Combat. Tatiana is a thief,
// Roderic a ranger, Norric a cleric, Ghiselle a wizard.
func ctxFight(t *testing.T, member, shape int, inCombat bool) *World {
	w := newWorld(t)
	c := LoadArena(w, BaseRes+4)
	w.Combat = c
	if inCombat {
		w.Party.SetLocation(LocationCombat)
		c.MonsterShape = shape
	}
	c.ActiveMember = member
	c.Members[member].X, c.Members[member].Y = 5, 5
	for _, m := range c.Monsters {
		m.HP = 0
	}
	return w
}

func combatKeys(w *World) []string {
	var out []string
	for _, o := range menuFor(w, ScopeCombat) {
		out = append(out, o.Key+":"+o.Label)
	}
	return out
}

func TestCombatMenuRangedFirst(t *testing.T) {
	w := ctxFight(t, 1, 0, false) // Roderic, a ranger
	w.Member(1).Bytes[48] = 5     // a bow
	wantStrings(t, combatKeys(w)[:3], "A:Attack (Bow)", "!:Cast (Magic bolt)", "C:Cast spell")
}

func TestCombatMenuDaggerAndLastSpell(t *testing.T) {
	w := ctxFight(t, 1, 0, false)
	w.Party.SetWeapons(1, 0)
	w.LastSpell[1] = 5 // Fireball
	wantStrings(t, combatKeys(w)[:3], "!:Cast (Fireball)", "C:Cast spell", "A:Attack")
	w.Party.SetWeapons(1, 1)
	if combatKeys(w)[0] != "A:Attack (Dagger)" {
		t.Fatal(combatKeys(w))
	}
}

func TestCombatMenuNoQuickSpellForThief(t *testing.T) {
	for _, k := range combatKeys(ctxFight(t, 0, 0, false)) {
		if strings.HasPrefix(k, "!") {
			t.Fatal(k)
		}
	}
	w := ctxFight(t, 1, 0, false)
	w.Member(1).SetMana(0)
	if q := findKey(menuFor(w, ScopeCombat), "!"); q == nil || !q.Disabled {
		t.Fatal(q)
	}
}

func TestRepelWhyNot(t *testing.T) {
	w := ctxFight(t, 1, 0x30, true)
	if RepelWhyNot(w, 0) != "" || RepelWhyNot(w, 16) != "No undead here." {
		t.Fatal("orcs")
	}
	w.SpellFlags.Repond = true
	if RepelWhyNot(w, 0) != "Tried already this fight." {
		t.Fatal("spent")
	}
	w.Party.SetLocation(LocationSosaria)
	if RepelWhyNot(w, 0) != "Only in a fight." || RepelWhyNot(w, 1) != "" {
		t.Fatal("out of a fight")
	}
}

func TestRepelLeadsCombatMenu(t *testing.T) {
	for _, c := range []struct {
		member, shape int
		want          string
	}{{1, 0x30, "%:Cast (Repel orcs)"}, {1, 0x32, "%:Cast (Repel undead)"}, {3, 0x30, "%:Cast (Repel orcs)"}, {2, 0x32, "%:Cast (Repel undead)"}} {
		if k := combatKeys(ctxFight(t, c.member, c.shape, true))[0]; k != c.want {
			t.Error(c, k)
		}
	}
}

func TestRepelLeftOffWhereUseless(t *testing.T) {
	none := func(w *World) {
		t.Helper()
		for _, k := range combatKeys(w) {
			if strings.HasPrefix(k, "%") {
				t.Fatal(k)
			}
		}
	}
	none(ctxFight(t, 0, 0x30, true)) // a thief casts nothing
	none(ctxFight(t, 3, 0x32, true)) // a wizard has no Repel undead
	none(ctxFight(t, 2, 0x30, true)) // nor a cleric Repel orcs
	none(ctxFight(t, 1, 0x34, true)) // giants: neither
	spent := ctxFight(t, 1, 0x30, true)
	spent.SpellFlags.Repond = true
	none(spent)
}

func TestQuickSpellOffUselessRepel(t *testing.T) {
	giants := ctxFight(t, 1, 0x34, true)
	giants.LastSpell[1] = 0 // Repel orcs, last cast
	if combatKeys(giants)[0] != "!:Cast (Magic bolt)" {
		t.Fatal(combatKeys(giants))
	}
	orcs := ctxFight(t, 1, 0x30, true)
	orcs.LastSpell[1] = 0
	wantStrings(t, combatKeys(orcs)[:2], "%:Cast (Repel orcs)", "C:Cast spell")
}

func TestRepelGreyedInSpellLists(t *testing.T) {
	w := ctxFight(t, 1, 0x34, true)
	o := SpellOption(w, 0, "A", 1)
	if last := o.Card.Lines[len(o.Card.Lines)-1]; !o.Disabled || last != (CardLine{"No orcs here.", CardToneWorse}) {
		t.Fatal(o.Disabled, last)
	}
	w.Combat.MonsterShape = 0x30
	if SpellOption(w, 0, "A", 1).Disabled {
		t.Fatal("disabled against orcs")
	}
	w.SpellFlags.Repond = true
	lines := SpellOption(w, 0, "A", 1).Card.Lines
	if lines[len(lines)-2].Text != "Tried already" || lines[len(lines)-1].Text != "this fight." {
		t.Fatal(lines)
	}
	// Every spell's card still fits the panel.
	for _, s := range []struct {
		shape int
		spent bool
	}{{0x30, true}, {0x34, false}} {
		w := ctxFight(t, 1, s.shape, true)
		w.SpellFlags = SpellFlags{s.spent, s.spent}
		for spell := range 35 {
			if n := len(SpellOption(w, spell, "A", 1).Card.Lines); n > CardRows {
				t.Fatal(spell, n)
			}
		}
	}
}

func TestRepelCastSpendsTry(t *testing.T) {
	w := ctxFight(t, 1, 0x30, true)
	io := newFakeIO(w.Resources)
	if !RepelCast(w, io, 1) || !w.SpellFlags.Repond || w.LastSpell[1] != 0 {
		t.Fatal("cast")
	}
	if RepelCast(w, io, 1) {
		t.Fatal("cast twice")
	}
}

func TestCombatMenuNonCaster(t *testing.T) {
	w := ctxFight(t, 0, 0, false)
	keys := menuFor(w, ScopeCombat)
	if keys[0].Key != "A" || findKey(keys, "!") != nil {
		t.Fatal(menuLabels(keys))
	}
}

func TestViewMapEverywhere(t *testing.T) {
	w := newWorld(t)
	if !slices.Contains(menuLabels(menuFor(w, ScopeField)), "View map") {
		t.Fatal("field")
	}
	w.Party.SetLocation(LocationAmbrosia)
	if !slices.Contains(menuLabels(menuFor(w, ScopeField)), "View map") {
		t.Fatal("Ambrosia")
	}
}
