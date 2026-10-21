package game

import (
	"math"
	"slices"
	"strconv"
	"strings"
)

// The main game loop: Game() in UltimaMain.c. Each iteration redraws the
// map, waits for a key (or times out and passes the turn), dispatches the
// command, then runs the end-of-turn processing. The loop blocks in IO
// while the UI keeps animating.

// IdlePassMs is how long the game waits for a key before passing the turn (the original: ~5 seconds).
const IdlePassMs = 5000

// whirlpoolTickMs: the original polled the whirlpool roughly every twelfth of a second.
const whirlpoolTickMs = 83

// Messages table entries the loop prints.
const (
	msgDescend     = 32
	msgKlimb       = 68
	msgQuitAndSave = 76
	msgOnlySurface = 77
	msgMoves       = 78
)

// GameOptions are the game's hooks; either may be nil.
type GameOptions struct {
	// Save is called on Q on the surface, and by the autosave at every
	// town, castle and dungeon door; true if the game was saved. (QuitSave)
	Save func(w *World) bool
	// Load restores the last save into the world; true if there was one. Offered after a party wipe.
	Load func(w *World) bool
}

// Game runs the field loop on a World.
type Game struct {
	World   *World
	IO      IO
	options GameOptions
	hooks   TurnHooks
}

// NewGame wires a game loop to a world and its UI.
func NewGame(w *World, io IO, options GameOptions) *Game {
	g := &Game{World: w, IO: io, options: options}
	g.hooks = TurnHooks{
		Attack:   func(i int) { AttackMonster(w, io, i) },
		BombTrap: func() { BombTrap(w, io) },
		ShowBall: func(vx, vy int) {
			ShowBall(w, io, w.Constrain(w.X-5+vx), w.Constrain(w.Y-5+vy), ShapeFireBall)
		},
		GoWhirlpool: g.goWhirlpool,
		DungeonTurn: func() {},
		Save:        g.autosave,
	}
	return g
}

// locationMusic is the music for the party's current location.
func (g *Game) locationMusic() int {
	switch g.World.Party.Location() {
	case LocationTown:
		return MusicTown
	case LocationCastle:
		if g.World.InExodusCastle() {
			return MusicExodusCastle
		}
		return MusicCastle
	case LocationAmbrosia:
		return MusicAmbrosia
	}
	return MusicSosaria
}

func (g *Game) setMusic(track int) {
	g.World.Music = track
	g.IO.Music(track)
}

// Run runs until World.Done is set (Quit): the while (!gDone) loop in Game().
func (g *Game) Run() {
	w, io := g.World, g.IO
	w.Done = false
	w.TimeNegate = 0
	io.ShowGameFrame()
	io.UpdateStats(true)
	io.ShowWind()
	io.ShowMoons()
	g.setMusic(g.locationMusic())
	w.LoadLastSave = nil
	if g.options.Load != nil {
		w.LoadLastSave = func() bool { return g.options.Load(w) }
	}
	g.autosave() // so "last save" always exists once the journey starts

	for !w.Done {
		io.RedrawMap()
		io.UpdateStats(false)
		w.Resurrecting = false
		CheckAllDead(w, io)
		if w.Done {
			return
		}
		// A new game's first turn points to the Tips, once.
		if !w.TipsNudged {
			io.Print(TipsNudge)
			w.TipsNudged = true
		}
		io.Prompt()

		key := g.waitForCommand()
		if w.Done {
			return
		}
		switch key {
		case KeyEscape, KeyPause:
			// The game stops where it stands; no turn passes.
			if io.ShowPause() == "Q" {
				g.quit()
			}
			continue
		case "J", "j":
			io.ShowJournal() // no turn passes
			continue
		case ViewMapKey:
			io.ShowMap() // no turn passes
			continue
		}
		w.CommandCancelled = false
		g.Dispatch(key)
		if w.Done {
			return
		}
		if w.TakeCancelled() {
			continue // backed out at a prompt: no turn passes
		}
		EndTurn(w, io, g.hooks)
		if wanted := g.locationMusic(); w.Music != wanted && w.Party.Location() != LocationCombat {
			g.setMusic(wanted)
		}
	}
}

// waitForCommand waits for a key while the whirlpool keeps moving. After
// the idle time with no input the turn passes by itself; with the timer
// off it never does. Only ticks that ran out with no input count.
func (g *Game) waitForCommand() string {
	remaining := math.Inf(1)
	if limit := g.World.TimeLimit(IdlePassMs); limit > 0 {
		remaining = float64(limit)
	}
	for {
		WhirlpoolTick(g.World, g.IO, g.hooks)
		if remaining <= 0 {
			return KeySpace
		}
		tick := math.Min(whirlpoolTickMs, remaining)
		if key := g.IO.WaitCommand(ScopeField, int(tick)); key != "" {
			return key
		}
		remaining -= tick
	}
}

// autosave saves silently at the doors of towns, castles and dungeons (and
// when the journey starts): the surface, at the door's square.
func (g *Game) autosave() bool {
	if g.World.Resurrecting || g.options.Save == nil {
		return false
	}
	return g.options.Save(g.World)
}

// announceSave saves at a door and says so.
func (g *Game) announceSave() {
	if g.autosave() {
		g.IO.Print("(saved)\n")
	}
}

// Dispatch runs one command key: the key switch in Game() and LetterCommand().
func (g *Game) Dispatch(key string) {
	w, io := g.World, g.IO
	if !w.DiagonalMoves && slices.Contains(DiagonalKeys, key) {
		return // no diagonals on the Apple II
	}
	if moveName := MoveForKey(key, w.DiagonalMoves); moveName != "" {
		bump := Move(w, io, moveName)
		if bump == nil {
			return
		}
		// Walking into things does what a player would have typed next.
		switch bump.Kind {
		case "person":
			// Talk with the first living member; Lord British sees whoever is due a level.
			speaker := 0
			for m := range 4 {
				if w.MemberAlive(m) {
					speaker = m
					break
				}
			}
			if w.Monsters().Type(bump.Index) == MapValueLordBritish {
				if speaker = WhoSeesLordBritish(w, io); speaker < 0 {
					return
				}
			}
			TalkTo(w, io, bump.Index, speaker)
		case "monster":
			io.Print("Attack\n")
			AttackToward(w, io, bump.DX, bump.DY)
		case "counter":
			TransactToward(w, io, bump.DX, bump.DY)
		case "door":
			io.Print("Unlock\n")
			UnlockToward(w, io, bump.DX, bump.DY)
		}
		return
	}
	switch strings.ToUpper(key) {
	case " ":
		Pass(io)
	case "A":
		Attack(w, io)
	case "B":
		Board(w, io)
	case "C":
		Cast(w, io)
	case QuickCastKey:
		if !QuickHeal(w, io) {
			What2(io)
		}
	case SafeChestKey:
		if !QuickSafeChest(w, io) {
			What2(io)
		}
	case LightKey:
		if !QuickLight(w, io) {
			What2(io)
		}
	case "D":
		io.PrintMessage(msgDescend)
		What2(io)
	case "E":
		g.enter()
	case "F":
		Fire(w, io)
	case "G":
		GetChest(w, io, 0, "command")
	case "I":
		IgniteTorch(w, io)
	case "K":
		io.PrintMessage(msgKlimb)
		What2(io)
	case "L":
		Look(w, io)
	case "M":
		ModifyOrder(w, io)
	case "N":
		NegateTime(w, io)
	case "O":
		OtherCommand(w, io)
	case "P":
		PeerGem(w, io)
	case "Q":
		g.quit()
	case "R":
		ReadyWeapon(w, io)
	case "S":
		Steal(w, io)
	case "T":
		Transact(w, io)
	case "U":
		Unlock(w, io)
	case "V":
		Volume(w, io)
	case "W":
		WearArmour(w, io)
	case "X":
		Exit(w, io)
	case "Y":
		Yell(w, io)
	case "Z":
		Stats(w, io)
	}
	// Unknown keys are ignored but still cost a turn, as in the original.
}

// enter is E: enter a place. Dungeons and shrines have their own loops.
func (g *Game) enter() {
	w, io := g.World, g.IO
	switch Enter(w, io) {
	case "shrine":
		EnterShrine(w, io)
	case "dungeon":
		g.announceSave()
		RunDungeon(w, io)
		if w.Resurrecting || w.Done {
			return
		}
		ExitToSurface(w, io, &g.hooks)
		io.ShowGameFrame()
		io.UpdateStats(true)
		io.ShowMoons()
	case "castle":
		// Once Exodus is destroyed his castle stands empty and harmless.
		if w.Current.ID == MapIDExodusCastle && w.Party.ExodusDestroyed() {
			SafeExodus(w)
		}
		g.setMusic(g.locationMusic())
		g.announceSave()
	case "town":
		g.setMusic(g.locationMusic())
		g.announceSave()
	}
}

// quit is Q: save (only on the surface) and return to the main menu. (QuitSave)
func (g *Game) quit() {
	w, io := g.World, g.IO
	io.PrintMessage(msgQuitAndSave)
	if !w.OnSurface() {
		io.PrintMessage(msgOnlySurface)
		io.Sound(SoundError1)
		return
	}
	w.Party.SetSurfaceX(w.X)
	w.Party.SetSurfaceY(w.Y)
	saved := g.options.Save != nil && g.options.Save(w)
	io.Print(strconv.Itoa(w.Party.Moves()))
	io.PrintMessage(msgMoves)
	if !saved {
		io.Print("(not saved)\n")
	}
	w.Done = true
}

// goWhirlpool: the whirlpool caught the party. On Sosaria it carries them to
// Ambrosia; on Ambrosia it brings them home. (GoWhirlPool)
func (g *Game) goWhirlpool() {
	w, io := g.World, g.IO
	w.Party.SetShape(PartyShapeWhirlpool)
	io.RedrawMap()
	io.PrintMessage(111)
	g.setMusic(MusicNone)
	io.Sound(SoundSink)
	io.Pause(3000)

	switch {
	case w.OnSurface():
		w.Party.SetSurfaceX(w.X)
		w.Party.SetSurfaceY(w.Y)
		w.PutXYVal(MapValueWater, w.X, w.Y)
		w.Whirlpool.X, w.Whirlpool.Y = 2, 0x3e
		w.Party.SetShape(PartyShapeFrigate)
		io.ClearTiles()
		io.PrintMessage(112)
		w.EnterMap(MapIDAmbrosia)
		io.Pause(3000)
		w.Party.SetShape(PartyShapeOnFoot)
		w.X, w.Y = 32, 54
		w.Party.SetLocation(LocationAmbrosia)
		io.PrintMessage(113)
		g.setMusic(MusicAmbrosia)
		w.Journal.Ambrosia = true
		JournalCheck(w, io)
	case w.Party.Location() == LocationAmbrosia:
		w.ReturnToSurface()
		io.PrintMessage(114)
		io.ClearTiles()
		io.PrintMessage(115)
		w.X, w.Y = w.Party.SurfaceX(), w.Party.SurfaceY()
		w.Party.SetLocation(LocationSosaria)
		w.Party.SetShape(PartyShapeFrigate)
		g.setMusic(MusicSosaria)
	default:
		// In a town or castle: dropped on a random water square.
		for value := 0xff; value != MapValueWater; {
			w.X = w.Rng.Range(0, w.MapSize()-1)
			w.Y = w.Rng.Range(0, w.MapSize()-1)
			value = w.GetXYVal(w.X, w.Y)
		}
		g.setMusic(g.locationMusic())
	}
	io.RedrawMap()
	io.Prompt()
}
