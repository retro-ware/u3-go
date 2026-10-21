package ui

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/retro-ware/u3-go/internal/game"
)

func TestControllerStandIns(t *testing.T) {
	cases := map[string]string{"w": game.KeyUp, "a": game.KeyLeft, "s": game.KeyDown, "d": game.KeyRight,
		game.KeyEnter: game.KeyA, "z": game.KeyA, "Z": game.KeyA, "x": game.KeyB, "X": game.KeyB, "b": game.KeyB, "B": game.KeyB,
		"c": game.KeyX, "C": game.KeyX, "v": game.KeyY, "V": game.KeyY, "y": game.KeyY, "Y": game.KeyY,
		game.KeyEscape: game.KeyEscape, game.KeyUp: game.KeyUp, "q": "q", " ": " "}
	for in, want := range cases {
		if got := controllerKeyFor(in); got != want {
			t.Errorf("controllerKeyFor(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCommandMenusLeaveOutButtons(t *testing.T) {
	for scope, menu := range CommandMenus {
		for _, o := range menu {
			if o.Key == " " || o.Key == game.KeyEscape || o.Key == "Q" || o.Key == "H" {
				t.Errorf("%s menu has %q", scope, o.Key)
			}
		}
	}
}

func TestGamepadReader(t *testing.T) {
	k := NewKeyboard()
	var r GamepadReader
	var pushed []string
	k.onInput = func() { pushed = append(pushed, "") }
	poll := func(ms int, p PadState, waiting bool) {
		k.mu.Lock()
		k.waiting = waiting
		k.queue = nil
		k.mu.Unlock()
		r.Poll(time.Duration(ms)*time.Millisecond, map[int]PadState{0: p}, k, nil)
	}
	for ms := 0; ms < 2000; ms += 16 {
		poll(ms, PadState{A: true}, true)
	}
	if len(pushed) != 1 || k.LastSource() != sourceGamepad {
		t.Fatalf("held A pushed %d keys", len(pushed))
	}
	pushed = nil
	first, rate := int(gamepadRepeatFirst/time.Millisecond), int(gamepadRepeat/time.Millisecond)
	right := PadState{Right: true}
	poll(0, right, true)
	poll(first-1, right, true)
	if len(pushed) != 1 {
		t.Fatal("repeated too soon")
	}
	poll(first, right, true)
	poll(first+rate-1, right, true)
	if len(pushed) != 2 {
		t.Fatalf("want 2 after first repeat, got %d", len(pushed))
	}
	poll(first+2*rate, right, false) // busy: nothing piles up
	if len(pushed) != 2 {
		t.Fatal("repeated while busy")
	}
	poll(first+2*rate, right, true)
	poll(first+2*rate, PadState{}, true)
	poll(first+2*rate, right, true) // released and pressed again: at once
	if len(pushed) != 4 {
		t.Fatalf("want 4, got %d", len(pushed))
	}
}

func TestMenuKeys(t *testing.T) {
	options := []game.MenuOption{{Key: "J", Label: "Journey onward"}, {Key: "X", Label: "Hidden", Hidden: true},
		{Key: "O", Label: "Organize a party"}, {Key: "S", Label: "Settings", Disabled: true}}
	var visible []game.MenuOption
	var labels []string
	for _, o := range options {
		if !o.Hidden {
			visible = append(visible, o)
			labels = append(labels, o.Label)
		}
	}
	m := layoutMenu("Options", labels, 1)
	check := func(key string, act, index int) {
		t.Helper()
		if a, i := applyMenuKey(m, options, visible, key); a != act || (act == actChoose && i != index) {
			t.Errorf("key %q: got %d,%d want %d,%d", key, a, i, act, index)
		}
	}
	check(game.KeyDown, actMove, 0)
	check(game.KeyLeft, actNone, 0)
	check(game.KeyUp, actMove, 0)
	check(game.KeyUp, actMove, 0)
	if m.Cursor != 2 {
		t.Fatalf("cursor %d", m.Cursor)
	}
	m.Cursor = 1
	check(game.KeyA, actChoose, 2)
	check(game.KeyEnter, actChoose, 2)
	check("o", actChoose, 2)
	check("X", actChoose, 1)
	check("S", actRefused, 0)
	m.Cursor = 2
	check(game.KeyA, actRefused, 0)
	check("q", actNone, 0)
	check(game.KeyB, actBack, 0)
	check(game.KeyEscape, actBack, 0)
	if a, _ := applyMenuKey(layoutMenu("Empty", nil, 1), nil, nil, game.KeyA); a != actBack {
		t.Error("A on an empty menu should back out")
	}
	if menuSounds.move != game.SoundStep || menuSounds.choose != game.SoundAttack || menuSounds.back != "Swish1" {
		t.Error("menu sounds")
	}
}

func TestOnScreenKeyboard(t *testing.T) {
	m := layoutKeyboard("Type")
	columns := len(onScreenKeyboard[0])
	if (m.Width-3)/columns != 2 {
		t.Fatalf("column width %d", (m.Width-3)/columns)
	}
	press := func(from string, moves ...string) string {
		m.Cursor = slices.Index(oskKeys, from)
		for _, k := range moves {
			if !moveCursor(m, k) {
				t.Fatal("not a move")
			}
		}
		return oskKeys[m.Cursor]
	}
	cases := []struct{ from, move, want string }{
		{"9", game.KeyDown, oskDone}, {"0", game.KeyDown, oskDelete}, {"3", game.KeyDown, oskDelete},
		{"-", game.KeyDown, oskDone}, {oskDone, game.KeyUp, "-"}, {oskDelete, game.KeyUp, "0"},
		{oskDelete, game.KeyRight, oskDone}, {oskDone, game.KeyRight, oskDelete}, {oskDone, game.KeyLeft, oskDelete},
		{oskDone, game.KeyDown, "M"}, {oskDelete, game.KeyDown, "A"}, {"M", game.KeyUp, oskDone},
		{"A", game.KeyUp, oskDelete}, {"A", game.KeyDown, "N"}, {"A", game.KeyLeft, "M"},
	}
	for _, c := range cases {
		if got := press(c.from, c.move); got != c.want {
			t.Errorf("%q then %q: %q, want %q", c.from, c.move, got, c.want)
		}
	}
	for _, k := range oskKeys {
		if k[0] < 0x20 || int(k[0])-0x20 >= fontGlyphs {
			t.Errorf("key %q outside the font", k)
		}
	}
	grid := layoutMenu("Type", oskKeys, columns)
	if grid.Width <= 22 || grid.X != (40-grid.Width)/2 {
		t.Errorf("grid at %d width %d", grid.X, grid.Width)
	}
	if list := layoutMenu("Options", []string{"Journey onward", "Settings"}, 1); list.X+list.Width > 23 {
		t.Error("list off the map")
	}
}

func TestWrapText(t *testing.T) {
	got := wrapText("the quick brown fox jumps over the lazy dog", 10, 3)
	want := []string{"the quick", "brown fox", "jumps over"}
	if !slices.Equal(got, want) {
		t.Errorf("%q", got)
	}
	if got := wrapText(strings.Repeat("x", 25), 10, 5); !slices.Equal(got, []string{"xxxxxxxxxx", "xxxxxxxxxx", "xxxxx"}) {
		t.Errorf("%q", got)
	}
}

func TestKeyboardQueue(t *testing.T) {
	var now time.Duration
	at := func(ms int) { now = time.Duration(ms) * time.Millisecond }
	k := NewKeyboard()
	k.now = func() time.Duration { return now }
	k.Push("a", "")
	at(100)
	k.Push("b", "")
	at(200)
	if a, _ := k.Next(0); a != "a" {
		t.Fatal(a)
	}
	if b, _ := k.Next(0); b != "b" {
		t.Fatal(b)
	}
	at(0)
	k.Push("x", "")
	k.Push("y", "")
	at(staleKeyMs + 50)
	k.Push("z", "")
	at(staleKeyMs + 100)
	if z, _ := k.Next(0); z != "z" || k.Waiting() {
		t.Fatal("stale keys kept")
	}
	at(1000)
	start := k.ActiveTime()
	at(1400)
	k.Pause(holdFocus)
	at(9000)
	if d := k.ActiveTime() - start; d != 400 {
		t.Errorf("paused clock moved: %d", d)
	}
	k.Resume(holdFocus)
	at(9100)
	if d := k.ActiveTime() - start; d != 500 {
		t.Errorf("clock %d", d)
	}
	k.Push(game.KeyUp, sourceGamepad)
	k.Push(game.KeyPause, "")
	if k.LastSource() != sourceGamepad {
		t.Error("game's own key changed the source")
	}
}

func TestKeyboardHandsToWaiter(t *testing.T) {
	k := NewKeyboard()
	got := make(chan string)
	go func() { key, _ := k.Next(0); got <- key }()
	for !k.Waiting() {
		time.Sleep(time.Millisecond)
	}
	k.Push("q", "")
	if key := <-got; key != "q" {
		t.Fatal(key)
	}
	if key, ok := k.Next(20); ok || key != "" {
		t.Fatal("timeout returned a key")
	}
}

func TestParseScript(t *testing.T) {
	got := ParseScript("J{enter}a{wait}{bogus")
	want := []string{"J", "\r", "a", scriptWait, "{", "b", "o", "g", "u", "s"}
	if !slices.Equal(got, want) {
		t.Errorf("%q", got)
	}
}
