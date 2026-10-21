package game

import (
	"slices"
	"strconv"
	"strings"
)

// Tactical combat: Combat(), CombatAttack(), DamageMonster(),
// FigureNewMonPosition(), Shoot() and their helpers from
// UltimaSpellCombat.c, plus AttackCode() from UltimaMain.c.
//
// A fight happens on an 11x11 arena chosen by the terrain (CONS resources).
// Up to four members and eight monsters take turns: each living member gets
// a command, then every monster moves, shoots, casts or attacks, until all
// monsters are dead or the whole party is. While World.Combat is set the
// viewport draws the arena instead of the map.

const (
	combatMsgKilled           = 131 // "KILLED! Exp.+"
	combatMsgVictory          = 132
	combatMsgConflict         = 133
	combatMsgPlayerTurnPrefix = 134 // "----PLAYER-"
	combatMsgPlayerTurnSuffix = 135 // "----\n "
	combatMsgCastSpell        = 136
	combatMsgReadyWeapon      = 138
	combatMsgZtats            = 139
	combatMsgNotUsable        = 140
	combatMsgPlr              = 141
	combatMsgMonsterMissed    = 142
	combatMsgMonsterHit       = 143
	combatMsgPlayerKilled     = 144
	combatMsgAttackDir        = 145 // " attack\nDir-"
	combatMsgHit              = 146
	combatMsgMissed           = 147
	combatMsgPilfered         = 148
	combatMsgPoisoned         = 149
	combatMsgPass             = 23  // commands' "Pass"
	combatMsgInvalidMove      = 116 // commands' "INVALID MOVE!"
)

// monsterHP is the monsters' starting hit points by tile index & 0x0F. (monhpstart)
var monsterHP = []int{0x20, 0x20, 0xf0, 0xf0, 0xc0, 0x60, 0xa0, 0x80, 0x30, 0x50, 0x70, 0xa0, 0xc0, 0xe0, 0xf0, 0xf0}

const (
	missing       = 255
	turnTimeoutMs = 4000 // how long a member has to act before the turn passes
	ballMs        = 80
	hitFrames     = 3 // a hit shows three frames of impact
	hitFrameMs    = 60
	autoPauseMs   = 250 // pause before each automatic turn, so the player can interrupt
)

// ChooseArena mirrors BackGround(): the CONS resource for the fight from
// the monster type and the terrain. 400 + n, n: 0 water, 1 brush, 2 floor,
// 3 forest, 4 grass, 5 mountains/shore, 6 ship vs land, 7 ship vs ship,
// 8 frigate boarding. Boarding a frigate means fighting thieves, so the
// monster shape may change.
func ChooseArena(w *World, monsterShape int) (id, shape int) {
	onShip := w.Party.Shape() == 0x16
	switch {
	case w.Party.Location() == LocationDungeon, monsterShape == ShapeFloor:
		return BaseRes + 2, monsterShape
	case monsterShape == ShapeGrass:
		return BaseRes + 4, monsterShape
	case monsterShape == 0x1e: // a pirate frigate: the crew are thieves
		if onShip {
			return BaseRes + 8, 0x2e
		}
		return BaseRes, 0x2e
	case onShip && monsterShape < 0x20:
		return BaseRes + 6, monsterShape
	case onShip:
		return BaseRes + 7, monsterShape
	case monsterShape < 0x20 && monsterShape > 0x14:
		return BaseRes + 5, monsterShape
	}
	switch w.GetXYVal(w.X, w.Y) >> 1 {
	case ShapeBrush:
		return BaseRes + 1, monsterShape
	case ShapeForest:
		return BaseRes + 3, monsterShape
	case ShapeFloor, ShapeForceField, ShapeLava:
		return BaseRes + 2, monsterShape
	}
	return BaseRes + 4, monsterShape
}

// LoadArena mirrors GetScreen(): a CONS resource is 121 arena shapes, then
// at 0x80 eight monster X, 0x88 Y, 0x90 tiles, 0x98 HP, then at 0xA0 four
// member X, 0xA4 Y, 0xA8 tiles, 0xAC shapes.
func LoadArena(w *World, id int) *CombatState {
	raw, ok := w.Resources.Combat[id]
	if !ok {
		panic("no combat arena " + strconv.Itoa(id))
	}
	c := &CombatState{
		Tiles:            slices.Clone(raw[:121]),
		PreviousLocation: w.Party.Location(),
		MarkedMember:     -1,
	}
	for i := range 8 {
		c.Monsters = append(c.Monsters, &Combatant{X: int(raw[0x80+i]), Y: int(raw[0x88+i]), TileUnder: int(raw[0x90+i])})
	}
	for i := range 4 {
		x, y := int(raw[0xa0+i]), int(raw[0xa4+i])
		c.Members = append(c.Members, &Combatant{X: x, Y: y, TileUnder: int(raw[0xa8+i]), Shape: int(raw[0xac+i]), Start: &Point{x, y}})
	}
	return c
}

// extraMonsters mirrors HowMany(): the number of monsters beyond the first.
func extraMonsters(w *World, monsterShape int) int {
	switch {
	case monsterShape == 0x26: // Lord British fights alone
		return 0
	case monsterShape == 0x24, w.InExodusCastle(): // guards come in force
		return 7
	}
	if loc := w.Party.Location(); loc < LocationTown || loc > LocationCastle {
		return w.Rng.Range(0, 7)
	}
	return 0
}

// MemberShape is the figure for a class letter: its own tile (68-78) as a shape.
func MemberShape(w *World, classLetter byte) int {
	return ClassTile(slices.Index(w.Resources.Misc.Career, classLetter)) * 2
}

// monsterDisplayShape is the shape drawn for the monsters, including variant rows.
func monsterDisplayShape(c *CombatState) int {
	s := c.MonsterShape
	if c.MonsterVariant != 0 && s >= 46 && s <= 63 {
		return (((s>>1)-23)*2 + 79 + c.MonsterVariant) * 2
	}
	return s
}

// MonsterName is the name of the monster type being fought, singular or plural. (PrintMonster)
func MonsterName(w *World, shape, variant int, plural bool) string {
	names := w.Resources.Strings["Tiles"]
	if plural {
		names = w.Resources.Strings["TilesPlural"]
	}
	i := shape >> 1
	if shape > 44 && variant > 0 {
		i = shape - 46 + 63 + variant
	}
	if i < 0 || i >= len(names) {
		return "?"
	}
	return names[i]
}

// ArenaTile is the arena shape at (x, y), 0 off the arena.
func ArenaTile(c *CombatState, x, y int) int {
	if x < 0 || x > 10 || y < 0 || y > 10 {
		return 0
	}
	return int(c.Tiles[y*ViewSize+x])
}

// MonsterAt mirrors CombatMonsterHere(): the living monster at (x, y), or -1.
func MonsterAt(c *CombatState, x, y int) int {
	for i := 7; i >= 0; i-- {
		if m := c.Monsters[i]; m.HP > 0 && m.X == x && m.Y == y {
			return i
		}
	}
	return -1
}

// MemberAt mirrors CombatCharacterHere(): the member at (x, y), or -1.
func MemberAt(c *CombatState, x, y int) int {
	for i := 3; i >= 0; i-- {
		if c.Members[i].X == x && c.Members[i].Y == y {
			return i
		}
	}
	return -1
}

// ShowBall draws a ball at an arena cell for one frame or, for a hit, the
// three frames of the impact (the "HIT" tile on the Apple II).
func ShowBall(w *World, io IO, x, y, shape int, hit bool) {
	if !hit {
		w.Ball = &Ball{X: x, Y: y, Shape: shape}
		io.RedrawMap()
		io.Pause(ballMs)
	} else {
		for frame := 1; frame <= hitFrames; frame++ {
			w.Ball = &Ball{X: x, Y: y, Shape: shape, Hit: frame}
			io.RedrawMap()
			io.Pause(hitFrameMs)
		}
	}
	w.Ball = nil
	io.RedrawMap()
}

// Shoot mirrors Shoot(): fly a ball from (x, y) along (dx, dy) until it
// leaves the arena or reaches a monster: that monster, or -1.
func Shoot(w *World, io IO, x, y, dx, dy, shape int) int {
	c := w.Combat
	for {
		x += dx
		y += dy
		if x < 0 || x > 10 || y < 0 || y > 10 {
			io.RedrawMap()
			return -1
		}
		ShowBall(w, io, x, y, shape, false)
		if mon := MonsterAt(c, x, y); mon >= 0 {
			return mon
		}
	}
}

// XPShare is one member's share of a kill's experience.
type XPShare struct{ Member, Share int }

// ShareExperience says who gets a kill's experience: with Balanced XP the
// living members share it, the killer then the others in marching order
// taking the odd points; otherwise the killer alone, as on the Apple II.
func ShareExperience(w *World, killer, exp int) []XPShare {
	if !w.BalancedXP {
		return []XPShare{{killer, exp}}
	}
	var living []int
	for _, m := range append([]int{killer}, slices.DeleteFunc([]int{0, 1, 2, 3}, func(m int) bool { return m == killer })...) {
		if w.MemberAlive(m) {
			living = append(living, m)
		}
	}
	if len(living) == 0 {
		return []XPShare{{killer, exp}}
	}
	each, odd := exp/len(living), exp%len(living)
	var out []XPShare
	for i, m := range living {
		share := each
		if i < odd {
			share++
		}
		if share > 0 {
			out = append(out, XPShare{m, share})
		}
	}
	return out
}

// DamageMonster mirrors DamageMonster(). Lord British cannot be hurt. A
// kill awards the experience for the monster's tier (ShareExperience).
func DamageMonster(w *World, io IO, which, damage, member int) {
	c := w.Combat
	if c.MonsterShape == 0x26 {
		return
	}
	m := c.Monsters[which]
	if m.HP-damage < 1 {
		exp := int(w.Resources.Misc.Experience[(c.MonsterShape>>1)&0x0f])
		io.PrintMessage(combatMsgKilled)
		io.Print(strconv.Itoa(exp) + "\n")
		for _, s := range ShareExperience(w, member, exp) {
			AddExperience(w, io, s.Member, s.Share)
		}
		m.HP = 0
		io.RedrawMap()
	} else {
		m.HP -= damage
	}
}

// showHit mirrors ShowHit() and the hit sound.
func showHit(w *World, io IO, x, y, shape int) {
	io.Sound(SoundHit)
	ShowBall(w, io, x, y, shape, true)
}

// monsterCanStand mirrors CombatValidMove(): may this monster type step onto this arena shape?
func monsterCanStand(monsterShape, tile int) bool {
	if monsterShape >= 0x16 && monsterShape < 0x20 {
		return tile == ShapeWater
	}
	return memberCanStand(tile)
}

// memberCanStand mirrors ValidMove(): may a member step onto this arena shape?
func memberCanStand(tile int) bool {
	return tile == ShapeGrass || tile == ShapeBrush || tile == ShapeForest || tile == ShapeFloor
}

// AttackMonster mirrors AttackCode(): a monster on the map is engaged. It
// leaves a chest on the terrain it stood on (pirates their frigate) and
// the fight begins.
func AttackMonster(w *World, io IO, index int) {
	m := w.Monsters()
	x, y := m.X(index), m.Y(index)
	under := m.TileUnder(index)
	if under != 0 {
		under = ((under >> 2) & 0x03) + MapValueChest
	}
	w.PutXYVal(under, x, y)
	shape, variant := m.Type(index)>>1, m.Variant(index)
	m.Clear(index)
	if shape == 0x1e && w.Party.Shape() != 0x16 {
		w.PutXYVal(MapValueFrigate, x, y)
	}
	io.RedrawMap()
	Combat(w, io, shape, variant)
}

// Combat mirrors Combat(): set up the arena, then alternate member and
// monster turns until one side is gone.
func Combat(w *World, io IO, monsterShape, variant int) {
	previousMusic := w.Music
	w.SpellFlags = SpellFlags{}
	io.Music(MusicNone)

	// Fighting in a town turns Lord British's guards hostile.
	mt := w.Monsters()
	for i := 31; i >= 0; i-- {
		if mt.Type(i) == MapValueLordBritish || mt.Type(i) == MapValueGuard {
			mt.SetHP(i, 0xc0)
		}
	}

	io.PrintMessage(combatMsgConflict)
	count := extraMonsters(w, monsterShape)
	io.Print(MonsterName(w, monsterShape, variant, count > 0))
	io.Print("\n\n")

	id, shapeFought := ChooseArena(w, monsterShape)
	c := LoadArena(w, id)
	c.MonsterShape = shapeFought
	c.MonsterVariant = variant
	c.PreviousLocation = w.Party.Location()
	w.Combat = c
	w.Party.SetLocation(LocationCombat)
	io.ShowWind()

	// Place the living members; dead ones sit out.
	for i := w.Party.Size() - 1; i >= 0; i-- {
		p := c.Members[i]
		if w.MemberAlive(i) {
			p.Shape = MemberShape(w, w.Member(i).ClassLetter())
			p.TileUnder = ArenaTile(c, p.X, p.Y)
		} else {
			p.X, p.Y = missing, missing
		}
	}
	for i := w.Party.Size(); i < 4; i++ {
		c.Members[i].X, c.Members[i].Y = missing, missing
	}

	// Place the monsters in random arena slots.
	shape := monsterDisplayShape(c)
	for remaining := count; remaining >= 0; {
		m := c.Monsters[w.Rng.Range(0, 7)]
		if m.HP == 0 {
			m.HP = w.Rng.Range(0, monsterHP[(c.MonsterShape>>1)&0x0f]) | 0x0f
			m.TileUnder = ArenaTile(c, m.X, m.Y)
			m.Shape = shape
			remaining--
		}
	}
	io.RedrawMap()
	io.Sound(SoundCombatStart)
	io.FlushKeys()
	io.Music(MusicCombat)

	defer func() {
		io.QueueKeys(nil)
		if w.Combat == c {
			w.Combat = nil
			c.MarkedMember = -1
		}
	}()
	for {
		// A new round: everyone ages, then each member acts.
		AgeChars(w, io)
		io.UpdateStats(false)
		for member := 0; member < w.Party.Size(); member++ {
			c.ActiveMember = member
			if w.Resurrecting {
				return
			}
			PlaceRevived(w, io)
			io.HighlightMember(member, true)
			if w.MemberAlive(member) {
				memberTurn(w, io, member)
			}
			// Inside Exodus' castle, time cannot be negated.
			if w.InExodusCastle() {
				w.TimeNegate = 0
			}
			c.MarkedMember = -1
			io.RedrawMap()
			io.HighlightMember(member, false)
			io.UpdateStats(false)
			if !slices.ContainsFunc(c.Monsters, func(m *Combatant) bool { return m.HP != 0 }) {
				victory(w, io, previousMusic)
				return
			}
		}
		// Time stopped: monsters lose their turn.
		if w.TimeNegate > 0 {
			w.TimeNegate--
			continue
		}
		for i := range 8 {
			if w.Resurrecting {
				break
			}
			if c.Monsters[i].HP != 0 {
				monsterTurn(w, io, i)
			}
		}
	}
}

// victory mirrors Victory().
func victory(w *World, io IO, previousMusic int) {
	c := w.Combat
	w.TimeNegate = 0
	io.PrintMessage(combatMsgVictory)
	io.Sound(SoundCombatVictory)
	w.Party.SetLocation(c.PreviousLocation)
	w.Combat = nil
	io.Music(previousMusic)
	io.ShowWind()
	io.RedrawMap()
}

// waitForCombatKey waits for the member's command while the UI marks their
// figure. After the turn timer the turn passes (a space), as on the Apple
// II; with the timer off the wait has no end.
func waitForCombatKey(w *World, io IO, member int) string {
	c := w.Combat
	c.MarkedMember = member
	c.MarkedAt = Now()
	limit := w.TimeLimit(turnTimeoutMs)
	c.MarkedFor = float64(limit) // 0: no countdown, the outline stays white
	c.MarkedFrozenAt = 0
	io.RedrawMap()
	defer func() {
		c.MarkedFor = 0 // the outline stays, steady, while the command's prompts run
		io.RedrawMap()
	}()
	if key := io.WaitCommand(ScopeCombat, limit); key != "" {
		return key
	}
	return KeySpace
}

// scriptTurn is auto-combat: a moment to interrupt (Escape, or B on a
// controller, turns it off as Cmd-. did on the Mac), then the member's turn
// is scripted as keys the normal prompts read.
func scriptTurn(w *World, io IO, member int) {
	pressed := io.WaitKeyOrTimeout(autoPauseMs)
	// The system buttons pause instead of interrupting.
	if pressed == KeyPause {
		io.ShowPause()
	}
	if pressed == KeyEscape || pressed == KeyB {
		w.AutoCombat = false
		if w.OnAutoCombatChange != nil {
			w.OnAutoCombatChange()
		}
		io.Print("Manual combat\n")
		return
	}
	io.QueueKeys(AutoCombatKeys(w, member))
}

func isLetterKey(key string) bool {
	return len(key) == 1 && (key[0] >= 'A' && key[0] <= 'Z' || key[0] >= 'a' && key[0] <= 'z')
}

// memberTurn reads and runs one member's command.
func memberTurn(w *World, io IO, member int) {
	c := w.Combat
	for {
		w.CommandCancelled = false
		io.PrintMessage(combatMsgPlayerTurnPrefix)
		io.Print(strconv.Itoa(member + 1))
		io.PrintMessage(combatMsgPlayerTurnSuffix)
		io.Prompt()

		if w.AutoCombat {
			scriptTurn(w, io, member)
		}
		key := waitForCombatKey(w, io, member)
		if key == KeyEscape || key == KeyPause {
			io.ShowPause() // the turn is not spent, and its timer stops with the game
			continue
		}
		if !w.DiagonalMoves && slices.Contains(DiagonalKeys, key) {
			return // refused, and the turn is spent
		}
		if move := MoveForKey(key, w.DiagonalMoves); move != "" {
			var delta combatStep
			delta.DX, delta.DY, delta.Message = MoveDelta(move)
			// Walking into a monster attacks it (a convenience of the TypeScript port).
			me := c.Members[member]
			if MonsterAt(c, me.X+delta.DX, me.Y+delta.DY) >= 0 {
				combatAttack(w, io, member, &delta)
				return
			}
			io.PrintMessage(delta.Message)
			HandleMove(w, io, member, delta.DX, delta.DY)
			return
		}
		switch strings.ToUpper(key) {
		case " ":
			io.PrintMessage(combatMsgPass)
			return
		case "A":
			combatAttack(w, io, member, nil)
			if w.TakeCancelled() {
				continue // backed out at the direction: ask again
			}
			return
		case "C":
			io.PrintMessage(combatMsgCastSpell)
			if Cast(w, io, member) {
				return
			}
			io.Print(" ")
			continue // cancelled: ask again
		case QuickCastKey:
			io.PrintMessage(combatMsgCastSpell)
			if !QuickCast(w, io, member) {
				io.PrintMessage(combatMsgNotUsable)
				io.Sound(SoundError2)
				continue
			}
			if w.TakeCancelled() {
				continue // backed out at the spell's direction, the mana given back
			}
			return
		case RepelKey:
			// The Repel spell that would work now, as the menu's first line offers it.
			io.PrintMessage(combatMsgCastSpell)
			if !RepelCast(w, io, member) {
				io.PrintMessage(combatMsgNotUsable)
				io.Sound(SoundError2)
				continue
			}
			return
		case "N":
			NegateTime(w, io)
			return
		case "R":
			io.PrintMessage(combatMsgReadyWeapon)
			ReadyWeapon(w, io, member)
			if w.TakeCancelled() {
				continue // backed out of the list: ask again
			}
			return
		case "V":
			Volume(w, io)
			return
		case "Z":
			io.PrintMessage(combatMsgZtats)
			Stats(w, io, member)
			continue // looking at the sheet is not acting: the turn is not spent
		}
		if !isLetterKey(key) {
			What2(io)
			return
		}
		io.PrintMessage(combatMsgNotUsable)
		io.Sound(SoundError2)
	}
}

// PlaceRevived puts a member brought back to life during a fight on the
// nearest open square to where they fell, or to their starting square.
func PlaceRevived(w *World, io IO) {
	c := w.Combat
	placed := false
	for member := 0; member < w.Party.Size(); member++ {
		me := c.Members[member]
		if !w.MemberAlive(member) || me.X != missing {
			continue
		}
		from := Point{5, 9}
		if me.DiedAt != nil {
			from = *me.DiedAt
		} else if me.Start != nil {
			from = *me.Start
		}
		spot, ok := nearestOpenSquare(c, from.X, from.Y)
		if !ok {
			continue
		}
		me.X, me.Y = spot.X, spot.Y
		me.TileUnder = ArenaTile(c, spot.X, spot.Y)
		me.Shape = MemberShape(w, w.Member(member).ClassLetter())
		placed = true
	}
	if placed {
		io.RedrawMap()
	}
}

// nearestOpenSquare is the open arena square nearest (x, y), by distance
// then reading order; false if the arena is full.
func nearestOpenSquare(c *CombatState, x, y int) (Point, bool) {
	var best Point
	bestDistance := -1
	for sy := range 11 {
		for sx := range 11 {
			if !memberCanStand(ArenaTile(c, sx, sy)) || MonsterAt(c, sx, sy) >= 0 || MemberAt(c, sx, sy) >= 0 {
				continue
			}
			if d := max(abs(sx-x), abs(sy-y)); bestDistance < 0 || d < bestDistance {
				bestDistance, best = d, Point{sx, sy}
			}
		}
	}
	return best, bestDistance >= 0
}

func sign(v int) int {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	}
	return 0
}

// HandleMove mirrors HandleMove(): step a member one cell if the arena allows.
func HandleMove(w *World, io IO, member, dx, dy int) {
	c := w.Combat
	p := c.Members[member]
	xs, ys := p.X+dx, p.Y+dy
	if xs < 0 || xs > 10 || ys < 0 || ys > 10 || !memberCanStand(ArenaTile(c, xs, ys)) ||
		MonsterAt(c, xs, ys) >= 0 || MemberAt(c, xs, ys) >= 0 {
		io.PrintMessage(combatMsgInvalidMove)
		io.Sound(SoundBump)
		return
	}
	io.Sound(SoundStep)
	p.X, p.Y = xs, ys
	p.TileUnder = ArenaTile(c, xs, ys)
}

// combatStep is a preset attack direction and the message that names it.
type combatStep struct{ DX, DY, Message int }

// CombatAttackForTest is the attack routine, for tests.
func CombatAttackForTest(w *World, io IO, member int) { combatAttack(w, io, member, nil) }

// combatAttack mirrors CombatAttack(). With preset the direction is already
// known (the member walked into a monster) and is echoed instead of asked for.
func combatAttack(w *World, io IO, member int, preset *combatStep) {
	c := w.Combat
	p := w.Member(member)
	me := c.Members[member]
	weapon := int(p.Bytes[48])
	io.Print(w.Resources.Strings["WeaponsArmour"][weapon])
	io.PrintMessage(combatMsgAttackDir)
	var dx, dy int
	if preset != nil {
		io.PrintMessage(preset.Message)
		dx, dy = preset.DX, preset.DY
	} else {
		dir := GetDirection(w, io, true, w.DiagonalMoves)
		if dir == nil {
			Cancelled(w, io)
			return
		}
		dx, dy = dir.DX, dir.DY
	}
	if dx == 0 && dy == 0 {
		return
	}
	io.Sound(SoundSwish[member])

	missed := func() {
		io.PrintMessage(combatMsgMissed)
		io.RedrawMap()
	}
	var target int
	if slices.Contains(Shoots, weapon) { // sling, bow, +2 bow, +4 bow
		if target = Shoot(w, io, me.X, me.Y, dx, dy, ShapeFireBall); target < 0 {
			missed()
			return
		}
	} else {
		target = MonsterAt(c, me.X+dx, me.Y+dy)
		// A dagger can be thrown at a distant foe and is used up. Only a
		// spare from the party's bag is thrown, keeping the one in hand.
		if target < 0 && (weapon != 1 || w.Party.Weapons(1) < 1) {
			missed()
			return
		}
		if target < 0 {
			w.Party.SetWeapons(1, w.Party.Weapons(1)-1)
			if target = Shoot(w, io, me.X, me.Y, dx, dy, ShapeFireBall); target < 0 {
				missed()
				return
			}
		}
	}
	// Only exotic weapons work in Exodus' castle.
	if w.InExodusCastle() && weapon != 15 {
		missed()
		return
	}
	if w.Rng.Range(0, 255) < 128 && p.Dexterity() < w.Rng.Range(0, 99) {
		missed()
		return
	}
	io.Print(MonsterName(w, c.MonsterShape, c.MonsterVariant, false))
	io.PrintMessage(combatMsgHit)
	m := c.Monsters[target]
	showHit(w, io, m.X, m.Y, ShapeFireBall)
	damage := w.Rng.Range(0, p.Strength()|1)
	damage += p.Strength() >> 1
	damage += weapon*3 + 4
	DamageMonster(w, io, target, damage, member)
}

// MonsterPlan is a monster's chosen target and step.
type MonsterPlan struct {
	Target     int // chosen member, or -1
	Distance   int // Manhattan distance to the target; 0 adjacent; 255 no target
	NewX, NewY int
	DX, DY     int
}

// PlanMonster mirrors FigureNewMonPosition(): the nearest reachable member
// and the step toward them (diagonal first, then vertical, then horizontal).
func PlanMonster(w *World, i int) MonsterPlan {
	c := w.Combat
	m := c.Monsters[i]
	plan := MonsterPlan{Target: -1, Distance: 255, NewX: m.X, NewY: m.Y}
	free := func(x, y int) bool { return monsterCanStand(c.MonsterShape, ArenaTile(c, x, y)) }
	for member := 0; member < w.Party.Size(); member++ {
		if !w.MemberAlive(member) {
			continue
		}
		p := c.Members[member]
		dx, dy := p.X-m.X, p.Y-m.Y
		ax, ay := abs(dx), abs(dy)
		if ax < 2 && ay < 2 {
			plan.Distance, plan.Target = 0, member
			if ax+ay < 2 {
				break
			}
			continue
		}
		if ax+ay >= plan.Distance {
			continue
		}
		sx, sy := sign(dx), sign(dy)
		nx, ny := m.X+sx, m.Y+sy
		if !free(nx, ny) {
			nx, ny = m.X, m.Y+sy
			if !free(nx, ny) {
				nx, ny = m.X+sx, m.Y
				if !free(nx, ny) {
					continue
				}
			}
		}
		plan = MonsterPlan{Target: member, Distance: ax + ay, NewX: nx, NewY: ny, DX: sx, DY: sy}
	}
	return plan
}

// monsterTurn runs one monster's turn.
func monsterTurn(w *World, io IO, i int) {
	c := w.Combat
	t := c.MonsterShape
	plan := PlanMonster(w, i)
	target := plan.Target
	if plan.Distance != 0 {
		// Dragons breathe fire from range half the time.
		if target >= 0 && w.Rng.Range(0, 255) < 128 && t == 0x3a {
			combatMonsterShoot(w, io, i, plan)
			return
		}
		// Spell casters may cast at a random member instead of moving.
		if w.Rng.Range(0, 0xc0) > 127 {
			if slices.Contains([]int{0x1a, 0x1c, 0x2c, 0x36, 0x3a, 0x3c}, t) {
				victim := w.Rng.Range(0, 255) & 3
				if w.Member(victim).Status() == 'G' && w.Party.MemberSlot(victim) >= 0 {
					io.FlashTiles()
					io.Sound(SoundMonsterSpell)
					monsterAttack(w, io, victim, ShapeMagicBall, true)
					return
				}
				// Otherwise fall through to moving.
			} else if t == 0x26 {
				moveMonster(w, io, i, plan)
				return
			}
		}
		if plan.Distance < 0x80 {
			moveMonster(w, io, i, plan)
		}
		return
	}
	monsterAttack(w, io, target, ShapeFireBall, false)
}

// moveMonster steps a monster to its planned cell. (monlb)
func moveMonster(w *World, io IO, i int, plan MonsterPlan) {
	c := w.Combat
	m := c.Monsters[i]
	if MonsterAt(c, plan.NewX, plan.NewY) >= 0 || MemberAt(c, plan.NewX, plan.NewY) >= 0 {
		return
	}
	m.X, m.Y = plan.NewX, plan.NewY
	m.TileUnder = ArenaTile(c, m.X, m.Y)
	io.RedrawMap()
}

// combatMonsterShoot is a dragon breathing fire along its line to the target. (monshoot)
func combatMonsterShoot(w *World, io IO, i int, plan MonsterPlan) {
	c := w.Combat
	m := c.Monsters[i]
	io.Sound(SoundShoot)
	x, y := m.X, m.Y
	for {
		x += plan.DX
		y += plan.DY
		if x < 0 || x > 10 || y < 0 || y > 10 {
			io.RedrawMap()
			return
		}
		if hit := MemberAt(c, x, y); hit >= 0 && hit < w.Party.Size() {
			damageMember(w, io, hit, ShapeFireBall)
			return
		}
		ShowBall(w, io, x, y, ShapeFireBall, false)
	}
}

// monsterAttack is a monster striking an adjacent member, or a caster's
// spell (the afternext .. plrhit block). Thieves pilfer, some poison, and
// inside Exodus' castle only exotic armour helps.
func monsterAttack(w *World, io IO, target, ballShape int, magic bool) {
	t := w.Combat.MonsterShape
	if target < 0 {
		return
	}
	if t == 0x1c || t == 0x3c || t == 0x38 {
		poison(w, io, target)
	} else if t == 0x2e {
		pilfer(w, io, target)
	}
	io.PrintMessage(combatMsgPlr)
	io.Print(strconv.Itoa(target + 1))
	io.Sound(SoundAttack)

	p := w.Member(target)
	autoHit := w.InExodusCastle() && p.Bytes[40] != 7
	if !autoHit && !magic && w.Rng.Range(0, int(p.Bytes[40])+0x10) >= 8 {
		io.PrintMessage(combatMsgMonsterMissed)
		return
	}
	io.PrintMessage(combatMsgMonsterHit)
	damageMember(w, io, target, ballShape)
}

// damageMember is the damage roll and its consequences. (c8777)
func damageMember(w *World, io IO, target, ballShape int) {
	c := w.Combat
	p := w.Member(target)
	me := c.Members[target]
	most := ((monsterHP[(c.MonsterShape>>1)&0x0f] >> 3) + p.MaxHitPoints()/100) | 1
	died := p.SubtractHitPoints(w.Rng.Range(0, most) + 1)
	// Fights in towns and castles hurt more (a quirk: location & 3, times 16).
	died = p.SubtractHitPoints((c.PreviousLocation&3)*16) || died
	if died {
		io.Sound(DeathSound(p.Sex()))
	}
	io.FlashMember(target)
	showHit(w, io, me.X, me.Y, ballShape)
	io.UpdateStats(false)
	if p.Status() == 'D' {
		io.PrintMessage(combatMsgPlayerKilled)
		me.DiedAt = &Point{me.X, me.Y}
		me.X, me.Y = missing, missing
		io.RedrawMap()
		io.UpdateStats(false)
		CheckAllDead(w, io)
	}
}

// pilfer mirrors Pilfer(): a thief steals a whole stack of one kind of
// weapon or armour nobody is using, from the party's bag.
func pilfer(w *World, io IO, member int) {
	if w.Rng.Range(0, 255) < 128 {
		item := w.Rng.Range(0, 15)
		if item == 0 || w.Party.Weapons(item) == 0 {
			return
		}
		w.Party.SetWeapons(item, 0)
	} else {
		item := w.Rng.Range(0, 7)
		if item == 0 || w.Party.Armour(item) == 0 {
			return
		}
		w.Party.SetArmour(item, 0)
	}
	io.PrintMessage(combatMsgPlr)
	io.Print(strconv.Itoa(member + 1))
	io.PrintMessage(combatMsgPilfered)
	io.Sound(SoundOuch)
}

// poison mirrors Poison(): one chance in four to poison a healthy member.
func poison(w *World, io IO, member int) {
	p := w.Member(member)
	if w.Rng.Range(0, 255)&0x03 != 0 || p.Status() != 'G' {
		return
	}
	p.SetStatus('P')
	io.PrintMessage(combatMsgPlr)
	io.Print(strconv.Itoa(member + 1))
	io.PrintMessage(combatMsgPoisoned)
	io.Sound(SoundOuch)
}
