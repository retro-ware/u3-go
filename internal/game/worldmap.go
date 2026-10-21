package game

import (
	"fmt"
	"math"
	"slices"
	"strings"
)

// The map (the TypeScript port's, after the Ultima V port's), in place of
// the box's cloth map: what the party has seen of Sosaria, Ambrosia and each
// town and castle (automap.go), and in a dungeon its level's auto-map. This
// decides what is on a page: which pages there are, the marks on each, their
// names, and where the d-pad takes the cursor; the UI draws them.

// MarkKind is what a mark stands for.
type MarkKind string

const (
	MarkParty    MarkKind = "party"
	MarkCastle   MarkKind = "castle"
	MarkTowne    MarkKind = "towne"
	MarkDungeon  MarkKind = "dungeon"
	MarkMoongate MarkKind = "moongate"
	MarkShrine   MarkKind = "shrine"
)

// MapMark is one mark on a page.
type MapMark struct {
	Kind   MarkKind
	X, Y   int
	Label  string // its name, or what it looks like where not learnt
	Detail string // where the party is, or what A does
	Opens  int    // the map A opens from it (a town or castle been in), or 0
	Phase  int    // a moongate's Trammel phase
	Party  bool   // the party is inside this place (its mark blinks)
	Unseen bool   // charted but its square not yet seen
}

// MapPage is a page: a 64-square map, or a dungeon's level.
type MapPage struct {
	Dungeon bool // a dungeon level rather than a map
	ID      int
	Level   int
}

const (
	dawn    = 408
	britain = 402 // "LCB Towne" in the game's files
)

// MapPages is the pages the map turns through: where the party is first,
// then Sosaria, then Ambrosia once seen.
func MapPages(w *World) []MapPage {
	var pages []MapPage
	if w.Party.Location() == LocationDungeon {
		pages = append(pages, MapPage{Dungeon: true, ID: w.Current.ID, Level: w.Dungeon.Level})
	} else if w.Current.Size == MapSide {
		pages = append(pages, MapPage{ID: w.Current.ID})
	}
	for _, id := range []int{MapIDSosaria, MapIDAmbrosia} {
		if slices.Contains(pages, MapPage{ID: id}) {
			continue
		}
		if id == MapIDSosaria || w.AutoMap.HasMap(id) {
			pages = append(pages, MapPage{ID: id})
		}
	}
	return pages
}

// PlaceName is a place's name from the game's files, its Macintosh
// apostrophe made plain (Britain is the port's).
func PlaceName(w *World, id int) string {
	if id == britain {
		return "Britain"
	}
	name, ok := w.Resources.MapNames[id]
	if !ok {
		name = "?"
	}
	return strings.NewReplacer("Õ", "'", "’", "'").Replace(name)
}

// Charted says whether a place is marked on Sosaria before its square is
// seen, as the cloth map had it: every town and Lord British's castle, not
// Dawn, Exodus' castle nor a dungeon.
func Charted(id int) bool {
	return id >= MapIDLordBritishCastle && id < MapIDFirstDungeon && id != MapIDExodusCastle && id != dawn
}

func placeKind(id int) MarkKind {
	switch {
	case id == MapIDLordBritishCastle || id == MapIDExodusCastle:
		return MarkCastle
	case id >= MapIDFirstDungeon:
		return MarkDungeon
	}
	return MarkTowne
}

// BeenIn says whether the party has been in a place.
func BeenIn(w *World, id int) bool {
	if id >= MapIDFirstDungeon && id <= MapIDLastDungeon {
		return w.AutoMap.HasDungeon(id)
	}
	return w.AutoMap.HasMap(id)
}

// PageTitle is what a page is called: its map's name, in a dungeon with the level.
func PageTitle(w *World, page MapPage) string {
	name := strings.TrimPrefix(PlaceName(w, page.ID), "Dungeon ")
	if page.Dungeon {
		return fmt.Sprintf("%s, level %d", name, page.Level+1)
	}
	return name
}

// plain is "Strength" from "STRENGTH".
func plain(word string) string {
	w := strings.ToLower(strings.TrimSpace(word))
	if w == "" {
		return w
	}
	return strings.ToUpper(w[:1]) + w[1:]
}

// Shrines is the squares of Ambrosia's shrines, from its map.
func Shrines(w *World) []Point {
	raw, ok := w.Resources.Maps[MapIDAmbrosia]
	var out []Point
	if !ok {
		return out
	}
	for y := range MapSide {
		for x := range MapSide {
			if raw[1+y*MapSide+x] == MapValueShrine {
				out = append(out, Point{x, y})
			}
		}
	}
	return out
}

// MarksOn is the marks on a page: the party first, where it is on this
// page, then the places it shows.
func MarksOn(w *World, page MapPage) []MapMark {
	var marks []MapMark
	seen := func(x, y int) bool { return w.AutoMap.SquareSeen(page.ID, x, y) }
	inDungeon := w.Party.Location() == LocationDungeon
	if page.Dungeon || (!inDungeon && w.Current.ID == page.ID) {
		marks = append(marks, MapMark{Kind: MarkParty, X: w.X, Y: w.Y, Label: "The party", Detail: "You are here"})
	}
	// Inside a town, castle or dungeon the party is that place's mark on Sosaria.
	inside := -1
	if w.Current.ID >= MapIDLordBritishCastle && w.Current.ID <= MapIDLastDungeon {
		inside = w.Current.ID
	}
	if page.Dungeon {
		return marks
	}
	misc := w.Resources.Misc
	switch page.ID {
	case MapIDSosaria:
		for id := MapIDLordBritishCastle; id <= MapIDLastDungeon; id++ {
			x, y := int(misc.LocationX[id-400]), int(misc.LocationY[id-400])
			here := id == inside
			unseen := !here && !seen(x, y)
			if x == 0xff || (unseen && !Charted(id)) {
				continue
			}
			known := here || BeenIn(w, id)
			// Dawn stands only while both moons are new.
			if id == dawn && !known && w.Surface.Tiles[y*MapSide+x] != MapValueTown {
				continue
			}
			kind := placeKind(id)
			opens := 0
			if known && kind != MarkDungeon {
				opens = id
			}
			looks := map[MarkKind]string{MarkCastle: "Castle?", MarkDungeon: "Dungeon?", MarkTowne: "Towne?"}[kind]
			var detail string
			switch {
			case here:
				detail = "Party inside"
			case opens != 0:
				detail = "A: its map"
			case known:
				detail = "Been in"
			case unseen:
				detail = "Undiscovered"
			default:
				detail = "Not yet entered"
			}
			label := looks
			if known {
				label = PlaceName(w, id)
			}
			mark := MapMark{Kind: kind, X: x, Y: y, Label: label, Detail: detail, Opens: opens, Party: here, Unseen: unseen}
			if here {
				marks = append([]MapMark{mark}, marks...)
			} else {
				marks = append(marks, mark)
			}
		}
		for _, phase := range w.GatesKnown {
			marks = append(marks, MapMark{Kind: MarkMoongate, X: int(misc.MoonX[phase]), Y: int(misc.MoonY[phase]),
				Label: "Moongate", Detail: "At this moon:", Phase: phase})
		}
	case MapIDAmbrosia:
		msgs := w.Resources.Strings["Messages"]
		for _, s := range Shrines(w) {
			if !seen(s.X, s.Y) {
				continue
			}
			stat := ""
			if i := 173 + s.X&3; i < len(msgs) {
				stat = msgs[i]
			}
			m := MapMark{Kind: MarkShrine, X: s.X, Y: s.Y, Label: "Shrine?", Detail: "Not yet entered"}
			if w.AutoMap.Visited(fmt.Sprintf("shrine:%d,%d", s.X, s.Y)) {
				m.Label, m.Detail = "Shrine of "+plain(stat), "Been in"
			}
			marks = append(marks, m)
		}
	}
	return marks
}

// MarkDirection is a d-pad direction on the map.
type MarkDirection string

const (
	DirUp    MarkDirection = "up"
	DirDown  MarkDirection = "down"
	DirLeft  MarkDirection = "left"
	DirRight MarkDirection = "right"
)

// StepMark is the mark the d-pad takes the cursor to from from: the nearest
// the way it points within a wide cone (across at most twice as far as
// along), else the nearest that way at all; from where there is none. On a
// map that wraps (side MapSide, Sosaria and Ambrosia) the shorter way round counts.
func StepMark(marks []MapMark, from int, dir MarkDirection, side int, wraps bool) int {
	if from < 0 || from >= len(marks) {
		return from
	}
	o := marks[from]
	half := side / 2
	near := func(d int) int {
		if wraps {
			return ((d+half)%side+side)%side - half
		}
		return d
	}
	best, bestCone, anyIndex, bestAny := from, math.MaxInt, from, math.MaxInt
	for i, m := range marks {
		if i == from {
			continue
		}
		dx, dy := near(m.X-o.X), near(m.Y-o.Y)
		var along, across int
		switch dir {
		case DirLeft:
			along, across = -dx, dy
		case DirRight:
			along, across = dx, dy
		case DirUp:
			along, across = -dy, dx
		default:
			along, across = dy, dx
		}
		if along <= 0 {
			continue
		}
		score := along + abs(across)*2
		if abs(across) <= along*2 && score < bestCone {
			best, bestCone = i, score
		}
		if score < bestAny {
			anyIndex, bestAny = i, score
		}
	}
	if bestCone < math.MaxInt {
		return best
	}
	return anyIndex
}

// Segment is a piece of coast from (X1, Y1) to (X2, Y2), in squares.
type Segment struct{ X1, Y1, X2, Y2 float64 }

// CoastSegments is the coasts of a map, for the chart: marching squares
// over land and water sampled at the squares' centres. On a map that wraps
// the cells over its edges are taken too, so a coast joins across them.
func CoastSegments(isLand func(x, y int) bool, side int, wraps bool) []Segment {
	at := func(x, y int) bool { return isLand((x%side+side)%side, (y%side+side)%side) }
	var out []Segment
	start, end := 0, side-1
	if wraps {
		start, end = -1, side
	}
	bit := func(b bool, v int) int {
		if b {
			return v
		}
		return 0
	}
	for y := start; y < end; y++ {
		for x := start; x < end; x++ {
			c := bit(at(x, y), 8) | bit(at(x+1, y), 4) | bit(at(x+1, y+1), 2) | bit(at(x, y+1), 1)
			if c == 0 || c == 15 {
				continue
			}
			fx, fy := float64(x), float64(y)
			top := [2]float64{fx + 1, fy + 0.5}
			right := [2]float64{fx + 1.5, fy + 1}
			bottom := [2]float64{fx + 1, fy + 1.5}
			left := [2]float64{fx + 0.5, fy + 1}
			seg := func(a, b [2]float64) { out = append(out, Segment{a[0], a[1], b[0], b[1]}) }
			switch c {
			case 1, 14:
				seg(left, bottom)
			case 2, 13:
				seg(bottom, right)
			case 3, 12:
				seg(left, right)
			case 4, 11:
				seg(top, right)
			case 5:
				seg(left, top)
				seg(bottom, right)
			case 6, 9:
				seg(top, bottom)
			case 7, 8:
				seg(left, top)
			case 10:
				seg(top, right)
				seg(left, bottom)
			}
		}
	}
	return out
}

// IsLandValue says whether a map value is land for the chart: all but open
// water (a whirlpool and a ship are on water).
func IsLandValue(v int) bool {
	return v != MapValueWater && v != MapValueWhirlpool && v != MapValueFrigate
}
