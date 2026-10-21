package ui

import (
	"math"
	"testing"
	"time"

	"github.com/retro-ware/u3-go/internal/game"
)

func TestDisplayStyles(t *testing.T) {
	if c := hexColour(displayStyles["Apple II Mono"].Phosphor); c != rgb(140, 248, 140) {
		t.Fatal(c)
	}
	var mono, light []string
	for _, s := range TileSets {
		if displayOf(s).Phosphor != "" {
			mono = append(mono, s)
		}
		if displayOf(s).LightGround {
			light = append(light, s)
		}
	}
	if len(mono) != 1 || mono[0] != "Apple II Mono" || len(light) != 1 || light[0] != "Macintosh B&W" {
		t.Fatal(mono, light)
	}
	if displayOf("No such set") != (DisplayStyle{}) {
		t.Fatal("unknown set has a style")
	}
	for set := range displayStyles {
		if indexOf(TileSets, set) < 0 {
			t.Error("not shipped:", set)
		}
	}
}

func TestMarkerColour(t *testing.T) {
	for _, c := range []struct {
		set        string
		start, end [3]uint8
	}{
		{"PC VGA", [3]uint8{255, 255, 255}, [3]uint8{128, 128, 128}},
		{"Macintosh B&W", [3]uint8{0, 0, 0}, [3]uint8{127, 127, 127}},
		{"Apple II Mono", [3]uint8{140, 248, 140}, [3]uint8{70, 124, 70}},
	} {
		for i, e := range []float64{0, 1} {
			got := markerColour(displayOf(c.set), e)
			want := [2][3]uint8{c.start, c.end}[i]
			if [3]uint8{got.R, got.G, got.B} != want {
				t.Errorf("%s at %v: %v, want %v", c.set, e, got, want)
			}
		}
	}
}

func TestScanlineBands(t *testing.T) {
	for _, height := range []int{576, 768, 956, 1080, 1440, 2160} {
		bands := scanlineBands(height)
		pitch := int(math.Round(float64(height) / screenLines))
		if want := int(math.Ceil(float64(height-(pitch-pitch/2)) / float64(pitch))); len(bands) != want {
			t.Errorf("%d: %d bands, want %d", height, len(bands), want)
		}
		for i, b := range bands {
			if b[0] != i*pitch+pitch-pitch/2 || b[1] != min(pitch/2, height-b[0]) || b[0]+b[1] > height {
				t.Fatalf("%d: band %d is %v", height, i, b)
			}
		}
	}
	if len(scanlineBands(200)) != 0 || len(scanlineBands(0)) != 0 || len(scanlineBands(384)) != screenLines {
		t.Fatal("small pictures")
	}
	if scanlineLevel <= 0.5 || scanlineLevel >= 0.8 {
		t.Fatal(scanlineLevel)
	}
}

// fakeClock makes a keyboard's clock settable, in milliseconds.
func fakeClock(k *Keyboard) func(ms int) {
	var now int
	k.now = func() time.Duration { return time.Duration(now) * time.Millisecond }
	return func(ms int) { now = ms }
}

func TestPauseHolds(t *testing.T) {
	k := NewKeyboard()
	at := fakeClock(k)
	var paused []string
	var resumed []time.Duration
	k.onPause = func(r string) { paused = append(paused, r) }
	k.onResume = func(d time.Duration) { resumed = append(resumed, d) }
	k.Pause(holdFocus) // the window went away
	k.Pause(holdMenu)  // and the menu it asked for opened
	if len(paused) != 1 || paused[0] != holdFocus || !k.Paused() {
		t.Fatal(paused)
	}
	at(5000)
	k.Resume(holdFocus) // focus came back: the menu is still up
	if !k.Paused() || len(resumed) != 0 {
		t.Fatal("resumed under the menu")
	}
	k.Resume(holdMenu)
	if k.Paused() || len(resumed) != 1 || resumed[0] != 5*time.Second {
		t.Fatal(resumed)
	}
	k.Resume(holdMenu) // let go twice
	k.Pause(holdMenu)
	k.Resume(holdFocus) // never held
	if !k.Paused() {
		t.Fatal("a hold not held let go")
	}
}

func TestPauseStopsTheClock(t *testing.T) {
	k := NewKeyboard()
	at := fakeClock(k)
	at(1000)
	start := k.ActiveTime()
	at(1200)
	k.Pause(holdMenu)
	at(9000)
	if d := k.ActiveTime() - start; d != 200 {
		t.Fatal(d)
	}
	k.Resume(holdMenu)
	if d := k.ActiveTime() - start; d != 200 {
		t.Fatal(d)
	}
}

func TestPauseButtons(t *testing.T) {
	press := func(p PadState) []string {
		k := NewKeyboard()
		var r GamepadReader
		r.Poll(0, map[int]PadState{0: p}, k, nil)
		var out []string
		for {
			key, ok := k.takeQueued()
			if !ok {
				return out
			}
			out = append(out, key)
		}
	}
	if got := press(PadState{Pause: true}); len(got) != 1 || got[0] != game.KeyPause {
		t.Fatal(got)
	}
	if got := press(PadState{A: true}); len(got) != 1 || got[0] != game.KeyA {
		t.Fatal(got)
	}
	m := layoutMenu("Paused", []string{"Resume", "Input", "Back"}, 1)
	options := []game.MenuOption{{Key: "E", Label: "Resume"}, {Key: "I", Label: "Input"}, {Key: "B", Label: "Back"}}
	if act, _ := applyMenuKey(m, options, options, game.KeyPause); act != actBack {
		t.Fatal(act)
	}
}
