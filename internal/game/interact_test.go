package game

import (
	"reflect"
	"slices"
	"strings"
	"testing"
)

func townWorld(t *testing.T) (*World, *FakeIO) {
	t.Helper()
	w := newWorld(t, 99)
	w.ReturnX, w.ReturnY = 46, 19
	w.Party.SetSurfaceX(46)
	w.Party.SetSurfaceY(19)
	w.EnterMap(MapIDFirstTown)
	w.Party.SetLocation(LocationTown)
	for i := range w.Current.Tiles {
		w.Current.Tiles[i] = MapValueFloor
	}
	clear(w.Monsters().Bytes)
	w.X, w.Y = 20, 20
	return w, newFakeIO(w.Resources)
}

func want(t *testing.T, cond bool, msg ...any) {
	t.Helper()
	if !cond {
		t.Fatal(msg...)
	}
}

func contains(io *FakeIO, s string) bool { return strings.Contains(io.out(), s) }

// placeMonster puts a creature of type v in slot 0 at (x, y) on floor.
func placeMonster(w *World, v, x, y int) *MonsterTable {
	m := w.Monsters()
	m.SetType(0, v)
	m.SetTileUnder(0, MapValueFloor)
	m.SetPosition(0, x, y)
	w.PutXYVal(v, x, y)
	return m
}

func TestSpeechDecodes(t *testing.T) {
	w := newWorld(t)
	talk := w.Resources.Talk[MapIDLordBritishCastle]
	want(t, strings.Contains(Speech(talk, 1), "WEST"), Speech(talk, 1))
	want(t, strings.Contains(Speech(talk, 2), "COOKIE"), Speech(talk, 2))
}

func TestTalkToTownspersonAndMerchant(t *testing.T) {
	w, io := townWorld(t)
	m := placeMonster(w, MapValueJester, 21, 20)
	m.SetHP(0, 0x02) // stationary, says line 2
	io.Keys = []string{KeyRight}
	Transact(w, io)
	want(t, contains(io, Speech(w.Current.Talk, 2)[:8]), io.out())

	w.Y = 17 // 17 & 7 = 1: grocer
	w.PutXYVal(MapValueLetterA, 21, 17)
	w.PutXYVal(MapValueMerchant, 22, 17)
	io.Keys = []string{KeyRight, "N"}
	io.Inputs = []string{"20"}
	gold, food := w.Party.Gold(), w.Party.Food()
	Transact(w, io)
	want(t, contains(io, "GROCER"))
	want(t, w.Party.Gold() == gold-20 && w.Party.Food() == food+20, w.Party.Gold(), w.Party.Food())
}

func TestShopSellsAndBuysBack(t *testing.T) {
	w, io := townWorld(t)
	p := w.Member(1)
	w.Party.SetGold(500)
	w.Party.SetSurfaceX(0)
	io.Keys = []string{"N", "B", "C", "N", KeyEscape}
	Shop(w, io, 3, 1)
	for _, s := range []string{"WEAPONS SHOP", "Readied: Dagger", "Here you are", "Ready it? (Y/N)-N"} {
		want(t, contains(io, s), s)
	}
	want(t, w.Party.Weapons(2) == 1 && p.Bytes[48] == 1 && w.Party.Gold() == 470)
	io.Keys = []string{"N", "S", "C", KeyEscape}
	Shop(w, io, 3, 1)
	want(t, contains(io, "Thank you") && w.Party.Weapons(2) == 0 && w.Party.Gold() == 500)
}

func TestShopReadiesOnTheSpot(t *testing.T) {
	w, io := townWorld(t)
	p := w.Member(1)
	w.Party.SetGold(500)
	w.Party.SetSurfaceX(0)
	io.Keys = []string{"N", "B", "G", "Y", KeyEscape}
	Shop(w, io, 3, 1)
	want(t, contains(io, "Sword Ready"), io.out())
	want(t, p.Bytes[48] == 6 && w.Party.Weapons(6) == 0 && w.Party.Weapons(1) == 1)
}

func TestShopNoOfferForUnusable(t *testing.T) {
	w, io := townWorld(t)
	w.Party.SetGold(500)
	w.Party.SetSurfaceX(0)
	io.Keys = []string{"N", "B", "H", KeyEscape}
	Shop(w, io, 3, 0)
	want(t, !contains(io, "Ready it?") && w.Party.Weapons(7) == 1)
}

func TestHealerCures(t *testing.T) {
	w, io := townWorld(t)
	w.Party.SetGold(300)
	w.Member(2).SetStatus('P')
	io.Keys = []string{"1", "Y", "3"}
	Shop(w, io, 2, 0)
	want(t, w.Member(2).Status() == 'G' && w.Party.Gold() == 200)
}

func TestPubDecline(t *testing.T) {
	w, io := townWorld(t)
	w.Party.SetGold(100)
	io.Inputs = []string{""}
	Shop(w, io, 0, 0)
	want(t, contains(io, "come again") && !contains(io, "SCUM") && !slices.Contains(io.Sounds, "Error1"))
	want(t, w.Party.Gold() == 100)
	io.Inputs = []string{"0"}
	Shop(w, io, 0, 0)
	want(t, !contains(io, "SCUM"))
	io.Inputs = []string{"3"}
	Shop(w, io, 0, 0)
	want(t, contains(io, "LEAVE MY SHOP") && slices.Contains(io.Sounds, "Error1") && w.Party.Gold() == 100)
}

func TestUnlockWalkedInto(t *testing.T) {
	w, io := townWorld(t)
	w.PutXYVal(MapValueLetterI, 21, 20)
	w.Party.SetKeys(1)
	UnlockToward(w, io, 1, 0)
	want(t, w.GetXYVal(21, 20) == MapValueFloor && w.Party.Keys() == 0)
}

func TestShopWalkedInto(t *testing.T) {
	w, io := townWorld(t)
	w.Y = 17
	w.PutXYVal(MapValueLetterA, 21, 17)
	w.PutXYVal(MapValueMerchant, 22, 17)
	io.Keys = []string{"N"}
	io.Inputs = []string{"10"}
	food, gold := w.Party.Food(), w.Party.Gold()
	TransactToward(w, io, 1, 0)
	want(t, contains(io, "GROCER") && w.Party.Food() == food+10 && w.Party.Gold() == gold-10)
}

func TestUnlockWithKey(t *testing.T) {
	w, io := townWorld(t)
	w.PutXYVal(MapValueLetterI, 21, 20)
	w.Party.SetKeys(1)
	io.Keys = []string{KeyRight}
	Unlock(w, io)
	want(t, w.GetXYVal(21, 20) == MapValueFloor && w.Party.Keys() == 0 && slices.Contains(io.Sounds, "Creak"))
}

func TestOpenChest(t *testing.T) {
	w, io := townWorld(t)
	w.PutXYVal(MapValueChest, w.X, w.Y)
	w.Member(0).Bytes[19] = 99
	gold := w.Party.Gold()
	io.Keys = []string{"1"}
	GetChest(w, io, 0, "command")
	want(t, w.GetXYVal(w.X, w.Y) == MapValueFloor && w.Party.Gold() > gold && contains(io, "GOLD+"), io.out())
}

func TestStealChest(t *testing.T) {
	w, io := townWorld(t)
	w.Member(0).Bytes[19] = 99
	w.PutXYVal(MapValueLetterA, 21, 20)
	w.PutXYVal(MapValueChest, 22, 20)
	io.Keys = []string{"1", KeyRight}
	Steal(w, io)
	want(t, w.GetXYVal(22, 20) == MapValueFloor && contains(io, "GOLD+"), io.out())
}

func TestReadyWeaponRules(t *testing.T) {
	w, io := townWorld(t)
	p := w.Member(0)
	w.Party.SetWeapons(7, 1)
	io.Keys = []string{"1", "H"}
	ReadyWeapon(w, io, -1)
	want(t, contains(io, "Not allowed"), io.out())
	io.Keys = []string{"1", "B"}
	ReadyWeapon(w, io, -1)
	want(t, contains(io, "Dagger Ready") && p.Bytes[48] == 1, io.out())
	io.Keys = []string{"1", "C"}
	WearArmour(w, io)
	want(t, contains(io, "Not owned"), io.out())
}

func TestReadySwapsThroughBag(t *testing.T) {
	w, io := townWorld(t)
	w.Party.SetWeapons(2, 1)
	io.Keys = []string{"1", "C"}
	ReadyWeapon(w, io, -1)
	want(t, w.Member(0).Bytes[48] == 2 && w.Party.Weapons(2) == 0 && w.Party.Weapons(1) == 1)
	w.Member(1).Bytes[48] = 0
	io.Keys = []string{"2", "B"}
	ReadyWeapon(w, io, -1)
	want(t, w.Member(1).Bytes[48] == 1 && w.Party.Weapons(1) == 0)
}

func TestStatsScreen(t *testing.T) {
	w, io := townWorld(t)
	io.Keys = []string{"1", KeyEnter, KeyEscape}
	Stats(w, io, -1)
	want(t, contains(io, "STR...") && contains(io, "H.P..."), io.out())
	for _, r := range w.Resources.Strings["Races"] {
		if r[0] == w.Member(0).Race() {
			want(t, contains(io, r), r)
			break
		}
	}
	want(t, contains(io, "Thief"))
}

func TestStatsListsBag(t *testing.T) {
	w, io := townWorld(t)
	w.Party.ClearGear()
	w.Party.SetWeapons(6, 1)
	w.Party.SetArmour(2, 1)
	io.Keys = append([]string{"1"}, slices.Repeat([]string{KeyEnter}, 40)...)
	Stats(w, io, -1)
	names := w.Resources.Strings["WeaponsArmour"]
	out := io.out()
	bag := out[strings.Index(out, "*PARTY WEAPONS*"):]
	want(t, strings.Contains(bag, "01-"+names[6]+"-(G)"), bag)
	want(t, strings.Contains(bag, "*PARTY ARMOUR*\n01-"+names[18]+"-(C)"), bag)
	want(t, !strings.Contains(bag, "Hands") && !strings.Contains(bag, "Skin"), bag)
}

func TestCastThiefAndCleric(t *testing.T) {
	w, io := townWorld(t)
	io.Keys = []string{"1"}
	Cast(w, io, -1)
	want(t, contains(io, "Not a mage"), io.out())
	cleric := w.Member(2)
	cleric.SetMana(20)
	cleric.SetHitPoints(10)
	io.Keys = []string{"3", "C", "3"}
	Cast(w, io, -1)
	want(t, contains(io, "SANCTU") && cleric.HitPoints() > 10 && cleric.Mana() == 10, io.out())
}

func TestBribeGuard(t *testing.T) {
	w, io := townWorld(t)
	m := placeMonster(w, MapValueGuard, 21, 20)
	io.Keys = []string{"1", KeyRight}
	io.Inputs = []string{"BRIBE"}
	w.Party.SetGold(150)
	OtherCommand(w, io, false)
	want(t, m.Type(0) == 0 && w.Party.Gold() == 50)
}

func TestCannonsSinkPirate(t *testing.T) {
	w := newWorld(t, 5)
	io := newFakeIO(w.Resources)
	for i := range w.Current.Tiles {
		w.Current.Tiles[i] = MapValueWater
	}
	clear(w.Monsters().Bytes)
	w.Party.SetShape(0x16)
	w.X, w.Y = 32, 32
	m := w.Monsters()
	m.SetType(0, MapValuePirate)
	m.SetTileUnder(0, MapValueWater)
	m.SetPosition(0, 34, 32)
	w.PutXYVal(MapValuePirate, 34, 32)
	destroyed := false
	for i := 0; i < 20 && !destroyed; i++ {
		io.Keys = []string{KeyRight}
		Fire(w, io)
		destroyed = m.Type(0) == 0
	}
	want(t, destroyed && contains(io, "destroyed!") && w.GetXYVal(34, 32) == MapValueWater)
}

func TestCreateCharacterAndFormParty(t *testing.T) {
	w := NewWorld(newWorld(t).Resources, 3)
	io := newFakeIO(w.Resources)
	io.Keys = []string{"5", "T", "F", "E", "W", "Y", " "}
	io.Inputs = []string{"Mira", "15,10,15,10"}
	CreateCharacter(w, io)
	p := w.Roster.Get(4)
	want(t, p.Name() == "Mira" && p.ClassLetter() == 'W' && p.Race() == 'E' && p.Intelligence() == 15 && p.HitPoints() == 100, p.Name())

	io.Keys = []string{"5", "D", " ", "M"}
	FormParty(w, io)
	want(t, w.Party.Formed() && !w.Party.RulesPending() && w.Starvation == StarvationMild)
	want(t, w.Party.Size() == 1 && w.Party.MemberSlot(0) == 4 && p.InParty() && w.X == 42)
}

func TestClearNotes(t *testing.T) {
	w := newWorld(t)
	io := newFakeIO(w.Resources)
	w.AutoMap.MarkSquares(MapIDSosaria, []Point{{42, 20}})
	w.AutoMap.Mark(412, 0, 1, 1)
	w.Journal.LordBritish = true
	w.Journal.Clues = append(w.Journal.Clues, "Moon#3")
	w.GatesKnown = append(w.GatesKnown, 3)
	io.Keys = []string{"Y", "1", "D", " ", "M"}
	FormParty(w, io)
	want(t, w.AutoMap.SquareSeen(MapIDSosaria, 42, 20) && slices.Equal(w.Journal.Clues, []string{"Moon#3"}))
	io.Keys = []string{"N"}
	ClearNotes(w, io)
	want(t, slices.Equal(w.GatesKnown, []int{3}))
	io.Keys = []string{"Y", " "}
	ClearNotes(w, io)
	want(t, !w.AutoMap.SquareSeen(MapIDSosaria, 42, 20) && !w.AutoMap.HasDungeon(412))
	want(t, reflect.DeepEqual(w.Journal, EmptyJournal()), w.Journal)
	want(t, len(w.GatesKnown) == 0 && contains(io, "(Notes cleared!)"))
}

func TestJourneyNeedsParty(t *testing.T) {
	w := NewWorld(newWorld(t).Resources, 3)
	io := newFakeIO(w.Resources)
	played := false
	io.Keys = []string{"J", " "}
	want(t, expectNoKeys(func() { MainMenu(w, io, func() { played = true }, MenuOptions{}) }), "menu returned")
	want(t, !played && contains(io, "(Not formed)"))
}

// titleIO answers the title menu with Quit when it is offered, else Settings, and stops at Settings.
type titleIO struct {
	*FakeIO
	t      *testing.T
	listed []string
}

type errStop struct{}

func (o *titleIO) ChooseFromList(options []MenuOption, place *MenuPlacement) string {
	o.listed = nil
	for _, op := range options {
		o.listed = append(o.listed, op.Label)
	}
	// The window (a border row either side of the entries) ends above the copyright line.
	if place.Row+len(options)+1 > TitleMenuLastRow {
		o.t.Errorf("menu window overruns: row %d, %d entries", place.Row, len(options))
	}
	if slices.Contains(o.listed, "Quit") {
		return "Q"
	}
	return "S"
}

func (o *titleIO) ShowSettings() { panic(errStop{}) }

func TestTitleQuit(t *testing.T) {
	w := newWorld(t)
	offered := func(withQuit bool) ([]string, int) {
		io := &titleIO{FakeIO: newFakeIO(w.Resources), t: t}
		calls := 0
		opts := MenuOptions{}
		if withQuit {
			opts.Quit = func() { calls++; panic(errStop{}) } // the app would be gone
		}
		func() {
			defer func() {
				if r := recover(); r != nil {
					if _, ok := r.(errStop); !ok {
						panic(r)
					}
				}
			}()
			MainMenu(w, io, func() {}, opts)
		}()
		return io.listed, calls
	}
	listed, _ := offered(false)
	want(t, !slices.Contains(listed, "Quit"), listed)
	listed, calls := offered(true)
	want(t, len(listed) > 0 && listed[len(listed)-1] == "Quit" && calls == 1, listed, calls)
}

func TestCreateCharacterCancelled(t *testing.T) {
	w := NewWorld(newWorld(t).Resources, 3)
	io := newFakeIO(w.Resources)
	io.Keys = []string{"6", "U", "F", "H", "F"}
	io.Inputs = []string{""}
	CreateCharacter(w, io)
	want(t, !w.Roster.Get(5).Exists())
}

func TestReformAndTerminate(t *testing.T) {
	w := newWorld(t, 3)
	io := newFakeIO(w.Resources)
	want(t, w.Party.Formed())
	io.Keys = []string{"Y", "2", "1", "D", " ", "C"}
	FormParty(w, io)
	want(t, w.Party.Size() == 2 && w.PoisonKills && w.Party.MemberSlot(0) == 1 && w.Party.MemberSlot(1) == 0)
	want(t, !w.Roster.Get(2).InParty())
	io.Keys = []string{"3", "N"}
	TerminateCharacter(w, io)
	want(t, w.Roster.Get(2).Exists())
	io.Keys = []string{"3", "Y", " "}
	TerminateCharacter(w, io)
	want(t, !w.Roster.Get(2).Exists())
}

func TestNoKeyLeft(t *testing.T) {
	w, io := townWorld(t)
	w.PutXYVal(MapValueLetterI, 21, 20)
	w.Party.SetKeys(0)
	UnlockToward(w, io, 1, 0)
	want(t, !contains(io, "Whose key") && contains(io, "None left") && w.GetXYVal(21, 20) == MapValueLetterI)
}

func shopWorld(t *testing.T, counter, y int) (*World, *FakeIO) {
	w, io := townWorld(t)
	w.Y = y
	w.PutXYVal(counter, 21, y)
	w.PutXYVal(MapValueMerchant, 22, y)
	w.Party.SetGold(20000)
	return w, io
}

func TestOrdinaryWeaponsShop(t *testing.T) {
	w, io := shopWorld(t, MapValueLetterA, 19)
	io.Keys = []string{"1", "N", "B", "H", "I", " "}
	TransactToward(w, io, 1, 0)
	want(t, w.Party.Weapons(7) == 1 && w.Party.Weapons(8) == 0 && contains(io, "maybe\nnext time"), io.out())
}

func TestOrdinaryArmourShop(t *testing.T) {
	w, io := shopWorld(t, MapValueLetterA, 20)
	io.Keys = []string{"1", "N", "B", "E", "F", " "}
	TransactToward(w, io, 1, 0)
	want(t, w.Party.Armour(4) == 1 && w.Party.Armour(5) == 0)
}

func TestDawnShops(t *testing.T) {
	w, io := shopWorld(t, MapValueLetterA, 19)
	w.Party.SetSurfaceX(37)
	io.Keys = []string{"1", "N", "B", "O", "P", " "}
	TransactToward(w, io, 1, 0)
	want(t, w.Party.Weapons(14) == 1 && w.Party.Weapons(15) == 0)
	aw, aio := shopWorld(t, MapValueLetterA, 20)
	aw.Party.SetSurfaceX(37)
	aio.Keys = []string{"1", "N", "B", "G", "H", " "}
	TransactToward(aw, aio, 1, 0)
	want(t, aw.Party.Armour(6) == 1 && aw.Party.Armour(7) == 0)
}

func TestLordBritishRaises(t *testing.T) {
	w, io := townWorld(t)
	placeMonster(w, MapValueLordBritish, 21, 20)
	p := w.Member(2)
	p.Bytes[30] = 1
	io.Keys = []string{"3"}
	TransactToward(w, io, 1, 0)
	want(t, contains(io, "Transact-3") && strings.Contains(strings.ToUpper(io.out()), "GREATER"), io.out())
	want(t, p.MaxHitPoints() == 200 && p.HitPoints() == 200 && w.Member(0).MaxHitPoints() == 100)
}

func TestChooseRules(t *testing.T) {
	w, io := townWorld(t)
	w.Party.SetRulesPending(true)
	io.Keys = []string{"C"}
	want(t, ChooseRules(w, io))
	want(t, w.PoisonKills && w.Starvation == StarvationClassic && !w.BalancedXP && w.Timer == TimerFast && !w.Party.RulesPending())
	w.Party.SetRulesPending(true)
	io.Keys = []string{"M"}
	ChooseRules(w, io)
	want(t, !w.PoisonKills && w.Starvation == StarvationMild && w.BalancedXP && w.Timer == TimerSlow)
	io.Keys = []string{"S"}
	ChooseRules(w, io)
	want(t, !w.PoisonKills && w.Starvation == StarvationNone && w.BalancedXP && w.Timer == TimerOff)
	w.Party.SetRulesPending(true)
	io.Keys = []string{KeyEscape}
	want(t, !ChooseRules(w, io) && w.Party.RulesPending())
}
