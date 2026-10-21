package game

import (
	"fmt"
	"slices"
	"testing"
)

const (
	testMines  = 417
	testDardin = 418
)

func cellAt(x, y int) int { return (y&15)<<4 | x&15 }

// onLevel is the party on a dungeon level, lit, its map blank or as if every passage had been walked.
func onLevel(t *testing.T, id, level int, all bool) (w *World, open func(int) bool, standOn func(int)) {
	w = newWorld(t)
	w.EnterDungeon(id)
	w.Party.SetLocation(LocationDungeon)
	w.Dungeon.Level = level
	w.Dungeon.Torch = 50
	w.AutoMap = NewAutoMap(nil)
	open = func(c int) bool { return w.GetXYDng(c&15, c>>4) != DungeonCellWall }
	if all {
		for c := range 256 {
			if open(c) {
				w.AutoMap.Mark(id, level, c&15, c>>4)
			}
		}
	}
	return w, open, func(c int) { w.X, w.Y = c&15, c>>4 }
}

func seamString(v int) string {
	if v < 0 {
		return "null"
	}
	return fmt.Sprint(v)
}

func TestLevelShapes(t *testing.T) {
	w := newWorld(t)
	var shapes []string
	for id := MapIDFirstDungeon; id <= MapIDLastDungeon; id++ {
		raw := w.Resources.Maps[id]
		for l := range 8 {
			s := LevelShapeOf(raw[l*256 : l*256+256])
			if s.SeamX == 0 && s.SeamY == 0 && !s.Loops {
				continue
			}
			loops := ""
			if s.Loops {
				loops = " loops"
			}
			shapes = append(shapes, fmt.Sprintf("%d.%d: %s %s%s", id, l+1, seamString(s.SeamX), seamString(s.SeamY), loops))
		}
	}
	want := []string{
		fmt.Sprintf("%d.2: null null loops", testMines),
		fmt.Sprintf("%d.3: null null loops", testMines),
		fmt.Sprintf("%d.4: 0 null", testMines),
		fmt.Sprintf("%d.7: null null loops", testMines),
		fmt.Sprintf("%d.1: null null loops", testDardin),
		fmt.Sprintf("%d.4: null null loops", testDardin),
	}
	if !slices.Equal(shapes, want) {
		t.Fatal(shapes)
	}
}

func firstOpenRow(w *World) int {
	for y := range 16 {
		if w.GetXYDng(15, y) != DungeonCellWall {
			return y
		}
	}
	return -1
}

func firstOpen(open func(int) bool) int {
	for c := range 256 {
		if open(c) {
			return c
		}
	}
	return -1
}

func TestAroundPartyEastEdgeWall(t *testing.T) {
	w, _, standOn := onLevel(t, MapIDFirstDungeon, 0, false)
	row := firstOpenRow(w)
	standOn(cellAt(15, row))
	w.AutoMap.Mark(w.Current.ID, 0, 15, row)
	at := AroundParty(w)
	if at(1, 0) != cellAt(0, row) || at(-15, 0) != -1 || at(2, 0) != -1 || at(0, 0) != cellAt(15, row) {
		t.Fatal("east edge")
	}
}

func TestAroundPartyDark(t *testing.T) {
	w, _, standOn := onLevel(t, MapIDFirstDungeon, 0, false)
	w.Dungeon.Torch = 0
	row := firstOpenRow(w)
	standOn(cellAt(15, row))
	at := AroundParty(w)
	if at(1, 0) != cellAt(0, row) || at(-15, 0) != -1 {
		t.Fatal("dark")
	}
}

func TestAroundPartyMatchesWholeLevel(t *testing.T) {
	var differ []string
	for id := MapIDFirstDungeon; id <= MapIDLastDungeon; id++ {
		for level := range 8 {
			w, open, standOn := onLevel(t, id, level, true)
			s := LevelShapeOf(w.Dungeon.Tiles[level*256 : level*256+256])
			if s.SeamX < 0 || s.SeamY < 0 {
				continue
			}
			for c := 0; c < 256; c += 3 {
				if !open(c) {
					continue
				}
				standOn(c)
				at, whole := AroundParty(w), WholeLevelOf(w)
				px, py := whole.Party[0], whole.Party[1]
				for j := -12; j <= 12; j++ {
					for i := -12; i <= 12; i++ {
						col, row := px+i, py+j
						want := -1
						if col >= 0 && col < whole.Cols && row >= 0 && row < whole.Rows {
							want = whole.At(col, row)
						}
						if at(i, j) != want {
							differ = append(differ, fmt.Sprintf("%d.%d from %d at %d,%d", id, level+1, c, i, j))
						}
					}
				}
			}
		}
	}
	if len(differ) > 0 {
		t.Fatal(differ[:min(10, len(differ))])
	}
}

func TestAroundPartyOtherParts(t *testing.T) {
	// Doom 3 has two parts, reached by different ladders.
	w, open, standOn := onLevel(t, MapIDFirstDungeon, 2, true)
	first := firstOpen(open)
	part := []int{first}
	for k := 0; k < len(part); k++ {
		c := part[k]
		for _, n := range []int{cellAt(c&15+1, c>>4), cellAt(c&15-1, c>>4), cellAt(c&15, c>>4+1), cellAt(c&15, c>>4-1)} {
			if open(n) && !slices.Contains(part, n) {
				part = append(part, n)
			}
		}
	}
	other := -1
	for c := range 256 {
		if open(c) && !slices.Contains(part, c) {
			other = c
			break
		}
	}
	standOn(first)
	if AroundParty(w)(other&15-first&15, other>>4-first>>4) != other {
		t.Fatal("other part")
	}
}

func TestAroundPartyAcrossEdge(t *testing.T) {
	w, open, standOn := onLevel(t, testMines, 3, true)
	x := -1
	for i := range 16 {
		if open(cellAt(i, 15)) && open(cellAt(i, 0)) {
			x = i
			break
		}
	}
	standOn(cellAt(x, 15))
	at := AroundParty(w)
	if at(0, 1) != cellAt(x, 0) || at(0, -15) != -1 {
		t.Fatal("edge")
	}
}

func TestAroundPartyLoops(t *testing.T) {
	w, open, standOn := onLevel(t, testDardin, 0, true)
	standOn(firstOpen(open))
	at := AroundParty(w)
	for j := -3; j <= 3; j++ {
		for i := -3; i <= 3; i++ {
			if at(i+16, j) != at(i, j) {
				t.Fatal(i, j)
			}
		}
	}
}

func TestWholeLevelCutAtSeams(t *testing.T) {
	w, _, standOn := onLevel(t, MapIDFirstDungeon, 0, false)
	row := firstOpenRow(w)
	standOn(cellAt(15, row))
	w.AutoMap.Mark(w.Current.ID, 0, 15, row)
	whole := WholeLevelOf(w)
	if whole.Cols != 17 || whole.Rows != 17 || whole.Party != [2]int{15, row} ||
		whole.At(16, row) != cellAt(0, row) || whole.At(0, row) != -1 || whole.At(15, row) != cellAt(15, row) {
		t.Fatalf("%+v", whole)
	}
}

func TestWholeLevelLoops(t *testing.T) {
	w, open, standOn := onLevel(t, testDardin, 0, true)
	standOn(firstOpen(open))
	whole := WholeLevelOf(w)
	if whole.Cols != 16 || whole.Rows != 16 || whole.Party != [2]int{w.X, w.Y} {
		t.Fatal("size")
	}
	for c := range 256 {
		if whole.At(c&15, c>>4) != c {
			t.Fatal(c)
		}
	}
}
