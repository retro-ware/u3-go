package game

import (
	"strings"
	"testing"
)

// memoryStore is an AutoMapStore in memory.
type memoryStore struct {
	text string
	set  bool
}

func (m *memoryStore) Read() (string, bool) { return m.text, m.set }
func (m *memoryStore) Write(t string)       { m.text, m.set = t, true }

func TestAutoMapMarksThe3x3Wrapping(t *testing.T) {
	m := NewAutoMap(nil)
	m.Mark(1, 0, 0, 0)
	if m.SeenCount(1, 0) != 9 || !m.IsSeen(1, 0, 15, 15) || !m.IsSeen(1, 0, 1, 1) || m.IsSeen(1, 0, 2, 2) {
		t.Fatal("3x3 mark wrong")
	}
	if m.IsSeen(1, 1, 0, 0) || m.IsSeen(2, 0, 0, 0) {
		t.Fatal("another level or dungeon marked")
	}
}

func TestAutoMapStoreRoundTrip(t *testing.T) {
	store := &memoryStore{}
	m := NewAutoMap(store)
	m.Mark(3, 2, 8, 8)
	if !strings.Contains(store.text, `"3:2"`) {
		t.Fatalf("store %q", store.text)
	}
	again := NewAutoMap(store)
	if !again.IsSeen(3, 2, 9, 7) {
		t.Fatal("not restored")
	}
	again.Clear()
	if NewAutoMap(store).SeenCount(3, 2) != 0 {
		t.Fatal("not cleared")
	}
}

func TestNextMapMode(t *testing.T) {
	if NextMapMode(MapModeOff) != MapModeSmall || NextMapMode(MapModeSmall) != MapModeFull || NextMapMode(MapModeFull) != MapModeOff {
		t.Fatal("cycle wrong")
	}
}

func automapDungeonWorld(t *testing.T) (*World, *FakeIO) {
	w := newWorld(t, 7)
	w.EnterDungeon(MapIDFirstDungeon)
	w.Party.SetLocation(LocationDungeon)
	w.X, w.Y = 1, 1
	w.Dungeon.Heading = 1
	for i := range 256 {
		w.Dungeon.Tiles[i] = DungeonCellWall
	}
	w.PutXYDng(DungeonCellLadderUp, 1, 1)
	w.PutXYDng(DungeonCellOpen, 2, 1)
	w.PutXYDng(DungeonCellOpen, 3, 1)
	return w, newFakeIO(w.Resources)
}

func TestAutoMapDarkRecordsNothing(t *testing.T) {
	w, io := automapDungeonWorld(t)
	io.Keys = []string{KeyUp, KeyDown, "K"}
	RunDungeon(w, io)
	if n := w.AutoMap.SeenCount(w.Current.ID, 0); n != 0 {
		t.Fatalf("seen %d", n)
	}
	w.X, w.Y = 2, 1
	if !CellVisible(w, 3, 2) || CellVisible(w, 4, 1) {
		t.Fatal("dark 3x3 wrong")
	}
}

func TestAutoMapTorchRecordsSteps(t *testing.T) {
	w, io := automapDungeonWorld(t)
	w.Party.SetTorches(1)
	io.Keys = []string{"I", KeyUp, KeyUp, KeyDown, KeyDown, "K"}
	RunDungeon(w, io)
	id := w.Current.ID
	// Stood at x=1, 2 and 3 on row 1: columns 0..4 of rows 0..2.
	if n := w.AutoMap.SeenCount(id, 0); n != 15 {
		t.Fatalf("seen %d", n)
	}
	if !w.AutoMap.IsSeen(id, 0, 4, 2) || w.AutoMap.IsSeen(id, 0, 5, 1) {
		t.Fatal("wrong cells")
	}
	w.Dungeon.Torch = 0
	w.X = 12
	if !CellVisible(w, 4, 2) || CellVisible(w, 8, 8) {
		t.Fatal("remembered cells wrong")
	}
}

func TestToggleMapNamesTheView(t *testing.T) {
	w, io := automapDungeonWorld(t)
	told := 0
	w.OnMapModeChange = func() { told++ }
	ToggleMap(w, io)
	if w.MapMode != MapModeSmall || !strings.Contains(io.out(), "View:\n1st person + map") {
		t.Fatal("small")
	}
	ToggleMap(w, io)
	if w.MapMode != MapModeFull || !strings.Contains(io.out(), "View:\nTop down") {
		t.Fatal("full")
	}
	ToggleMap(w, io)
	if !strings.Contains(io.out(), "View:\nFirst person") || told != 3 {
		t.Fatal("off")
	}
}

func TestStepCompass(t *testing.T) {
	w := newWorld(t, 7)
	w.EnterDungeon(MapIDFirstDungeon)
	w.Party.SetLocation(LocationDungeon)
	for i := range 256 {
		w.Dungeon.Tiles[i] = DungeonCellWall
	}
	w.PutXYDng(DungeonCellOpen, 5, 5)
	w.PutXYDng(DungeonCellOpen, 5, 4)
	w.PutXYDng(DungeonCellOpen, 6, 4)
	w.X, w.Y = 5, 5
	w.Dungeon.Heading = 1
	w.MapMode = MapModeFull
	io := newFakeIO(w.Resources)
	check := func(x, y, h int) {
		t.Helper()
		if w.X != x || w.Y != y || w.Dungeon.Heading != h {
			t.Fatalf("at %d,%d heading %d, want %d,%d %d", w.X, w.Y, w.Dungeon.Heading, x, y, h)
		}
	}
	StepCompass(w, io, 0)
	check(5, 4, 0)
	StepCompass(w, io, 1)
	check(6, 4, 1)
	StepCompass(w, io, 3)
	StepCompass(w, io, 3) // a wall; the party still turns to face it
	check(5, 4, 3)
	if !strings.Contains(io.out(), "North") || !strings.Contains(io.out(), "West") {
		t.Fatal(io.out())
	}
}
