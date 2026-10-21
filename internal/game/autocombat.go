package game

import (
	"slices"
	"strconv"
)

// Auto-combat: a port of LairWare's UltimaAutocombat.c, a Macintosh
// addition. The AI never touches the fight directly: each turn it decides
// what the active member should do and returns the keys a player would
// press, which combat feeds to the normal prompts (the C pushed them into
// Macro[]).
//
// The decision list, in priority order, per AutoCombat():
//  1. Nearly dead and in reach of a monster: step somewhere safe.
//  2. Orcs and a wizard type: Repond once. Skeletons and a cleric type: Pontori once.
//  3. A big threat and enough magic: the nameless wizard spell, or ZXKUQYB.
//  4. A cleric type with someone badly hurt: Sanctu on the weakest.
//  5. A ranged weapon (or a wizard's Mittar): fire at a monster in line,
//     else step to a square that lines one up next turn.
//  6. A hand weapon: attack an adjacent monster, else step toward the nearest.
//
// With diagonal moves off the member neither steps, attacks nor fires
// diagonally, though it still expects monsters to.

// nearlyDeadAt is "nearly dead": under a quarter of maximum hit points, at
// most 50 (the Mac's flat 50 was written for parties with hundreds).
func nearlyDeadAt(maxHitPoints int) int { return min(50, max(15, maxHitPoints/4)) }

// Sanctu is worth a turn when someone is under this share of their maximum and down by at least 20.
const (
	sanctuShare   = 0.6
	sanctuMinLoss = 20
)

const (
	acOrcs      = 0x30
	acSkeletons = 0x32
	acDragons   = 0x3a
)

var (
	acPoisonous = []int{0x1c, 0x3c, 0x38} // count double as a threat
	acMagicBows = []int{9, 13}
)

// directionKey is the keypad digit for a step of (dx, dy), or "" for none.
func directionKey(dx, dy int) string {
	switch [2]int{dx, dy} {
	case [2]int{-1, 1}:
		return "1"
	case [2]int{0, 1}:
		return "2"
	case [2]int{1, 1}:
		return "3"
	case [2]int{-1, 0}:
		return "4"
	case [2]int{1, 0}:
		return "6"
	case [2]int{-1, -1}:
		return "7"
	case [2]int{0, -1}:
		return "8"
	case [2]int{1, -1}:
		return "9"
	}
	return ""
}

// neighbours are the eight steps, orthogonal first as the original checked them.
var neighbours = [][2]int{{0, 1}, {-1, 1}, {1, 1}, {-1, 0}, {1, 0}, {0, -1}, {-1, -1}, {1, -1}}

// planner plans one member's turn.
type planner struct {
	w                *World
	member           int
	c                *CombatState
	futureX, futureY [8]int // where each monster is expected next turn (futureMonX/Y)
	diagonals        bool
}

func (pl *planner) allowed(dx, dy int) bool { return pl.diagonals || dx == 0 || dy == 0 }

// plan mirrors AutoCombat(): the keys to press, in order.
func (pl *planner) plan() []string {
	w, c := pl.w, pl.c
	p := w.Member(pl.member)
	classIndex := slices.Index(w.Resources.Misc.Career, p.ClassLetter())
	isMulti := classIndex == 8 || classIndex == 10                               // Druid, Ranger
	isWizard := classIndex == 2 || classIndex == 6 || classIndex == 9 || isMulti // Wizard, Lark, Alchemist
	isCleric := classIndex == 1 || classIndex == 4 || classIndex == 7 || isMulti // Cleric, Paladin, Illusionist
	magic := p.Mana()
	me := c.Members[pl.member]
	x, y := me.X, me.Y

	// Multi-class members are asked "Cleric or Wizard?" after C.
	wizardCast := func(letter string) []string {
		if isMulti {
			return []string{"C", "W", letter}
		}
		return []string{"C", letter}
	}
	clericCast := func(letter string) []string {
		if isMulti {
			return []string{"C", "C", letter}
		}
		return []string{"C", letter}
	}

	// Nearly dead: run to a square no monster can reach; nowhere to run, fight on.
	if pl.nearlyDead(pl.member) && pl.monsterCanAttack(x, y) {
		for _, d := range neighbours {
			if pl.allowed(d[0], d[1]) && !pl.monsterCanAttack(x+d[0], y+d[1]) && !pl.occupied(x+d[0], y+d[1]) {
				return []string{directionKey(d[0], d[1])}
			}
		}
	}

	// Repond against orcs, Pontori against skeletons, once per fight.
	if c.MonsterShape == acOrcs && !w.SpellFlags.Repond && isWizard {
		return wizardCast("A")
	}
	if c.MonsterShape == acSkeletons && !w.SpellFlags.Pontori && isCleric {
		return clericCast("A")
	}

	// The big spells when the threat is big.
	if magic >= 75 && isWizard && pl.threatValue() > 60 {
		return wizardCast("P")
	}
	if magic >= 70 && isCleric && pl.threatValue() > 80 {
		return clericCast("O")
	}

	// Sanctu for whoever needs it most, if anyone really does.
	if magic >= 10 && isCleric {
		worst, lowest := 1.0, -1
		for m := range 4 {
			if !w.MemberAlive(m) {
				continue
			}
			q := w.Member(m)
			share := float64(q.HitPoints()) / float64(max(1, q.MaxHitPoints()))
			if share < worst && q.MaxHitPoints()-q.HitPoints() >= sanctuMinLoss {
				worst, lowest = share, m
			}
		}
		if lowest >= 0 && worst < sanctuShare {
			return append(clericCast("C"), strconv.Itoa(lowest+1))
		}
	}

	// Ranged attacks: a missile weapon, or Mittar for a wizard without a magic bow.
	weapon := int(p.Bytes[48])
	castMittar := magic >= 5 && isWizard && !slices.Contains(acMagicBows, weapon)
	if slices.Contains(Shoots, weapon) || castMittar {
		if dir := pl.monsterLinedUp(x, y); dir != "" {
			if castMittar {
				return append(wizardCast("B"), dir)
			}
			return []string{"A", dir}
		}
		if pl.nearlyDead(pl.member) {
			return []string{KeySpace}
		}
		return []string{pl.lineUpToMonster()}
	}

	// Hand to hand: strike whatever is adjacent (a wounded member who could
	// not get away fights back), else close in unless wounded.
	if adjacent := pl.monsterNearby(x, y); adjacent != "" {
		return []string{"A", adjacent}
	}
	if pl.nearlyDead(pl.member) {
		return []string{KeySpace}
	}
	return []string{pl.dirToNearestMonster()}
}

// threatValue mirrors ThreatValue(): the experience of the monsters left, doubled if they poison.
func (pl *planner) threatValue() int {
	each := int(pl.w.Resources.Misc.Experience[(pl.c.MonsterShape>>1)&0x0f])
	total := 0
	for _, m := range pl.c.Monsters {
		if m.HP > 0 {
			total += each
		}
	}
	if slices.Contains(acPoisonous, pl.c.MonsterShape) {
		total *= 2
	}
	return total
}

// monsterNearby mirrors MonsterNearby(): the direction of an adjacent monster, or "".
func (pl *planner) monsterNearby(x, y int) string {
	for _, d := range [][2]int{{0, -1}, {-1, 0}, {1, 0}, {0, 1}, {-1, -1}, {-1, 1}, {1, -1}, {1, 1}} {
		if pl.allowed(d[0], d[1]) && MonsterAt(pl.c, x+d[0], y+d[1]) >= 0 {
			return directionKey(d[0], d[1])
		}
	}
	return ""
}

// monsterCanAttack mirrors MonsterCanAttack(): is (x, y) next to a monster, or in a dragon's line of fire?
func (pl *planner) monsterCanAttack(x, y int) bool {
	for _, d := range neighbours {
		if MonsterAt(pl.c, x+d[0], y+d[1]) >= 0 {
			return true
		}
	}
	if pl.c.MonsterShape == acDragons {
		for _, m := range pl.c.Monsters {
			if m.HP > 0 && abs(m.X-x) == abs(m.Y-y) {
				return true
			}
		}
	}
	return false
}

// nearlyDead mirrors NearlyDead(who), scaled to the member's maximum.
func (pl *planner) nearlyDead(member int) bool {
	p := pl.w.Member(member)
	return p.HitPoints() < nearlyDeadAt(p.MaxHitPoints())
}

// setupNow mirrors SetupNow(): predict no movement. (The Mac's SetupFuture()
// guess sent members chasing moving squares; the TypeScript port steers at
// where monsters are.)
func (pl *planner) setupNow() {
	for i := range 8 {
		pl.futureX[i], pl.futureY[i] = pl.c.Monsters[i].X, pl.c.Monsters[i].Y
	}
}

// occupied mirrors CombatCharHere(): a member, a monster, or a square a member cannot stand on.
func (pl *planner) occupied(x, y int) bool {
	c := pl.c
	if x < 0 || x > 10 || y < 0 || y > 10 || !memberCanStand(int(c.Tiles[y*11+x])) {
		return true
	}
	for _, m := range c.Members {
		if m.X == x && m.Y == y {
			return true
		}
	}
	return MonsterAt(c, x, y) >= 0
}

// dirToNearestMonster mirrors DirToNearestMonster(), done properly: a
// breadth-first search to the nearest square from which this member could
// strike a monster, stepping round comrades and walls. A pass when none.
func (pl *planner) dirToNearestMonster() string {
	c := pl.c
	me := c.Members[pl.member]
	var steps [][2]int
	for _, d := range neighbours {
		if pl.allowed(d[0], d[1]) {
			steps = append(steps, d)
		}
	}
	isGoal := func(x, y int) bool {
		for _, d := range steps {
			if MonsterAt(c, x+d[0], y+d[1]) >= 0 {
				return true
			}
		}
		return false
	}
	// first remembers the first step of the path to each square.
	first := map[int]string{me.Y*11 + me.X: ""}
	queue := [][2]int{{me.X, me.Y}}
	for len(queue) > 0 {
		x, y := queue[0][0], queue[0][1]
		queue = queue[1:]
		via := first[y*11+x]
		if via != "" && isGoal(x, y) {
			return via
		}
		for _, d := range steps {
			nx, ny := x+d[0], y+d[1]
			if _, seen := first[ny*11+nx]; nx < 0 || nx > 10 || ny < 0 || ny > 10 || pl.occupied(nx, ny) || seen {
				continue
			}
			if via == "" {
				first[ny*11+nx] = directionKey(d[0], d[1])
			} else {
				first[ny*11+nx] = via
			}
			queue = append(queue, [2]int{nx, ny})
		}
	}
	return KeySpace
}

// lineUpToMonster mirrors LineUpToMonster(): step to a neighbouring square
// from which a monster will be in line next turn, nearer the middle first.
func (pl *planner) lineUpToMonster() string {
	me := pl.c.Members[pl.member]
	in := -1
	if me.X < 6 {
		in = 1
	}
	for _, d := range [][2]int{{in, 0}, {-in, 0}, {0, -1}, {0, 1}, {in, -1}, {in, 1}, {-in, -1}, {-in, 1}} {
		if !pl.allowed(d[0], d[1]) || pl.occupied(me.X+d[0], me.Y+d[1]) {
			continue
		}
		if pl.monsterLinedUp(me.X+d[0], me.Y+d[1]) != "" {
			return directionKey(d[0], d[1])
		}
	}
	return pl.dirToNearestMonster()
}

// monsterLinedUp mirrors MonsterLinedUp(): the direction from (x, y) to the
// nearest predicted monster on the same row, column or diagonal, or "".
func (pl *planner) monsterLinedUp(x, y int) string {
	closest, closestDistance := -1, 128
	for i := range 8 {
		if pl.c.Monsters[i].HP == 0 {
			continue
		}
		fx, fy := pl.futureX[i], pl.futureY[i]
		if !(fx == x || fy == y || pl.diagonals && abs(x-fx) == abs(y-fy)) {
			continue
		}
		if d := abs(fx-x) + abs(fy-y); d < closestDistance {
			closestDistance, closest = d, i
		}
	}
	if closest < 0 {
		return ""
	}
	return directionKey(sign(pl.futureX[closest]-x), sign(pl.futureY[closest]-y))
}

// AutoCombatKeys decides the active member's turn: the keys to press, in
// order, or nil when there is no fight.
func AutoCombatKeys(w *World, member int) []string {
	if w.Combat == nil {
		return nil
	}
	pl := &planner{w: w, member: member, c: w.Combat, diagonals: w.DiagonalMoves}
	pl.setupNow()
	return pl.plan()
}
