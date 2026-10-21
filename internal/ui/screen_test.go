package ui

import (
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/retro-ware/u3-go/internal/game"
	"github.com/retro-ware/u3-go/internal/res"
)

// bareScreen is a screen with no world, for the message area.
func bareScreen(t *testing.T) *Screen {
	data := testData(t)
	g, err := LoadGraphics("Standard", data)
	if err != nil {
		t.Fatal(err)
	}
	return NewScreen(32, g, data, NewScriptKeyboard(nil, nil), NewSoundPlayer(nil, data), NewMusicPlayer(nil, data), nil)
}

func (s *Screen) rows() []string {
	var out []string
	for _, r := range s.textRows {
		if line := strings.TrimSpace(string(r)); line != "" {
			out = append(out, line)
		}
	}
	return out
}

func TestRepeatedTurnsFold(t *testing.T) {
	s := bareScreen(t)
	for range 3 {
		s.Prompt()
		s.Print("North\n")
	}
	s.Prompt()
	s.Print("South\n")
	got := strings.Join(s.rows(), "|")
	if got != "North (x3)|South" {
		t.Fatal(got)
	}
}

func TestPartlyRepeatedTurnUnfolds(t *testing.T) {
	s := bareScreen(t)
	s.Prompt()
	s.Print("Attack\nMissed!\n")
	s.Prompt()
	s.Print("Attack\nHit!\n") // the first line folded, then taken back
	got := strings.Join(s.rows(), "|")
	if got != "Attack|Missed!|Attack|Hit!" {
		t.Fatal(got)
	}
}

// dataScreen is a screen over a new game on the imported data in U3_MAC_DATA (skipped without).
func dataScreen(t *testing.T) (*Screen, *game.World) {
	data := testData(t)
	r, err := res.Load(data)
	if err != nil {
		t.Fatal(err)
	}
	w := game.NewWorld(r, 1)
	w.NewGame()
	g, err := LoadGraphics("Standard", data)
	if err != nil {
		t.Fatal(err)
	}
	s := NewScreen(32, g, data, NewScriptKeyboard(nil, nil), NewSoundPlayer(nil, data), NewMusicPlayer(nil, data), w)
	s.ShowGameFrame()
	return s, w
}

// lit counts the pixels in r that are not black.
func lit(img *image.RGBA, r image.Rectangle) int {
	n := 0
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if c := img.RGBAAt(x, y); c.R|c.G|c.B != 0 {
				n++
			}
		}
	}
	return n
}

func TestDungeonViewPaints(t *testing.T) {
	s, w := dataScreen(t)
	if _, err := game.StartAt(w, game.AtStart{Kind: "place", Name: "Dungeon Doom"}); err != nil {
		t.Fatal(err)
	}
	game.Enter(w, s) // the start's own key, as the field loop would play it
	w.Dungeon.Torch = 100
	s.RedrawMap()
	s.frameLocked(animationTick)
	view := s.cells(1, 1, 22, 22)
	if w.Party.Location() != game.LocationDungeon {
		t.Fatal("not in the dungeon")
	}
	if lit(s.c.RGBA, view) == 0 {
		t.Fatal("the dungeon view is black with a torch lit")
	}
	if dump := os.Getenv("U3_DUMP"); dump != "" {
		s.Snapshot(filepath.Join(dump, "dungeon-test.png"))
	}
}

func TestOverworldViewPaints(t *testing.T) {
	s, _ := dataScreen(t)
	s.RedrawMap()
	s.frameLocked(animationTick)
	if lit(s.c.RGBA, s.cells(1, 1, 22, 22)) == 0 {
		t.Fatal("the viewport is black")
	}
}
