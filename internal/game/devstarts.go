package game

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Development starts: a launch option puts the game in a place, so checking
// a fix or taking a screenshot is a restart instead of a play-through. The
// UI reads them (the TypeScript port took them from the address bar):
//
//	at=world:X,Y       on Sosaria at (X, Y)
//	at=ambrosia[:X,Y]  on Ambrosia, where the whirlpool leaves a party, or at (X, Y)
//	at=place:NAME      a town, castle or dungeon, entered from its square
//	                   (NAME as the map list has it, any case, spaces left
//	                   out; a start of it does: yew, lcb, perinian)
//	at=fight:NAME      a monster of that kind just east of the party, and a step into it

// Slug is a name as the starts match it: lower case, letters and digits only.
func Slug(name string) string {
	var sb strings.Builder
	for _, r := range strings.ToLower(name) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

// PickName is the entry of names that asked names: the same slug, else the
// only one whose slug starts with it (or with "dungeon" and it); "" for none.
func PickName(names []string, asked string) string {
	q := Slug(asked)
	if q == "" {
		return ""
	}
	for _, n := range names {
		if s := Slug(n); s == q || s == "dungeon"+q {
			return n
		}
	}
	var starts []string
	for _, n := range names {
		if s := Slug(n); strings.HasPrefix(s, q) || strings.HasPrefix(s, "dungeon"+q) {
			starts = append(starts, n)
		}
	}
	if len(starts) == 1 {
		return starts[0]
	}
	return ""
}

// TileSetAsked is the tile set a tiles= option asks for, or "".
func TileSetAsked(asked string, sets []string) string { return PickName(sets, asked) }

// AtStart is a parsed at= value.
type AtStart struct {
	Kind string // "world", "ambrosia", "place" or "fight"
	X, Y int    // world and ambrosia
	Name string // place and fight
}

var (
	atXY    = regexp.MustCompile(`^(world|ambrosia):(\d+),(\d+)$`)
	atNamed = regexp.MustCompile(`^(place|fight):(.+)$`)
)

// ParseAt reads an at= value, or nil when it is not one of the starts.
func ParseAt(value string) *AtStart {
	if m := atXY.FindStringSubmatch(value); m != nil {
		x, _ := strconv.Atoi(m[2])
		y, _ := strconv.Atoi(m[3])
		return &AtStart{Kind: m[1], X: x & 63, Y: y & 63}
	}
	if value == "ambrosia" {
		return &AtStart{Kind: "ambrosia", X: 32, Y: 54} // where the whirlpool leaves a party
	}
	if m := atNamed.FindStringSubmatch(value); m != nil {
		name, err := url.PathUnescape(m[2])
		if err != nil {
			name = m[2]
		}
		return &AtStart{Kind: m[1], Name: name}
	}
	return nil
}

type roamer struct {
	name           string
	value, terrain int
}

// roamers is the monsters that roam Sosaria: their map value and the terrain they stand on.
func roamers(w *World) []roamer {
	names := w.Resources.Strings["Tiles"]
	var out []roamer
	for i, shape := range SpawnTypes {
		terrain := MapValueGrass
		if i < len(SpawnTerrain) {
			terrain = SpawnTerrain[i]
		}
		r := roamer{names[shape], shape * 4, terrain}
		if j := slices.IndexFunc(out, func(o roamer) bool { return o.name == r.name }); j >= 0 {
			out[j] = r // a later entry of the same name wins, as Map.set did
		} else {
			out = append(out, r)
		}
	}
	return out
}

func roamerNames(rs []roamer) []string {
	var out []string
	for _, r := range rs {
		out = append(out, r.name)
	}
	return out
}

// StartNames is the names at=place: and at=fight: take.
func StartNames(w *World) (places, fights []string) {
	for _, id := range sortedMapIDs(w) {
		if id >= 400 && id <= 418 {
			places = append(places, w.Resources.MapNames[id])
		}
	}
	return places, roamerNames(roamers(w))
}

func sortedMapIDs(w *World) []int {
	var ids []int
	for id := range w.Resources.MapNames {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

// StartAt puts the party where at asks. It returns the key the game is to
// play first as a player would (the Enter that goes in, the step that starts
// the fight), or "", and an error saying why a start is not understood.
func StartAt(w *World, at AtStart) (string, error) {
	toSosaria := func() {
		if w.Party.Location() != LocationSosaria {
			w.Current = w.Surface
			w.Party.SetLocation(LocationSosaria)
		}
	}
	switch at.Kind {
	case "world":
		toSosaria()
		w.X, w.Y = at.X, at.Y
		w.Party.SetSurfaceX(at.X)
		w.Party.SetSurfaceY(at.Y)
		return "", nil
	case "ambrosia":
		w.EnterMap(MapIDAmbrosia)
		w.Party.SetLocation(LocationAmbrosia)
		w.Party.SetShape(0x7e) // on foot, as the whirlpool leaves a party
		w.X, w.Y = at.X, at.Y
		return "", nil
	case "place":
		places, _ := StartNames(w)
		name := PickName(places, at.Name)
		if name == "" {
			return "", fmt.Errorf("no place %q: %s", at.Name, strings.Join(places, ", "))
		}
		id := 0
		for _, i := range sortedMapIDs(w) {
			if w.Resources.MapNames[i] == name {
				id = i
				break
			}
		}
		index := -1
		for i := range 32 {
			if MapIDForLocation(i) == id {
				index = i
				break
			}
		}
		if index < 0 {
			return "", errors.New("no square for " + name)
		}
		toSosaria()
		misc := w.Resources.Misc
		w.X, w.Y = int(misc.LocationX[index]), int(misc.LocationY[index])
		w.Party.SetSurfaceX(w.X)
		w.Party.SetSurfaceY(w.Y)
		w.Party.SetShape(0x7e)
		// Dawn stands only while both moons are new.
		if index == 8 {
			w.MoonPhase = [2]int{0, 0}
			PlaceMoongates(w)
		}
		return "E", nil
	case "fight":
		all := roamers(w)
		names := roamerNames(all)
		name := PickName(names, at.Name)
		if name == "" {
			return "", fmt.Errorf("no monster %q: %s", at.Name, strings.Join(names, ", "))
		}
		toSosaria()
		r := all[slices.Index(names, name)]
		m := w.Monsters()
		slot := m.FreeSlot()
		if slot < 0 {
			return "", errors.New("no room for a monster")
		}
		x := w.Constrain(w.X + 1)
		m.SetType(slot, r.value)
		m.SetTileUnder(slot, r.terrain)
		m.SetPosition(slot, x, w.Y)
		m.SetHP(slot, 0xc0)
		m.SetVariant(slot, 0)
		w.PutXYVal(r.value, x, w.Y)
		return KeyRight, nil
	}
	return "", errors.New("unknown start " + at.Kind)
}
