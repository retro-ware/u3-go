package game

import (
	"slices"
	"strings"
)

// Casting: Cast(), ProcessMagic(), Spell() and the spell effects from
// UltimaSpellCombat.c. Wizard spells are 0-15 (keys A-P), cleric spells
// 16-31 (keys A-P), and three post-Exodus wizard extras 32-34 (Q, R, S:
// Terraform, Armageddon, Flotellum). A spell costs its letter index times
// five magic points.

const (
	spellMsgCastByWhom    = 119
	spellMsgNotAMage      = 121
	spellMsgMpTooLow      = 122
	spellMsgSpellType     = 123
	spellMsgClericSpell   = 124
	spellMsgWizardSpell   = 125
	spellMsgCureWhom      = 127
	spellMsgResurrectWhom = 128
	spellMsgRecallWhom    = 129
	spellMsgHealWhom      = 130
	spellMsgDirection     = 85
	spellMsgFailed        = 86
)

// spellSounds is the sound Flashriek() plays for each spell, as an index into spellSoundNames.
var spellSounds = []int{0, 1, 2, 3, 4, 1, 5, 1, 2, 7, 0, 1, 7, 0, 0, 0, 0, 7, 6, 2, 4, 3, 5, 6, 5, 2, 6, 7, 1, 6, 0, 6, 7, 0, 0}

var spellSoundNames = []string{
	SoundBigDeath, SoundImmolate, SoundTorchIgnite, SoundDownwards,
	SoundUpwards, SoundInvocation, SoundHeal, SoundMiscSpell,
}

// Spell choices that are not spells.
const (
	spellCancelled = -2
	spellNotAMage  = -1
)

// extraSpellNames are the book names of spells 32-34.
var extraSpellNames = []string{"TERRAFORM", "ARMAGEDDON", "FLOTELLUM"}

// careerIndex is a member's class as an index into the career table, or -1.
func careerIndex(w *World, p *PlayerRecord) int {
	return slices.Index(w.Resources.Misc.Career, p.ClassLetter())
}

// Cast mirrors Cast(). Outside combat member is -1 and the player is asked
// who casts. It is false if nothing was cast (so combat asks again).
func Cast(w *World, io IO, member int) bool {
	if member < 0 {
		io.PrintMessage(spellMsgCastByWhom)
		n := io.ChooseMember(nil)
		if n == 0 {
			Cancelled(w, io)
		}
		if n < 1 || n > 4 {
			return false
		}
		member = n - 1
		if !w.MemberAlive(member) {
			Incapacitated(io)
			return false
		}
	}
	wizard, cleric := SpellBooks(w, member)
	spell := spellNotAMage
	switch {
	case wizard && cleric:
		spell = chooseEither(w, io, member)
	case cleric:
		spell = chooseCleric(w, io, member)
	case wizard:
		spell = chooseWizard(w, io, member)
	}
	if spell == spellCancelled {
		Cancelled(w, io)
		return false
	}
	if spell == spellNotAMage {
		io.PrintMessage(spellMsgNotAMage)
		return false
	}
	ProcessMagic(w, io, member, spell)
	if w.CommandCancelled {
		return false // backed out at the spell's own prompt, the mana given back
	}
	w.LastSpell[member] = spell // for the combat menu's "Cast (spell)" shortcut
	return true
}

// SpellName is the plain name of a spell, as the menus show it.
func SpellName(spell int) string {
	if spell >= 0 && spell < len(spellInfo) {
		return spellInfo[spell].name
	}
	return "?"
}

// SpellCost is what a spell costs in mana; the three extras have their own prices.
func SpellCost(spell int) int {
	switch spell {
	case 32:
		return 10
	case 33:
		return 85
	case 34:
		return 90
	}
	return (spell & 0x0f) * 5
}

// SpellBooks says which books a member's class casts from: Druids and
// Rangers both, Clerics, Paladins and Illusionists the cleric's, Wizards,
// Larks and Alchemists the wizard's, the rest neither.
func SpellBooks(w *World, member int) (wizard, cleric bool) {
	ci := careerIndex(w, w.Member(member))
	return slices.Contains(wizardCasters, ci), slices.Contains(clericCasters, ci)
}

// repels are the Repel spells, Repond (wizard's A) and Pontori (cleric's A),
// each for one type of foe, once a fight.
var repels = map[int]struct {
	shape int
	flag  func(*SpellFlags) *bool
	foes  string
}{
	0:  {0x30, func(f *SpellFlags) *bool { return &f.Repond }, "orcs"},    // orcs, goblins and trolls
	16: {0x32, func(f *SpellFlags) *bool { return &f.Pontori }, "undead"}, // skeletons, ghouls and zombies
}

// RepelWhyNot is why a Repel spell would fail now: not in a fight, foes not
// of its type, or the fight's one try already spent. "" where it could
// work, and for every other spell.
func RepelWhyNot(w *World, spell int) string {
	r, ok := repels[spell]
	if !ok {
		return ""
	}
	c := w.Combat
	if w.Party.Location() != LocationCombat || c == nil {
		return "Only in a fight."
	}
	if c.MonsterShape != r.shape {
		return "No " + r.foes + " here."
	}
	if *r.flag(&w.SpellFlags) {
		return "Tried already this fight."
	}
	return ""
}

// RepelSpell is the Repel spell member could cast to some purpose now, or -1.
func RepelSpell(w *World, member int) int {
	wizard, cleric := SpellBooks(w, member)
	if wizard && RepelWhyNot(w, 0) == "" {
		return 0
	}
	if cleric && RepelWhyNot(w, 16) == "" {
		return 16
	}
	return -1
}

// RepelCast casts the Repel spell that would work now (the combat menu's
// first line); false where none would.
func RepelCast(w *World, io IO, member int) bool {
	spell := RepelSpell(w, member)
	if spell < 0 {
		return false
	}
	io.Print(SpellName(spell) + "\n")
	ProcessMagic(w, io, member, spell)
	w.LastSpell[member] = spell
	return true
}

// QuickSpell is the spell the combat menu offers as "Cast (spell)": the
// member's last spell, unless it is a Repel spell that cannot work now, else
// Magic bolt for wizard-book classes and Heal for the cleric-only ones. -1
// for a member with no magic.
func QuickSpell(w *World, member int) int {
	if last, ok := w.LastSpell[member]; ok && RepelWhyNot(w, last) == "" {
		return last
	}
	switch careerIndex(w, w.Member(member)) {
	case 2, 6, 8, 9, 10: // Wizard, Lark, Druid, Alchemist, Ranger
		return 1 // Mittar, magic bolt
	case 1, 4, 7: // Cleric, Paladin, Illusionist
		return 18 // Sanctu, heal
	}
	return -1
}

// QuickCast casts the quick spell straight away; false when the member has none.
func QuickCast(w *World, io IO, member int) bool {
	spell := QuickSpell(w, member)
	if spell < 0 {
		return false
	}
	io.Print(SpellName(spell) + "\n")
	ProcessMagic(w, io, member, spell)
	w.LastSpell[member] = spell
	return true
}

// spellInfo is each spell's plain name, for the menus, and what it does,
// for the hint line. Labels stay under 16 characters.
var spellInfo = []struct{ name, does string }{
	{"Repel orcs", "may destroy an orc pack, once a fight"},
	{"Magic bolt", "a bolt at one foe"},
	{"Light", "lights a dungeon for a while"},
	{"Down a level", "sink to the level below"},
	{"Up a level", "rise to the level above"},
	{"Fireball", "a strong bolt at one foe"},
	{"Far teleport", "a random spot on Sosaria"},
	{"Mind bolt", "a bolt as strong as your mind"},
	{"Long light", "lights a dungeon for a long time"},
	{"Cleric spell", "cast any cleric spell"},
	{"Blast all foes", "hurts every foe on the field"},
	{"Death bolt", "slays one foe"},
	{"Stop time", "foes stand still for twenty turns"},
	{"Mind storm", "hurts every foe, twice your mind"},
	{"Weaken all", "every foe drops to 5 hit points"},
	{"Slay all foes", "may slay every foe on the field"},
	{"Repel undead", "may destroy the undead, once a fight"},
	{"Safe chest", "opens a chest without its trap"},
	{"Heal", "heals one member a little"},
	{"Light", "lights a dungeon for a while"},
	{"Up a level", "rise to the level above"},
	{"Down a level", "sink to the level below"},
	{"Near teleport", "a random spot on this level"},
	{"Cure poison", "cures one member"},
	{"Leave dungeon", "straight out to the surface"},
	{"Long light", "lights a dungeon for a long time"},
	{"Great heal", "heals one member a lot"},
	{"Show the map", "a map, as a gem would show"},
	{"Death bolt", "slays one foe"},
	{"Resurrect", "raises the dead; may leave ashes"},
	{"Slay all foes", "may slay every foe on the field"},
	{"Recall ashes", "restores ashes, for 5 wisdom"},
	{"Terraform", "sets the square ahead to any tile"},
	{"Armageddon", "every creature becomes a chest"},
	{"Conjure frigate", "a frigate on the nearest water"},
}

// spellBookName is a spell's name in the book (MITTAR, SANCTU ...).
func spellBookName(w *World, spell int) string {
	if spell >= 32 {
		return extraSpellNames[spell-32]
	}
	if s := w.Resources.Strings["Spells"]; spell < len(s) {
		return s[spell]
	}
	return ""
}

// SpellOption is the menu option for a spell: its letter and plain name,
// and a card with the book's name, its cost against the caster's mana and
// what it does. Greyed when the caster cannot afford it or (a Repel spell)
// it cannot work.
func SpellOption(w *World, spell int, letter string, member int) MenuOption {
	book := strings.TrimSpace(spellBookName(w, spell))
	cost := SpellCost(spell)
	name := SpellName(spell)
	mana := w.Member(member).Mana()
	does := spellInfo[spell].does
	does = strings.ToUpper(does[:1]) + does[1:]
	why := RepelWhyNot(w, spell)
	if book == "" {
		book = "Nameless"
	}
	manaTone := CardToneNone
	if mana < cost {
		manaTone = CardToneWorse
	}
	lines := []CardLine{
		{book, CardToneDim}, {}, {CardRow("Cost", itoa(cost)), ""}, {CardRow("Mana", itoa(mana)), manaTone}, {},
	}
	for _, t := range CardWrap(does + ".") {
		lines = append(lines, CardLine{Text: t})
	}
	if why != "" {
		lines = append(lines, CardLine{})
		for _, t := range CardWrap(why) {
			lines = append(lines, CardLine{t, CardToneWorse})
		}
	}
	return MenuOption{Key: letter, Label: letter + " " + name, Disabled: cost > mana || why != "", Card: &Card{Title: name, Lines: lines}}
}

func chooseEither(w *World, io IO, member int) int {
	io.PrintMessage(spellMsgSpellType)
	key := io.ChooseOption([]MenuOption{{Key: "W", Label: "Wizard spell"}, {Key: "C", Label: "Cleric spell"}}, EchoLine, nil)
	if w.Done {
		return spellCancelled
	}
	switch key {
	case "W":
		return chooseWizard(w, io, member)
	case "C":
		return chooseCleric(w, io, member)
	}
	return spellCancelled
}

func chooseCleric(w *World, io IO, member int) int {
	io.PrintMessage(spellMsgClericSpell)
	var options []MenuOption
	for _, l := range "ABCDEFGHIJKLMNOP" {
		options = append(options, SpellOption(w, int(l-'A')+16, string(l), member))
	}
	key := io.ChooseOption(options, EchoLine, nil)
	if key == "" || w.Done {
		return spellCancelled
	}
	return int(key[0]-'A') + 16
}

func chooseWizard(w *World, io IO, member int) int {
	io.PrintMessage(spellMsgWizardSpell)
	letters := "ABCDEFGHIJKLMNOP"
	if w.Party.ExodusDestroyed() {
		letters += "QRS"
	}
	number := func(l byte) int {
		if l >= 'Q' {
			return int(l-'Q') + 32
		}
		return int(l - 'A')
	}
	var options []MenuOption
	for i := range len(letters) {
		options = append(options, SpellOption(w, number(letters[i]), letters[i:i+1], member))
	}
	key := io.ChooseOption(options, EchoLine, nil)
	if key == "" || w.Done {
		return spellCancelled
	}
	return number(key[0])
}

// payForSpell is the first half of ProcessMagic(): pay and announce. False
// (with "MP too low") when the caster cannot.
func payForSpell(w *World, io IO, member, spell int) bool {
	p := w.Member(member)
	cost := SpellCost(spell)
	if cost > p.Mana() {
		io.PrintMessage(spellMsgMpTooLow)
		return false
	}
	p.SetMana(p.Mana() - cost)
	io.UpdateStats(false)
	io.Print("\n")
	io.Print(spellBookName(w, spell))
	io.Print("\n\n")
	return true
}

// ProcessMagic mirrors ProcessMagic(): pay for the spell, announce it, cast it.
func ProcessMagic(w *World, io IO, member, spell int) {
	if !payForSpell(w, io, member, spell) {
		return
	}
	spellEffect(w, io, member, spell)
}

const (
	Heal      = 18 // Sanctu: 10..30 hit points
	GreatHeal = 26 // Sanctu Mani: 20..100 hit points
	// healTypical is a Heal's typical yield; a larger wound calls for Great heal.
	healTypical = 20
)

// Classes (career-table indices) with each book.
var (
	clericCasters = []int{1, 4, 7, 8, 10} // Cleric, Paladin, Illusionist, Druid, Ranger
	wizardCasters = []int{2, 6, 8, 9, 10} // Wizard, Lark, Druid, Alchemist, Ranger
)

// HealPlan is who heals whom, with what.
type HealPlan struct{ Caster, Spell, Target int }

// HealPlanFor is the healing the party most needs right now, for the menu's
// "Cast (Heal)" shortcut outside combat: the living member missing the most
// hit points is the target; Heal serves a wound up to a Heal's typical yield
// and Great heal a larger one, the other standing in when only it is
// affordable; the caster is the living cleric-book member with the most
// mana who can pay. Nil when nobody is hurt or nobody can help. (healPlan)
func HealPlanFor(w *World) *HealPlan {
	var living []int
	for m := range 4 {
		if w.MemberAlive(m) {
			living = append(living, m)
		}
	}
	gap := func(m int) int { return w.Member(m).MaxHitPoints() - w.Member(m).HitPoints() }
	var wounded []int
	for _, m := range living {
		if gap(m) > 0 {
			wounded = append(wounded, m)
		}
	}
	if len(wounded) == 0 {
		return nil
	}
	slices.SortStableFunc(wounded, func(a, b int) int { return gap(b) - gap(a) })
	target := wounded[0]
	var casters []int
	for _, m := range living {
		if slices.Contains(clericCasters, careerIndex(w, w.Member(m))) {
			casters = append(casters, m)
		}
	}
	slices.SortStableFunc(casters, func(a, b int) int { return w.Member(b).Mana() - w.Member(a).Mana() })
	order := []int{Heal, GreatHeal}
	if gap(target) > healTypical {
		order = []int{GreatHeal, Heal}
	}
	for _, spell := range order {
		for _, m := range casters {
			if w.Member(m).Mana() >= SpellCost(spell) {
				return &HealPlan{m, spell, target}
			}
		}
	}
	return nil
}

// QuickHeal casts the planned heal straight away: no "who casts", no spell
// menu, no "heal whom". False when there is no plan.
func QuickHeal(w *World, io IO) bool {
	plan := HealPlanFor(w)
	if plan == nil {
		return false
	}
	io.Print(SpellName(plan.Spell) + "\non " + w.Member(plan.Target).Name() + "\n") // two lines: the message area is 16 wide
	if !payForSpell(w, io, plan.Caster, plan.Spell) {
		return true
	}
	amount := w.Rng.Range(0, 20) + 10
	if plan.Spell == GreatHeal {
		amount = w.Rng.Range(0, 80) + 20
	}
	healMember(w, io, plan.Spell, amount, plan.Target)
	w.LastSpell[plan.Caster] = plan.Spell
	return true
}

// SafeChest is Appar Unem: opens a chest without its trap.
const SafeChest = 17

// SafeChestCaster is who would cast Safe chest for the menu's shortcut: the
// living cleric-book member with the most mana who can pay. -1 when nobody
// can, or the party is not on a chest.
func SafeChestCaster(w *World) int {
	if !ChestHere(w) {
		return -1
	}
	best := -1
	for m := range 4 {
		p := w.Member(m)
		if !w.MemberAlive(m) || !slices.Contains(clericCasters, careerIndex(w, p)) || p.Mana() < SpellCost(SafeChest) {
			continue
		}
		if best < 0 || p.Mana() > w.Member(best).Mana() {
			best = m
		}
	}
	return best
}

// LightSpells are the light spells, strongest first: Long light (250 turns)
// in either book, then Light (10 turns), cheaper book first.
var LightSpells = []int{8, 25, 2, 19} // Dag Lorum, Sominae, Lorum, Luminae

// LightPlan is who lights the dungeon, with what.
type LightPlan struct{ Caster, Spell int }

// hasBookFor says whether a class (career-table index) has the book a spell is in.
func hasBookFor(career, spell int) bool {
	if spell < 16 {
		return slices.Contains(wizardCasters, career)
	}
	return slices.Contains(clericCasters, career)
}

// LightPlanFor is who lights the dungeon for the menu's "Cast (Long light)"
// or "Cast (Light)" shortcut: the strongest light spell anyone living can
// cast, Long light over Light; among those able, the most mana wins and a
// tie goes to party order. Nil unless the dungeon is dark and someone can. (lightPlan)
func LightPlanFor(w *World) *LightPlan {
	if w.Party.Location() != LocationDungeon || w.Dungeon.Torch > 0 {
		return nil
	}
	canCast := func(m, spell int) bool {
		p := w.Member(m)
		return hasBookFor(careerIndex(w, p), spell) && p.Mana() >= SpellCost(spell)
	}
	for _, tier := range [][]int{LightSpells[:2], LightSpells[2:]} {
		var best *LightPlan
		for m := range 4 {
			if !w.MemberAlive(m) {
				continue
			}
			// Each member casts the tier's cheaper spell they have the book for.
			for _, spell := range tier {
				if canCast(m, spell) {
					if best == nil || w.Member(m).Mana() > w.Member(best.Caster).Mana() {
						best = &LightPlan{m, spell}
					}
					break
				}
			}
		}
		if best != nil {
			return best
		}
	}
	return nil
}

// QuickLight casts the planned light straight away; false when there is no plan.
func QuickLight(w *World, io IO) bool {
	plan := LightPlanFor(w)
	if plan == nil {
		return false
	}
	io.Print(SpellName(plan.Spell) + "\nby " + w.Member(plan.Caster).Name() + "\n")
	ProcessMagic(w, io, plan.Caster, plan.Spell)
	w.LastSpell[plan.Caster] = plan.Spell
	return true
}

// QuickSafeChest casts Safe chest on the chest underfoot straight away;
// false when there is no caster or no chest.
func QuickSafeChest(w *World, io IO) bool {
	caster := SafeChestCaster(w)
	if caster < 0 {
		return false
	}
	io.Print(SpellName(SafeChest) + "\nby " + w.Member(caster).Name() + "\n")
	ProcessMagic(w, io, caster, SafeChest)
	w.LastSpell[caster] = SafeChest
	return true
}

// Failed mirrors Failed().
func Failed(io IO) {
	io.PrintMessage(spellMsgFailed)
	io.Sound(SoundFailedSpell)
}

// refund: backed out at a spell's own prompt after paying, so the mana
// comes back and the turn is not spent (the original said "Failed").
func refund(w *World, io IO, caster, spell int) {
	p := w.Member(caster)
	p.SetMana(p.Mana() + SpellCost(spell))
	io.UpdateStats(false)
	Cancelled(w, io)
}

// SpellFlashes is where each spell's flash lands with the Standard tiles:
// bolts and self-directed spells the caster's square, heals the
// recipient's, everything else the whole view. (SPELL_FLASH)
var SpellFlashes = map[int]SpellFlash{
	1: FlashCaster, 5: FlashCaster, 7: FlashCaster, 9: FlashCaster, 11: FlashCaster, // Mittar Fulgar Mentar Fal Divi Decorp
	17: FlashCaster, 27: FlashCaster, 28: FlashCaster, // Appar Unem, Vieda, Excuun
	18: FlashTarget, 23: FlashTarget, 26: FlashTarget, 29: FlashTarget, 31: FlashTarget, // the heals and raisings
}

// flashriek mirrors Flashriek(): flash and sound. member is the caster, or a heal's recipient.
func flashriek(io IO, spell, member int) {
	kind, ok := SpellFlashes[spell]
	if !ok {
		kind = FlashWindow
	}
	sound := 7
	if spell < len(spellSounds) {
		sound = spellSounds[spell]
	}
	io.FlashSpell(kind, member, spellSoundNames[sound])
}

// spellEffect mirrors Spell(): the effect of each spell.
func spellEffect(w *World, io IO, member, spell int) {
	if w.Done {
		return
	}
	p := w.Member(member)
	inCombat := w.Party.Location() == LocationCombat
	c := w.Combat

	// chooseTarget asks whom (heal, cure, raise): the 0-based member, or -1
	// after refunding or failing.
	chooseTarget := func(msg int) int {
		io.PrintMessage(msg)
		n := io.ChooseMember(nil)
		if n == 0 {
			refund(w, io, member, spell)
			return -1
		}
		if n < 1 || n > 4 {
			Failed(io)
			return -1
		}
		return n - 1
	}

	switch spell {
	case 0: // Repond: destroys orcs (once per fight)
		if !inCombat || c == nil || c.MonsterShape != 0x30 || w.SpellFlags.Repond {
			Failed(io)
			return
		}
		w.SpellFlags.Repond = true
		if w.Rng.Range(0, 255) < 128 {
			Failed(io)
			return
		}
		bigDeath(w, io, member, spell, 255)
	case 1: // Mittar
		projectile(w, io, member, spell, w.Rng.Range(0, 40)|0x10)
	case 2, 19: // Lorum, Luminae
		w.Dungeon.Torch = 10
		flashriek(io, spell, member)
	case 3, 21: // Dor Acron, Rec Du
		downLevel(w, io, spell)
	case 4, 20: // Sur Acron, Rec Su
		upLevel(w, io, spell)
	case 5: // Fulgar
		projectile(w, io, member, spell, 75)
	case 6: // Dag Acron: teleport to a random spot on Sosaria
		flashriek(io, spell, member)
		if !w.OnSurface() || (w.Party.Shape() == 0x16 && !w.Party.ExodusDestroyed()) {
			Failed(io)
			return
		}
		wanted := MapValueGrass
		if w.Party.Shape() == 0x16 {
			wanted = MapValueWater
		}
		for value := -1; value != wanted; {
			w.X = w.Rng.Range(0, w.MapSize()-1)
			w.Y = w.Rng.Range(0, w.MapSize()-1)
			value = w.GetXYVal(w.X, w.Y)
		}
	case 7: // Mentar
		projectile(w, io, member, spell, p.Intelligence())
	case 8, 25: // Dag Lorum, Sominae
		w.Dungeon.Torch = 250
		flashriek(io, spell, member)
	case 9: // Fal Divi: cast any cleric spell
		flashriek(io, spell, member)
		if chosen := chooseCleric(w, io, member); chosen >= 0 {
			ProcessMagic(w, io, member, chosen)
		}
	case 10: // Noxum
		bigDeath(w, io, member, spell, 75)
	case 11, 28: // Decorp, Excuun
		projectile(w, io, member, spell, 255)
	case 12: // Altair
		w.TimeNegate = 20
	case 13: // Dag Mentar
		bigDeath(w, io, member, spell, p.Intelligence()*2)
	case 14: // Necorp: every monster drops to 5 hit points
		necorp(w, io, spell)
	case 15, 30: // (nameless), ZXKUQYB
		bigDeath(w, io, member, spell, 255)
	case 16: // Pontori: destroys undead (once per fight)
		if !inCombat || c == nil || c.MonsterShape != 0x32 || w.SpellFlags.Pontori {
			Failed(io)
			return
		}
		w.SpellFlags.Pontori = true
		if w.Rng.Range(0, 255) < 128 {
			Failed(io)
			return
		}
		bigDeath(w, io, member, spell, 255)
	case 17: // Appar Unem: open a chest safely
		flashriek(io, spell, member)
		if w.Rng.Range(0, 255)&0x03 == 0 {
			Failed(io)
			return
		}
		w.ChestTrapsArmed = false
		GetChest(w, io, member, ChestBySpell)
	case 18: // Sanctu
		heal(w, io, member, spell, w.Rng.Range(0, 20)+10)
	case 22: // Lib Rec: random spot on this dungeon level
		flashriek(io, spell, member)
		if w.Party.Location() != LocationDungeon {
			Failed(io)
			return
		}
		RelocateInDungeon(w)
	case 23: // Alcort: cure poison
		t := chooseTarget(spellMsgCureWhom)
		if t < 0 {
			return
		}
		io.FlashMember(t)
		flashriek(io, spell, t)
		target := w.Member(t)
		if target.Status() != 'P' {
			Failed(io)
			return
		}
		target.SetStatus('G')
	case 24: // Sequitu: leave the dungeon
		if w.Party.Location() != LocationDungeon {
			Failed(io)
			return
		}
		flashriek(io, spell, member)
		w.Dungeon.Level = 0
		w.Dungeon.Exit = true
	case 26: // Sanctu Mani
		heal(w, io, member, spell, w.Rng.Range(0, 80)+20)
	case 27: // Vieda: show the map
		flashriek(io, spell, member)
		if w.Party.Location() == LocationDungeon {
			io.ShowMiniDungeon()
		} else {
			io.ShowMiniMap()
		}
	case 29: // Surmandum: resurrect
		if inCombat {
			Failed(io)
			return
		}
		t := chooseTarget(spellMsgResurrectWhom)
		if t < 0 {
			return
		}
		io.FlashMember(t)
		flashriek(io, spell, t)
		target := w.Member(t)
		if target.Status() != 'D' {
			Failed(io)
			return
		}
		if w.Rng.Range(0, 255)&0x03 == 0 {
			target.SetStatus('A')
			Failed(io)
			return
		}
		target.SetStatus('G')
	case 31: // Anju Sermani: recall from ashes, at the cost of 5 wisdom
		if inCombat {
			Failed(io)
			return
		}
		t := chooseTarget(spellMsgRecallWhom)
		if t < 0 {
			return
		}
		io.FlashMember(t)
		flashriek(io, spell, t)
		target := w.Member(t)
		if target.Status() != 'A' {
			Failed(io)
			return
		}
		target.SetStatus('G')
		p.Bytes[21] -= 5
	case 32: // Terraform: place any tile (or creature) on an adjacent square
		if inCombat {
			Failed(io)
			return
		}
		io.PrintMessage(spellMsgDirection)
		dir := GetDirection(w, io, false, w.DiagonalMoves)
		if dir == nil {
			refund(w, io, member, spell)
			return
		}
		io.Print("TypeNum-")
		value := InputNumber(io, 2)
		flashriek(io, spell, member)
		x, y := w.Constrain(w.X+dir.DX), w.Constrain(w.Y+dir.DY)
		if value > 12 && value < 31 {
			m := w.Monsters()
			if slot := m.FreeSlot(); slot >= 0 {
				m.SetType(slot, value*4)
				m.SetTileUnder(slot, w.GetXYVal(x, y))
				m.SetPosition(slot, x, y)
				m.SetHP(slot, 0x40)
				m.SetVariant(slot, 0)
			}
		}
		w.PutXYVal(value*4, x, y)
		io.Print("\n")
	case 33: // Armageddon: every creature on the map becomes a chest
		if inCombat {
			Failed(io)
			return
		}
		flashriek(io, spell, member)
		m := w.Monsters()
		for i := range 32 {
			if m.Type(i) == 0 {
				continue
			}
			under := m.TileUnder(i)
			if under != 0 {
				under = (under>>2)&0x03 + MapValueChest
			}
			w.PutXYVal(under, m.X(i), m.Y(i))
			m.Clear(i)
		}
	case 34: // Flotellum: conjure a frigate on the nearest water
		if inCombat {
			Failed(io)
			return
		}
		best, destX, destY := 99, -1, -1
		for ty := -5; ty <= 5; ty++ {
			for tx := -5; tx <= 5; tx++ {
				if tx == 0 && ty == 0 || w.GetXYVal(w.X+tx, w.Y+ty) != MapValueWater {
					continue
				}
				if d := abs(tx) + abs(ty); d < best {
					best, destX, destY = d, w.Constrain(w.X+tx), w.Constrain(w.Y+ty)
				}
			}
		}
		if destX < 0 {
			Failed(io)
			return
		}
		flashriek(io, spell, member)
		w.PutXYVal(MapValueFrigate, destX, destY)
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// projectile mirrors Projectile(): a bolt in a chosen direction that damages the first monster hit.
func projectile(w *World, io IO, member, spell, damage int) {
	c := w.Combat
	if c == nil {
		Failed(io)
		return
	}
	io.PrintMessage(spellMsgDirection)
	dir := GetDirection(w, io, false, w.DiagonalMoves)
	if dir == nil {
		refund(w, io, member, spell)
		return
	}
	flashriek(io, spell, member)
	me := c.Members[member]
	hit := Shoot(w, io, me.X, me.Y, dir.DX, dir.DY, ShapeMagicBall)
	if hit < 0 {
		Failed(io)
		return
	}
	io.Sound(SoundHit)
	ShowBall(w, io, c.Monsters[hit].X, c.Monsters[hit].Y, ShapeMagicBall, true)
	DamageMonster(w, io, hit, damage, member)
}

// bigDeath mirrors BigDeath(): damage every monster, each with a 3 in 4 chance of being struck.
func bigDeath(w *World, io IO, member, spell, damage int) {
	c := w.Combat
	if c == nil {
		Failed(io)
		return
	}
	flashriek(io, spell, member)
	for i := 7; i >= 0; i-- {
		if w.Rng.Range(0, 255)&0x03 == 0 {
			continue
		}
		m := c.Monsters[i]
		if m.HP == 0 {
			continue
		}
		io.Sound(SoundHit)
		ShowBall(w, io, m.X, m.Y, ShapeMagicBall, true)
		DamageMonster(w, io, i, damage, member)
	}
}

// necorp mirrors Necorp().
func necorp(w *World, io IO, spell int) {
	c := w.Combat
	if c == nil {
		Failed(io)
		return
	}
	flashriek(io, spell, 0)
	for i := 7; i >= 0; i-- {
		m := c.Monsters[i]
		if m.HP == 0 {
			continue
		}
		m.HP = 5
		io.Sound(SoundHit)
		ShowBall(w, io, m.X, m.Y, ShapeMagicBall, true)
	}
}

// heal mirrors Heal(): ask whom, then heal them.
func heal(w *World, io IO, caster, spell, amount int) {
	io.PrintMessage(spellMsgHealWhom)
	n := io.ChooseMember(nil)
	if n == 0 {
		refund(w, io, caster, spell)
		return
	}
	if n < 1 || n > 4 {
		Failed(io)
		return
	}
	healMember(w, io, spell, amount, n-1)
}

func healMember(w *World, io IO, spell, amount, target int) {
	w.Member(target).AddHitPoints(amount)
	io.FlashMember(target)
	flashriek(io, spell, target)
	io.UpdateStats(false)
}

// RelocateInDungeon mirrors RelocateDungeon(): a random open cell on the current level.
func RelocateInDungeon(w *World) {
	xs, ys := 0, 0
	for value := -1; value != 0; {
		xs = w.Rng.Range(0, 15)
		ys = w.Rng.Range(0, 15)
		value = w.GetXYDng(xs, ys)
	}
	w.X, w.Y = xs, ys
}

// downLevel mirrors DownLevel().
func downLevel(w *World, io IO, spell int) {
	if w.Party.Location() != LocationDungeon {
		Failed(io)
		return
	}
	flashriek(io, spell, 0)
	if w.Dungeon.Level > 6 {
		Failed(io)
		return
	}
	w.Dungeon.Level++
	RelocateInDungeon(w)
}

// upLevel mirrors UpLevel(): from the top level this leaves the dungeon.
func upLevel(w *World, io IO, spell int) {
	if w.Party.Location() != LocationDungeon {
		Failed(io)
		return
	}
	flashriek(io, spell, 0)
	w.Dungeon.Level--
	if w.Dungeon.Level < 0 {
		w.Dungeon.Level = 0
		w.Dungeon.Exit = true
		return
	}
	RelocateInDungeon(w)
}
