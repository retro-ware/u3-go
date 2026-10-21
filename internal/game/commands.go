package game

// The player's keyboard commands. Each is a port of the matching routine in
// UltimaMain.c (North(), Board(), Enter() ...). They print their own
// feedback and change the world; the game loop runs the end-of-turn
// processing afterwards. Commands that deal with other creatures live in
// interact.go, those that deal with the party's own records in actions.go.

// 1-based indices into the Messages string table. Names match the text.
const (
	MsgDismount     = 17
	MsgPass         = 23
	MsgNorth        = 24
	MsgSouth        = 25
	MsgEast         = 26
	MsgWest         = 27
	MsgBoard        = 29
	MsgMountHorse   = 30
	MsgBoardFrigate = 31
	MsgDescend      = 32
	MsgEnter        = 33
	MsgEnterDungeon = 34
	MsgEnterTown    = 35
	MsgEnterCastle  = 36
	MsgKlimb        = 68
	MsgLook         = 69
	MsgExit         = 102
	MsgExitCraft    = 103
	MsgWhat         = 106
	MsgArrowWhat    = 107
	MsgNotHere      = 108
	MsgInvalidMove  = 116
	MsgCancelled    = 120
	MsgSouthwest    = 250
	MsgSoutheast    = 251
	MsgNorthwest    = 252
	MsgNortheast    = 253
)

// Party shapes (Party[1]).
const (
	PartyShapeOnFoot    = 0x7e
	PartyShapeHorse     = 0x14
	PartyShapeFrigate   = 0x16
	PartyShapeWhirlpool = 0x18
)

// What prints "WHAT?": the command made no sense. (What)
func What(io IO) {
	io.PrintMessage(MsgWhat)
	io.Sound(SoundError2)
}

// What2 prints "<-WHAT?": the command made no sense here. (What2)
func What2(io IO) {
	io.PrintMessage(MsgArrowWhat)
	io.Sound(SoundError1)
}

// Cancelled: the player backed out of a command at a prompt before it did
// anything. "Cancelled.", and the turn is not spent (World.CommandCancelled).
// The original spent the turn.
func Cancelled(w *World, io IO) {
	io.PrintMessage(MsgCancelled)
	w.CommandCancelled = true
}

// NotHere prints "Not here!". (NotHere)
func NotHere(io IO) {
	io.PrintMessage(MsgNotHere)
	io.Sound(SoundError1)
}

// NoGo prints "INVALID MOVE!" with a bump. (NoGo)
func NoGo(io IO) {
	io.PrintMessage(MsgInvalidMove)
	io.Sound(SoundBump)
	io.FlushKeys()
}

// ValidTrans mirrors ValidTrans(): a frigate cannot sail straight into the
// wind (until Exodus is destroyed). dir is 1 north, 2 east, 3 south, 4 west.
func ValidTrans(w *World, dir int) bool {
	if w.Party.ExodusDestroyed() {
		return true
	}
	if w.Party.Shape() == PartyShapeFrigate && w.WindEnabled {
		return w.WindDirection == 0 || dir != w.WindDirection
	}
	return true
}

// ValidDir mirrors ValidDir(): may the party step onto a square with this
// map value? Trying has side effects: force fields and lava hurt anyone
// without the protecting Mark, and footsteps make a sound.
func ValidDir(w *World, io IO, value int) bool {
	if w.Party.Shape() == PartyShapeFrigate {
		return value == MapValueWater || value == MapValueWhirlpool
	}
	goodPlace := false
	if value == MapValueForceField {
		io.FlashTiles()
		io.Sound(SoundForceField)
		for m := range 4 {
			if w.Party.MemberSlot(m) < 0 {
				continue
			}
			p := w.Member(m)
			if p.Marks()&0x10 == 0 {
				// No Mark of Force: the first unprotected member takes 99 damage and the party is stopped.
				if p.SubtractHitPoints(99) {
					io.Sound(DeathSound(p.Sex()))
				}
				io.FlashMember(m)
				io.Sound(SoundHit)
				io.FlashTiles()
				goodPlace = false
				break
			}
			goodPlace = true
		}
	}
	if value == MapValueLava {
		goodPlace = true
		io.Sound(SoundAttack)
		for m := range 4 {
			if w.Party.MemberSlot(m) < 0 {
				continue
			}
			p := w.Member(m)
			if p.Marks()&0x20 == 0 && p.Alive() {
				if p.SubtractHitPoints(50) {
					io.Sound(DeathSound(p.Sex()))
				}
				io.FlashMember(m)
				io.Sound(SoundHit)
			}
		}
	}
	if value == MapValueMoonGate || value == MapValueShrine {
		value = MapValueGrass
	}
	if value < MapValueWhirlpool && value != MapValueWater && value != MapValueMountains {
		goodPlace = true
		if w.Party.Shape() == PartyShapeHorse {
			io.Sound(SoundHorseWalk)
		} else {
			io.Sound(SoundStep)
		}
	}
	return goodPlace
}

// MoveName names a step: "north", "southwest" ...
type MoveName string

const (
	MoveNorth     MoveName = "north"
	MoveSouth     MoveName = "south"
	MoveEast      MoveName = "east"
	MoveWest      MoveName = "west"
	MoveNortheast MoveName = "northeast"
	MoveSoutheast MoveName = "southeast"
	MoveSouthwest MoveName = "southwest"
	MoveNorthwest MoveName = "northwest"
)

type moveSpec struct {
	dx, dy, message int
	compass         []int // compass directions that must not be against the wind
}

var moves = map[MoveName]moveSpec{
	MoveNorth:     {0, -1, MsgNorth, []int{1}},
	MoveSouth:     {0, 1, MsgSouth, []int{3}},
	MoveEast:      {1, 0, MsgEast, []int{2}},
	MoveWest:      {-1, 0, MsgWest, []int{4}},
	MoveNortheast: {1, -1, MsgNortheast, []int{1, 2}},
	MoveSoutheast: {1, 1, MsgSoutheast, []int{2, 3}},
	MoveSouthwest: {-1, 1, MsgSouthwest, []int{3, 4}},
	MoveNorthwest: {-1, -1, MsgNorthwest, []int{1, 4}},
}

// MoveDelta is the step and message of a movement (combat moves on the arena with it).
func MoveDelta(name MoveName) (dx, dy, message int) {
	s := moves[name]
	return s.dx, s.dy, s.message
}

// BumpKind is what a move ran into.
type BumpKind string

const (
	BumpPerson  BumpKind = "person"  // a townsperson (monster slot Index): talk to them
	BumpMonster BumpKind = "monster" // a monster or pirate ship on the surface: attack it
	BumpCounter BumpKind = "counter" // a shop counter with the merchant behind it: transact
	BumpDoor    BumpKind = "door"    // a locked door beside the party: unlock it
)

// Bump is what a move ran into instead of moving; the game turns each into
// the command a player would have typed (a convenience of the TypeScript port).
type Bump struct {
	Kind   BumpKind
	Index  int // BumpPerson
	DX, DY int // the others
}

// CounterWithMerchant says whether (xs, ys) is a counter (a wall or letter
// tile) with a merchant behind it. (Transact)
func CounterWithMerchant(w *World, xs, ys, dx, dy int) bool {
	tile := w.GetXYVal(xs, ys)
	if tile < MapValueWall2 || tile >= MapValueSnakeBottom {
		return false
	}
	return w.GetXYVal(xs+dx, ys+dy) == MapValueMerchant
}

// Move mirrors North(), South() ... NorthWest(). It returns what stood in
// the way, or nil when the party moved or was simply blocked.
func Move(w *World, io IO, name MoveName) *Bump {
	s := moves[name]
	if s.dx != 0 {
		w.HorseFacingEast = s.dx > 0
	}
	io.PrintMessage(s.message)
	for _, c := range s.compass {
		if !ValidTrans(w, c) {
			NoGo(io)
			return nil
		}
	}
	xs, ys := w.Constrain(w.X+s.dx), w.Constrain(w.Y+s.dy)
	// Walking into a creature does what a player would have typed next. The
	// table also holds things that are not creatures (a horse, a chest, the
	// Floor entries before Exodus); those are walked onto like any square.
	if who := w.Monsters().At(xs, ys); who >= 0 && w.Monsters().Type(who) >= MapValueSerpent {
		if w.InTownOrCastle() {
			return &Bump{Kind: BumpPerson, Index: who}
		}
		return &Bump{Kind: BumpMonster, DX: s.dx, DY: s.dy}
	}
	if w.InTownOrCastle() && CounterWithMerchant(w, xs, ys, s.dx, s.dy) {
		return &Bump{Kind: BumpCounter, DX: s.dx, DY: s.dy}
	}
	// A locked door, in a town or castle or before two of Ambrosia's
	// shrines. Doors only open sideways, as Unlock requires.
	if w.GetXYVal(xs, ys) == MapValueLetterI && s.dx != 0 && s.dy == 0 {
		return &Bump{Kind: BumpDoor, DX: s.dx, DY: s.dy}
	}
	if !ValidDir(w, io, w.GetXYVal(w.X+s.dx, w.Y+s.dy)) {
		NoGo(io)
		return nil
	}
	// The squares the leader came through, for the follower line.
	w.Trail = append([]Point{{w.X, w.Y}}, w.Trail...)
	if len(w.Trail) > 3 {
		w.Trail = w.Trail[:3]
	}
	w.X, w.Y = xs, ys
	return nil
}

// DiagonalKeys are the keypad digits for diagonal steps.
var DiagonalKeys = []string{"1", "3", "7", "9"}

// MoveForKey maps a key to a movement, or "". Arrow keys and the keypad both work.
func MoveForKey(key string, allowDiagonal bool) MoveName {
	switch key {
	case KeyUp, "8":
		return MoveNorth
	case KeyDown, "2":
		return MoveSouth
	case KeyRight, "6":
		return MoveEast
	case KeyLeft, "4":
		return MoveWest
	}
	if !allowDiagonal {
		return ""
	}
	switch key {
	case "7":
		return MoveNorthwest
	case "9":
		return MoveNortheast
	case "1":
		return MoveSouthwest
	case "3":
		return MoveSoutheast
	}
	return ""
}

// Direction is an adjacent square chosen at a "Direction-" prompt.
type Direction struct{ XS, YS, DX, DY int }

// GetDirection mirrors GetDirection(): ask for a direction and echo its
// name. Nil when the player cancels. allowSpace lets the space bar mean "no
// direction" (combat); allowDiagonal is usually w.DiagonalMoves.
func GetDirection(w *World, io IO, allowSpace, allowDiagonal bool) *Direction {
	key := io.ChooseDirection(allowSpace, allowDiagonal)
	if key == "" || w.Done {
		return nil
	}
	if key == KeySpace {
		io.PrintMessage(173) // "None"
		return &Direction{w.X, w.Y, 0, 0}
	}
	name := MoveForKey(key, allowDiagonal)
	if name == "" {
		return nil
	}
	s := moves[name]
	io.PrintMessage(s.message)
	return &Direction{w.X + s.dx, w.Y + s.dy, s.dx, s.dy}
}

// Pass is the space bar. (Pass)
func Pass(io IO) { io.PrintMessage(MsgPass) }

// Board is B: mount a horse or board a ship the party is standing on. (Board)
func Board(w *World, io IO) {
	if w.Party.Shape() != PartyShapeOnFoot {
		io.PrintMessage(MsgBoard)
		What2(io)
		return
	}
	switch w.GetXYVal(w.X, w.Y) {
	case MapValueHorse:
		w.PutXYVal(MapValueGrass, w.X, w.Y)
		w.Party.SetShape(PartyShapeHorse)
		io.PrintMessage(MsgMountHorse)
		io.Sound(SoundMountHorse)
	case MapValueFrigate:
		w.PutXYVal(MapValueWater, w.X, w.Y)
		w.Party.SetShape(PartyShapeFrigate)
		io.PrintMessage(MsgBoardFrigate)
		w.Journal.Ship = true
		JournalCheck(w, io)
	default:
		io.PrintMessage(MsgBoard)
		What2(io)
	}
}

// Exit is X: leave the horse or ship, which stays behind on the map. (Exit)
func Exit(w *World, io IO) {
	if w.Party.Shape() == PartyShapeOnFoot {
		io.PrintMessage(MsgExit)
		io.PrintMessage(MsgWhat)
		io.Sound(SoundError1)
		return
	}
	if w.GetXYVal(w.X, w.Y) > MapValueGrass {
		// Only water (for a ship) or grass (for a horse) can hold the mount.
		io.PrintMessage(MsgExit)
		io.Print("\n")
		NotHere(io)
		return
	}
	w.PutXYVal(w.Party.Shape()*2, w.X, w.Y)
	if w.Party.Shape() == PartyShapeHorse {
		io.PrintMessage(MsgDismount)
	} else {
		io.PrintMessage(MsgExitCraft)
	}
	w.Party.SetShape(PartyShapeOnFoot)
}

// stringAt is entry i of a string table, or "?" past its end.
func stringAt(table []string, i int) string {
	if i >= 0 && i < len(table) {
		return table[i]
	}
	return "?"
}

// Look is L: name what is on an adjacent square. (Look)
func Look(w *World, io IO) {
	io.PrintMessage(MsgLook)
	dir := GetDirection(w, io, false, w.DiagonalMoves)
	if dir == nil {
		Cancelled(w, io)
		return
	}
	io.Print("->")
	value := w.GetXYVal(dir.XS, dir.YS)
	mon := w.Monsters().At(w.Constrain(dir.XS), w.Constrain(dir.YS))
	tiles := w.Resources.Strings["Tiles"]
	if mon < 0 {
		io.Print(stringAt(tiles, value>>2))
	} else {
		// Monsters are named in the plural on the surface, singular in towns and castles.
		names := tiles
		if !w.InTownOrCastle() {
			names = w.Resources.Strings["TilesPlural"]
		}
		typ := w.Monsters().Type(mon) >> 1
		variant := w.Monsters().Variant(mon)
		if typ > 44 && variant > 0 {
			io.Print(stringAt(names, typ-46+63+variant))
		} else {
			io.Print(stringAt(names, typ>>1))
		}
	}
	io.Print("\n")
}

// Entered is what Enter went into.
type Entered string

const (
	EnteredNone    Entered = "none"
	EnteredTown    Entered = "town"
	EnteredCastle  Entered = "castle"
	EnteredDungeon Entered = "dungeon"
	EnteredShrine  Entered = "shrine"
)

// Enter is E: enter the town, castle or dungeon the party stands on.
// (Enter) Dungeons and Ambrosia's shrines are started by the caller, which
// owns those loops.
func Enter(w *World, io IO) Entered {
	loc := w.Party.Location()
	if loc != LocationSosaria && loc != LocationAmbrosia {
		io.PrintMessage(MsgEnter)
		What(io)
		return EnteredNone
	}
	if loc == LocationAmbrosia {
		if w.GetXYVal(w.X, w.Y) != MapValueShrine {
			io.PrintMessage(MsgEnter)
			What2(io)
			return EnteredNone
		}
		return EnteredShrine
	}
	misc := w.Resources.Misc
	place := -1
	for i := range 32 {
		if int(misc.LocationX[i]) == w.X && int(misc.LocationY[i]) == w.Y {
			place = i
		}
	}
	if place < 0 {
		io.PrintMessage(MsgEnter)
		What2(io)
		return EnteredNone
	}
	w.Party.SetSurfaceX(w.X)
	w.Party.SetSurfaceY(w.Y)
	w.ReturnX, w.ReturnY = w.X, w.Y

	id := MapIDForLocation(place)
	switch w.GetXYVal(w.X, w.Y) >> 1 {
	case ShapeDungeon:
		io.PrintMessage(MsgEnter)
		io.PrintMessage(MsgEnterDungeon)
		w.EnterDungeon(id)
		w.Party.SetLocation(LocationDungeon)
		w.X, w.Y = 1, 1
		w.Dungeon.Heading = 1
		return EnteredDungeon
	case ShapeTown:
		io.PrintMessage(MsgEnter)
		io.PrintMessage(MsgEnterTown)
		w.EnterMap(id)
		w.Party.SetLocation(LocationTown)
		w.X, w.Y = 1, 32
		return EnteredTown
	case ShapeCastle:
		io.PrintMessage(MsgEnter)
		io.PrintMessage(MsgEnterCastle)
		w.EnterMap(id)
		w.Party.SetLocation(LocationCastle)
		w.X, w.Y = 32, 62
		return EnteredCastle
	}
	What2(io)
	return EnteredNone
}
