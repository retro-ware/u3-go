package game

// Where the dungeon's maps draw each cell of a level (the TypeScript port's,
// after the Ultima V port's dungeonMap.ts). A level is 16 cells a side and
// wraps. Fifty of the 56 levels lie inside a wall along their first column
// and row, which bounds them through the wrap; a few have passages across an
// edge or right round. So each level is looked at for its seams (a column or
// row of wall all the way across) and whether its passages loop (LevelShape),
// and drawn by them: round the party (AroundParty: Top down and the 5x5 map)
// or the whole level, still (WholeLevel: View map). Only what CellVisible
// shows is drawn.

const levelSide = 16

// at16 is a cell's place in the level's grid (y * 16 + x), wrapped.
func at16(x, y int) int { return (y&15)<<4 | x&15 }

// passable: anything but a plain wall (doors, secret ones too, are walked through).
func passable(v byte) bool { return v != DungeonCellWall }

var steps4 = [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}

// LevelShape is a level's seams and whether its passages loop.
type LevelShape struct {
	SeamX int  // the column of wall all the way down, or -1
	SeamY int  // the row of wall all the way across, or -1
	Loops bool // a passage runs right round the level into itself
}

func levelCellsOf(w *World) []byte {
	from := w.Dungeon.Level * levelSide * levelSide
	return w.Dungeon.Tiles[from : from+levelSide*levelSide]
}

// walksRound says whether the passages walked out from start come back to a
// cell at another place: round the level.
func walksRound(cells []byte, start int, walked map[int]bool) bool {
	placed := map[int][2]int{start: {0, 0}}
	queue := [][2]int{{0, 0}}
	sx, sy := start&15, start>>4
	for len(queue) > 0 {
		q := queue[0]
		queue = queue[1:]
		for _, s := range steps4 {
			dx, dy := q[0]+s[0], q[1]+s[1]
			c := at16(sx+dx, sy+dy)
			if !passable(cells[c]) {
				continue
			}
			if was, ok := placed[c]; ok {
				if was != [2]int{dx, dy} {
					return true
				}
				continue
			}
			placed[c] = [2]int{dx, dy}
			queue = append(queue, [2]int{dx, dy})
		}
	}
	for c := range placed {
		walked[c] = true
	}
	return false
}

// LevelShapeOf is a level's seams and whether its passages loop (levelShape).
func LevelShapeOf(cells []byte) LevelShape {
	solid := func(cell func(k int) int) bool {
		for k := range levelSide {
			if cells[cell(k)] != DungeonCellWall {
				return false
			}
		}
		return true
	}
	s := LevelShape{SeamX: -1, SeamY: -1}
	for n := levelSide - 1; n >= 0; n-- {
		if solid(func(k int) int { return at16(n, k) }) {
			s.SeamX = n
		}
		if solid(func(k int) int { return at16(k, n) }) {
			s.SeamY = n
		}
	}
	// With both seams no passage crosses an edge, so none can come round.
	if s.SeamX < 0 || s.SeamY < 0 {
		walked := map[int]bool{}
		for c := 0; c < levelSide*levelSide && !s.Loops; c++ {
			if passable(cells[c]) && !walked[c] {
				s.Loops = walksRound(cells, c, walked)
			}
		}
	}
	return s
}

// AroundParty is the level round the party: the cell (y * 16 + x) to draw i
// cells east and j south of it, or -1 for none. Its passages each where they
// lie from the party, the rock beside a passage shown, nothing past a seam;
// where the level loops, the grid round the party as the first-person view has it.
func AroundParty(w *World) func(i, j int) int {
	cells := levelCellsOf(w)
	shape := LevelShapeOf(cells)
	px, py := w.X&15, w.Y&15
	shown := func(c int) bool { return CellVisible(w, c&15, c>>4) }
	if shape.Loops {
		return func(i, j int) int {
			if c := at16(px+i, py+j); shown(c) {
				return c
			}
			return -1
		}
	}
	placed := map[[2]int]int{}
	walked := map[int]bool{}
	walk := func(i0, j0 int) {
		queue := [][2]int{{i0, j0}}
		walked[at16(px+i0, py+j0)] = true
		if _, ok := placed[[2]int{i0, j0}]; !ok {
			placed[[2]int{i0, j0}] = at16(px+i0, py+j0)
		}
		for len(queue) > 0 {
			q := queue[0]
			queue = queue[1:]
			for _, s := range steps4 {
				i, j := q[0]+s[0], q[1]+s[1]
				c := at16(px+i, py+j)
				if !passable(cells[c]) || walked[c] {
					continue
				}
				walked[c] = true
				if _, ok := placed[[2]int{i, j}]; !ok {
					placed[[2]int{i, j}] = c
				}
				queue = append(queue, [2]int{i, j})
			}
		}
	}
	walk(0, 0)
	along := func(g, p, seam int) int {
		if seam < 0 {
			return (g-p+8)&15 - 8
		}
		return (g-seam)&15 - (p-seam)&15
	}
	for c := range levelSide * levelSide {
		if passable(cells[c]) && !walked[c] {
			walk(along(c&15, px, shape.SeamX), along(c>>4, py, shape.SeamY))
		}
	}
	return func(i, j int) int {
		if here, ok := placed[[2]int{i, j}]; ok {
			if shown(here) {
				return here
			}
			return -1
		}
		// Rock, where it bounds a passage shown beside it (a corner's too).
		c := at16(px+i, py+j)
		if passable(cells[c]) || !shown(c) {
			return -1
		}
		for b := -1; b <= 1; b++ {
			for a := -1; a <= 1; a++ {
				if next, ok := placed[[2]int{i + a, j + b}]; ok && shown(next) {
					return c
				}
			}
		}
		return -1
	}
}

// WholeLevel is the whole level for the View map.
type WholeLevel struct {
	Cols, Rows int                    // 17 along a seam's way (its wall at both ends), else 16
	Party      [2]int                 // where the party is on it
	At         func(col, row int) int // the cell to draw at (col, row), or -1
}

type levelAxis struct {
	cut  bool
	seam int
	size int
}

func (a levelAxis) grid(p int) int {
	if a.cut {
		return (p + a.seam) & 15
	}
	return p & 15
}
func (a levelAxis) place(g int) int {
	if a.cut {
		return (g - a.seam) & 15
	}
	return g & 15
}
func (a levelAxis) end(p int) bool { return a.cut && (p == 0 || p == levelSide) }
func (a levelAxis) off(p int) bool { return a.cut && (p < 0 || p > levelSide) }

// WholeLevelOf is the level cut at its seams, each seam's wall drawn at
// both ends where it bounds a passage shown beside it there (wholeLevel).
func WholeLevelOf(w *World) WholeLevel {
	cells := levelCellsOf(w)
	shape := LevelShapeOf(cells)
	shown := func(c int) bool { return CellVisible(w, c&15, c>>4) }
	axis := func(seam int) levelAxis {
		cut := seam >= 0 && !shape.Loops
		size := levelSide
		if cut {
			size++
		}
		return levelAxis{cut, seam, size}
	}
	x, y := axis(shape.SeamX), axis(shape.SeamY)
	return WholeLevel{
		Cols:  x.size,
		Rows:  y.size,
		Party: [2]int{x.place(w.X), y.place(w.Y)},
		At: func(col, row int) int {
			c := at16(x.grid(col), y.grid(row))
			if !shown(c) {
				return -1
			}
			if !x.end(col) && !y.end(row) {
				return c
			}
			for b := -1; b <= 1; b++ {
				for a := -1; a <= 1; a++ {
					if x.off(col+a) || y.off(row+b) {
						continue
					}
					if next := at16(x.grid(col+a), y.grid(row+b)); passable(cells[next]) && shown(next) {
						return c
					}
				}
			}
			return -1
		},
	}
}
