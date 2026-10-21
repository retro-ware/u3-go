package game

// Wandering monsters on the surface and NPCs in towns: spawning and
// movement. SpawnMonster(), MoveMonsters(), GetMonsterDir(),
// ValidMonsterDir() and GetHeading() from UltimaMisc.c.
//
// As on the Apple II a monster is in two places at once: the map holds its
// value where it stands, and the monster table remembers the terrain
// underneath so it can be restored when the monster moves.

// SpawnTypes is the monster tiles (map value / 4) that can spawn on the
// surface, by the random draw in SpawnMonster; SpawnTerrain (MonBegin) is
// the terrain each starts on: sea monsters on water, the rest on grass.
var (
	SpawnTypes   = []int{24, 23, 25, 20, 26, 27, 13, 28, 22, 14, 15, 29, 30, 24}
	SpawnTerrain = []int{4, 4, 4, 4, 4, 4, 0, 4, 4, 0, 0, 4, 4}
)

// MonsterHooks are the callbacks the movement code needs from the game loop.
type MonsterHooks struct {
	Attack   func(monsterIndex int)       // a monster reached the party: start combat (AttackCode)
	BombTrap func()                       // a fireball hit the party (BombTrap)
	ShowBall func(vx, vy, background int) // draw a fireball at viewport cell (vx, vy) for one frame
}

// Heading is the sign of a distance as the 6502 saw it: reduced to a byte,
// bit 7 decides. GetHeading() with true 8-bit wrap-around.
func Heading(value int) int {
	b := value & 0xff
	switch {
	case b == 0:
		return 0
	case b > 127:
		return -1
	}
	return 1
}

// MonsterCanEnter mirrors ValidMonsterDir(): may a monster of this type
// stand on this terrain? Sea monsters need water; land monsters grass,
// brush, forest and floor.
func MonsterCanEnter(terrain, monsterType int) bool {
	if monsterType > 0x28 && monsterType < 0x40 {
		return terrain == MapValueWater
	}
	return terrain == MapValueGrass || terrain == MapValueBrush || terrain == MapValueForest || terrain == MapValueFloor
}

// MonsterStep is the step a monster would take toward the party.
type MonsterStep struct {
	XS, YS       int // target square after the step
	DX, DY       int // direction, each -1..1
	DistX, DistY int // signed distance to the party (shooting range)
}

// GetMonsterDirection mirrors GetMonsterDir(). On the surface the distance
// is multiplied by four before taking the sign: the 8-bit overflow then
// makes monsters more than 32 squares away approach the short way round.
func GetMonsterDirection(w *World, i int) MonsterStep {
	m := w.Monsters()
	distX, distY := w.X-m.X(i), w.Y-m.Y(i)
	scale := 1
	if w.OnSurface() {
		scale = 4
	}
	dx, dy := Heading(distX*scale), Heading(distY*scale)
	return MonsterStep{w.Constrain(m.X(i) + dx), w.Constrain(m.Y(i) + dy), dx, dy, distX, distY}
}

// SpawnMonster mirrors SpawnMonster(): on the surface, with probability
// 7/135 a turn, a new monster in the highest free slot at a random spot of
// the right terrain. While the whole party is still at 150 max HP only the
// three weakest types appear.
func SpawnMonster(w *World) {
	if !w.OnSurface() || w.Rng.Range(0, 134) < 128 {
		return
	}
	m := w.Monsters()
	slot := m.FreeSlot()
	if slot < 0 {
		return
	}
	allBeginners := true
	for _, p := range w.Members() {
		if p.MaxHitPoints() > 150 {
			allBeginners = false
		}
	}
	t := w.Rng.Range(0, 12) & w.Rng.Range(0, 12)
	if allBeginners {
		t = w.Rng.Range(0, 2)
	}
	x := w.Rng.Range(0, w.MapSize()-1)
	y := w.Rng.Range(0, w.MapSize()-1)
	if x == w.X || y == w.Y || w.GetXYVal(x, y) != SpawnTerrain[t] {
		return
	}
	m.SetType(slot, SpawnTypes[t]*4)
	m.SetTileUnder(slot, SpawnTerrain[t])
	m.SetPosition(slot, x, y)
	m.SetHP(slot, 0xc0)
	variant := 0
	if w.Rng.Range(0, 1) != 0 {
		variant = w.Rng.Range(1, 2)
	}
	m.SetVariant(slot, variant)
	// After Exodus is destroyed monsters (other than pirates) merely wander.
	if w.Party.ExodusDestroyed() && m.Type(slot) != MapValuePirate {
		m.SetHP(slot, 0x40)
	}
	w.PutXYVal(m.Type(slot), x, y)
}

// stepMonster moves monster i to the step's square; if blocked it tries a
// vertical-only step, then a horizontal-only one, else stays. Shooters fire
// after moving or failing to. (the move7AAA block of MoveMonsters())
func stepMonster(w *World, i int, step MonsterStep, hooks *MonsterHooks, io IO) {
	m := w.Monsters()
	t := m.Type(i)
	blocked := func(x, y int) bool { return !MonsterCanEnter(w.GetXYVal(x, y), t) || m.At(x, y) >= 0 }
	xs, ys := step.XS, step.YS
	if blocked(xs, ys) {
		xs = m.X(i)
		if blocked(xs, ys) {
			xs, ys = w.Constrain(m.X(i)+step.DX), m.Y(i)
			if blocked(xs, ys) {
				if isShooter(t) {
					monsterShoot(w, i, hooks, io)
				}
				return
			}
		}
	}
	if xs == w.X && ys == w.Y {
		return
	}
	// Restore the terrain, move, remember the new terrain, place the monster.
	w.PutXYVal(m.TileUnder(i), m.X(i), m.Y(i))
	m.SetPosition(i, xs, ys)
	m.SetTileUnder(i, w.GetXYVal(xs, ys))
	w.PutXYVal(m.DisplayValue(i), xs, ys)
	if isShooter(t) {
		monsterShoot(w, i, hooks, io)
	}
}

// isShooter: pirates and dragons fire at the party.
func isShooter(t int) bool { return t == MapValuePirate || t == MapValueDragon }

// monsterShoot mirrors the moveshoot block: half the time a shooter within
// the viewport fires a ball three squares toward the party. Mountains,
// walls and void stop it; reaching the party triggers BombTrap.
func monsterShoot(w *World, i int, hooks *MonsterHooks, io IO) {
	if w.Rng.Range(0, 255) > 127 {
		return
	}
	step := GetMonsterDirection(w, i)
	vx, vy := ViewCentre-step.DistX, ViewCentre-step.DistY
	out := func() bool { return vx < 0 || vx >= ViewSize || vy < 0 || vy >= ViewSize }
	if out() {
		return
	}
	io.RedrawMap()
	io.Sound(SoundShoot)
	for range 3 {
		vx += step.DX
		vy += step.DY
		if out() {
			return
		}
		under := w.GetXYVal(w.X-ViewCentre+vx, w.Y-ViewCentre+vy) >> 1
		if under == ShapeMountains || under == ShapeWall || under == ShapeVoid {
			return
		}
		hooks.ShowBall(vx, vy, under)
		if vx == ViewCentre && vy == ViewCentre {
			hooks.BombTrap()
			return
		}
	}
}

// MoveMonsters mirrors MoveMonsters(): every occupied slot acts once a
// turn, highest first. On the surface (before Exodus falls) every monster
// closes in and attacks on contact; in towns, and on the surface after,
// each NPC's behaviour bits decide.
func MoveMonsters(w *World, io IO, hooks *MonsterHooks) {
	m := w.Monsters()
	for i := 31; i >= 0; i-- {
		if m.Type(i) == 0 {
			continue
		}
		behaviour := m.Behaviour(i)
		if w.OnSurface() && !w.Party.ExodusDestroyed() {
			behaviour = BehaviourAttack
		}
		switch behaviour {
		case BehaviourStationary:
			continue
		case BehaviourWander:
			if w.Rng.Range(0, 255) < 128 {
				continue
			}
			xs := w.Constrain(m.X(i) + Heading(w.Rng.Range(0, 255)))
			if xs == 0 {
				continue // never wander onto the exit row or column of a town
			}
			ys := w.Constrain(m.Y(i) + Heading(w.Rng.Range(0, 255)))
			if ys == 0 {
				continue
			}
			stepMonster(w, i, MonsterStep{XS: xs, YS: ys, DX: xs - m.X(i), DY: ys - m.Y(i)}, hooks, io)
			continue
		}
		// Follow and Attack both head toward the party.
		step := GetMonsterDirection(w, i)
		if behaviour == BehaviourAttack && step.XS == w.X && step.YS == w.Y {
			hooks.Attack(i)
			return // combat ends the monsters' turn
		}
		stepMonster(w, i, step, hooks, io)
	}
}
