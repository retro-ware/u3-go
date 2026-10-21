package game

import (
	"encoding/json"
	"math"
	"regexp"
	"slices"
	"strings"
	"testing"
)

const (
	testYew  = 404
	testDawn = 408
)

func placeAt(w *World, id int) Point {
	return Point{int(w.Resources.Misc.LocationX[id-400]), int(w.Resources.Misc.LocationY[id-400])}
}

func voids(v *Viewport) int {
	n := 0
	for _, c := range v.Cells {
		if c.Base == ShapeVoid {
			n++
		}
	}
	return n
}

func TestAutoMapKeepsSquaresThroughStore(t *testing.T) {
	store := &memoryStore{}
	m := NewAutoMap(store)
	m.MarkSquares(MapIDSosaria, []Point{{0, 0}, {63, 63}, {10, 20}})
	m.MarkSquares(testYew, []Point{{1, 32}, {2, 32}})
	m.Visit("shrine:6,6")
	again := NewAutoMap(store)
	if !again.SquareSeen(MapIDSosaria, 10, 20) || !again.SquareSeen(MapIDSosaria, 63, 63) || again.SquareSeen(MapIDSosaria, 11, 20) {
		t.Fatal("squares")
	}
	if again.MapSeenCount(MapIDSosaria) != 3 || again.MapSeenCount(testYew) != 2 || !again.Visited("shrine:6,6") {
		t.Fatal("counts")
	}
	again.Clear()
	if NewAutoMap(store).HasMap(testYew) {
		t.Fatal("not cleared")
	}
}

func TestAutoMapReadsOldStore(t *testing.T) {
	b, _ := json.Marshal(map[string]string{"412:0": strings.Repeat("1", 256)})
	m := NewAutoMap(&memoryStore{string(b), true})
	if m.SeenCount(412, 0) != 256 || m.HasMap(MapIDSosaria) || !m.HasDungeon(412) {
		t.Fatal("old store")
	}
}

func TestRecordViewRecordsWhatTheViewShows(t *testing.T) {
	w := newWorld(t)
	w.AutoMap = NewAutoMap(nil)
	v := BuildViewport(w, w.X, w.Y)
	RecordView(w, v)
	if w.AutoMap.MapSeenCount(MapIDSosaria) != 121-voids(v) || !w.AutoMap.SquareSeen(MapIDSosaria, w.X, w.Y) {
		t.Fatal("record")
	}
}

func TestRecordViewNothingInDungeon(t *testing.T) {
	w := newWorld(t)
	w.AutoMap = NewAutoMap(nil)
	w.Party.SetLocation(LocationDungeon)
	RecordView(w, BuildViewport(w, w.X, w.Y))
	if w.AutoMap.HasMap(MapIDSosaria) {
		t.Fatal("recorded")
	}
}

func TestRecordViewOnlyInSight(t *testing.T) {
	w := newWorld(t)
	w.AutoMap = NewAutoMap(nil)
	v := BuildViewport(w, w.X, w.Y)
search:
	for y := range MapSide {
		for x := range MapSide {
			if voids(v) >= 20 {
				break search
			}
			w.X, w.Y = x, y
			v = BuildViewport(w, x, y)
		}
	}
	hidden := voids(v)
	if hidden < 20 {
		t.Fatal("no hidden view found")
	}
	RecordView(w, v)
	for i, c := range v.Cells {
		x, y := w.Constrain(v.OriginX+i%11), w.Constrain(v.OriginY+i/11)
		if w.AutoMap.SquareSeen(MapIDSosaria, x, y) != (c.Base != ShapeVoid) {
			t.Fatal("cell", i)
		}
	}
	if w.AutoMap.MapSeenCount(MapIDSosaria) != 121-hidden {
		t.Fatal("count")
	}
}

func TestRecordViewNothingPastTownEdge(t *testing.T) {
	w := newWorld(t)
	w.AutoMap = NewAutoMap(nil)
	w.Party.SetLocation(LocationTown)
	w.EnterMap(testYew)
	w.X, w.Y = 1, 32
	RecordView(w, BuildViewport(w, w.X, w.Y))
	for y := range MapSide {
		for x := 7; x < MapSide; x++ {
			if w.AutoMap.SquareSeen(testYew, x, y) {
				t.Fatal(x, y)
			}
		}
	}
	if !w.AutoMap.HasMap(testYew) {
		t.Fatal("nothing")
	}
}

func TestPeerGemRecordsNothing(t *testing.T) {
	w := newWorld(t)
	w.AutoMap = NewAutoMap(nil)
	w.Party.SetGems(1)
	io := newFakeIO(w.Resources)
	io.Keys = []string{" "}
	PeerGem(w, io)
	if w.AutoMap.HasMap(MapIDSosaria) {
		t.Fatal("recorded")
	}
}

func findMark(marks []MapMark, f func(MapMark) bool) MapMark {
	if i := slices.IndexFunc(marks, f); i >= 0 {
		return marks[i]
	}
	return MapMark{}
}

func TestMarksChartedFromStart(t *testing.T) {
	w := newWorld(t)
	w.AutoMap = NewAutoMap(nil)
	page := MapPage{ID: MapIDSosaria}
	marks := MarksOn(w, page)
	if marks[0].Kind != MarkParty {
		t.Fatal("party first")
	}
	places := marks[1:]
	charted := []int{400, 402, 403, 404, 405, 406, 407, 409, 410, 411}
	if len(places) != len(charted) {
		t.Fatalf("%d places", len(places))
	}
	for i, id := range charted {
		m := places[i]
		want := "Towne?"
		if i == 0 {
			want = "Castle?"
		}
		if (Point{m.X, m.Y}) != placeAt(w, id) || m.Label != want || m.Detail != "Undiscovered" || !m.Unseen || m.Opens != 0 {
			t.Fatalf("%d: %+v", id, m)
		}
	}
	w.AutoMap.MarkSquares(MapIDSosaria, []Point{placeAt(w, testYew), placeAt(w, 412)})
	now := MarksOn(w, page)
	yew := findMark(now, func(m MapMark) bool { return (Point{m.X, m.Y}) == placeAt(w, testYew) })
	if yew.Label != "Towne?" || yew.Detail != "Not yet entered" || yew.Unseen {
		t.Fatalf("%+v", yew)
	}
	d := findMark(now, func(m MapMark) bool { return m.Kind == MarkDungeon })
	if d.Label != "Dungeon?" || d.Detail != "Not yet entered" {
		t.Fatalf("%+v", d)
	}
}

func TestMarksNamedOnceBeenIn(t *testing.T) {
	w := newWorld(t)
	w.AutoMap = NewAutoMap(nil)
	w.AutoMap.MarkSquares(MapIDSosaria, []Point{placeAt(w, testYew), placeAt(w, 412)})
	w.AutoMap.MarkSquares(testYew, []Point{{1, 32}})
	w.AutoMap.Mark(412, 0, 1, 1)
	marks := MarksOn(w, MapPage{ID: MapIDSosaria})
	if y := findMark(marks, func(m MapMark) bool { return m.Opens == testYew }); y.Label != "Yew" || y.Detail != "A: its map" {
		t.Fatalf("%+v", y)
	}
	if d := findMark(marks, func(m MapMark) bool { return m.Kind == MarkDungeon }); d.Label != "Dungeon Doom" || d.Opens != 0 || d.Detail != "Been in" {
		t.Fatalf("%+v", d)
	}
}

func TestMarksBritain(t *testing.T) {
	w := newWorld(t)
	w.AutoMap = NewAutoMap(nil)
	w.AutoMap.MarkSquares(MapIDSosaria, []Point{placeAt(w, MapIDFirstTown)})
	w.AutoMap.MarkSquares(MapIDFirstTown, []Point{{1, 32}})
	m := findMark(MarksOn(w, MapPage{ID: MapIDSosaria}), func(m MapMark) bool { return m.Opens == MapIDFirstTown })
	if m.Label != "Britain" || PageTitle(w, MapPage{ID: MapIDFirstTown}) != "Britain" {
		t.Fatalf("%+v", m)
	}
}

func TestMarksDawnOnlyWhileItStands(t *testing.T) {
	w := newWorld(t)
	w.AutoMap = NewAutoMap(nil)
	p := placeAt(w, testDawn)
	dawn := func() bool {
		return slices.ContainsFunc(MarksOn(w, MapPage{ID: MapIDSosaria}), func(m MapMark) bool { return m.X == p.X && m.Y == p.Y })
	}
	if dawn() {
		t.Fatal("charted")
	}
	w.AutoMap.MarkSquares(MapIDSosaria, []Point{p})
	w.Surface.Tiles[p.Y*MapSide+p.X] = MapValueGrass
	if dawn() {
		t.Fatal("not standing")
	}
	w.Surface.Tiles[p.Y*MapSide+p.X] = MapValueTown
	if !dawn() {
		t.Fatal("standing")
	}
}

func TestMarksPlaceThePartyIsIn(t *testing.T) {
	w := newWorld(t)
	w.AutoMap = NewAutoMap(nil)
	w.Party.SetLocation(LocationTown)
	w.EnterMap(testYew)
	marks := MarksOn(w, MapPage{ID: MapIDSosaria})
	if m := marks[0]; m.Kind != MarkTowne || m.Label != "Yew" || !m.Party || m.Opens != testYew || m.Detail != "Party inside" {
		t.Fatalf("%+v", m)
	}
	if slices.ContainsFunc(marks, func(m MapMark) bool { return m.Kind == MarkParty }) {
		t.Fatal("party mark")
	}
	if m := MarksOn(w, MapPage{ID: testYew})[0]; m.Kind != MarkParty || m.X != w.X || m.Y != w.Y {
		t.Fatalf("%+v", m)
	}
	if p := MapPages(w); !slices.Equal(p, []MapPage{{ID: testYew}, {ID: MapIDSosaria}}) {
		t.Fatalf("%+v", p)
	}
}

func TestMarksMoongates(t *testing.T) {
	w := newWorld(t)
	w.AutoMap = NewAutoMap(nil)
	w.GatesKnown = []int{0, 5}
	var got [][3]int
	for _, m := range MarksOn(w, MapPage{ID: MapIDSosaria}) {
		if m.Kind == MarkMoongate {
			got = append(got, [3]int{m.X, m.Y, m.Phase})
		}
	}
	misc := w.Resources.Misc
	want := [][3]int{{int(misc.MoonX[0]), int(misc.MoonY[0]), 0}, {int(misc.MoonX[5]), int(misc.MoonY[5]), 5}}
	if !slices.Equal(got, want) {
		t.Fatal(got)
	}
}

func TestMarksShrines(t *testing.T) {
	w := newWorld(t)
	w.AutoMap = NewAutoMap(nil)
	w.AutoMap.MarkSquares(MapIDAmbrosia, []Point{{6, 6}, {55, 27}})
	w.AutoMap.Visit("shrine:6,6")
	var s []MapMark
	for _, m := range MarksOn(w, MapPage{ID: MapIDAmbrosia}) {
		if m.Kind == MarkShrine {
			s = append(s, m)
		}
	}
	if len(s) != 2 || !regexp.MustCompile(`^Shrine of [A-Z][a-z]+$`).MatchString(s[0].Label) || s[1].Label != "Shrine?" {
		t.Fatalf("%+v", s)
	}
	if !slices.ContainsFunc(MapPages(w), func(p MapPage) bool { return p.ID == MapIDAmbrosia }) {
		t.Fatal("no Ambrosia page")
	}
}

func TestStepMark(t *testing.T) {
	mk := func(x, y int) MapMark { return MapMark{Kind: MarkTowne, X: x, Y: y} }
	marks := []MapMark{mk(10, 10), mk(14, 10), mk(10, 30), mk(60, 11)}
	if StepMark(marks, 0, DirRight, MapSide, true) != 1 || StepMark(marks, 0, DirDown, MapSide, true) != 2 ||
		StepMark(marks, 0, DirLeft, MapSide, true) != 3 || StepMark(marks, 0, DirLeft, MapSide, false) != 0 ||
		StepMark(marks, 0, DirUp, MapSide, true) != 0 {
		t.Fatal("step")
	}
}

func TestCoastSegmentsIsland(t *testing.T) {
	coast := CoastSegments(func(x, y int) bool { return x == 1 && y == 1 }, 3, false)
	if len(coast) != 4 {
		t.Fatal(len(coast))
	}
	for _, s := range coast {
		if math.Abs(s.X1-1.5)+math.Abs(s.Y1-1.5) != 0.5 || math.Abs(s.X2-1.5)+math.Abs(s.Y2-1.5) != 0.5 {
			t.Fatal(s)
		}
	}
	if len(CoastSegments(func(int, int) bool { return true }, 4, true)) != 0 {
		t.Fatal("all land")
	}
}
