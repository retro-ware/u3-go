package game

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/retro-ware/u3-go/internal/res"
)

// Shared test helpers: the oracle resources from the TypeScript port's
// public/data/resources.json, and a fake IO that records what the game
// printed and played (tests/helpers.ts).

const resourcesJSON = "../../ref/ultima3/web/public/data/resources.json"

var (
	resOnce   sync.Once
	resCached *res.Resources
	resErr    error
)

// loadTestResources loads the oracle bundle once per run, skipping the test when ref/ is absent.
func loadTestResources(t testing.TB) *res.Resources {
	t.Helper()
	resOnce.Do(func() { resCached, resErr = readBundle(resourcesJSON) })
	if resErr != nil {
		t.Skipf("oracle resources unavailable: %v", resErr)
	}
	return resCached
}

func readBundle(path string) (*res.Resources, error) {
	raw, err := os.ReadFile(filepath.FromSlash(path))
	if err != nil {
		return nil, err
	}
	var bundle map[string]json.RawMessage
	if err := json.Unmarshal(raw, &bundle); err != nil {
		return nil, err
	}
	type entry struct {
		Name string `json:"name"`
		Data string `json:"data"`
	}
	section := func(key string) (map[int][]byte, map[int]string, error) {
		var s map[string]entry
		if err := json.Unmarshal(bundle[key], &s); err != nil {
			return nil, nil, fmt.Errorf("%s: %w", key, err)
		}
		bytes, names := map[int][]byte{}, map[int]string{}
		for k, e := range s {
			id, err := strconv.Atoi(k)
			if err != nil {
				return nil, nil, err
			}
			b, err := base64.StdEncoding.DecodeString(e.Data)
			if err != nil {
				return nil, nil, err
			}
			bytes[id], names[id] = b, e.Name
		}
		return bytes, names, nil
	}
	r := &res.Resources{}
	var misc, party, roster map[int][]byte
	for _, s := range []struct {
		key   string
		dst   *map[int][]byte
		names *map[int]string
	}{
		{"maps", &r.Maps, &r.MapNames}, {"monsters", &r.Monsters, nil}, {"talk", &r.Talk, nil},
		{"combat", &r.Combat, nil}, {"misc", &misc, nil}, {"party", &party, nil}, {"roster", &roster, nil},
	} {
		b, n, err := section(s.key)
		if err != nil {
			return nil, err
		}
		*s.dst = b
		if s.names != nil {
			*s.names = n
		}
	}
	if err := json.Unmarshal(bundle["strings"], &r.Strings); err != nil {
		return nil, err
	}
	m := func(i int) []byte { return misc[BaseRes+i] }
	r.Misc = res.Misc{
		MoonX: m(0)[:8], MoonY: m(0)[8:16], Career: m(1)[:11], WeaponUse: m(2)[:11], ArmourUse: m(3)[:11],
		LocationX: m(4)[:32], LocationY: m(4)[32:64], Experience: m(5)[:16],
	}
	r.DefaultParty, r.DefaultRoster = party[500], roster[500]
	return r, nil
}

// newWorld is a fresh world on Sosaria with the default party, seed 1234 by default.
func newWorld(t testing.TB, seed ...uint32) *World {
	t.Helper()
	s := uint32(1234)
	if len(seed) > 0 {
		s = seed[0]
	}
	w := NewWorld(loadTestResources(t), s)
	w.NewGame()
	return w
}

// errNoKeys is what FakeIO panics with when a test runs out of keys, so a
// test cannot hang (the TypeScript fake threw).
type errNoKeys struct{ output string }

func (e errNoKeys) Error() string { return "FakeIO: no more keys. Output so far:\n" + e.output }

// expectNoKeys runs f and reports whether it stopped by running out of keys.
func expectNoKeys(f func()) (ran bool) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(errNoKeys); !ok {
				panic(r)
			}
			ran = true
		}
	}()
	f()
	return false
}

// FakeIO records output instead of drawing. Keys come from Keys (after the
// scripted Macro); when they run dry WaitKey panics with errNoKeys while
// WaitKeyOrTimeout returns "" (an idle turn). Text input comes from Inputs.
type FakeIO struct {
	res          *res.Resources
	Output       strings.Builder
	Sounds       []string
	Keys         []string
	Macro        []string
	KeyProvider  func() string // supplies keys after Keys runs dry; "" for none
	Inputs       []string
	Redraws      int
	OnRedraw     func() // called on each RedrawMap, for tests that act mid-fight
	Images       []string
	MusicTrack   int
	Flashes      []string
	Paused       int
	PauseAnswer  string
	WordsOffered []string
}

func newFakeIO(r *res.Resources) *FakeIO { return &FakeIO{res: r} }

func (f *FakeIO) out() string { return f.Output.String() }

func (f *FakeIO) next() string {
	if len(f.Macro) > 0 {
		k := f.Macro[0]
		f.Macro = f.Macro[1:]
		return k
	}
	if len(f.Keys) > 0 {
		k := f.Keys[0]
		f.Keys = f.Keys[1:]
		return k
	}
	if f.KeyProvider != nil {
		return f.KeyProvider()
	}
	return ""
}

func (f *FakeIO) Print(text string) { f.Output.WriteString(text) }
func (f *FakeIO) PrintMessage(n int) {
	if m := f.res.Strings["Messages"]; n >= 1 && n <= len(m) {
		f.Output.WriteString(m[n-1])
	} else {
		fmt.Fprintf(&f.Output, "[msg %d]", n)
	}
}
func (f *FakeIO) Prompt() { f.Output.WriteString("\n>") }
func (f *FakeIO) WaitKey() string {
	k := f.next()
	if k == "" {
		panic(errNoKeys{f.out()})
	}
	return k
}
func (f *FakeIO) WaitKeyOrTimeout(int) string          { return f.next() }
func (f *FakeIO) FlushKeys()                           {}
func (f *FakeIO) QueueKeys(keys []string)              { f.Macro = slices.Clone(keys) }
func (f *FakeIO) WaitCommand(CommandScope, int) string { return f.WaitKeyOrTimeout(0) }
func (f *FakeIO) ChooseMember([]int) int {
	key := strings.ToUpper(f.WaitKey())
	if key == KeyEscape {
		f.Output.WriteString(" \n")
		return 0
	}
	f.Output.WriteString(key + "\n")
	return int(key[0]) - '0'
}
func (f *FakeIO) ChooseDirection(allowNone, _ bool) string {
	for {
		key := f.WaitKey()
		if key == KeyEscape {
			return ""
		}
		if slices.Contains([]string{KeyUp, KeyDown, KeyLeft, KeyRight, "1", "2", "3", "4", "6", "7", "8", "9"}, key) {
			return key
		}
		if allowNone && key == KeySpace {
			return key
		}
	}
}
func (f *FakeIO) ChooseOption(options []MenuOption, echo Echo, _ *MenuPlacement) string {
	for {
		key := strings.ToUpper(f.WaitKey())
		if key == KeyEscape {
			return ""
		}
		if !slices.ContainsFunc(options, func(o MenuOption) bool { return o.Key == key }) {
			continue
		}
		if echo != EchoNone {
			f.Output.WriteString(key)
		}
		if echo == EchoLine {
			f.Output.WriteString("\n")
		}
		return key
	}
}
func (f *FakeIO) ShowSettings()     {}
func (f *FakeIO) ShowPause() string { f.Paused++; return f.PauseAnswer }
func (f *FakeIO) ShowJournal()      {}
func (f *FakeIO) ShowMap()          {}
func (f *FakeIO) ChooseFromList(options []MenuOption, place *MenuPlacement) string {
	return f.ChooseOption(options, EchoNone, place)
}
func (f *FakeIO) AllocatePoints(labels []string, _, _, _ int, _ *MenuPlacement) []int {
	t := f.input()
	if t == "" {
		return nil
	}
	var values []int
	for _, v := range strings.Split(t, ",") {
		n, _ := strconv.Atoi(v)
		values = append(values, n)
	}
	if len(values) != len(labels) {
		panic(fmt.Sprintf("FakeIO: expected %d values", len(labels)))
	}
	return values
}
func (f *FakeIO) input() string {
	if len(f.Inputs) == 0 {
		panic("FakeIO: no more inputs")
	}
	t := f.Inputs[0]
	f.Inputs = f.Inputs[1:]
	return t
}
func (f *FakeIO) InputText(_ int, _ bool, words []string) string {
	f.WordsOffered = words
	t := f.input()
	f.Output.WriteString(t)
	return t
}
func (f *FakeIO) InputTextAt(_, _, max int, numbersOnly bool) string {
	return f.InputText(max, numbersOnly, nil)
}
func (f *FakeIO) Sound(name string) { f.Sounds = append(f.Sounds, name) }
func (f *FakeIO) Music(track int)   { f.MusicTrack = track }
func (f *FakeIO) RedrawMap() {
	if f.OnRedraw != nil {
		f.OnRedraw()
	}
	f.Redraws++
}
func (f *FakeIO) FlashTiles() {}
func (f *FakeIO) FlashSpell(kind SpellFlash, _ int, sound string) {
	f.Flashes = append(f.Flashes, string(kind))
	f.Sounds = append(f.Sounds, sound)
}
func (f *FakeIO) FlashMember(int)               {}
func (f *FakeIO) HighlightMember(int, bool)     {}
func (f *FakeIO) UpdateStats(bool)              {}
func (f *FakeIO) ShowWind()                     {}
func (f *FakeIO) ShowMoons()                    {}
func (f *FakeIO) ClearTiles()                   {}
func (f *FakeIO) ShowImage(name string)         { f.Images = append(f.Images, name) }
func (f *FakeIO) ShowMiniMap()                  {}
func (f *FakeIO) ShowMiniDungeon()              {}
func (f *FakeIO) PlayEnding()                   {}
func (f *FakeIO) Pause(int)                     {}
func (f *FakeIO) ShowTitle()                    {}
func (f *FakeIO) ShowGameFrame()                {}
func (f *FakeIO) ClearBottom()                  {}
func (f *FakeIO) TextAt(_, _ int, text string)  { f.Output.WriteString(text) }
func (f *FakeIO) CentreText(_ int, text string) { f.Output.WriteString(text + "\n") }

var _ IO = (*FakeIO)(nil)
