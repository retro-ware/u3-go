package game

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// The eight kinds of shop, entered by transacting with a merchant across a
// counter. A port of Shop(), Clerical(), GuildPay(), GuildGive(),
// WeaponList() and ArmourList() from UltimaMisc.c. Which shop a merchant
// runs is the party's Y & 7: 0 pub, 1 grocer, 2 healer, 3 weapons, 4 armour,
// 5 guild, 6 oracle, 7 horses.

// Messages table entries the shops print.
const (
	shopsMsgWelcomePub        = 185
	shopsMsgHaveADrink        = 186
	shopsMsgLeaveMyShop       = 187
	shopsMsgCantPay           = 188
	shopsMsgAnotherDrink      = 189
	shopsMsgPleasure          = 190
	shopsMsgGrocer            = 191
	shopsMsgRations           = 192
	shopsMsgCantPayGrocer     = 193
	shopsMsgAnythingElse      = 194
	shopsMsgComeAgain         = 195
	shopsMsgClerical          = 196
	shopsMsgClericalMenu      = 197
	shopsMsgCuring            = 198
	shopsMsgCureWhom          = 199
	shopsMsgHealing           = 200
	shopsMsgHealWhom          = 201
	shopsMsgResurrection      = 202
	shopsMsgResurrectWhom     = 203
	shopsMsgRecalling         = 204
	shopsMsgRecallWhom        = 205
	shopsMsgWeaponsShop       = 206
	shopsMsgBuyOrSell         = 207
	shopsMsgYourInterest      = 208
	shopsMsgNoGold            = 209
	shopsMsgHereYouAre        = 210
	shopsMsgForSale           = 211
	shopsMsgDontOwn           = 212
	shopsMsgReady             = 83 // " Ready" (shared with actions)
	shopsMsgThankYou          = 213
	shopsMsgTooMuchGold       = 214
	shopsMsgMaybeNextTime     = 215
	shopsMsgArmourShop        = 216
	shopsMsgGuild             = 217
	shopsMsgYourNeed          = 218
	shopsMsgGuildThanks       = 219
	shopsMsgGuildAnythingElse = 220
	shopsMsgRadrion           = 221
	shopsMsgOffering          = 222
	shopsMsgMoreOffering      = 223
	shopsMsgFareWell          = 224
	shopsMsgEmporium          = 225
	shopsMsgHorsesCost        = 226
	shopsMsgWillYouBuy        = 227
	shopsMsgTooBad            = 228
	shopsMsgNoGoldHorse       = 229
	shopsMsgRideFast          = 230
	shopsMsgNoFunds           = 231
	shopsMsgNoOfferings       = 232
	shopsMsgNotGoldEnough     = 233
	shopsMsgFareThee          = 234
	shopsMsgAvailable         = 237
	shopsMsgNotEnoughRoom     = 260
	shopsMsgGuildPrices       = 261
)

func shopsError(io IO) { io.Sound(SoundError1) }

// leadingInt is parseInt's reading of a number: its leading digits, 0 for none.
func leadingInt(s string) int {
	s = strings.TrimSpace(s)
	end := 0
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	n, _ := strconv.Atoi(s[:end])
	return n
}

func priceOf(world *World, index int) int {
	return leadingInt(world.Resources.Strings["WeaponsArmour"][index])
}

// Shop mirrors Shop(): shopNumber is the Y coordinate & 7, member 0..3.
func Shop(world *World, io IO, shopNumber, member int) {
	p := world.Member(member)
	switch shopNumber {
	case 0:
		pub(world, io)
	case 1:
		Grocer(world, io)
	case 2:
		healer(world, io, p)
	case 3:
		equipmentShop(world, io, p, true)
	case 4:
		equipmentShop(world, io, p, false)
	case 5:
		guild(world, io)
	case 6:
		oracle(world, io)
	default:
		horses(world, io)
	}
}

// pub: pay at least 7 gold for a drink and a rumour. More gold, better rumours.
func pub(world *World, io IO) {
	io.PrintMessage(shopsMsgWelcomePub)
	for {
		io.PrintMessage(shopsMsgHaveADrink)
		paid := InputNumber(io, 2)
		io.Print("\n\n")
		// No amount declines the drink without a scene.
		if paid == 0 {
			io.PrintMessage(shopsMsgComeAgain)
			return
		}
		if paid < 7 {
			io.PrintMessage(shopsMsgLeaveMyShop)
			shopsError(io)
			return
		}
		if world.Party.Gold() < paid {
			io.PrintMessage(shopsMsgCantPay)
			shopsError(io)
			return
		}
		world.Party.SetGold(world.Party.Gold() - paid)
		rumour := min(9, paid/10)
		io.Print(world.Resources.Strings["Pub"][rumour])
		HearClue(world, "Pub:"+strconv.Itoa(rumour)) // noted in the journal after (interact.go)
		io.PrintMessage(shopsMsgAnotherDrink)
		if !YesNo(io) {
			io.Print("N")
			io.PrintMessage(shopsMsgPleasure)
			return
		}
		io.Print("Y")
	}
}

// Grocer sells rations at one gold each, into the party's food. Nobody needs to be named.
func Grocer(world *World, io IO) {
	io.PrintMessage(shopsMsgGrocer)
	for {
		io.PrintMessage(shopsMsgRations)
		amount := InputNumber(io, 5)
		if amount == 0 {
			io.Print("\n\n")
			return
		}
		if amount > FoodMax-world.Party.Food() {
			io.PrintMessage(shopsMsgNotEnoughRoom)
			io.Print("\n\n")
			shopsError(io)
			continue
		}
		if world.Party.Gold() < amount {
			io.PrintMessage(shopsMsgCantPayGrocer)
			shopsError(io)
			return
		}
		world.Party.SetGold(world.Party.Gold() - amount)
		world.AddFood(amount)
		io.UpdateStats(false)
		io.PrintMessage(shopsMsgAnythingElse)
		if !YesNo(io) {
			io.Print("N\n\n")
			io.PrintMessage(shopsMsgComeAgain)
			return
		}
		io.Print("Y\n\n")
	}
}

// clericalPay mirrors Clerical(): confirm and pay; true if paid.
func clericalPay(world *World, io IO, cost int) bool {
	if !YesNo(io) {
		io.Print("N\n\n")
		io.PrintMessage(shopsMsgNoOfferings)
		return false
	}
	if cost > world.Party.Gold() {
		io.Print("Y\n\n")
		io.PrintMessage(shopsMsgNotGoldEnough)
		shopsError(io)
		return false
	}
	world.Party.SetGold(world.Party.Gold() - cost)
	io.Print("Y\n")
	return true
}

// spellNoise mirrors SpellNoize(): the healer's flourish.
func spellNoise(io IO, member int) {
	io.FlashTiles()
	io.FlashMember(member)
	io.Sound(SoundHeal)
	io.PrintMessage(shopsMsgFareThee)
}

// healer: cure, heal, resurrect, recall.
func healer(world *World, io IO, _ *PlayerRecord) {
	io.PrintMessage(shopsMsgClerical)
	io.PrintMessage(shopsMsgClericalMenu)
	choice := io.ChooseOption([]MenuOption{
		{Key: "1", Label: "Curing (100 gp)"},
		{Key: "2", Label: "Healing (200 gp)"},
		{Key: "3", Label: "Resurrection (500 gp)"},
		{Key: "4", Label: "Recalling (900 gp)"},
		{Key: "0", Label: "Nothing"},
	}, EchoNone, nil)
	if choice == "" || choice == "0" {
		io.Print("0\n")
		return
	}
	revive := func(from byte) func(*PlayerRecord) {
		return func(t *PlayerRecord) {
			if t.Status() == from {
				t.SetStatus('G')
			}
		}
	}
	services := map[string]struct {
		ask, whom, cost int
		apply           func(*PlayerRecord)
	}{
		"1": {shopsMsgCuring, shopsMsgCureWhom, 100, revive('P')},
		"2": {shopsMsgHealing, shopsMsgHealWhom, 200, func(t *PlayerRecord) { t.SetHitPoints(t.MaxHitPoints()) }},
		"3": {shopsMsgResurrection, shopsMsgResurrectWhom, 500, revive('D')},
		"4": {shopsMsgRecalling, shopsMsgRecallWhom, 900, revive('A')},
	}
	s := services[choice]
	io.PrintMessage(s.ask)
	if !clericalPay(world, io, s.cost) {
		return
	}
	io.PrintMessage(s.whom)
	n := io.ChooseMember(nil)
	io.Print("\n")
	if n < 1 || n > 4 {
		io.Sound(SoundBump)
		return
	}
	spellNoise(io, n-1)
	s.apply(world.Member(n - 1))
	io.UpdateStats(false)
}

// unusable greys stock the member at the counter cannot use (still for sale, for someone else).
const unusable = "#808080"

// gearBases is where weapons' or armours' names, prices and in-use byte live.
func gearBases(isWeapon bool) (nameBase, priceBase, inUseBase int) {
	if isWeapon {
		return 0, 24, 48
	}
	return 16, 40, 40
}

// priceList prints the price list a few lines at a time. (WeaponList, ArmourList)
func priceList(world *World, io IO, isWeapon bool, last int) {
	names := world.Resources.Strings["WeaponsArmour"]
	io.PrintMessage(shopsMsgAvailable)
	nameBase, priceBase, _ := gearBases(isWeapon)
	pageBreaks := []int{4, 6}
	if isWeapon {
		pageBreaks = []int{5, 7, 11, 14}
	}
	for i := 1; i < last; i++ {
		name := names[nameBase+i]
		if !strings.HasSuffix(name, "s") && len(name) < 7 {
			name += "s"
		}
		price := names[priceBase+i] + "gp"
		label := string(rune(64+i)) + ":" + name
		io.Print(label + fmt.Sprintf("%*s", 16-len(label), price) + "\n")
		if slices.Contains(pageBreaks, i) {
			io.WaitKey()
		}
	}
	io.WaitKey()
}

// equipmentShop buys or sells weapons or armour (Shop cases 3 and 4). Gear
// belongs to the party (this port), so purchases go into the bag and sales
// come out of it; the member at the counter matters for what their class
// can use: those items are greyed, each ware is weighed against theirs on a
// card, and after buying something better suited the shop offers to ready
// or wear it on the spot.
func equipmentShop(world *World, io IO, p *PlayerRecord, isWeapon bool) {
	names := world.Resources.Strings["WeaponsArmour"]
	nameBase, priceBase, inUseBase := gearBases(isWeapon)
	// Ordinary shops stop at the 2-H sword and plate; Dawn (surface X = 37)
	// sells up to the +4 weapons and +2 plate. Exotics are never for sale. (opnum)
	fullStock := world.Party.SurfaceX() == 37
	last := 5 // exclusive letter index
	switch {
	case isWeapon && fullStock:
		last = 15
	case isWeapon:
		last = 8
	case fullStock:
		last = 7
	}
	// Dawn's shops are the journal's magic arms, found.
	if fullStock {
		if isWeapon {
			world.Journal.MagicWeapons = true
		} else {
			world.Journal.MagicArmour = true
		}
	}

	// What the shop stocks (letters B..), with prices, weighed on a card to buy.
	wares := func(weighed bool) []MenuOption {
		var list []MenuOption
		for i := 1; i < last; i++ {
			letter := string(rune(65 + i))
			o := MenuOption{Key: letter, Label: letter + " " + names[nameBase+i] + " " + names[priceBase+i] + "gp"}
			if !world.CanUse(p, isWeapon, i) {
				o.Colour = unusable
			}
			if weighed {
				c := GearCard(world, p, isWeapon, i, CardFoot{IsPrice: true, Price: priceOf(world, priceBase+i)})
				o.Card = &c
			}
			list = append(list, o)
		}
		return list
	}
	owned := func() []MenuOption {
		var list []MenuOption
		for _, o := range wares(false) {
			if world.Party.Gear(isWeapon, int(o.Key[0])-65) > 0 {
				list = append(list, o)
			}
		}
		return list
	}
	nothing := MenuOption{Key: " ", Label: "Nothing"}
	or := func(k, def string) string {
		if k == "" {
			return def
		}
		return k
	}

	if isWeapon {
		io.PrintMessage(shopsMsgWeaponsShop)
	} else {
		io.PrintMessage(shopsMsgArmourShop)
	}
	key := io.ChooseOption([]MenuOption{{Key: "Y", Label: "Yes, list"}, {Key: "N", Label: "No"}}, EchoNone, nil)
	io.Print(or(key, "N"))
	if key == "Y" {
		priceList(world, io, isWeapon, last)
	}
	inUse := "Worn"
	if isWeapon {
		inUse = "Readied"
	}
	io.Print("\n" + inUse + ": " + names[nameBase+int(p.Bytes[inUseBase])])
	io.PrintMessage(shopsMsgBuyOrSell)
	key = io.ChooseOption([]MenuOption{{Key: "B", Label: "Buy"}, {Key: "S", Label: "Sell"}}, EchoNone, nil)
	io.Print(or(key, "S"))

	mode := key
	for {
		if mode == "B" {
			io.PrintMessage(shopsMsgYourInterest)
			k := io.ChooseOption(append(wares(true), nothing), EchoNone, nil)
			if k == "" || k < "B" || int(k[0])-'A' >= last {
				break
			}
			index := int(k[0]) - 'A'
			io.Print(k)
			price := priceOf(world, priceBase+index)
			if price > world.Party.Gold() {
				io.PrintMessage(shopsMsgNoGold)
				return
			}
			if world.Party.Gear(isWeapon, index) > 98 {
				io.PrintMessage(shopsMsgNotEnoughRoom)
				io.Print("\n\n")
				shopsError(io)
				continue
			}
			world.Party.SetGold(world.Party.Gold() - price)
			world.AddGear(isWeapon, index, 1)
			io.PrintMessage(shopsMsgHereYouAre)
			// Something new this member can use: offer to put it to use now.
			if index != int(p.Bytes[inUseBase]) && world.CanUse(p, isWeapon, index) {
				verb := "Wear"
				if isWeapon {
					verb = "Ready"
				}
				io.Print("\n" + verb + " it? (Y/N)-")
				yes := io.ChooseOption([]MenuOption{{Key: "Y", Label: "Yes"}, {Key: "N", Label: "No"}}, EchoNone, nil)
				io.Print(or(yes, "N") + "\n")
				if yes == "Y" {
					world.Equip(p, isWeapon, index)
					io.Print(names[nameBase+index])
					io.PrintMessage(shopsMsgReady)
				}
			}
		} else {
			io.PrintMessage(shopsMsgForSale)
			k := io.ChooseOption(append(owned(), nothing), EchoNone, nil)
			if k == "" || k < "B" || int(k[0])-'A' >= last {
				break
			}
			index := int(k[0]) - 'A'
			io.Print(k)
			if world.Party.Gear(isWeapon, index) < 1 {
				io.PrintMessage(shopsMsgDontOwn) // only what is in the bag sells
				return
			}
			if !AddGold(world, priceOf(world, priceBase+index), false) {
				io.PrintMessage(shopsMsgTooMuchGold)
				shopsError(io)
				return
			}
			world.Party.SetGear(isWeapon, index, world.Party.Gear(isWeapon, index)-1)
			io.PrintMessage(shopsMsgThankYou)
			mode = "S"
		}
	}
	io.PrintMessage(shopsMsgMaybeNextTime)
}

// guild sells keys, torches (five at a time), powders and gems into the party's supply (this port).
func guild(world *World, io IO) {
	world.Journal.Guild = true // the journal's supplies, found
	io.PrintMessage(shopsMsgGuild)
	pa := world.Party
	type item struct {
		cost, quantity int
		get            func() int
		set            func(int)
	}
	items := map[string]item{
		"T": {30, 5, pa.Torches, pa.SetTorches},
		"K": {50, 1, pa.Keys, pa.SetKeys},
		"P": {90, 1, pa.Powders, pa.SetPowders},
		"G": {75, 1, pa.Gems, pa.SetGems},
	}
	for {
		io.PrintMessage(shopsMsgGuildPrices)
		io.PrintMessage(shopsMsgYourNeed)
		key := io.ChooseOption([]MenuOption{
			{Key: "K", Label: "Keys (50 gp)"},
			{Key: "T", Label: "Torches x5 (30 gp)"},
			{Key: "P", Label: "Powders (90 gp)"},
			{Key: "G", Label: "Gems (75 gp)"},
			{Key: "N", Label: "Nothing"},
		}, EchoNone, nil)
		it, ok := items[key]
		if !ok {
			io.Print("N\n\n")
			io.PrintMessage(shopsMsgGuildThanks)
			return
		}
		io.Print(key + "\n")
		if it.get()+it.quantity > 99 {
			io.PrintMessage(shopsMsgNotEnoughRoom)
			shopsError(io)
			continue
		}
		if pa.Gold() < it.cost {
			io.PrintMessage(shopsMsgNoFunds)
			return
		}
		pa.SetGold(pa.Gold() - it.cost)
		it.set(it.get() + it.quantity)
		io.PrintMessage(shopsMsgGuildAnythingElse)
		if !YesNo(io) {
			io.Print("N\n\n")
			io.PrintMessage(shopsMsgGuildThanks)
			return
		}
	}
}

// oracle is Radrion the prophet: offerings of hundreds of gold buy verses of his rhyme.
func oracle(world *World, io IO) {
	world.Journal.Radrion = true // the journal's Radrion, met
	io.PrintMessage(shopsMsgRadrion)
	for {
		io.PrintMessage(shopsMsgOffering)
		key := io.ChooseOption(LetterOptions("0123456789", goldOfferLabel), EchoNone, nil)
		digit := 0
		if key != "" {
			digit = int(key[0] - '0')
		}
		io.Print(strconv.Itoa(digit) + "\n\n")
		cost := digit * 100
		if cost > world.Party.Gold() {
			io.PrintMessage(shopsMsgCantPay)
			shopsError(io)
			return
		}
		world.Party.SetGold(world.Party.Gold() - cost)
		io.Print(world.Resources.Strings["Radrion"][digit])
		HearClue(world, "Radrion:"+strconv.Itoa(digit))
		io.PrintMessage(shopsMsgMoreOffering)
		if !YesNo(io) {
			io.Print("N\n")
			io.PrintMessage(shopsMsgFareWell)
			return
		}
		io.Print("Y\n")
	}
}

// horses is the equine emporium: a horse for 200 gold per member.
func horses(world *World, io IO) {
	cost := world.Party.Size() * 200
	io.PrintMessage(shopsMsgEmporium)
	io.Print(strconv.Itoa(world.Party.Size()))
	io.PrintMessage(shopsMsgHorsesCost)
	io.Print(strconv.Itoa(cost))
	io.PrintMessage(shopsMsgWillYouBuy)
	if !YesNo(io) {
		io.Print("N\n\n")
		io.PrintMessage(shopsMsgTooBad)
		return
	}
	if world.Party.Gold() < cost {
		io.Print("Y\n\n")
		io.PrintMessage(shopsMsgNoGoldHorse)
		shopsError(io)
		return
	}
	world.Party.SetGold(world.Party.Gold() - cost)
	io.Print("Y\n\n")
	io.PrintMessage(shopsMsgRideFast)
	world.Party.SetShape(0x14)
	io.RedrawMap()
}
