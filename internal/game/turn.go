package game

import "slices"

// Everything that happens after the player's command each turn:
// Routine6E35() (the end-of-turn routine at Apple II $6E35), AgeChars(),
// MoonGateUpdate(), HandleMoonStep(), FinishAll(), DoWind() and WhirlPool().

const (
	turnMsgExitToSosaria  = 182
	turnMsgPoison         = 183
	turnMsgStarving       = 184
	turnMsgWhirlpoolSinks = 110
)

// PlaceMoongates mirrors the map half of DrawMoonGateStuff(): Dawn appears
// when both moons are new, the eight gate squares turn to grass, then the
// gate Trammel's phase selects opens.
func PlaceMoongates(w *World) {
	if !w.OnSurface() {
		return
	}
	misc := w.Resources.Misc
	dawnX, dawnY := int(misc.LocationX[8]), int(misc.LocationY[8])
	if w.MoonPhase[0] == 0 && w.MoonPhase[1] == 0 {
		w.PutXYVal(MapValueTown, dawnX, dawnY)
	} else if w.GetXYVal(dawnX, dawnY) == MapValueTown {
		w.PutXYVal(MapValueForest, dawnX, dawnY)
	}
	for i := range 8 {
		w.PutXYVal(MapValueGrass, int(misc.MoonX[i]), int(misc.MoonY[i]))
	}
	w.PutXYVal(MapValueMoonGate, int(misc.MoonX[w.MoonPhase[0]]), int(misc.MoonY[w.MoonPhase[0]]))
}

// MoonGateUpdate mirrors MoonGateUpdate(): advance the moon timers once per surface turn.
func MoonGateUpdate(w *World, io IO) {
	if !w.OnSurface() {
		return
	}
	periods := [2]int{12, 4}
	for i := range 2 {
		w.MoonTimer[i]--
		if w.MoonTimer[i] < 1 {
			w.MoonTimer[i] = periods[i]
			w.MoonPhase[i] = (w.MoonPhase[i] + 1) % 8
		}
	}
	io.ShowMoons()
	PlaceMoongates(w)
}

// HandleMoonStep mirrors HandleMoonStep(): the party stepped onto a
// moongate. On the surface Felucca's phase picks the destination; elsewhere
// (Ambrosia) the party lands on a random grass square.
func HandleMoonStep(w *World, io IO) {
	misc := w.Resources.Misc
	if w.OnSurface() {
		phase := w.MoonPhase[1]
		w.X, w.Y = int(misc.MoonX[phase]), int(misc.MoonY[phase])
		if !slices.Contains(w.GatesKnown, phase) {
			w.GatesKnown = append(w.GatesKnown, phase) // View map marks the gates come out of
		}
	} else {
		for value := -1; value != MapValueGrass; value = w.GetXYVal(w.X, w.Y) {
			w.X = w.Rng.Range(0, w.MapSize()-1)
			w.Y = w.Rng.Range(0, w.MapSize()-1)
		}
	}
	io.FlashTiles()
	io.Sound(SoundMoongate)
	io.RedrawMap()
	io.FlashTiles()
	io.Sound(SoundMoongate)
}

// DoWind mirrors DoWind(): every 32 turns a new wind direction.
func DoWind(w *World, io IO) {
	if w.Party.Location() == LocationDungeon {
		return
	}
	w.WindTimer--
	old := w.WindDirection
	if w.WindTimer < 0 {
		w.WindTimer = 32
		next := w.WindDirection
		for next == w.WindDirection {
			next = w.Rng.Range(0, 8)
			if next > 4 {
				next -= 4
			}
		}
		w.WindDirection = next
	}
	if old != w.WindDirection {
		io.ShowWind()
	}
}

// AgeChars mirrors AgeChars(): every turn on the surface, every fourth
// elsewhere. Each member regains mana by class, eats, takes poison damage,
// and every tenth call heals a point.
func AgeChars(w *World, io IO) {
	if !w.OnSurface() {
		w.AgeTimer[0]--
		if w.AgeTimer[0] > 0 {
			return
		}
		w.AgeTimer[0] = 4
	}
	w.AgeTimer[1]--
	if w.AgeTimer[1] < 0 {
		w.AgeTimer[1] = 9
	}
	careers := w.Resources.Misc.Career
	cleric, wizard, paladin, lark := careers[1], careers[2], careers[4], careers[6]
	illusionist, druid, alchemist, ranger := careers[7], careers[8], careers[9], careers[10]

	starving := false
	for m := 3; m >= 0; m-- {
		if w.Party.MemberSlot(m) < 0 {
			continue
		}
		p := w.Member(m)
		c := p.ClassLetter()
		// Mana regenerates toward a class-dependent cap.
		if c == wizard && p.Mana() < p.Intelligence() {
			p.SetMana(p.Mana() + 1)
		}
		if c == cleric && p.Mana() < p.Wisdom() {
			p.SetMana(p.Mana() + 1)
		}
		if (c == lark || c == druid || c == alchemist) && p.Mana() < p.Intelligence()>>1 {
			p.SetMana(p.Mana() + 1)
		}
		if (c == paladin || c == illusionist || c == druid) && p.Mana() < p.Wisdom()>>1 {
			p.SetMana(p.Mana() + 1)
		}
		if c == ranger && p.Mana() < p.Intelligence()>>1 && p.Mana() < p.Wisdom()>>1 {
			p.SetMana(p.Mana() + 1)
		}
		if !p.Alive() {
			continue
		}
		if w.EatFromPool() {
			// Food is the party's: one warning a tick, damage by the Starvation setting.
			starving = true
			floor := p.HitPoints()
			switch w.Starvation {
			case StarvationClassic:
				floor = 0
			case StarvationMild:
				floor = (p.MaxHitPoints() + 1) / 2
			}
			if loss := min(5, max(0, p.HitPoints()-floor)); loss > 0 && p.SubtractHitPoints(loss) {
				io.Sound(DeathSound(p.Sex()))
			}
		}
		if p.Status() == 'P' && (w.PoisonKills || p.HitPoints() > 1) {
			if p.SubtractHitPoints(1) {
				io.Sound(DeathSound(p.Sex()))
			}
			io.FlashMember(m)
			io.PrintMessage(turnMsgPoison)
		}
		if w.AgeTimer[1] == 0 {
			p.AddHitPoints(1)
		}
	}
	if starving {
		io.PrintMessage(turnMsgStarving)
		io.Sound(SoundHit)
	}
	io.UpdateStats(false)
}

// WhirlpoolTick mirrors WhirlPool(): every idle tick, acting every fourth.
// The whirlpool drifts across the oceans, sometimes changing course, sinks
// any frigate it touches, and drags the party under if it reaches them.
func WhirlpoolTick(w *World, io IO, hooks *TurnHooks) {
	w.WhirlpoolTimer--
	if w.WhirlpoolTimer > 0 {
		return
	}
	w.WhirlpoolTimer = 4
	if !w.OnSurface() {
		return
	}
	wp := &w.Whirlpool
	newCourse := func() {
		dir := w.Rng.Range(0, 7)
		wp.DX = [8]int{0, 1, 1, 1, 0, -1, -1, -1}[dir]
		wp.DY = [8]int{1, 1, 0, -1, -1, -1, 0, 1}[dir]
	}
	if w.Rng.Range(0, 7) == 0 {
		newCourse()
	} else {
		nx, ny := w.Constrain(wp.X+wp.DX), w.Constrain(wp.Y+wp.DY)
		switch w.GetXYVal(nx, ny) {
		case MapValueWater:
			w.PutXYVal(MapValueWhirlpool, nx, ny)
			w.PutXYVal(MapValueWater, wp.X, wp.Y)
			wp.X, wp.Y = nx, ny
			io.RedrawMap()
		case MapValueFrigate:
			// Sink the ship: the whirlpool takes its square, water the old one.
			w.PutXYVal(MapValueWhirlpool, nx, ny)
			w.PutXYVal(MapValueWater, wp.X, wp.Y)
			wp.X, wp.Y = nx, ny
			io.RedrawMap()
			// A party standing on its ship goes down with it, as a party aboard does.
			if wp.X == w.X && wp.Y == w.Y {
				hooks.GoWhirlpool()
				return
			}
			io.Sound(SoundSink)
			io.PrintMessage(turnMsgWhirlpoolSinks)
			io.Print("\n")
			io.Prompt()
			return
		default:
			newCourse()
		}
	}
	if wp.X == w.X && wp.Y == w.Y {
		hooks.GoWhirlpool()
	}
}

// TurnHooks are the callbacks into parts of the game outside the turn logic.
type TurnHooks struct {
	MonsterHooks
	GoWhirlpool func()      // the party has been swallowed by the whirlpool (GoWhirlPool)
	DungeonTurn func()      // dungeon turn processing
	Save        func() bool // save at a door (autosave); true if saved; may be nil
}

// ExitToSurface mirrors Routine6E6B(): the party walked off the edge of a
// town or castle and returns to Sosaria where it entered. hooks may be nil.
func ExitToSurface(w *World, io IO, hooks *TurnHooks) {
	w.X, w.Y = w.ReturnX, w.ReturnY
	w.Party.SetLocation(LocationSosaria)
	w.Party.SetSurfaceX(w.X)
	w.Party.SetSurfaceY(w.Y)
	w.Trail = nil
	w.ReturnToSurface()
	// After Exodus' defeat the land creatures merely wander. (PullSosaria)
	if w.Party.ExodusDestroyed() {
		m := w.Monsters()
		for i := range 32 {
			if m.Type(i) >= 0x40 {
				m.SetHP(i, 0x40)
			}
		}
	}
	io.PrintMessage(turnMsgExitToSosaria)
	// The door is a save point; the message area has no room on the same line.
	if hooks != nil && hooks.Save != nil && hooks.Save() {
		io.Print("(saved)\n")
	}
	io.ShowWind()
}

// EndTurn mirrors Routine6E35(), run after every command: count the move,
// advance the moons, leave a town from its edge, age the party, moongates
// and the whirlpool, Exodus' castle fireballs, then the monsters.
func EndTurn(w *World, io IO, hooks *TurnHooks) {
	w.Party.IncrementMoves()
	MoonGateUpdate(w, io)
	if w.Party.Location() == LocationDungeon {
		hooks.DungeonTurn()
		return
	}
	if w.InTownOrCastle() && (w.X == 0 || w.Y == 0) {
		ExitToSurface(w, io, hooks)
	}
	AgeChars(w, io)

	here := w.GetXYVal(w.X, w.Y)
	if here == MapValueMoonGate {
		HandleMoonStep(w, io)
	}
	if here == MapValueWhirlpool {
		hooks.GoWhirlpool()
	}
	if w.InExodusCastle() {
		// Random fireballs rain down inside Exodus' castle.
		w.TimeNegate = 0
		vx, vy := w.Rng.Range(0, ViewSize), w.Rng.Range(0, ViewSize)
		if vx < ViewSize && vy < ViewSize {
			under := w.GetXYVal(w.X-ViewCentre+vx, w.Y-ViewCentre+vy) >> 1
			if vx == ViewCentre && vy == ViewCentre {
				hooks.ShowBall(vx, vy, under)
				hooks.BombTrap()
			} else if under == ShapeFloor {
				hooks.ShowBall(vx, vy, under)
				io.Sound(SoundHit)
			}
		}
	}
	FinishAll(w, io, hooks)
}

// FinishAll mirrors FinishAll(): spawn and move monsters unless time is
// negated or the mount skips this turn.
func FinishAll(w *World, io IO, hooks *TurnHooks) {
	if w.SkipMonstersThisTurn() {
		return
	}
	if w.TimeNegate > 0 {
		w.TimeNegate--
		return
	}
	SpawnMonster(w)
	MoveMonsters(w, io, &hooks.MonsterHooks)
}
