package game

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

// cmdFlatWorld is a flat grass Sosaria with the party at (32, 32) and no monsters.
func cmdFlatWorld(t *testing.T) (*World, *FakeIO) {
	w := newWorld(t)
	for i := range w.Current.Tiles {
		w.Current.Tiles[i] = MapValueGrass
	}
	clear(w.Monsters().Bytes)
	w.X, w.Y = 32, 32
	return w, newFakeIO(w.Resources)
}

// cmdNoHooks are turn hooks that do nothing.
func cmdNoHooks() *TurnHooks {
	return &TurnHooks{
		MonsterHooks: MonsterHooks{Attack: func(int) {}, BombTrap: func() {}, ShowBall: func(int, int, int) {}},
		GoWhirlpool:  func() {},
		DungeonTurn:  func() {},
	}
}

func cmdCountSounds(sounds []string, name string) int {
	n := 0
	for _, s := range sounds {
		if s == name {
			n++
		}
	}
	return n
}

func TestMoveWalksOntoGrass(t *testing.T) {
	w, io := cmdFlatWorld(t)
	Move(w, io, MoveNorth)
	if w.Y != 31 || !strings.Contains(io.out(), "North") || !slices.Contains(io.Sounds, "Step") {
		t.Fatalf("y=%d out=%q sounds=%v", w.Y, io.out(), io.Sounds)
	}
}

func TestMoveRefusesWaterAndMountains(t *testing.T) {
	w, io := cmdFlatWorld(t)
	w.PutXYVal(MapValueWater, 33, 32)
	w.PutXYVal(MapValueMountains, 31, 32)
	Move(w, io, MoveEast)
	Move(w, io, MoveWest)
	if w.X != 32 || !strings.Contains(io.out(), "INVALID MOVE!") || cmdCountSounds(io.Sounds, "Bump") != 2 {
		t.Fatalf("x=%d out=%q sounds=%v", w.X, io.out(), io.Sounds)
	}
}

func TestMoveWrapsSurface(t *testing.T) {
	w, io := cmdFlatWorld(t)
	w.X = 0
	Move(w, io, MoveWest)
	if w.X != 63 {
		t.Fatal(w.X)
	}
}

func TestFrigateSailsWaterNotIntoWind(t *testing.T) {
	w, io := cmdFlatWorld(t)
	w.Party.SetShape(PartyShapeFrigate)
	for i := range w.Current.Tiles {
		w.Current.Tiles[i] = MapValueWater
	}
	w.WindDirection = 0
	Move(w, io, MoveNorth)
	if w.Y != 31 {
		t.Fatal(w.Y)
	}
	w.WindDirection = 1 // north wind
	Move(w, io, MoveNorth)
	if w.Y != 31 || !strings.Contains(io.out(), "INVALID MOVE!") {
		t.Fatal(w.Y)
	}
	w.PutXYVal(MapValueGrass, 32, 30)
	w.WindDirection = 0
	Move(w, io, MoveNorth)
	if w.Y != 31 {
		t.Fatal(w.Y)
	}
}

func TestLavaBurns(t *testing.T) {
	w, io := cmdFlatWorld(t)
	w.PutXYVal(MapValueLava, 32, 31)
	before := w.Member(0).HitPoints()
	Move(w, io, MoveNorth)
	if w.Y != 31 || w.Member(0).HitPoints() != before-50 {
		t.Fatal(w.Y, w.Member(0).HitPoints())
	}
}

func TestBoardAndExitHorse(t *testing.T) {
	w, io := cmdFlatWorld(t)
	w.PutXYVal(MapValueHorse, 32, 32)
	Board(w, io)
	if w.Party.Shape() != PartyShapeHorse || w.GetXYVal(32, 32) != MapValueGrass {
		t.Fatal("board")
	}
	Exit(w, io)
	if w.Party.Shape() != PartyShapeOnFoot || w.GetXYVal(32, 32) != MapValueHorse {
		t.Fatal("exit")
	}
	if !strings.Contains(io.out(), "Mount horse!") || !strings.Contains(io.out(), "Dismount") {
		t.Fatal(io.out())
	}
}

func TestCannotLeaveShipOnLand(t *testing.T) {
	w, io := cmdFlatWorld(t)
	w.Party.SetShape(PartyShapeFrigate)
	w.PutXYVal(MapValueBrush, 32, 32)
	Exit(w, io)
	if w.Party.Shape() != PartyShapeFrigate || !strings.Contains(io.out(), "Not here!") {
		t.Fatal(io.out())
	}
}

func TestLookNamesTerrainAndMonsters(t *testing.T) {
	w, io := cmdFlatWorld(t)
	w.PutXYVal(MapValueForest, 32, 31)
	io.Keys = []string{KeyUp, KeyLeft}
	Look(w, io)
	if !strings.Contains(io.out(), "Look-North\n->Forest") {
		t.Fatal(io.out())
	}
	m := w.Monsters()
	m.SetType(0, MapValueOrc)
	m.SetVariant(0, 2)
	m.SetPosition(0, 31, 32)
	w.PutXYVal(MapValueOrc+2, 31, 32)
	Look(w, io)
	if !strings.Contains(io.out(), "->Trolls") {
		t.Fatal(io.out())
	}
}

func TestEnterAndLeaveTown(t *testing.T) {
	w := newWorld(t)
	io := newFakeIO(w.Resources)
	lx, ly := int(w.Resources.Misc.LocationX[2]), int(w.Resources.Misc.LocationY[2])
	w.X, w.Y = lx, ly
	Enter(w, io)
	if w.Party.Location() != LocationTown || w.Current.ID != MapIDFirstTown || w.X != 1 || w.Y != 32 {
		t.Fatal("enter", w.Party.Location(), w.Current.ID, w.X, w.Y)
	}
	if !strings.Contains(io.out(), "Enter town!") {
		t.Fatal(io.out())
	}
	// Walking west onto column 0 leaves the town at the end of the turn.
	w.PutXYVal(MapValueGrass, 0, 32)
	Move(w, io, MoveWest)
	EndTurn(w, io, cmdNoHooks())
	if w.Party.Location() != LocationSosaria || w.X != lx || w.Y != ly || !strings.Contains(io.out(), "Exit to Sosaria!") {
		t.Fatal("leave", w.X, w.Y, io.out())
	}
}

func TestEnterOnGrass(t *testing.T) {
	w, io := cmdFlatWorld(t)
	Enter(w, io)
	if !strings.Contains(io.out(), "<-WHAT?") || w.Party.Location() != LocationSosaria {
		t.Fatal(io.out())
	}
}

func TestTurnCountsMovesAndFood(t *testing.T) {
	w, io := cmdFlatWorld(t)
	food := w.Party.Food()
	for range 10 {
		EndTurn(w, io, cmdNoHooks())
	}
	// Four members each eat a tenth of a ration per turn from the pool.
	if w.Party.Moves() != 40 || w.Party.Food() != food-4 {
		t.Fatal(w.Party.Moves(), w.Party.Food(), food)
	}
}

func TestOneMoongateOpen(t *testing.T) {
	w, _ := cmdFlatWorld(t)
	w.MoonPhase = [2]int{3, 5}
	PlaceMoongates(w)
	misc := w.Resources.Misc
	for i := range 8 {
		want := MapValueGrass
		if i == 3 {
			want = MapValueMoonGate
		}
		if v := w.GetXYVal(int(misc.MoonX[i]), int(misc.MoonY[i])); v != want {
			t.Fatal(i, v)
		}
	}
}

func TestMonsterCanEnter(t *testing.T) {
	cases := []struct {
		terrain, monster int
		want             bool
	}{
		{MapValueGrass, MapValueOrc, true}, {MapValueWater, MapValueOrc, false}, {MapValueWater, MapValuePirate, true},
		{MapValueGrass, MapValuePirate, false}, {MapValueMountains, MapValueDragon, false},
	}
	for _, c := range cases {
		if MonsterCanEnter(c.terrain, c.monster) != c.want {
			t.Error(c)
		}
	}
}

func TestSpawnMonsterRecordsTerrain(t *testing.T) {
	w, _ := cmdFlatWorld(t)
	spawned := 0
	for i := 0; i < 2000 && spawned == 0; i++ {
		SpawnMonster(w)
		for s := range 32 {
			if w.Monsters().Type(s) != 0 {
				spawned++
			}
		}
	}
	if spawned == 0 {
		t.Fatal("nothing spawned")
	}
	m := w.Monsters()
	s := 31
	if m.FreeSlot() == 31 {
		s = 30
	}
	if m.TileUnder(s) != MapValueGrass || w.GetXYVal(m.X(s), m.Y(s)) != m.Type(s) {
		t.Fatal(m.TileUnder(s))
	}
}

func TestMonsterClosesInAndAttacks(t *testing.T) {
	w, io := cmdFlatWorld(t)
	m := w.Monsters()
	m.SetType(0, MapValueOrc)
	m.SetTileUnder(0, MapValueGrass)
	m.SetPosition(0, 35, 32)
	m.SetHP(0, 0xc0)
	w.PutXYVal(MapValueOrc, 35, 32)
	attacked := -1
	hooks := cmdNoHooks()
	hooks.Attack = func(i int) { attacked = i }
	MoveMonsters(w, io, &hooks.MonsterHooks)
	if m.X(0) != 34 || w.GetXYVal(35, 32) != MapValueGrass || w.GetXYVal(34, 32) != MapValueOrc {
		t.Fatal(m.X(0))
	}
	MoveMonsters(w, io, &hooks.MonsterHooks)
	if m.X(0) != 33 {
		t.Fatal(m.X(0))
	}
	MoveMonsters(w, io, &hooks.MonsterHooks)
	if attacked != 0 {
		t.Fatal(attacked)
	}
}

func TestMonsterApproachesShortWayRound(t *testing.T) {
	w, io := cmdFlatWorld(t)
	w.X = 2
	m := w.Monsters()
	m.SetType(0, MapValueOrc)
	m.SetTileUnder(0, MapValueGrass)
	m.SetPosition(0, 60, 32)
	m.SetHP(0, 0xc0)
	w.PutXYVal(MapValueOrc, 60, 32)
	MoveMonsters(w, io, &cmdNoHooks().MonsterHooks)
	if m.X(0) != 61 { // east, across the seam, not 58 squares west
		t.Fatal(m.X(0))
	}
}

// cmdFloorTown is the first town, all floor, no monsters, the party at (20, 20).
func cmdFloorTown(t *testing.T) (*World, *FakeIO) {
	w := newWorld(t)
	w.EnterMap(MapIDFirstTown)
	w.Party.SetLocation(LocationTown)
	for i := range w.Current.Tiles {
		w.Current.Tiles[i] = MapValueFloor
	}
	clear(w.Monsters().Bytes)
	w.X, w.Y = 20, 20
	return w, newFakeIO(w.Resources)
}

func TestWalkingIntoTownsperson(t *testing.T) {
	w, io := cmdFloorTown(t)
	m := w.Monsters()
	m.SetType(3, MapValueJester)
	m.SetTileUnder(3, MapValueFloor)
	m.SetPosition(3, 21, 20)
	w.PutXYVal(MapValueJester, 21, 20)
	if b := Move(w, io, MoveEast); b == nil || *b != (Bump{Kind: BumpPerson, Index: 3}) || w.X != 20 {
		t.Fatal(b)
	}
	if b := Move(w, io, MoveWest); b != nil || w.X != 19 {
		t.Fatal(b)
	}
}

func TestExodusLavaAndDiagonals(t *testing.T) {
	w := newWorld(t)
	at := func(x int) int { return int(w.Surface.Tiles[0x35*64+x]) }
	if w.DiagonalMoves || at(0x0a) != MapValueCastle || at(0x09) != MapValueLava {
		t.Fatal("start")
	}
	w.SetDiagonalMoves(true)
	if at(0x09) != MapValueMountains || at(0x0b) != MapValueMountains {
		t.Fatal("on")
	}
	w.SetDiagonalMoves(false)
	if at(0x0b) != MapValueLava {
		t.Fatal("off")
	}
}

func TestDirectionPromptDiagonals(t *testing.T) {
	w := newWorld(t)
	io := newFakeIO(w.Resources)
	io.Keys = []string{"9"}
	if d := GetDirection(w, io, false, w.DiagonalMoves); d != nil {
		t.Fatal(d)
	}
	w.SetDiagonalMoves(true)
	io.Keys = []string{"9"}
	if d := GetDirection(w, io, false, w.DiagonalMoves); d == nil || d.DX != 1 || d.DY != -1 {
		t.Fatal(d)
	}
}

func TestBumpCounter(t *testing.T) {
	w, io := cmdFloorTown(t)
	w.PutXYVal(MapValueLetterA, 21, 20)
	w.PutXYVal(MapValueMerchant, 22, 20)
	if b := Move(w, io, MoveEast); b == nil || *b != (Bump{Kind: BumpCounter, DX: 1}) {
		t.Fatal(b)
	}
	// A bare counter with nothing behind it is just a wall.
	w.PutXYVal(MapValueFloor, 22, 20)
	if b := Move(w, io, MoveEast); b != nil || w.X != 20 {
		t.Fatal(b)
	}
}

func TestBumpDoorFromSideOnly(t *testing.T) {
	w, io := cmdFloorTown(t)
	w.PutXYVal(MapValueLetterI, 21, 20)
	if b := Move(w, io, MoveEast); b == nil || *b != (Bump{Kind: BumpDoor, DX: 1}) {
		t.Fatal(b)
	}
	w.PutXYVal(MapValueLetterI, 20, 19)
	if b := Move(w, io, MoveNorth); b != nil || w.Y != 20 {
		t.Fatal(b)
	}
}

func TestAmbrosiaDoorAndKey(t *testing.T) {
	w := newWorld(t)
	w.EnterMap(MapIDAmbrosia)
	w.Party.SetLocation(LocationAmbrosia)
	clear(w.Monsters().Bytes)
	w.X, w.Y = 11, 6
	w.Party.SetKeys(1)
	io := newFakeIO(w.Resources)
	if w.GetXYVal(10, 6) != MapValueLetterI {
		t.Fatal("no door")
	}
	if b := Move(w, io, MoveWest); b == nil || *b != (Bump{Kind: BumpDoor, DX: -1}) {
		t.Fatal(b)
	}
	UnlockToward(w, io, -1, 0)
	if w.Party.Keys() != 0 {
		t.Fatal(w.Party.Keys())
	}
	if b := Move(w, io, MoveWest); b != nil || w.X != 10 {
		t.Fatal(b, w.X)
	}
}

func TestBumpMonsterOnSurface(t *testing.T) {
	w := newWorld(t)
	io := newFakeIO(w.Resources)
	for i := range w.Current.Tiles {
		w.Current.Tiles[i] = MapValueGrass
	}
	clear(w.Monsters().Bytes)
	m := w.Monsters()
	m.SetType(0, MapValueOrc)
	m.SetPosition(0, w.X+1, w.Y)
	m.SetHP(0, 0x80)
	x := w.X
	if b := Move(w, io, MoveEast); b == nil || *b != (Bump{Kind: BumpMonster, DX: 1}) || w.X != x {
		t.Fatal(b)
	}
}

func TestPeerGem(t *testing.T) {
	w, io := cmdFlatWorld(t)
	w.Party.SetGems(2)
	PeerGem(w, io)
	if strings.Contains(io.out(), "Whose gem") || w.Party.Gems() != 1 {
		t.Fatal(io.out())
	}
	w.Party.SetGems(0)
	PeerGem(w, io)
	if !strings.Contains(io.out(), "None left") {
		t.Fatal(io.out())
	}
}

func TestPoisonStopsAtOne(t *testing.T) {
	w, io := cmdFlatWorld(t)
	p := w.Member(0)
	p.SetStatus('P')
	p.SetHitPoints(2)
	w.Party.SetFood(500)
	EndTurn(w, io, cmdNoHooks()) // ageing runs every surface turn
	if p.HitPoints() != 1 {
		t.Fatal(p.HitPoints())
	}
	EndTurn(w, io, cmdNoHooks())
	if p.HitPoints() != 1 || p.Status() != 'P' {
		t.Fatal(p.HitPoints())
	}
	w.PoisonKills = true
	EndTurn(w, io, cmdNoHooks())
	if p.HitPoints() != 0 || p.Status() != 'D' {
		t.Fatal(p.HitPoints(), p.Status())
	}
}

func TestStarvationSettings(t *testing.T) {
	w, io := cmdFlatWorld(t)
	w.Party.SetFood(0)
	hp := w.Member(0).HitPoints() // 100
	w.Starvation = StarvationNone
	EndTurn(w, io, cmdNoHooks())
	if w.Member(0).HitPoints() != hp || strings.Count(io.out(), "STARVING!") != 1 {
		t.Fatal(w.Member(0).HitPoints(), io.out())
	}
	w.Starvation = StarvationMild
	for range 20 {
		EndTurn(w, io, cmdNoHooks())
	}
	if w.Member(0).HitPoints() != 50 {
		t.Fatal(w.Member(0).HitPoints())
	}
	w.Starvation = StarvationClassic
	for range 20 {
		EndTurn(w, io, cmdNoHooks())
	}
	if w.Member(0).Status() != 'D' {
		t.Fatal(w.Member(0).Status())
	}
}

func TestTurnTimerLimits(t *testing.T) {
	w := newWorld(t)
	check := func(want ...int) {
		t.Helper()
		for i, ms := range []int{4000, 5000, 6000} {
			if got := w.TimeLimit(ms); got != want[i] {
				t.Fatal(w.Timer, ms, got)
			}
		}
	}
	if w.Timer != TimerFast {
		t.Fatal(w.Timer)
	}
	check(4000, 5000, 6000)
	w.Timer = TimerSlow
	check(10000, 12000, 14000)
	w.Timer = TimerOff
	if w.TimeLimit(4000) != 0 {
		t.Fatal("off has a limit")
	}
}

// cmdTimeoutIO records each WaitCommand's limit and lets every one run out.
type cmdTimeoutIO struct {
	*FakeIO
	waits []int
}

func (t *cmdTimeoutIO) WaitCommand(_ CommandScope, ms int) string {
	t.waits = append(t.waits, ms)
	return ""
}

func TestIdleFieldTurnPasses(t *testing.T) {
	w := newWorld(t)
	io := &cmdTimeoutIO{FakeIO: newFakeIO(w.Resources)}
	g := NewGame(w, io, GameOptions{})
	if k := g.waitForCommand(); k != KeySpace {
		t.Fatal(k)
	}
	sum := 0
	for _, ms := range io.waits {
		sum += ms
	}
	if sum != IdlePassMs {
		t.Fatal(sum)
	}
}

func TestMoongatesRemembered(t *testing.T) {
	w := newWorld(t)
	io := newFakeIO(w.Resources)
	misc := w.Resources.Misc
	w.MoonPhase = [2]int{2, 5}
	HandleMoonStep(w, io)
	if w.X != int(misc.MoonX[5]) || w.Y != int(misc.MoonY[5]) || !slices.Equal(w.GatesKnown, []int{5}) {
		t.Fatal(w.X, w.Y, w.GatesKnown)
	}
	HandleMoonStep(w, io)
	if !slices.Equal(w.GatesKnown, []int{5}) {
		t.Fatal(w.GatesKnown)
	}
	w.MoonPhase = [2]int{2, 0}
	HandleMoonStep(w, io)
	if !slices.Equal(w.GatesKnown, []int{5, 0}) {
		t.Fatal(w.GatesKnown)
	}
	raw, err := json.Marshal(Serialize(w))
	if err != nil {
		t.Fatal(err)
	}
	var data SaveData
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	b := NewWorld(w.Resources, 0)
	if !Restore(b, data) || !slices.Equal(b.GatesKnown, []int{5, 0}) {
		t.Fatal(b.GatesKnown)
	}
	data.Version, data.Moongates = 5, nil
	if !Restore(b, data) || len(b.GatesKnown) != 0 {
		t.Fatal(b.GatesKnown)
	}
	w.NewGame()
	if len(w.GatesKnown) != 0 {
		t.Fatal(w.GatesKnown)
	}
}

func TestFloorEntriesBeforeExodusAreWalkedOver(t *testing.T) {
	w := newWorld(t)
	io := newFakeIO(w.Resources)
	w.EnterMap(401)
	w.Party.SetLocation(LocationCastle)
	w.X, w.Y = 31, 15
	w.PutXYVal(MapValueFloor, 31, 15)
	i := w.Monsters().At(31, 14)
	if i < 0 || w.Monsters().Type(i) != MapValueFloor {
		t.Fatal(i)
	}
	if b := Move(w, io, MoveNorth); b != nil || w.X != 31 || w.Y != 14 {
		t.Fatal(b, w.X, w.Y)
	}
}

// The whirlpool test that takes a party standing on its ship replaced
// world.rng.range with a stub; Random is not swappable here, so it is not ported.

func TestWhirlpoolToAmbrosia(t *testing.T) {
	w := newWorld(t)
	io := newFakeIO(w.Resources)
	g := NewGame(w, io, GameOptions{})
	g.goWhirlpool()
	if w.Party.Location() != LocationAmbrosia {
		t.Fatal(w.Party.Location())
	}
	messages := w.Resources.Strings["Messages"]
	for _, n := range []int{111, 112, 113} {
		if !strings.Contains(io.out(), messages[n-1]) {
			t.Fatal(n)
		}
	}
	for _, line := range strings.Split(io.out(), "\n") {
		if len(line) > 16 {
			t.Fatalf("line too long: %q", line)
		}
	}
}
