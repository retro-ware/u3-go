package game

import (
	"slices"
	"strings"
	"testing"
)

func dungeonWorld(t *testing.T) (*World, *FakeIO) {
	t.Helper()
	w := newWorld(t, 7)
	w.EnterDungeon(MapIDFirstDungeon)
	w.Party.SetLocation(LocationDungeon)
	w.X, w.Y = 1, 1
	w.Dungeon.Heading = 1
	return w, newFakeIO(w.Resources)
}

func fillLevel(w *World, v byte) {
	for i := range 256 {
		w.Dungeon.Tiles[i] = v
	}
}

func TestDungeonLoadsAtEntrance(t *testing.T) {
	w, _ := dungeonWorld(t)
	if len(w.Dungeon.Tiles) != 2048 {
		t.Fatal(len(w.Dungeon.Tiles))
	}
	if w.GetXYDng(1, 1)&DungeonCellLadderUp != DungeonCellLadderUp {
		t.Fatal("no ladder up at the entrance")
	}
}

func TestDungeonMovement(t *testing.T) {
	w, io := dungeonWorld(t)
	fillLevel(w, DungeonCellWall)
	for x := 1; x < 6; x++ {
		w.PutXYDng(DungeonCellOpen, x, 1)
	}
	w.PutXYDng(DungeonCellLadderUp, 1, 1)
	w.Dungeon.Heading = 1
	Forward(w, io)
	if w.X != 2 {
		t.Fatal("forward", w.X)
	}
	Retreat(w, io)
	if w.X != 1 {
		t.Fatal("retreat", w.X)
	}
	Turn(w, io, true)
	if w.Dungeon.Heading != 2 {
		t.Fatal("turn", w.Dungeon.Heading)
	}
	Forward(w, io)
	if w.Y != 1 || !strings.Contains(io.out(), "INVALID MOVE!") {
		t.Fatal("walked through a wall")
	}
	Turn(w, io, false)
	Turn(w, io, false)
	if w.Dungeon.Heading != 0 {
		t.Fatal("heading", w.Dungeon.Heading)
	}
}

func TestDungeonLadders(t *testing.T) {
	w, io := dungeonWorld(t)
	w.PutXYDng(DungeonCellLadderDown, 1, 1)
	Descend(w, io)
	if w.Dungeon.Level != 1 {
		t.Fatal(w.Dungeon.Level)
	}
	w.PutXYDng(DungeonCellLadderUp, 1, 1)
	Klimb(w, io)
	if w.Dungeon.Level != 0 {
		t.Fatal(w.Dungeon.Level)
	}
	w.PutXYDng(DungeonCellLadderUp, 1, 1)
	Klimb(w, io)
	if !w.Dungeon.Exit {
		t.Fatal("did not leave")
	}
}

func TestDungeonViewRotates(t *testing.T) {
	w, _ := dungeonWorld(t)
	fillLevel(w, DungeonCellOpen)
	w.X, w.Y = 8, 8
	w.PutXYDng(DungeonCellWall, 8, 7)
	w.Dungeon.Heading = 0
	if CellAtLocation(w, 4) != DungeonCellWall {
		t.Fatal("ahead")
	}
	w.Dungeon.Heading = 1
	if CellAtLocation(w, 4) != DungeonCellOpen || CellAtLocation(w, 1) != DungeonCellWall {
		t.Fatal("rotated")
	}
}

func hasOp(ops []DungeonDrawOp, kind DungeonDrawKind, location int) bool {
	return slices.ContainsFunc(ops, func(o DungeonDrawOp) bool { return o.Kind == kind && (location < 0 || o.Location == location) })
}

func TestDungeonViewWallHides(t *testing.T) {
	w, _ := dungeonWorld(t)
	fillLevel(w, DungeonCellOpen)
	w.X, w.Y = 8, 8
	w.Dungeon.Heading = 0
	w.PutXYDng(DungeonCellWall, 8, 6)
	w.PutXYDng(DungeonCellChest, 8, 5)
	ops := BuildDungeonView(w)
	if !hasOp(ops, DrawWall, 12) || hasOp(ops, DrawChest, -1) {
		t.Fatal(ops)
	}
	w.PutXYDng(DungeonCellChest, 8, 7)
	if !hasOp(BuildDungeonView(w), DrawChest, 4) {
		t.Fatal("chest ahead not shown")
	}
}

func TestDungeonViewDoor(t *testing.T) {
	w, _ := dungeonWorld(t)
	fillLevel(w, DungeonCellOpen)
	w.X, w.Y = 8, 8
	w.Dungeon.Heading = 0
	w.PutXYDng(DungeonCellDoor, 8, 7)
	var kinds []DungeonDrawKind
	for _, o := range BuildDungeonView(w) {
		if o.Location == 4 {
			kinds = append(kinds, o.Kind)
		}
	}
	if !slices.Equal(kinds, []DungeonDrawKind{DrawWall, DrawDoor}) {
		t.Fatal(kinds)
	}
}

func TestDungeonLoopClimbsOut(t *testing.T) {
	w, io := dungeonWorld(t)
	fillLevel(w, DungeonCellWall)
	w.PutXYDng(DungeonCellLadderUp, 1, 1)
	w.PutXYDng(DungeonCellOpen, 2, 1)
	io.Keys = []string{KeyUp, KeyDown, "K"}
	RunDungeon(w, io)
	if !w.Dungeon.Exit {
		t.Fatal("still inside")
	}
	for _, s := range []string{"It's dark!", "Advance", "Klimb"} {
		if !strings.Contains(io.out(), s) {
			t.Fatalf("missing %q", s)
		}
	}
}

func TestDungeonTorchAndFountain(t *testing.T) {
	w, io := dungeonWorld(t)
	fillLevel(w, DungeonCellWall)
	w.PutXYDng(DungeonCellLadderUp, 1, 1)
	w.PutXYDng(DungeonCellFountain, 2, 1) // x=2: the damage fountain
	w.Party.SetTorches(1)
	hp := w.Member(0).HitPoints()
	io.Keys = []string{"I", KeyUp, "1", "0", KeyDown, "K"}
	RunDungeon(w, io)
	if w.Dungeon.Torch <= 0 || !slices.Contains(io.Images, "Fountain") || !strings.Contains(io.out(), "ARGH! BLAH! YUK!") {
		t.Fatal(io.out())
	}
	if w.Member(0).HitPoints() != hp-25 || !w.Dungeon.Exit {
		t.Fatal(w.Member(0).HitPoints(), hp)
	}
}

func TestDungeonFountainAsksOnArrivalOnly(t *testing.T) {
	w, io := dungeonWorld(t)
	fillLevel(w, DungeonCellWall)
	w.PutXYDng(DungeonCellLadderUp, 1, 1)
	w.PutXYDng(DungeonCellFountain, 2, 1)
	w.Party.SetTorches(1)
	io.Keys = []string{"I", KeyUp, "0", KeyLeft, KeyRight, " ", KeyDown, "K"}
	RunDungeon(w, io)
	if n := strings.Count(io.out(), "who\nwill drink?"); n != 1 {
		t.Fatal("asked", n)
	}
	if !w.Dungeon.Exit {
		t.Fatal("still inside")
	}
}

func TestDungeonPartyTorches(t *testing.T) {
	w, io := dungeonWorld(t)
	w.Party.SetTorches(3)
	IgniteTorch(w, io)
	if strings.Contains(io.out(), "Whose torch") || w.Party.Torches() != 2 || w.Dungeon.Torch != 255 {
		t.Fatal(io.out(), w.Party.Torches(), w.Dungeon.Torch)
	}
}

func TestDungeonLightShortcut(t *testing.T) {
	w, io := dungeonWorld(t)
	fillLevel(w, DungeonCellWall)
	w.PutXYDng(DungeonCellLadderUp, 1, 1)
	for m := range 4 {
		w.Member(m).SetMana(0)
	}
	io.Keys = []string{LightKey, "K"}
	RunDungeon(w, io)
	if w.Dungeon.Torch != 0 || !strings.Contains(io.out(), "WHAT?") {
		t.Fatal("lit without mana")
	}
	w.Dungeon.Exit = false
	w.Member(3).SetMana(40)
	io.Keys = []string{LightKey, "K"}
	RunDungeon(w, io)
	if w.Dungeon.Torch <= 200 || !strings.Contains(io.out(), "Long light") {
		t.Fatal(w.Dungeon.Torch)
	}
}
