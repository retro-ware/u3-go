package game

import "slices"

// What the party stands on or next to, turned into the commands most likely
// wanted now. Controller mode puts these first in the command menu, so the
// common case is one button away: Enter on a town, Board on a horse, Klimb
// on a ladder. Pure game logic; nothing here knows how menus are drawn.

// HasMagic is false for fighters, thieves and barbarians.
func HasMagic(w *World, p *PlayerRecord) bool {
	return !slices.Contains([]int{0, 3, 5}, careerIndex(w, p))
}

// canCastAny says whether anyone living (or member, when >= 0) is a caster.
// The first spell of each book costs nothing, so mana is no bar.
func canCastAny(w *World, member int) bool {
	if member >= 0 {
		return HasMagic(w, w.Member(member))
	}
	for m := range 4 {
		if w.Party.MemberSlot(m) >= 0 && w.MemberAlive(m) && HasMagic(w, w.Member(m)) {
			return true
		}
	}
	return false
}

// Availability is how a command stands now: hidden where it makes no sense
// (no chest to get), disabled where it is possible but nothing is on hand
// (no gem, no torch), or ok. Commands not listed are always ok.
type Availability string

const (
	AvailabilityOK       Availability = "ok"
	AvailabilityDisabled Availability = "disabled"
	AvailabilityHidden   Availability = "hidden"
)

// Keys of the shortcuts this port adds to the menus.
const (
	LightKey     = "$" // "Cast (Light)" in the dungeon menu: light a dark dungeon at once
	ViewMapKey   = "#" // "View map" in the field menu: the map of what the party has seen
	SafeChestKey = "@" // "Cast (Safe chest)" in the field and dungeon menus
	QuickCastKey = "!" // "Cast (spell)" in the combat menu: the member's quick spell
	RepelKey     = "%" // "Cast (Repel orcs/undead)" in the combat menu, where one would work
)

// CommandAvailability is the state of each command that is not plainly ok.
func CommandAvailability(w *World, scope CommandScope) map[string]Availability {
	out := map[string]Availability{}
	set := func(key string, ok, disabledNotHidden bool) {
		if ok {
			return
		}
		out[key] = AvailabilityHidden
		if disabledNotHidden {
			out[key] = AvailabilityDisabled
		}
	}
	switch scope {
	case ScopeCombat:
		me := 0
		if w.Combat != nil {
			me = w.Combat.ActiveMember
		}
		set("C", canCastAny(w, me), true)
		set("N", w.Party.Powders() > 0, true)
		return out
	case ScopeDungeon:
		cell := w.GetXYDng(w.X, w.Y)
		open := cell < DungeonCellWall
		set("K", open && cell&DungeonCellLadderUp != 0, false)
		set("D", open && cell&DungeonCellLadderDown != 0, false)
		set("G", open && cell&DungeonCellChest != 0, false)
		set("I", w.Party.Torches() > 0, true)
		set("P", w.Party.Gems() > 0, true)
		set("N", w.Party.Powders() > 0, true)
		set("C", canCastAny(w, -1), true)
		set("M", w.Party.Size() > 1, false)
		return out
	}

	// The field: surface, Ambrosia, towns and castles.
	here := w.GetXYVal(w.X, w.Y)
	shape := w.Party.Shape()
	loc := w.Party.Location()
	inTown := w.InTownOrCastle()

	creature, door, counter := false, false, false
	steps := orthogonalSteps
	if w.DiagonalMoves {
		steps = append(slices.Clone(orthogonalSteps), Point{-1, -1}, Point{1, -1}, Point{-1, 1}, Point{1, 1})
	}
	for _, s := range steps {
		xs, ys := w.Constrain(w.X+s.X), w.Constrain(w.Y+s.Y)
		if w.Monsters().At(xs, ys) >= 0 {
			creature = true
		}
		// A door to either side: in a town or castle, or before two of Ambrosia's shrines.
		if s.X != 0 && s.Y == 0 && w.GetXYVal(xs, ys) == MapValueLetterI {
			door = true
		}
		if inTown {
			tile := w.GetXYVal(xs, ys)
			if tile >= MapValueWall2 && tile < MapValueSnakeBottom && w.GetXYVal(xs+s.X, ys+s.Y) == MapValueChest {
				counter = true
			}
		}
	}

	entrance := false
	switch loc {
	case LocationSosaria:
		misc := w.Resources.Misc
		for i := range 32 {
			if int(misc.LocationX[i]) == w.X && int(misc.LocationY[i]) == w.Y {
				entrance = true
			}
		}
	case LocationAmbrosia:
		entrance = here == MapValueShrine
	}

	set("A", creature, false)
	set("T", inTown, false)
	set("E", entrance, false)
	set("B", shape == PartyShapeOnFoot && (here == MapValueHorse || here == MapValueFrigate), false)
	set("X", shape == PartyShapeHorse || shape == PartyShapeFrigate, false)
	set("F", shape == PartyShapeFrigate, false)
	set("G", here >= MapValueChest && here <= MapValueChest+3, false)
	set("U", door, false)
	if door {
		set("U", w.Party.Keys() > 0, true)
	}
	set("S", counter, false)
	set(ViewMapKey, true, false) // the map of what the party has seen, wherever it is
	set("M", w.Party.Size() > 1, false)
	set("C", canCastAny(w, -1), true)
	set("N", w.Party.Powders() > 0, true)
	set("P", w.Party.Gems() > 0, true)
	return out
}

// CommandMenu is the controller command menu for a scope: the commands the
// surroundings call for first, impossible ones left out, ones with nothing
// on hand greyed. template is the full list with its labels.
func CommandMenu(w *World, scope CommandScope, template []MenuOption) []MenuOption {
	availability := CommandAvailability(w, scope)
	var shown []MenuOption
	for _, o := range template {
		if availability[o.Key] == AvailabilityHidden {
			continue
		}
		o.Disabled = availability[o.Key] == AvailabilityDisabled
		shown = append(shown, o)
	}
	if scope == ScopeCombat && w.Combat != nil {
		return combatMenu(w, shown)
	}
	// Outside combat, a wounded member a caster can help puts "Cast (Heal)" first (see HealPlanFor).
	var suggested []string
	if plan := HealPlanFor(w); plan != nil {
		shown = append([]MenuOption{{Key: QuickCastKey, Label: "Cast (" + SpellName(plan.Spell) + ")"}}, shown...)
		suggested = append(suggested, QuickCastKey)
	}
	menu := Prioritise(shown, append(suggested, SuggestedCommands(w, scope)...))
	// On a chest, a caster who can pay for Safe chest puts "Cast (Safe chest)" under Get chest.
	if get := slices.IndexFunc(menu, func(o MenuOption) bool { return o.Key == "G" }); get >= 0 && SafeChestCaster(w) >= 0 {
		menu = slices.Insert(menu, get+1, MenuOption{Key: SafeChestKey, Label: "Cast (" + SpellName(SafeChest) + ")"})
	}
	// In a dark dungeon, a caster who can light it puts "Cast (Long light)" or "Cast (Light)" under Ignite torch.
	if light := LightPlanFor(w); light != nil {
		ignite := slices.IndexFunc(menu, func(o MenuOption) bool { return o.Key == "I" })
		// TypeScript's splice(-1 + 1) puts it first when there is no Ignite line.
		menu = slices.Insert(menu, ignite+1, MenuOption{Key: LightKey, Label: "Cast (" + SpellName(light.Spell) + ")"})
	}
	return menu
}

// rangedWeapons are the sling and the bows. A dagger counts only with a
// spare in the bag, so the last one is never thrown.
var rangedWeapons = []int{3, 5, 9, 13}

// RangedWeaponName is the name of the member's ranged weapon, or "" if what they hold is for melee.
func RangedWeaponName(w *World, member int) string {
	weapon := int(w.Member(member).Bytes[48])
	if slices.Contains(rangedWeapons, weapon) || (weapon == 1 && w.Party.Weapons(1) >= 1) {
		return w.Resources.Strings["WeaponsArmour"][weapon]
	}
	return ""
}

// combatMenu is the combat menu for the member whose turn it is. A Repel
// spell that would work comes first of all: it costs nothing and may end
// the fight at once. A ranged weapon in hand puts "Attack (Bow)" next,
// since melee is done by walking into the foe; a caster gets "Cast
// (spell)" ahead of the plain Cast, next of all when there is no ranged
// weapon. Attack is suggested for melee only beside a foe.
func combatMenu(w *World, shown []MenuOption) []MenuOption {
	me := w.Combat.ActiveMember
	ranged := RangedWeaponName(w, me)
	repel := RepelSpell(w, me)
	spell := -1
	if HasMagic(w, w.Member(me)) {
		spell = QuickSpell(w, me)
	}
	// The quick spell, unless it is the Repel spell already offered above it.
	quickShown := spell >= 0 && spell != repel
	options := slices.Clone(shown)
	for i := range options {
		if options[i].Key == "A" && ranged != "" {
			options[i].Label = "Attack (" + ranged + ")"
		}
	}
	if quickShown {
		quick := MenuOption{Key: QuickCastKey, Label: "Cast (" + SpellName(spell) + ")", Disabled: SpellCost(spell) > w.Member(me).Mana()}
		options = append([]MenuOption{quick}, options...)
	}
	if repel >= 0 {
		options = append([]MenuOption{{Key: RepelKey, Label: "Cast (" + SpellName(repel) + ")"}}, options...)
	}
	var casting []string
	if quickShown {
		casting = append(casting, QuickCastKey)
	}
	if spell >= 0 || repel >= 0 {
		casting = append(casting, "C")
	}
	var first []string
	if ranged != "" {
		first = append([]string{"A"}, casting...)
	} else {
		first = append(casting, combatSuggestions(w)...)
	}
	if repel >= 0 {
		first = append([]string{RepelKey}, first...)
	}
	return Prioritise(options, first)
}

// orthogonalSteps are the four orthogonal steps.
var orthogonalSteps = []Point{{0, -1}, {0, 1}, {-1, 0}, {1, 0}}

// SuggestedCommands is the command letters that fit the party's surroundings, most useful first.
func SuggestedCommands(w *World, scope CommandScope) []string {
	switch scope {
	case ScopeCombat:
		return combatSuggestions(w)
	case ScopeDungeon:
		return dungeonSuggestions(w)
	}
	return fieldSuggestions(w)
}

func fieldSuggestions(w *World) []string {
	var out []string
	here := w.GetXYVal(w.X, w.Y)
	shape := w.Party.Shape()
	loc := w.Party.Location()
	if loc == LocationSosaria || loc == LocationAmbrosia {
		if here == MapValueTown || here == MapValueCastle || here == MapValueDungeon || here == MapValueShrine {
			out = append(out, "E")
		}
	}
	if shape == PartyShapeOnFoot && (here == MapValueHorse || here == MapValueFrigate) {
		out = append(out, "B")
	}
	if shape == PartyShapeHorse || shape == PartyShapeFrigate {
		out = append(out, "X")
	}
	// A chest left by a monster carries the terrain it stood on in its low bits.
	if here >= MapValueChest && here <= MapValueChest+3 {
		out = append([]string{"G"}, out...)
	}
	// Things one step away.
	person, door, monster := false, false, false
	for _, s := range orthogonalSteps {
		xs, ys := w.Constrain(w.X+s.X), w.Constrain(w.Y+s.Y)
		mon := w.Monsters().At(xs, ys)
		if w.InTownOrCastle() {
			if mon >= 0 || CounterWithMerchant(w, xs, ys, s.X, s.Y) {
				person = true
			}
		} else if mon >= 0 {
			monster = true
		}
		// Ambrosia has doors too; they only open sideways.
		if s.X != 0 && s.Y == 0 && w.GetXYVal(xs, ys) == MapValueLetterI {
			door = true
		}
	}
	if monster {
		out = append(out, "A")
	}
	if monster && shape == PartyShapeFrigate {
		out = append(out, "F")
	}
	if person {
		out = append(out, "T")
	}
	if door {
		out = append(out, "U")
	}
	return out
}

func dungeonSuggestions(w *World) []string {
	var out []string
	if cell := w.GetXYDng(w.X, w.Y); cell < DungeonCellWall {
		if cell&DungeonCellLadderUp != 0 {
			out = append(out, "K")
		}
		if cell&DungeonCellLadderDown != 0 {
			out = append(out, "D")
		}
		if cell&DungeonCellChest != 0 {
			out = append(out, "G")
		}
	}
	if w.Dungeon.Torch == 0 && w.Party.Torches() > 0 {
		out = append(out, "I")
	}
	return out
}

func combatSuggestions(w *World) []string {
	c := w.Combat
	if c == nil {
		return nil
	}
	me := c.Members[c.ActiveMember]
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if dx == 0 && dy == 0 || !w.DiagonalMoves && dx != 0 && dy != 0 {
				continue
			}
			if MonsterAt(c, me.X+dx, me.Y+dy) >= 0 {
				return []string{"A"}
			}
		}
	}
	return nil
}

// Prioritise reorders menu options so the suggested keys come first, in suggestion order.
func Prioritise(options []MenuOption, suggested []string) []MenuOption {
	var first []int
	for _, k := range suggested {
		if i := slices.IndexFunc(options, func(o MenuOption) bool { return o.Key == k }); i >= 0 && !slices.Contains(first, i) {
			first = append(first, i)
		}
	}
	out := make([]MenuOption, 0, len(options))
	for _, i := range first {
		out = append(out, options[i])
	}
	for i, o := range options {
		if !slices.Contains(first, i) {
			out = append(out, o)
		}
	}
	return out
}
