package game

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/retro-ware/u3-go/internal/res"
)

// All mutable game state that is not part of the party or roster records:
// the current map, its monsters, the party's position, the whirlpool, the
// moons, the wind and the turn timers.
//
// The original kept the "current Sosaria" on disk (resource 419) and pushed
// a copy before entering a town. Here World.Surface always holds it; entering
// a town loads that town into World.Current and leaving points Current back
// at Surface.

// Map resource IDs, as LoadUltimaMap() numbers them.
const (
	BaseRes                = res.BaseRes
	MapIDLordBritishCastle = res.LordBritishCastle
	MapIDExodusCastle      = res.ExodusCastle
	MapIDFirstTown         = res.FirstTown
	MapIDFirstDungeon      = res.FirstDungeon
	MapIDLastDungeon       = res.LastDungeon
	MapIDSosaria           = res.Sosaria
	MapIDAmbrosia          = res.Ambrosia
)

// MapState is a loaded map plus the creatures living on it.
type MapState struct {
	ID       int           // resource ID, 400..421
	Size     int           // edge length: 64 for every surface, town and castle map
	Tiles    []byte        // Size*Size map values, row major
	Monsters *MonsterTable //
	Talk     []byte        // NPC dialogue for towns and castles
}

// Whirlpool is the whirlpool that roams Sosaria, saved with the surface map.
type Whirlpool struct{ X, Y, DX, DY int }

// Point is a map or arena position.
type Point struct{ X, Y int }

// Combatant is one creature in a combat arena: up to eight monsters and four
// members. A member who is not present has X = Y = 255.
type Combatant struct {
	X, Y      int
	TileUnder int    // the arena shape under the creature, restored when it moves or dies
	Shape     int    // shape drawn for the creature
	HP        int    // monsters only: hit points, 0 = slot unused
	DiedAt    *Point // members only: where they fell, so a revival can put them back nearby
	Start     *Point // members only: the starting square, the fallback for a revival
}

// BlockExodusApproach mirrors BlockExodus(): the lava either side of Exodus'
// castle stays without diagonal moves and becomes mountains with them, as on
// the Mac, so the castle is reached only by sea.
func BlockExodusApproach(m *MapState, diagonalMoves bool) {
	at := func(x, y int) byte { return m.Tiles[y*m.Size+x] }
	if at(0x0a, 0x35) != MapValueCastle || at(0x0b, 0x36) != MapValueWater || at(0x0c, 0x35) != MapValueMountains {
		return
	}
	flank := byte(MapValueLava)
	if diagonalMoves {
		flank = MapValueMountains
	}
	m.Tiles[0x35*m.Size+0x09] = flank
	m.Tiles[0x35*m.Size+0x0b] = flank
}

// mapSlips are single-byte slips in the shipped maps, put right as a map
// loads, only where the wrong value is still there.
var mapSlips = []struct{ id, x, y, wrong, right int }{
	// Death Gulch, the armoury's east wall: a force field where a wall belongs.
	{411, 20, 56, MapValueForceField, MapValueWall},
}

// CombatState is the state of a fight (combat.go).
type CombatState struct {
	Tiles            []byte       // 11x11 arena shapes
	Monsters         []*Combatant //
	Members          []*Combatant //
	MonsterShape     int          // shape of the monster type being fought
	MonsterVariant   int          // variant 0..2 of that type
	PreviousLocation int          // Party[3] before combat, restored on victory
	ActiveMember     int          // whose turn it is, 0..3
	MarkedMember     int          // whose turn the UI outlines, or -1
	MarkedAt         float64      // when the mark was set (ms) ...
	MarkedFor        float64      // ... and how long the member has to act; 0 once chosen
	MarkedFrozenAt   float64      // when the outline's fade was frozen, or 0
}

// Now is the clock in milliseconds the combat turn marks use (performance.now()).
var Now = func() float64 { return float64(time.Now().UnixNano()) / 1e6 }

// HoldCombatMark stops a combat turn's outline where it is while something
// holds the game up (the Pause menu), so the turn cannot expire behind it.
// The returned function starts the fade again from where it stopped.
func HoldCombatMark(c *CombatState, now func() float64) func() {
	if now == nil {
		now = Now
	}
	var fading *CombatState
	if c != nil && c.MarkedFor > 0 && c.MarkedFrozenAt == 0 {
		fading = c
		fading.MarkedFrozenAt = now()
	}
	return func() {
		if fading == nil || fading.MarkedFrozenAt == 0 {
			return
		}
		fading.MarkedAt += now() - fading.MarkedFrozenAt
		fading.MarkedFrozenAt = 0
	}
}

// DungeonState is the state inside a dungeon.
type DungeonState struct {
	Tiles   []byte // 8 levels x 16 x 16 cells
	Level   int    // 0..7
	Heading int    // 0 north, 1 east, 2 south, 3 west
	Torch   int    // turns of light remaining; 0 is darkness (gTorch)
	Exit    bool   // the party is leaving (gExitDungeon)
}

// Dungeon cell values. The high bits mark walls; low bits are features.
const (
	DungeonCellOpen       = 0x00
	DungeonCellTimeLord   = 0x01
	DungeonCellFountain   = 0x02
	DungeonCellWind       = 0x03
	DungeonCellTrap       = 0x04
	DungeonCellMark       = 0x05
	DungeonCellGremlins   = 0x06
	DungeonCellWriting    = 0x08
	DungeonCellLadderUp   = 0x10
	DungeonCellLadderDown = 0x20
	DungeonCellChest      = 0x40
	DungeonCellWall       = 0x80
	DungeonCellSecretDoor = 0xa0
	DungeonCellDoor       = 0xc0
)

// Starvation is how an empty larder bites: see World.Starvation.
type Starvation string

const (
	StarvationNone    Starvation = "none"
	StarvationMild    Starvation = "mild"
	StarvationClassic Starvation = "classic"
)

// StarvationModes lists the settings in menu order.
var StarvationModes = []Starvation{StarvationNone, StarvationMild, StarvationClassic}

// Timer is the turn timer: see World.Timer.
type Timer string

const (
	TimerSlow Timer = "slow"
	TimerFast Timer = "fast"
	TimerOff  Timer = "off"
)

// TimerModes lists the settings in menu order.
var TimerModes = []Timer{TimerSlow, TimerFast, TimerOff}

// Ball is a spell ball or cannon shot drawn over the map or arena. Its
// coordinates are map coordinates outside combat, arena cells during it.
type Ball struct {
	X, Y, Shape int
	Hit         int // 0 while flying; 1..3 for the frames of a hit
}

// SpellFlags are the once-per-fight flags of Repond and Pontori (g5521, g56E7).
type SpellFlags struct{ Repond, Pontori bool }

// World is the whole game state.
type World struct {
	Resources *res.Resources
	Rng       *Random
	Party     *Party
	Roster    *Roster

	Surface        *MapState // persistent overworld state
	Current        *MapState // the map the party is on: Surface, or a town/castle/dungeon
	Whirlpool      Whirlpool
	WhirlpoolTimer int // idle ticks until the whirlpool next moves (gWhirlCtr)

	X, Y             int // party position on Current (xpos, ypos)
	ReturnX, ReturnY int // where to reappear on Sosaria after a town (zp[0xE3], zp[0xE4])

	// Moon phases: MoonTimer counts turns to the next change, MoonPhase is
	// 0..7. Index 0 is Trammel (every 12 turns), 1 Felucca (every 4), which
	// picks the open moongate.
	MoonTimer, MoonPhase [2]int

	GatesKnown []int // moongates the party has come out of (0..7); View map marks them
	TipsNudged bool  // whether this game has pointed to Tips

	WindDirection int // 0 calm, 1 north, 2 east, 3 south, 4 west
	WindTimer     int
	WindEnabled   bool

	AgeTimer   [2]int // turn counters of AgeChars() (gTime[0], gTime[1])
	TimeNegate int    // turns during which monsters do not move (Negate Time)

	mountToggle int // alternates each turn while riding or sailing (zp[0xCD])

	HorseFacingEast  bool // cosmetic only
	Done             bool // the game should stop (gDone)
	Resurrecting     bool // nested loops unwind (gResurrect)
	CommandCancelled bool // the player backed out of a command before it did anything

	Combat  *CombatState // fight in progress, or nil
	Dungeon DungeonState // while Party.Location() == LocationDungeon
	Ball    *Ball        // a ball or shot in flight, or nil

	LastCard        int      // which card slot Exodus expects next, 0x1E..0x21 (lastCard)
	AutoMap         *AutoMap // the dungeon auto-map; the UI may replace it with one that persists
	MapMode         MapMode  // how the auto-map shows in dungeons
	OnMapModeChange func()   // called when the map mode changes
	ChestTrapsArmed bool     // cleared by Appar Unem / Steal so chests do not trigger traps

	Music        int  // the music track that should be playing (gSongNext)
	SoundEnabled bool // sound effects on or off (the V command)

	// LoadLastSave restores the last save into this world and reports
	// whether there was one. Used after a party wipe.
	LoadLastSave func() bool

	// Trail is the squares the leader last walked through in a town, most
	// recent first, at most three: the other members are drawn on them.
	Trail []Point

	OnRulesChange      func() // the game changed a rule setting itself
	OnAutoCombatChange func() // the game turned auto-combat off itself

	BalancedXP    bool        // a kill's experience is shared among the living members
	LastSpell     map[int]int // each member's last spell this session
	Journal       JournalState
	AutoCombat    bool
	PoisonKills   bool
	Starvation    Starvation
	Timer         Timer
	DiagonalMoves bool // may the party move, attack and fire diagonally
	SpellFlags    SpellFlags
}

// NewWorld builds a world on the pristine resources; seed 0 seeds from the clock.
func NewWorld(r *res.Resources, seed uint32) *World {
	w := &World{
		Resources:       r,
		Rng:             NewRandom(seed),
		Party:           NewParty(make([]byte, PartyRecordSize)),
		Roster:          NewRoster(make([]byte, PlayerRecordSize*RosterSize)),
		Whirlpool:       Whirlpool{2, 0x3e, 1, 1},
		WhirlpoolTimer:  4,
		MoonTimer:       [2]int{12, 4},
		MoonPhase:       [2]int{4, 4},
		TipsNudged:      true,
		WindEnabled:     true,
		AgeTimer:        [2]int{10, 9},
		Dungeon:         DungeonState{Tiles: make([]byte, 2048)},
		LastCard:        0x1e,
		AutoMap:         NewAutoMap(nil),
		MapMode:         MapModeOff,
		ChestTrapsArmed: true,
		SoundEnabled:    true,
		BalancedXP:      true,
		LastSpell:       map[int]int{},
		Journal:         EmptyJournal(),
		Starvation:      StarvationMild,
		Timer:           TimerFast,
	}
	w.Surface = w.LoadMapState(MapIDSosaria)
	w.Current = w.Surface
	return w
}

// TakeCancelled reports whether the last command was backed out of, clearing the flag.
func (w *World) TakeCancelled() bool {
	c := w.CommandCancelled
	w.CommandCancelled = false
	return c
}

// MapSize is the current map's edge length.
func (w *World) MapSize() int { return w.Current.Size }

// Constrain wraps a coordinate onto the map; Sosaria wraps like a torus.
func (w *World) Constrain(v int) int {
	n := w.Current.Size
	return (v%n + n) % n
}

// GetXYVal reads the map value at (x, y), wrapping. Off the edge of a town,
// castle or dungeon it is grass, so the party can walk out. (GetXYVal)
func (w *World) GetXYVal(x, y int) int {
	c := w.Current
	value := int(c.Tiles[w.Constrain(y)*c.Size+w.Constrain(x)])
	if w.Party.Location() != LocationSosaria && (x < 0 || x >= c.Size || y < 0 || y >= c.Size) {
		return MapValueGrass
	}
	return value
}

// PutXYVal writes a map value, wrapping. (PutXYVal)
func (w *World) PutXYVal(value, x, y int) {
	c := w.Current
	c.Tiles[w.Constrain(y)*c.Size+w.Constrain(x)] = byte(value)
}

// Monsters is the current map's monster table.
func (w *World) Monsters() *MonsterTable { return w.Current.Monsters }

// OnSurface is true on Sosaria.
func (w *World) OnSurface() bool { return w.Party.Location() == LocationSosaria }

// InTownOrCastle is true on the maps left by walking off the edge.
func (w *World) InTownOrCastle() bool {
	l := w.Party.Location()
	return l == LocationTown || l == LocationCastle
}

// LoadMapState builds a fresh MapState from the pristine resources (the
// parts of LoadUltimaMap() for surface, town and castle maps). A MAPS
// resource is one byte of edge length (0 means 256), then the values;
// Sosaria carries four more bytes: whirlpool x, y, dx, dy.
func (w *World) LoadMapState(id int) *MapState {
	raw, ok := w.Resources.Maps[id]
	if !ok {
		panic("no map resource " + itoa(id))
	}
	size := int(raw[0])
	if size == 0 {
		size = 256
	}
	tiles := slices.Clone(raw[1 : 1+size*size])
	monsters := make([]byte, 256)
	copy(monsters, w.Resources.Monsters[id])
	talk := make([]byte, 256)
	if t, ok := w.Resources.Talk[id]; ok {
		talk = slices.Clone(t)
	}
	if id == MapIDSosaria && len(raw) >= 1+size*size+4 {
		t := 1 + size*size
		w.Whirlpool = Whirlpool{int(raw[t]), int(raw[t+1]), int(int8(raw[t+2])), int(int8(raw[t+3]))}
	}
	for _, s := range mapSlips {
		if s.id == id && int(tiles[s.y*size+s.x]) == s.wrong {
			tiles[s.y*size+s.x] = byte(s.right)
		}
	}
	m := &MapState{ID: id, Size: size, Tiles: tiles, Monsters: NewMonsterTable(monsters), Talk: talk}
	if id == MapIDSosaria {
		BlockExodusApproach(m, w.DiagonalMoves)
	}
	return m
}

// EnterMap enters a town or castle: Current becomes a fresh copy of it.
func (w *World) EnterMap(id int) {
	w.Current = w.LoadMapState(id)
	w.Trail = nil // everyone stands on the door square until they walk
}

// EnterDungeon enters a dungeon. A dungeon MAPS resource is eight 16x16
// levels; its TLKS resource holds the misty writing, exposed as Current.Talk.
func (w *World) EnterDungeon(id int) {
	raw := w.Resources.Maps[id]
	if len(raw) < 2048 {
		panic("no dungeon resource " + itoa(id))
	}
	copy(w.Dungeon.Tiles, raw[:2048])
	w.Dungeon.Level, w.Dungeon.Heading, w.Dungeon.Torch, w.Dungeon.Exit = 0, 1, 0, false
	talk := make([]byte, 256)
	if t, ok := w.Resources.Talk[id]; ok {
		talk = slices.Clone(t)
	}
	w.Current = &MapState{ID: id, Size: 16, Tiles: w.Dungeon.Tiles, Monsters: NewMonsterTable(make([]byte, 256)), Talk: talk}
}

// GetXYDng is the dungeon cell at (x, y) on the current level, wrapping at 16. (GetXYDng)
func (w *World) GetXYDng(x, y int) int {
	return int(w.Dungeon.Tiles[w.Dungeon.Level*256+((y+16)&0x0f)*16+((x+16)&0x0f)])
}

// PutXYDng writes a dungeon cell on the current level.
func (w *World) PutXYDng(value, x, y int) {
	w.Dungeon.Tiles[w.Dungeon.Level*256+(y&0x0f)*16+(x&0x0f)] = byte(value)
}

// ReturnToSurface goes back to the overworld.
func (w *World) ReturnToSurface() { w.Current = w.Surface }

// ResetSosaria restores Sosaria to its pristine state. (ResetSosaria)
func (w *World) ResetSosaria() {
	w.Surface = w.LoadMapState(MapIDSosaria)
	if w.Party.Location() == LocationSosaria {
		w.Current = w.Surface
	}
}

// MapIDForLocation is the resource ID of a location table entry, as Enter()
// has it: entries above 18 are Ambrosia's.
func MapIDForLocation(placeIndex int) int {
	if placeIndex > 18 {
		return BaseRes + placeIndex + 3
	}
	return BaseRes + placeIndex
}

// Members is the roster records of the party members, in marching order.
func (w *World) Members() []*PlayerRecord {
	var out []*PlayerRecord
	for m := range 4 {
		if s := w.Party.MemberSlot(m); s >= 0 {
			out = append(out, w.Roster.Get(s))
		}
	}
	return out
}

// Member is the record of member 0..3; slot 0 for an empty position, as in C.
func (w *World) Member(m int) *PlayerRecord { return w.Roster.Get(max(0, w.Party.MemberSlot(m))) }

// memberIndices is the positions 0..3 that hold someone.
func (w *World) memberIndices() []int {
	var out []int
	for m := range 4 {
		if w.Party.MemberSlot(m) >= 0 {
			out = append(out, m)
		}
	}
	return out
}

// PoolPurses empties the members' own purses (the Apple II fields) into the
// party's pool when a party is formed.
func (w *World) PoolPurses() {
	for _, m := range w.memberIndices() {
		p := w.Member(m)
		w.Party.SetGold(w.Party.Gold() + p.Gold())
		b := p.Bytes
		w.Party.SetFoodHundredths(w.Party.FoodHundredths() + int(b[32])*10000 + int(b[33])*100 + int(b[34]))
		p.SetGold(0)
		b[32], b[33], b[34] = 0, 0, 0
	}
}

// SplitPool shares the pool out evenly when a party is dispersed (at most 9999 each).
func (w *World) SplitPool() {
	members := w.memberIndices()
	if len(members) == 0 {
		return
	}
	goldEach := min(9999, w.Party.Gold()/len(members))
	foodEach := min(9999, w.Party.Food()/len(members))
	for _, m := range members {
		p := w.Member(m)
		p.SetGold(goldEach)
		p.Bytes[32], p.Bytes[33], p.Bytes[34] = byte(foodEach/100), byte(foodEach%100), 0
	}
	w.Party.SetGold(0)
	w.Party.SetFoodHundredths(0)
}

// PoolGear moves each member's bag into the party's, leaving them what they
// have readied and worn.
func (w *World) PoolGear() {
	for _, m := range w.memberIndices() {
		p := w.Member(m)
		for _, isWeapon := range []bool{true, false} {
			base, last := gearRange(isWeapon)
			for i := 1; i <= last; i++ {
				n := int(p.Bytes[base+i])
				if int(p.Bytes[base]) == i && n > 0 {
					n-- // the one in use stays with the member
				}
				w.Party.SetGear(isWeapon, i, w.Party.Gear(isWeapon, i)+n)
				p.Bytes[base+i] = 0
			}
		}
	}
}

func gearRange(isWeapon bool) (base, last int) {
	if isWeapon {
		return 48, 15
	}
	return 40, 7
}

// SplitGear is the reverse of PoolGear, for dispersing: each member keeps
// what is in hand, and the bag is dealt out one at a time, first to members
// whose class can use the item, else to anyone.
func (w *World) SplitGear() {
	members := w.memberIndices()
	if len(members) == 0 {
		return
	}
	for _, isWeapon := range []bool{true, false} {
		base, last := gearRange(isWeapon)
		for _, m := range members {
			p := w.Member(m)
			if p.Bytes[base] > 0 {
				p.Bytes[base+int(p.Bytes[base])] = 1
			}
		}
		for i := 1; i <= last; i++ {
			var able []int
			for _, m := range members {
				if w.CanUse(w.Member(m), isWeapon, i) {
					able = append(able, m)
				}
			}
			takers := members
			if len(able) > 0 {
				takers = able
			}
			next := 0
			for n := w.Party.Gear(isWeapon, i); n > 0; n-- {
				p := w.Member(takers[next%len(takers)])
				next++
				p.Bytes[base+i] = byte(min(99, int(p.Bytes[base+i])+1))
			}
		}
	}
	w.Party.ClearGear()
}

// PoolSupplies gathers the members' gems, keys, powders and torches into the party record.
func (w *World) PoolSupplies() {
	for _, m := range w.memberIndices() {
		p := w.Member(m)
		w.Party.SetGems(w.Party.Gems() + p.Gems())
		w.Party.SetKeys(w.Party.Keys() + p.Keys())
		w.Party.SetPowders(w.Party.Powders() + p.Powders())
		w.Party.SetTorches(w.Party.Torches() + p.Torches())
		p.Bytes[37], p.Bytes[38], p.Bytes[39] = 0, 0, 0
		p.SetTorches(0)
	}
}

// SplitSupplies deals the supplies out one at a time round the members.
func (w *World) SplitSupplies() {
	members := w.memberIndices()
	if len(members) == 0 {
		return
	}
	deal := func(total, offset int) {
		for i := range total {
			p := w.Member(members[i%len(members)])
			p.Bytes[offset] = byte(min(99, int(p.Bytes[offset])+1))
		}
	}
	deal(w.Party.Gems(), 37)
	deal(w.Party.Keys(), 38)
	deal(w.Party.Powders(), 39)
	deal(w.Party.Torches(), 15)
	w.Party.ClearSupplies()
}

// CanUse says whether a member's class may ready the weapon or wear the
// armour. Exotics suit everyone. (weaponUseTable)
func (w *World) CanUse(p *PlayerRecord, isWeapon bool, index int) bool {
	if index == 0 {
		return true
	}
	if _, last := gearRange(isWeapon); index == last {
		return true
	}
	misc := w.Resources.Misc
	classIndex := max(0, slices.Index(misc.Career, p.ClassLetter()))
	table := misc.ArmourUse
	if isWeapon {
		table = misc.WeaponUse
	}
	return 65+index < int(table[classIndex])
}

// Equip readies a weapon or wears armour from the bag: the old item goes
// back in, the new one comes out. It is false when the bag has none.
func (w *World) Equip(p *PlayerRecord, isWeapon bool, index int) bool {
	base, _ := gearRange(isWeapon)
	old := int(p.Bytes[base])
	if index == old {
		return true
	}
	if index > 0 && w.Party.Gear(isWeapon, index) < 1 {
		return false
	}
	if old > 0 {
		w.Party.SetGear(isWeapon, old, w.Party.Gear(isWeapon, old)+1)
	}
	if index > 0 {
		w.Party.SetGear(isWeapon, index, w.Party.Gear(isWeapon, index)-1)
	}
	p.Bytes[base] = byte(index)
	return true
}

// AddGear puts found or bought gear in the bag (capped at 99 of a kind).
func (w *World) AddGear(isWeapon bool, index, amount int) {
	w.Party.SetGear(isWeapon, index, w.Party.Gear(isWeapon, index)+amount)
}

// AddGold adds gold to the pool; false if any was lost to the cap.
func (w *World) AddGold(amount int) bool {
	before := w.Party.Gold()
	w.Party.SetGold(before + amount)
	return w.Party.Gold() == before+amount
}

// AddFood adds whole rations to the pool; false if any were lost to the cap.
func (w *World) AddFood(rations int) bool {
	before := w.Party.FoodHundredths()
	w.Party.SetFoodHundredths(before + rations*100)
	return w.Party.FoodHundredths() == before+rations*100
}

// EatFromPool is one member's meal, a tenth of a ration; true when the pool
// is empty and the member goes hungry.
func (w *World) EatFromPool() bool {
	if w.Party.FoodHundredths() < 10 {
		w.Party.SetFoodHundredths(0)
		return true
	}
	w.Party.SetFoodHundredths(w.Party.FoodHundredths() - 10)
	return false
}

// MemberAlive mirrors CheckAlive(member).
func (w *World) MemberAlive(m int) bool { return w.Party.MemberSlot(m) >= 0 && w.Member(m).Alive() }

// NewGame starts from the shipped default roster and party, as if "Form
// party" had chosen the first four characters.
func (w *World) NewGame() {
	copy(w.Party.Bytes, w.Resources.DefaultParty)
	copy(w.Roster.Bytes, w.Resources.DefaultRoster)
	for m := range w.Party.Size() {
		w.Member(m).SetInParty(true)
	}
	w.PoolPurses()
	w.PoolGear()
	w.PoolSupplies()
	w.Party.SetRulesPending(true) // "Choose Thine Adventure!" at the first Journey onward
	w.Journal = EmptyJournal()
	w.GatesKnown = nil
	w.TipsNudged = false
	w.Party.SetLocation(LocationSosaria)
	w.Party.SetShape(0x7e)
	w.X, w.Y = w.Party.SurfaceX(), w.Party.SurfaceY()
	w.ReturnX, w.ReturnY = w.X, w.Y
	w.ResetSosaria()
}

// SkipMonstersThisTurn mirrors the zp[0xCD] toggle in FinishAll(): on a
// horse or ship monsters act only every other turn. True on the skipped turns.
func (w *World) SkipMonstersThisTurn() bool {
	s := w.Party.Shape()
	if s != 0x14 && s != 0x16 {
		return false
	}
	w.mountToggle = 255 - w.mountToggle
	return w.mountToggle < 128
}

// InExodusCastle mirrors ExodusCastle(): inside Exodus' castle while Exodus
// lives. During combat the location before the fight counts.
func (w *World) InExodusCastle() bool {
	if w.Party.ExodusDestroyed() {
		return false
	}
	loc := w.Party.Location()
	if loc == LocationCombat && w.Combat != nil {
		loc = w.Combat.PreviousLocation
	}
	if loc != LocationCastle {
		return false
	}
	return w.Party.SurfaceX() == int(w.Resources.Misc.LocationX[1])
}

// TownLine is a townsperson's line as the talk table has it, line breaks as
// spaces: town is a map name ("Moon"), n the speaker number.
func (w *World) TownLine(town string, n int) string {
	ids := make([]int, 0, len(w.Resources.MapNames))
	for id := range w.Resources.MapNames {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		if w.Resources.MapNames[id] != town {
			continue
		}
		if talk, ok := w.Resources.Talk[id]; ok {
			return strings.TrimSpace(strings.ReplaceAll(Speech(talk, n), "\n", " "))
		}
		return ""
	}
	return ""
}

// TimeLimit is the wait for a key under the timer setting, from the Apple
// II's wait in ms; 0 for no limit.
func (w *World) TimeLimit(originalMs int) int {
	switch w.Timer {
	case TimerOff:
		return 0
	case TimerSlow:
		return originalMs*2 + 2000
	}
	return originalMs
}

// SetDiagonalMoves changes the setting and fixes the land around Exodus' castle to match.
func (w *World) SetDiagonalMoves(on bool) {
	w.DiagonalMoves = on
	BlockExodusApproach(w.Surface, on)
}

func itoa(n int) string { return strconv.Itoa(n) }
