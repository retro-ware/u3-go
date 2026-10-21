package game

import (
	"fmt"
)

// Commands that act on the party's own records rather than the world:
// chests, marching order, negate time, peering at gems, readying weapons,
// wearing armour, torches, stats. Ports of the matching routines in
// UltimaMain.c, plus AddExp(), AddGold(), AddItem() and StealDisarmFail()
// from UltimaMisc.c.

const (
	actMsgGetChest      = 40
	actMsgNoSuchPlayer  = 41
	actMsgAcidTrap      = 42
	actMsgPoisonTrap    = 43
	actMsgBombTrap      = 44
	actMsgGasTrap       = 45
	actMsgTrapEvaded    = 46
	actMsgGold          = 47
	actMsgOverflowing   = 48
	actMsgAndA          = 49
	actMsgIgniteTorch   = 64
	actMsgNoneLeft      = 67
	actMsgModifyOrder   = 70
	actMsgAborted       = 71
	actMsgPlr           = 72
	actMsgExchanged     = 73
	actMsgReadyFor      = 79
	actMsgWeapon        = 80
	actMsgNotOwned      = 81
	actMsgNotAllowed    = 82
	actMsgReady         = 83
	actMsgVolume        = 97
	actMsgVolumeOn      = 98
	actMsgVolumeOff     = 99
	actMsgWearFor       = 100
	actMsgArmour        = 101
	actMsgZtats         = 105
	actMsgIncapacitated = 126
)

// Incapacitated prints "Incapacitated!": the chosen member is dead or unable. (Incap)
func Incapacitated(io IO) {
	io.PrintMessage(actMsgIncapacitated)
	io.Sound(SoundError1)
}

// AddExperience mirrors AddExp(): experience is capped at 9899 and levels up every 100.
func AddExperience(w *World, io IO, member, amount int) {
	p := w.Member(member)
	exp := int(p.Bytes[30])*100 + int(p.Bytes[31])
	oldLevel := exp / 100
	exp = min(9899, exp+amount)
	if exp/100 > oldLevel {
		io.Sound(SoundExpLevelUp)
	}
	p.Bytes[30], p.Bytes[31] = byte(exp/100), byte(exp%100)
	io.UpdateStats(false)
}

// AddGold mirrors AddGold(), on the party's pool. False if the cap cut the
// amount short; with overflow the surplus is lost, without it nothing is added.
func AddGold(w *World, gold int, overflow bool) bool {
	if !overflow && w.Party.Gold()+gold > GoldMax {
		return false
	}
	return w.AddGold(gold)
}

// AddItem mirrors AddItem(): item counts are capped at 99.
func AddItem(p *PlayerRecord, offset, amount int) {
	p.Bytes[offset] = byte(min(99, int(p.Bytes[offset])+amount))
}

// StealDisarmFails mirrors StealDisarmFail(): true if the attempt fails.
// Dexterity is the base; thieves get a big bonus, barbarians,
// illusionists, rangers and alchemists a smaller one.
func StealDisarmFails(w *World, p *PlayerRecord) bool {
	factor := p.Dexterity()
	switch careerIndex(w, p) {
	case 3:
		factor += 0x80
	case 5, 7, 9, 10:
		factor += 0x40
	}
	return w.Rng.Range(0, 255) > factor
}

// BombTrap mirrors BombTrap(): every living member takes damage.
func BombTrap(w *World, io IO) {
	for m := range w.Party.Size() {
		if !w.MemberAlive(m) {
			continue
		}
		p := w.Member(m)
		io.FlashMember(m)
		io.Sound(SoundHit)
		died := p.SubtractHitPoints(w.Rng.Range(0, 255) & 0x77)
		died = p.SubtractHitPoints((w.Dungeon.Level+1)*8) || died
		if died {
			io.Sound(DeathSound(p.Sex()))
		}
	}
	io.UpdateStats(false)
}

// ChestHere says whether the party stands on a chest: a chest tile on the
// surface or in a town, a chest cell in a dungeon.
func ChestHere(w *World) bool {
	switch w.Party.Location() {
	case LocationDungeon:
		cell := w.GetXYDng(w.X, w.Y)
		return cell < DungeonCellWall && cell&DungeonCellChest != 0
	case LocationCombat:
		return false
	}
	tile := w.GetXYVal(w.X, w.Y)
	return tile >= MapValueChest && tile <= MapValueChest+3
}

// ChestHow is how GetChest opens a chest.
type ChestHow string

const (
	ChestByCommand ChestHow = "command" // ask who opens it, check for traps
	ChestBySpell   ChestHow = "spell"   // Appar Unem: member chosen, traps disarmed
	ChestBySteal   ChestHow = "steal"   // Steal: straight to the loot
)

// GetChest mirrors GetChest().
func GetChest(w *World, io IO, member int, how ChestHow) {
	if how == ChestByCommand {
		w.ChestTrapsArmed = true
		io.PrintMessage(actMsgGetChest)
		n := io.ChooseMember(nil)
		if n == 0 {
			Cancelled(w, io)
			return
		}
		if n < 1 || n > 4 {
			io.PrintMessage(actMsgNoSuchPlayer)
			io.Sound(SoundError1)
			return
		}
		member = n - 1
		if !w.MemberAlive(member) {
			Incapacitated(io)
			return
		}
	}
	p := w.Member(member)

	if how != ChestBySteal {
		if w.Party.Location() != LocationDungeon {
			tile := w.GetXYVal(w.X, w.Y)
			if tile < MapValueChest || tile > MapValueChest+3 {
				NotHere(io)
				return
			}
			// The chest's low bits say what it sat on.
			under := (tile & 0x3) * 4
			if under == 0 {
				under = MapValueFloor
			}
			w.PutXYVal(under, w.X, w.Y)
		} else {
			if w.GetXYDng(w.X, w.Y) != 0x40 {
				NotHere(io)
				return
			}
			w.PutXYDng(0, w.X, w.Y)
		}

		if w.ChestTrapsArmed && w.Rng.Range(0, 255) <= 127 {
			trap := w.Rng.Range(0, 255) & w.Rng.Range(0, 255) & 0x03
			evaded := func() {
				io.PrintMessage(actMsgTrapEvaded)
				io.Sound(SoundOuch)
			}
			switch trap {
			case 0:
				io.PrintMessage(actMsgAcidTrap)
				if !StealDisarmFails(w, p) {
					evaded()
				} else {
					io.FlashMember(member)
					io.Sound(SoundHit)
					if p.SubtractHitPoints(w.Rng.Range(0, 255) & 0x37) {
						io.Sound(DeathSound(p.Sex()))
					}
				}
			case 1:
				io.PrintMessage(actMsgPoisonTrap)
				if !StealDisarmFails(w, p) {
					evaded()
				} else {
					io.FlashMember(member)
					io.Sound(SoundHit)
					p.SetStatus('P')
				}
			case 2:
				io.PrintMessage(actMsgBombTrap)
				if !StealDisarmFails(w, p) {
					evaded()
				} else {
					BombTrap(w, io)
					return
				}
			default:
				io.PrintMessage(actMsgGasTrap)
				if !StealDisarmFails(w, p) {
					evaded()
				} else {
					for m := range 4 {
						if !w.MemberAlive(m) {
							continue
						}
						io.FlashMember(m)
						io.Sound(SoundHit)
						w.Member(m).SetStatus('P')
					}
				}
			}
		}
	}

	// The loot.
	io.Sound(SoundCreak)
	gold := w.Rng.Range(0, 100)
	if gold < 30 {
		gold += 30
	}
	io.PrintMessage(actMsgGold)
	io.Print(itoa(gold) + "\n")
	if !AddGold(w, gold, true) {
		io.PrintMessage(actMsgOverflowing)
		io.Sound(SoundError1)
	}
	io.UpdateStats(false)
	if w.Rng.Range(0, 255) > 63 {
		return
	}
	names := w.Resources.Strings["WeaponsArmour"]
	if weapon := w.Rng.Range(0, 255); weapon < 128 {
		weapon = w.Rng.Range(0, 255) & weapon & 0x07
		if weapon != 0 {
			io.PrintMessage(actMsgAndA)
			io.Print(names[weapon] + "\n")
			w.AddGear(true, weapon, 1)
			return
		}
	}
	if armour := w.Rng.Range(0, 255); armour < 128 {
		armour = w.Rng.Range(0, 255) & armour & 0x03
		if armour != 0 {
			io.PrintMessage(actMsgAndA)
			io.Print(names[armour+16] + "\n")
			w.AddGear(false, armour, 1)
		}
	}
}

// IgniteTorch mirrors Ignite(). Torches are the party's: no "whose torch".
func IgniteTorch(w *World, io IO) {
	io.PrintMessage(actMsgIgniteTorch)
	if w.Party.Location() != LocationDungeon {
		NotHere(io)
		return
	}
	if w.Party.Torches() < 1 {
		io.PrintMessage(actMsgNoneLeft)
		return
	}
	w.Party.SetTorches(w.Party.Torches() - 1)
	io.Sound(SoundTorchIgnite)
	w.Dungeon.Torch = 255
}

// ModifyOrder mirrors ModifyOrder(): swap two members' positions.
func ModifyOrder(w *World, io IO) {
	io.PrintMessage(actMsgModifyOrder)
	a := io.ChooseMember(nil)
	if a == 0 {
		Cancelled(w, io)
		return
	}
	if a < 1 || a > 4 {
		io.PrintMessage(actMsgAborted)
		return
	}
	io.PrintMessage(actMsgPlr)
	b := io.ChooseMember(nil)
	if b == 0 {
		Cancelled(w, io)
		return
	}
	if b < 1 || b > 4 || a == b {
		io.PrintMessage(actMsgAborted)
		return
	}
	ra := w.Party.MemberRosterNumber(a - 1)
	w.Party.SetMemberRosterNumber(a-1, w.Party.MemberRosterNumber(b-1))
	w.Party.SetMemberRosterNumber(b-1, ra)
	io.UpdateStats(true)
	io.PrintMessage(actMsgExchanged)
}

// NegateTime mirrors NegateTime(): a powder from the party's supply stops time for ten turns.
func NegateTime(w *World, io IO) {
	io.Print("Negate time!\n") // the original went on to ask whose powder
	if w.Party.Powders() < 1 {
		io.PrintMessage(actMsgNoneLeft)
		return
	}
	w.Party.SetPowders(w.Party.Powders() - 1)
	w.TimeNegate = 10
}

// PeerGem mirrors PeerGem(), with a gem from the party's supply.
func PeerGem(w *World, io IO) {
	io.Print("Peer at gem!\n") // the original went on to ask whose gem
	if w.Party.Gems() < 1 {
		io.PrintMessage(actMsgNoneLeft)
		return
	}
	w.Party.SetGems(w.Party.Gems() - 1)
	if w.Party.Location() == LocationDungeon {
		io.ShowMiniDungeon()
	} else {
		io.ShowMiniMap()
	}
}

// OwnedItems is the menu of weapons (A..P) or armour (A..H) a member may
// ready or wear, up to last: letter, name and how many the bag holds, the
// item in use marked. Hands/skin (A) are always there; kinds the bag lacks
// are hidden (a keyboard may still type them and get "Not owned!"); kinds
// the class may not use are greyed. Each has a gear card.
func OwnedItems(w *World, p *PlayerRecord, isWeapon bool, last byte) []MenuOption {
	names := w.Resources.Strings["WeaponsArmour"]
	base, nameBase, letters := 40, 16, "ABCDEFGH"
	if isWeapon {
		base, nameBase, letters = 48, 0, "ABCDEFGHIJKLMNOP"
	}
	var options []MenuOption
	for i := range len(letters) {
		letter := letters[i]
		if letter > last {
			break
		}
		index := int(letter - 'A')
		count := w.Party.Gear(isWeapon, index)
		inUse := int(p.Bytes[base]) == index
		tail := ""
		switch {
		case index == 0:
		case inUse:
			tail = " (worn"
			if isWeapon {
				tail = " (in hand"
			}
			if count > 0 {
				tail += ", +" + itoa(count)
			}
			tail += ")"
		default:
			tail = " x" + itoa(count)
		}
		card := GearCard(w, p, isWeapon, index, CardFoot{Held: count})
		options = append(options, MenuOption{
			Key:      string(letter),
			Label:    string(letter) + " " + names[nameBase+index] + tail,
			Hidden:   index > 0 && count < 1 && !inUse,
			Disabled: !w.CanUse(p, isWeapon, index),
			Card:     &card,
		})
	}
	return options
}

// equipPrompt is ReadyWeapon() and WearArmour() after the member is known.
func equipPrompt(w *World, io IO, p *PlayerRecord, isWeapon bool) {
	errorMsg := func(msg int) {
		io.PrintMessage(msg)
		io.Sound(SoundError1)
	}
	msg, last, nameBase := actMsgArmour, byte('H'), 16
	if isWeapon {
		msg, last, nameBase = actMsgWeapon, 'P', 0
	}
	io.PrintMessage(msg)
	key := io.ChooseOption(OwnedItems(w, p, isWeapon, last), EchoKey, nil)
	if key == "" {
		Cancelled(w, io)
		return
	}
	if key[0] < 'A' || key[0] > last {
		errorMsg(actMsgNotOwned)
		return
	}
	index := int(key[0] - 'A')
	if !w.CanUse(p, isWeapon, index) {
		errorMsg(actMsgNotAllowed)
		return
	}
	if !w.Equip(p, isWeapon, index) {
		errorMsg(actMsgNotOwned)
		return
	}
	io.Print("\n")
	io.Print(w.Resources.Strings["WeaponsArmour"][index+nameBase])
	io.PrintMessage(actMsgReady)
}

// chooseOwner asks for a member after msg: 0-based, or -1 after cancelling or an error.
func chooseOwner(w *World, io IO, msg int) int {
	io.PrintMessage(msg)
	n := io.ChooseMember(nil)
	if n == 0 {
		Cancelled(w, io)
		return -1
	}
	if n < 1 || n > 4 {
		io.PrintMessage(actMsgNoSuchPlayer)
		io.Sound(SoundError1)
		return -1
	}
	return n - 1
}

// ReadyWeapon mirrors ReadyWeapon(); member -1 asks whose. Each class may
// use weapons only up to a letter in the weapon-use table; exotics (P) suit everyone.
func ReadyWeapon(w *World, io IO, member int) {
	if member < 0 {
		if member = chooseOwner(w, io, actMsgReadyFor); member < 0 {
			return
		}
	}
	equipPrompt(w, io, w.Member(member), true)
}

// WearArmour mirrors WearArmour(). Exotic armour (H) suits everyone.
func WearArmour(w *World, io IO) {
	if m := chooseOwner(w, io, actMsgWearFor); m >= 0 {
		equipPrompt(w, io, w.Member(m), false)
	}
}

// Volume mirrors Volume(): toggle sound effects.
func Volume(w *World, io IO) {
	io.PrintMessage(actMsgVolume)
	w.SoundEnabled = !w.SoundEnabled
	if w.SoundEnabled {
		io.PrintMessage(actMsgVolumeOn)
	} else {
		io.PrintMessage(actMsgVolumeOff)
	}
}

// statCards are the marks and cards Ztats lists, in its order.
var statCards = []struct {
	bit   int
	label string
}{
	{0x08, "CARD OF DEATH"}, {0x02, "CARD OF SOL"}, {0x01, "CARD OF LOVE"}, {0x04, "CARD OF MOONS"},
	{0x10, "MARK OF FORCE"}, {0x20, "MARK OF FIRE"}, {0x40, "MARK OF SNAKE"}, {0x80, "MARK OF KINGS"},
}

// Stats mirrors the classic Stats(): a member's record a few lines at a
// time, waiting for a key between sections; Escape stops early. member -1 asks whose.
func Stats(w *World, io IO, member int) {
	if member < 0 {
		io.PrintMessage(actMsgZtats)
		n := io.ChooseMember(nil)
		if n == 0 {
			Cancelled(w, io)
			return
		}
		if n < 1 || n > 4 {
			io.Print("\n")
			return
		}
		member = n - 1
	}
	p := w.Member(member)
	pad := func(v, width int) string { return fmt.Sprintf("%0*d", width, v) }
	wait := func() bool {
		key := io.WaitKey()
		if key == KeyEscape || key == KeyB || w.Done {
			io.Print("\n")
			return true
		}
		return false
	}
	names := w.Resources.Strings["WeaponsArmour"]

	io.Print(p.Name())
	// Race and class (this port shows neither in the character boxes).
	race := ""
	for _, r := range w.Resources.Strings["Races"] {
		if r != "" && r[0] == p.Race() {
			race = r
			break
		}
	}
	career := ""
	if classes := w.Resources.Strings["Classes"]; len(classes) > 0 {
		career = classes[max(0, careerIndex(w, p))]
	}
	if who := race + " " + career; len(who) <= 16 {
		io.Print("\n" + who)
	} else {
		io.Print("\n" + race + "\n" + career)
	}
	io.Print(fmt.Sprintf("\nSTR...%s\nDEX...%s\nINT...%s\nWIS...%s", pad(p.Strength(), 2), pad(p.Dexterity(), 2), pad(p.Intelligence(), 2), pad(p.Wisdom(), 2)))
	if wait() {
		return
	}
	lines := []string{
		"\nH.P..." + pad(p.HitPoints(), 4),
		"\nH.M..." + pad(p.MaxHitPoints(), 4),
		"\nEXP..." + pad(int(p.Bytes[30])*100+int(p.Bytes[31]), 4),
		// The party's supplies (pooled): the same on every page.
		"\nGEMS.." + pad(w.Party.Gems(), 2),
		"\nKEYS.." + pad(w.Party.Keys(), 2),
		"\nPOWD.." + pad(w.Party.Powders(), 2),
		"\nTRCH.." + pad(w.Party.Torches(), 2),
	}
	for _, line := range lines {
		io.Print(line)
		if wait() {
			return
		}
	}
	for _, c := range statCards {
		if p.Marks()&c.bit == 0 {
			continue
		}
		io.Print("\n" + c.label)
		if wait() {
			return
		}
	}
	io.Print("\nWEAPON:" + names[p.Bytes[48]])
	if wait() {
		return
	}
	io.Print("\nARMOUR:" + names[int(p.Bytes[40])+16])
	if wait() {
		return
	}
	// The party's bag (pooled gear): the same list on every member's page.
	// The original's list ended with hands and skin, which in a bag read as
	// items the party carries; WEAPON and ARMOUR above already name them.
	io.Print("\n*PARTY WEAPONS*\n")
	for x := 15; x >= 1; x-- {
		if w.Party.Weapons(x) == 0 {
			continue
		}
		io.Print(pad(w.Party.Weapons(x), 2) + "-" + names[x] + "-(" + string(rune('A'+x)) + ")")
		if wait() {
			return
		}
		io.Print("\n")
	}
	io.Print("*PARTY ARMOUR*\n")
	for x := 7; x >= 1; x-- {
		if w.Party.Armour(x) == 0 {
			continue
		}
		io.Print(pad(w.Party.Armour(x), 2) + "-" + names[x+16] + "-(" + string(rune('A'+x)) + ")")
		if wait() {
			return
		}
		io.Print("\n")
	}
}
