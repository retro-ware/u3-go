package game

import (
	"strconv"
	"strings"
)

// Commands that involve other creatures and objects on the map: talking,
// shopping, attacking, firing a ship's cannons, stealing, unlocking doors,
// yelling, and Other. Ports of Transact(), Attack(), Fire(), Steal(),
// Unlock(), Yell() from UltimaMain.c, Speak() from UltimaText.c and
// OtherCommand(), Shrine(), SafeExodus() from UltimaMisc.c.

// Messages table entries interact.go prints.
const (
	interactMsgAttack          = 28
	interactMsgFire            = 38
	interactMsgFireDirect      = 39
	interactMsgSteal           = 84
	interactMsgDirection       = 85
	interactMsgFailed          = 86
	interactMsgWatchOut        = 87
	interactMsgWhoTransacts    = 88
	interactMsgGoodDay         = 89
	interactMsgWelcomeMyChild  = 90
	interactMsgExperienceMore  = 91
	interactMsgNoMore          = 92
	interactMsgSeekMarkOfKings = 93
	interactMsgThouArtGreater  = 94
	interactMsgUnlock          = 95
	interactMsgNoneLeft        = 67
	interactMsgDestroyed       = 117
	interactMsgYell            = 104
	interactMsgShrineWelcome   = 235
	interactMsgShrineOffering  = 178
	interactMsgThenBeOff       = 179
	interactMsgCantCheat       = 180
	interactMsgShazam          = 181
	interactMsgOtherCommand    = 236
	interactMsgCmd             = 238
	interactMsgDir             = 239
	interactMsgCards           = 240
	interactMsgCongratulations = 241
	interactMsgMoves           = 242
	interactMsgExotics         = 243
	interactMsgStrangeCard     = 244
	interactMsgNotEnoughGold   = 245
	interactMsgYellEvocare     = 246
	interactMsgEveryoneHates   = 247
	interactMsgNoEffect        = 248
	interactMsgAllIsCalm       = 254
	interactMsgExodusNoMore    = 262
	interactMsgSosariaThanks   = 263
	interactMsgEnterShrine     = 37
)

// Transact talks to whoever or whatever is in a chosen direction: NPCs,
// Lord British, or a shop counter. (Transact) The direction comes first, so
// "who" is asked only when it matters.
func Transact(world *World, io IO) {
	io.PrintMessage(interactMsgDirection)
	dir := GetDirection(world, io, false, world.DiagonalMoves)
	if dir == nil {
		Cancelled(world, io)
		return
	}
	TransactToward(world, io, dir.DX, dir.DY)
}

// WhoSeesLordBritish asks who stands before Lord British. The prompt lists
// just the members due a level; with nobody due it lists everyone. A living
// member's index, or -1.
func WhoSeesLordBritish(world *World, io IO) int {
	var due []int
	for m := range 4 {
		if world.MemberAlive(m) && LevelUpDue(world.Member(m)) {
			due = append(due, m)
		}
	}
	io.PrintMessage(interactMsgWhoTransacts)
	return chosenLiving(world, io, io.ChooseMember(due))
}

// WhoTransacts is the "Who will Transact-" prompt: a living member's index, or -1.
func WhoTransacts(world *World, io IO) int {
	io.PrintMessage(interactMsgWhoTransacts)
	return chosenLiving(world, io, io.ChooseMember(nil))
}

func chosenLiving(world *World, io IO, n int) int {
	if n == 0 {
		Cancelled(world, io)
	}
	if n < 1 || n > 4 {
		return -1
	}
	if !world.MemberAlive(n - 1) {
		Incapacitated(io)
		return -1
	}
	return n - 1
}

// firstLiving is the first living member, or 0.
func firstLiving(world *World) int {
	for m := range 4 {
		if world.MemberAlive(m) {
			return m
		}
	}
	return 0
}

// TransactToward transacts with whatever is one step (dx, dy) away, asking who only when it matters.
func TransactToward(world *World, io IO, dx, dy int) {
	xs := world.Constrain(world.X + dx)
	ys := world.Constrain(world.Y + dy)
	mon := world.Monsters().At(xs, ys)

	if mon < 0 {
		// No one there: perhaps a counter with a merchant behind it.
		if !CounterWithMerchant(world, xs, ys, dx, dy) {
			NotHere(io)
			return
		}
		shopNumber := world.Y & 0x07
		member := firstLiving(world)
		partyGoods := shopNumber == 1 || shopNumber == 5 // grocer and guild sell to the party
		if !partyGoods {
			member = WhoTransacts(world, io)
			if member < 0 {
				return
			}
		}
		previousMusic := world.Music
		world.Music = MusicShop
		io.Music(MusicShop)
		if !partyGoods {
			io.HighlightMember(member, true)
		}
		Shop(world, io, shopNumber, member)
		JournalCheck(world, io) // a guild or Dawn's magic arms found, a rumour or a verse heard
		io.HighlightMember(member, false)
		world.Music = previousMusic
		io.Music(previousMusic)
		return
	}

	// Lord British judges one member; townspeople talk to whoever is in front.
	member := firstLiving(world)
	if world.Monsters().Type(mon) == MapValueLordBritish {
		member = WhoSeesLordBritish(world, io)
		if member < 0 {
			return
		}
	}
	TalkTo(world, io, mon, member)
}

// TalkTo talks to the creature in monster slot mon: a townsperson's line,
// or Lord British's audience. Also used when the party walks into someone.
func TalkTo(world *World, io IO, mon, member int) {
	p := world.Member(member)
	m := world.Monsters()
	if m.Type(mon) != MapValueLordBritish {
		person := m.TalkIndex(mon)
		if person == 0 {
			if world.Party.ExodusDestroyed() {
				io.PrintMessage(interactMsgExodusNoMore)
			} else {
				io.PrintMessage(interactMsgGoodDay)
			}
			return
		}
		io.Print(Speech(world.Current.Talk, person))
		if town, ok := world.Resources.MapNames[world.Current.ID]; ok && town != "" && HearTownLine(world, town, person) {
			JournalCheck(world, io)
		}
		return
	}

	// Lord British raises levels once the character has the experience.
	io.Music(MusicLordBritish)
	io.PrintMessage(interactMsgWelcomeMyChild)
	world.Journal.LordBritish = true
	lordBritishSpeaks(world, io, p)
	JournalCheck(world, io)
}

// lordBritishSpeaks is the rest of the audience: a level, or one of the king's excuses.
func lordBritishSpeaks(world *World, io IO, p *PlayerRecord) {
	level := int(p.Bytes[30])
	hpmax := p.MaxHitPoints()
	if hpmax%100 == 50 {
		hpmax -= 50 // old 150/250 style values
	}
	hpmax /= 100
	if level < hpmax {
		if world.Party.ExodusDestroyed() {
			io.PrintMessage(interactMsgSosariaThanks)
		} else {
			io.PrintMessage(interactMsgExperienceMore)
		}
		return
	}
	if hpmax >= 25 && !world.Party.ExodusDestroyed() {
		io.PrintMessage(interactMsgNoMore)
		return
	}
	if hpmax > 4 && p.Marks()&0x80 == 0 {
		HearClue(world, "LB:mark")
		io.PrintMessage(interactMsgSeekMarkOfKings)
		return
	}
	newMax := min(9950, p.MaxHitPoints()+100)
	gained := newMax - p.MaxHitPoints()
	p.Bytes[28], p.Bytes[29] = byte(newMax/256), byte(newMax%256)
	p.AddHitPoints(gained) // this port: the new hit points are given, not just room for them
	io.PrintMessage(interactMsgThouArtGreater)
	io.FlashTiles()
	io.Sound(SoundLBLevelRise)
	io.FlashTiles()
	io.UpdateStats(false)
}

// Attack is A: attack a monster on an adjacent square. (Attack)
func Attack(world *World, io IO) {
	io.PrintMessage(interactMsgAttack)
	dir := GetDirection(world, io, false, world.DiagonalMoves)
	if dir == nil {
		Cancelled(world, io)
		return
	}
	AttackToward(world, io, dir.DX, dir.DY)
}

// AttackToward attacks whatever is one step (dx, dy) away.
func AttackToward(world *World, io IO, dx, dy int) {
	mon := world.Monsters().At(world.Constrain(world.X+dx), world.Constrain(world.Y+dy))
	if mon < 0 {
		NotHere(io)
		return
	}
	AttackMonster(world, io, mon)
}

// Fire is F: fire the frigate's cannons three squares in a direction. (Fire)
func Fire(world *World, io IO) {
	io.PrintMessage(interactMsgFire)
	if world.Party.Shape() != PartyShapeFrigate {
		What2(io)
		return
	}
	io.PrintMessage(interactMsgFireDirect)
	dir := GetDirection(world, io, false, world.DiagonalMoves)
	if dir == nil {
		Cancelled(world, io)
		return
	}
	io.Sound(SoundShoot)
	xs, ys := world.X, world.Y
	for r := 3; r > 0; r-- {
		xs = world.Constrain(xs + dir.DX)
		ys = world.Constrain(ys + dir.DY)
		mon := world.Monsters().At(xs, ys)
		if mon < 0 {
			ShowBall(world, io, xs, ys, ShapeFireBall, false)
			continue
		}
		// A hit. Everything dodges half the time, pirates half the rest too.
		io.Sound(SoundHit)
		ShowBall(world, io, xs, ys, ShapeFireBall, false)
		m := world.Monsters()
		if world.Rng.Range(0, 255) < 128 {
			io.RedrawMap()
			return
		}
		if m.Type(mon) == MapValuePirate && world.Rng.Range(0, 255) < 128 {
			io.RedrawMap()
			return
		}
		ShowBall(world, io, xs, ys, ShapeFireBall, true)
		io.Print(MonsterName(world, m.Type(mon)>>1, m.Variant(mon), true))
		io.PrintMessage(interactMsgDestroyed)
		world.PutXYVal(m.TileUnder(mon), xs, ys)
		m.Clear(mon)
		return
	}
	io.RedrawMap()
}

// Steal is S: steal from a shop's chest across the counter. (Steal)
func Steal(world *World, io IO) {
	io.PrintMessage(interactMsgSteal)
	n := io.ChooseMember(nil)
	if n == 0 {
		Cancelled(world, io)
		return
	}
	if n < 1 || n > 4 {
		return
	}
	member := n - 1
	if !world.MemberAlive(member) {
		Incapacitated(io)
		return
	}
	io.PrintMessage(interactMsgDirection)
	dir := GetDirection(world, io, false, world.DiagonalMoves)
	if dir == nil {
		Cancelled(world, io)
		return
	}

	fail := func() {
		if world.Rng.Range(0, 255)&0x03 != 0 {
			io.PrintMessage(interactMsgFailed)
			return
		}
		// Caught: every guard turns hostile.
		m := world.Monsters()
		for i := 31; i >= 0; i-- {
			if m.Type(i) == MapValueGuard {
				m.SetHP(i, 0xc0)
			}
		}
		io.PrintMessage(interactMsgWatchOut)
		io.Sound(SoundOuch)
		io.Sound(SoundAlarm)
	}

	if StealDisarmFails(world, world.Member(member)) {
		fail()
		return
	}
	if counter := world.GetXYVal(dir.XS, dir.YS); counter < 0x94 || counter > 0xe4 {
		fail()
		return
	}
	xs := world.Constrain(dir.XS + dir.DX)
	ys := world.Constrain(dir.YS + dir.DY)
	if world.GetXYVal(xs, ys) != MapValueChest {
		fail()
		return
	}
	world.PutXYVal(MapValueFloor, xs, ys)
	GetChest(world, io, member, "steal")
}

// Unlock is U: unlock a door with a key. Doors unlock only sideways. (Unlock)
func Unlock(world *World, io IO) {
	io.PrintMessage(interactMsgUnlock)
	dir := GetDirection(world, io, false, false)
	if dir == nil {
		Cancelled(world, io)
		return
	}
	UnlockToward(world, io, dir.DX, dir.DY)
}

// UnlockToward unlocks the door one step (dx, dy) away with one of the party's keys.
func UnlockToward(world *World, io IO, dx, dy int) {
	xs, ys := world.X+dx, world.Y+dy
	if dx == 0 && dy != 0 {
		NotHere(io)
		return
	}
	if world.GetXYVal(xs, ys) != MapValueLetterI {
		NotHere(io)
		return
	}
	if world.Party.Keys() < 1 {
		io.PrintMessage(interactMsgNoneLeft)
		return
	}
	world.Party.SetKeys(world.Party.Keys() - 1)
	io.Sound(SoundCreak)
	// The doorway becomes whatever is to its left (or grass if that is not terrain).
	value := 0
	if mon := world.Monsters().At(xs-1, ys); mon >= 0 {
		value = world.Monsters().TileUnder(mon)
	} else {
		value = world.GetXYVal(xs-1, ys)
	}
	if value > MapValueFloor {
		value = MapValueGrass
	}
	world.PutXYVal(value, xs, ys)
	io.RedrawMap()
}

// Yell is Y: Other under another name, as on the Apple II. EVOCARE works
// from Other as well here; Y stays for keyboard habit. (Yell)
func Yell(world *World, io IO) {
	io.PrintMessage(interactMsgYell)
	OtherCommand(world, io, true)
}

// evocare: with the Mark of the Snake, beside the serpent, the party crosses it.
func evocare(world *World, io IO, member int) {
	p := world.Member(member)
	if p.Marks()&0x40 == 0 {
		io.PrintMessage(interactMsgNoEffect)
		return
	}
	oy := world.Y
	if world.GetXYVal(world.X, world.Y-1) == MapValueSnakeTop {
		oy -= 3
	}
	if world.GetXYVal(world.X, world.Y+1) == MapValueSnakeBottom {
		oy += 3
	}
	if oy == world.Y {
		io.PrintMessage(interactMsgNoEffect)
		return
	}
	world.Y = oy
	io.ClearTiles()
	io.Sound(SoundInvocation)
	io.Pause(1000)
	io.RedrawMap()
	world.Journal.SerpentParted = true
	JournalCheck(world, io)
}

// OtherWords is every word Other answers to but PISSOFF.
var OtherWords = []string{"SEARCH", "BRIBE", "PRAY", "EVOCARE", "INSERT", "DIG", "PAXUM", "SCREAM"}

// OtherCommand mirrors OtherCommand(): a typed word. PAXUM calms nearby
// monsters, SCREAM screams, INSERT places a card in Exodus, DIG digs up
// exotics, SEARCH finds cards at shrines, BRIBE pays off a guard, PRAY at
// the right spot reveals a word, EVOCARE parts the serpent, PISSOFF angers everyone.
func OtherCommand(world *World, io IO, fromYell bool) {
	if !fromYell {
		io.PrintMessage(interactMsgOtherCommand)
	}
	n := io.ChooseMember(nil)
	if n == 0 {
		Cancelled(world, io)
		return
	}
	if n < 1 || n > 4 {
		return
	}
	member := n - 1
	if !world.MemberAlive(member) {
		Incapacitated(io)
		return
	}
	p := world.Member(member)
	io.PrintMessage(interactMsgCmd)
	// A controller is offered the words learnt; a keyboard types any, as on the Apple II.
	word := strings.ToUpper(io.InputText(8, false, KnownWords(world)))
	io.Print("\n")
	if word == "" {
		Cancelled(world, io)
		return
	}

	switch word {
	case "PAXUM":
		if world.OnSurface() {
			io.PrintMessage(interactMsgNoEffect)
			return
		}
		io.PrintMessage(interactMsgAllIsCalm)
		m := world.Monsters()
		for i := range 32 {
			if m.Type(i) == 0 || m.HP(i) != 0xc0 {
				continue
			}
			if abs(m.X(i)-world.X) < 6 && abs(m.Y(i)-world.Y) < 6 {
				m.SetHP(i, 0x40)
			}
		}
	case "SCREAM":
		io.Print("\nAIEEEEE!\n\n")
		io.Sound(DeathSound(p.Sex()))
	case "INSERT":
		insertCard(world, io, member)
	case "DIG":
		if !world.OnSurface() {
			NotHere(io)
			return
		}
		switch {
		case world.X == 0x21 && world.Y == 0x03:
			world.AddGear(true, 15, 1) // exotic weapon
		case world.X == 0x13 && world.Y == 0x2c:
			world.AddGear(false, 7, 1) // exotic armour
		default:
			NotHere(io)
			return
		}
		io.PrintMessage(interactMsgExotics)
		JournalCheck(world, io)
	case "SEARCH":
		if world.GetXYVal(world.X, world.Y) != MapValueShrine {
			NotHere(io)
			return
		}
		p.Bytes[14] |= 1 << (world.X & 0x03)
		io.PrintMessage(interactMsgStrangeCard)
		JournalCheck(world, io)
	case "BRIBE":
		io.PrintMessage(interactMsgDir)
		dir := GetDirection(world, io, false, world.DiagonalMoves)
		if dir == nil {
			Cancelled(world, io)
			return
		}
		mon := world.Monsters().At(world.Constrain(dir.XS), world.Constrain(dir.YS))
		if mon < 0 {
			NotHere(io)
			return
		}
		if world.Party.Gold() < 100 {
			io.PrintMessage(interactMsgNotEnoughGold)
			io.Sound(SoundError1)
			return
		}
		world.Party.SetGold(world.Party.Gold() - 100)
		m := world.Monsters()
		if m.Type(mon) != MapValueGuard {
			io.PrintMessage(interactMsgNoEffect)
			return
		}
		world.PutXYVal(m.TileUnder(mon), m.X(mon), m.Y(mon))
		m.Clear(mon)
	case "PRAY":
		if world.Party.Location() != LocationTown ||
			world.Party.SurfaceX() != int(world.Resources.Misc.LocationX[4]) ||
			world.X != 0x30 || world.Y != 0x30 {
			io.PrintMessage(interactMsgNoEffect)
			return
		}
		io.PrintMessage(interactMsgYellEvocare)
		HearClue(world, "Yew:pray") // the word learnt
		JournalCheck(world, io)
	case "EVOCARE":
		evocare(world, io, member)
	case "PISSOFF":
		io.PrintMessage(interactMsgEveryoneHates)
		for i := range 32 {
			world.Monsters().SetHP(i, 0xc0)
		}
	default:
		io.PrintMessage(interactMsgNoEffect)
	}
}

// insertCard is INSERT: place a card into Exodus. The four cards must go in
// order (Love, Sol, Moons, Death) into the four slots; a wrong card is
// fatal. After the fourth, Exodus is destroyed and the game is won.
func insertCard(world *World, io IO, member int) {
	p := world.Member(member)
	io.PrintMessage(interactMsgDir)
	dir := GetDirection(world, io, false, world.DiagonalMoves)
	if dir == nil {
		Cancelled(world, io)
		return
	}
	if world.GetXYVal(dir.XS, dir.YS) != MapValueExodus {
		NotHere(io)
		return
	}
	io.PrintMessage(interactMsgCards)
	names := map[string]string{"D": "Card of Death", "S": "Card of Sol", "L": "Card of Love", "M": "Card of Moons"}
	key := io.ChooseOption(LetterOptions("DSLM", func(l string) string { return names[l] }), EchoLine, nil)
	if key == "" {
		Cancelled(world, io)
		return
	}
	slot := map[string]int{"L": 0x1e, "S": 0x1f, "M": 0x20, "D": 0x21}[key]
	if slot == 0 {
		What2(io)
		return
	}
	if p.Marks()&(1<<(slot-0x1e)) == 0 {
		io.PrintMessage(interactMsgNoneLeft)
		return
	}

	if dir.XS != slot || slot != world.LastCard {
		// Wrong card or wrong slot: the character is struck down.
		io.FlashMember(member)
		io.Sound(SoundHit)
		io.Pause(250)
		p.SetHitPoints(1)
		p.SubtractHitPoints(255)
		io.Sound(DeathSound(p.Sex()))
		io.UpdateStats(false)
		return
	}
	world.LastCard++
	for range 5 {
		world.Ball = &Ball{X: dir.XS, Y: dir.YS, Shape: ShapeMagicBall}
		io.RedrawMap()
		io.Sound(SoundHit)
		io.Pause(250)
		world.Ball = nil
		io.RedrawMap()
		io.Sound(SoundHit)
		io.Pause(250)
	}
	world.PutXYVal(MapValueFloor, dir.XS, dir.YS)
	io.RedrawMap()
	if world.LastCard != 0x22 {
		return
	}

	// Exodus is destroyed.
	world.Party.Bytes[15] = 1
	world.Music = MusicShrine
	io.Music(MusicShrine)
	io.PrintMessage(interactMsgCongratulations)
	io.Print(strconv.Itoa(world.Party.Moves()))
	io.PrintMessage(interactMsgMoves)
	io.PlayEnding()
	SafeExodus(world)
	io.RedrawMap()
	io.Print("\n")
	JournalCheck(world, io)
}

// SafeExodus mirrors SafeExodus(): after the victory the castle is emptied and its traps removed.
func SafeExodus(world *World) {
	m := world.Monsters()
	for i := range 32 {
		if m.HP(i) > 0 {
			world.PutXYVal(m.TileUnder(i), m.X(i), m.Y(i))
			m.SetHP(i, 0)
			m.Clear(i)
		}
	}
	for y := 1; y < 12; y++ {
		for x := 0x1e; x < 0x22; x++ {
			world.PutXYVal(MapValueFloor, x, y)
		}
	}
}

// shrineMax is the race-dependent caps on each attribute at the shrines. (shMax)
var shrineMax = []int{75, 75, 99, 75, 25, 75, 99, 75, 50, 99, 75, 50, 75, 99, 75, 75, 75, 50, 75, 99}

// StatLimit is a race's limit on a stat at the shrines: race 0..4 as Races
// lists them, stat 0 strength, 1 dexterity, 2 intelligence, 3 wisdom.
func StatLimit(race, stat int) int { return shrineMax[race+[]int{0, 5, 15, 10}[stat]] }

// EnterShrine mirrors Shrine(): pay hundreds of gold to raise strength,
// dexterity, intelligence or wisdom (by the shrine's X & 3), up to the
// race's cap. After Exodus is destroyed the cap no longer applies.
func EnterShrine(world *World, io IO) {
	io.PrintMessage(MsgEnter)
	io.PrintMessage(interactMsgEnterShrine)
	n := io.ChooseMember(nil)
	if n == 0 {
		Cancelled(world, io)
		return
	}
	if n < 1 || n > 4 {
		io.Sound(SoundError1)
		return
	}
	member := n - 1
	if !world.MemberAlive(member) {
		Incapacitated(io)
		return
	}
	p := world.Member(member)

	previousMusic := world.Music
	io.Music(MusicShrine)
	io.ShowImage("Shrine")
	world.AutoMap.Visit("shrine:" + strconv.Itoa(world.X) + "," + strconv.Itoa(world.Y)) // the map names it from now
	which := world.X & 0x03
	attribute := world.Resources.Strings["Messages"][173+which]
	io.PrintMessage(interactMsgShrineWelcome)
	io.Print(strings.Repeat(" ", max(0, (16-len(attribute))/2)) + attribute + "\n\n")

	race := 0
	for i, r := range world.Resources.Strings["Races"] {
		if r != "" && r[0] == p.Race() {
			race = i
			break
		}
	}
	limit := StatLimit(race, which)
	offset := 18 + which

	io.PrintMessage(interactMsgShrineOffering)
	key := io.ChooseOption(LetterOptions("0123456789", goldOfferLabel), EchoKey, nil)
	amount := 0
	if key != "" {
		amount = int(key[0] - '0')
	}
	leave := func() {
		io.RedrawMap()
		world.Music = previousMusic
		io.Music(MusicAmbrosia)
	}
	if amount == 0 {
		io.PrintMessage(interactMsgThenBeOff)
		leave()
		return
	}
	if world.Party.Gold() < amount*100 {
		io.PrintMessage(interactMsgCantCheat)
		io.Sound(SoundError1)
		leave()
		return
	}
	world.Party.SetGold(world.Party.Gold() - amount*100)
	io.PrintMessage(interactMsgShazam)
	io.FlashMember(member)
	io.FlashTiles()
	io.Sound(SoundShrine)
	p.Bytes[offset] = byte(min(99, int(p.Bytes[offset])+amount))
	if int(p.Bytes[offset]) > limit && !world.Party.ExodusDestroyed() {
		p.Bytes[offset] = byte(limit)
	}
	io.UpdateStats(true)
	leave()
}

// goldOfferLabel labels the digits of an offering in hundreds of gold.
func goldOfferLabel(d string) string {
	if d == "0" {
		return "0 (nothing)"
	}
	return d + " = " + d + "00 gold"
}
