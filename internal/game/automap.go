package game

import (
	"encoding/base64"
	"encoding/json"
	"slices"
	"strconv"
	"strings"
)

// What the party has seen, for the maps (the TypeScript port's; the original
// had graph paper): the dungeon auto-map (cells of each level the party has
// stood next to with a lit torch), the squares of Sosaria, Ambrosia and each
// town and castle in the party's line of sight (RecordView), and the shrines
// visited. It is only a mask; the maps are drawn from the live level data
// under it. It is saved with the game as ToText.

// AutoMapStore persists the mask: a file, or nothing in tests.
type AutoMapStore interface {
	Read() (string, bool)
	Write(text string)
}

// MapMode is the dungeon's view (L): first person, first person with a 5x5
// piece of the auto-map, or the level top down.
type MapMode string

const (
	MapModeOff   MapMode = "off"
	MapModeSmall MapMode = "small"
	MapModeFull  MapMode = "full"
)

// MapModes lists the views in the order L cycles them.
var MapModes = []MapMode{MapModeOff, MapModeSmall, MapModeFull}

// ViewNames is each view as the player is told it (at most 16 columns).
var ViewNames = map[MapMode]string{MapModeOff: "First person", MapModeSmall: "1st person + map", MapModeFull: "Top down"}

// MapSide is the side of Sosaria, Ambrosia, the towns and the castles.
const MapSide = 64

const (
	levelCells = 256
	mapCells   = MapSide * MapSide
	mapPrefix  = "map:"
	visitsKey  = "visits"
)

// packFlags packs a map's flags eight to a byte, as base64.
func packFlags(flags []byte) string {
	b := make([]byte, len(flags)/8)
	for i, v := range flags {
		if v != 0 {
			b[i>>3] |= 1 << (i & 7)
		}
	}
	return base64.StdEncoding.EncodeToString(b)
}

func unpackFlags(text string, cells int) []byte {
	raw, err := base64.StdEncoding.DecodeString(text)
	if err != nil || len(raw)*8 != cells {
		return nil
	}
	flags := make([]byte, cells)
	for i := range flags {
		flags[i] = raw[i>>3] >> (i & 7) & 1
	}
	return flags
}

// AutoMap is the seen mask.
type AutoMap struct {
	seen    map[string][]byte // "dungeonId:level" -> 256 flags
	squares map[int][]byte    // a 64-square map's id -> 4096 flags
	visits  map[string]bool   // what else the party has been in ("shrine:x,y")
	store   AutoMapStore
}

// NewAutoMap is a mask loaded from store, which may be nil.
func NewAutoMap(store AutoMapStore) *AutoMap {
	a := &AutoMap{seen: map[string][]byte{}, squares: map[int][]byte{}, visits: map[string]bool{}, store: store}
	if store != nil {
		if text, ok := store.Read(); ok {
			a.parse(text)
		}
	}
	return a
}

// ToText is the mask for the save: JSON, each level's flags as 256 '0'/'1'
// characters ("id:level"), each map's packed ("map:id"), the visits joined
// by "|" ("visits").
func (a *AutoMap) ToText() string {
	data := map[string]string{}
	for k, mask := range a.seen {
		var sb strings.Builder
		for _, v := range mask {
			sb.WriteByte('0' + v)
		}
		data[k] = sb.String()
	}
	for id, flags := range a.squares {
		data[mapPrefix+strconv.Itoa(id)] = packFlags(flags)
	}
	if len(a.visits) > 0 {
		var v []string
		for k := range a.visits {
			v = append(v, k)
		}
		slices.Sort(v)
		data[visitsKey] = strings.Join(v, "|")
	}
	out, _ := json.Marshal(data)
	return string(out)
}

// FromText replaces the mask with one from ToText; bad text is a blank map.
func (a *AutoMap) FromText(text string) {
	clear(a.seen)
	clear(a.squares)
	clear(a.visits)
	a.parse(text)
	a.save()
}

func (a *AutoMap) parse(raw string) {
	var data map[string]any
	if raw == "" || json.Unmarshal([]byte(raw), &data) != nil {
		return
	}
	for key, v := range data {
		flags, ok := v.(string)
		if !ok {
			continue
		}
		switch {
		case key == visitsKey:
			for _, s := range strings.Split(flags, "|") {
				if s != "" {
					a.visits[s] = true
				}
			}
		case strings.HasPrefix(key, mapPrefix):
			id, err := strconv.Atoi(key[len(mapPrefix):])
			if u := unpackFlags(flags, mapCells); err == nil && u != nil {
				a.squares[id] = u
			}
		case len(flags) == levelCells:
			mask := make([]byte, levelCells)
			for i := range mask {
				mask[i] = b2u(flags[i] == '1')
			}
			a.seen[key] = mask
		}
	}
}

func (a *AutoMap) save() {
	if a.store != nil {
		a.store.Write(a.ToText())
	}
}

func levelKey(dungeon, level int) string { return strconv.Itoa(dungeon) + ":" + strconv.Itoa(level) }

// Mark records the 3x3 around (x, y) on the level, wrapping as the dungeon does.
func (a *AutoMap) Mark(dungeon, level, x, y int) {
	key := levelKey(dungeon, level)
	mask := a.seen[key]
	if mask == nil {
		mask = make([]byte, levelCells)
		a.seen[key] = mask
	}
	changed := false
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			i := ((y+dy)&0x0f)*16 + (x+dx)&0x0f
			if mask[i] == 0 {
				changed = true
			}
			mask[i] = 1
		}
	}
	if changed {
		a.save()
	}
}

// IsSeen says whether a dungeon cell has been seen.
func (a *AutoMap) IsSeen(dungeon, level, x, y int) bool {
	mask := a.seen[levelKey(dungeon, level)]
	return mask != nil && mask[(y&0x0f)*16+x&0x0f] == 1
}

func count(b []byte) int {
	n := 0
	for _, v := range b {
		n += int(v)
	}
	return n
}

// SeenCount is how many cells of the level have been seen.
func (a *AutoMap) SeenCount(dungeon, level int) int { return count(a.seen[levelKey(dungeon, level)]) }

// HasDungeon says whether any level of a dungeon has been seen.
func (a *AutoMap) HasDungeon(dungeon int) bool {
	prefix := strconv.Itoa(dungeon) + ":"
	for k, mask := range a.seen {
		if strings.HasPrefix(k, prefix) && slices.Contains(mask, 1) {
			return true
		}
	}
	return false
}

// MarkSquares records squares of a surface map, town or castle as seen.
func (a *AutoMap) MarkSquares(mapID int, squares []Point) {
	flags := a.squares[mapID]
	if flags == nil {
		flags = make([]byte, mapCells)
		a.squares[mapID] = flags
	}
	changed := false
	for _, s := range squares {
		i := (s.Y&(MapSide-1))*MapSide + s.X&(MapSide-1)
		if flags[i] == 0 {
			changed = true
		}
		flags[i] = 1
	}
	if changed {
		a.save()
	}
}

// SquareSeen says whether a map square has been seen.
func (a *AutoMap) SquareSeen(mapID, x, y int) bool {
	flags := a.squares[mapID]
	return flags != nil && flags[(y&(MapSide-1))*MapSide+x&(MapSide-1)] == 1
}

// MapSeenCount is how many squares of a map have been seen.
func (a *AutoMap) MapSeenCount(mapID int) int { return count(a.squares[mapID]) }

// HasMap says whether any square of a map has been seen.
func (a *AutoMap) HasMap(mapID int) bool { return a.MapSeenCount(mapID) > 0 }

// Visit records a visit ("shrine:x,y").
func (a *AutoMap) Visit(key string) {
	if a.visits[key] {
		return
	}
	a.visits[key] = true
	a.save()
}

// Visited says whether the party has been in key.
func (a *AutoMap) Visited(key string) bool { return a.visits[key] }

// Clear forgets everything: a new game starts with blank maps.
func (a *AutoMap) Clear() {
	clear(a.seen)
	clear(a.squares)
	clear(a.visits)
	a.save()
}

// CellVisible says whether the auto-map may show cell (x, y) of the current
// level: seen before, or in the 3x3 round the party while it is dark (shown
// but never recorded, so torches keep their worth).
func CellVisible(w *World, x, y int) bool {
	if w.AutoMap.IsSeen(w.Current.ID, w.Dungeon.Level, x, y) {
		return true
	}
	if w.Dungeon.Torch > 0 {
		return false
	}
	near := func(d int) int {
		d &= 0x0f
		if d == 15 {
			return 1
		}
		return d
	}
	return near(x-w.X) <= 1 && near(y-w.Y) <= 1
}

// NextMapMode cycles off -> small -> full -> off.
func NextMapMode(mode MapMode) MapMode {
	return MapModes[(slices.Index(MapModes, mode)+1)%len(MapModes)]
}

// RecordView records what the party can see of the 64-square map it is on:
// every square of the view its line of sight leaves open. Nothing is recorded
// in a dungeon or a fight.
func RecordView(w *World, view *Viewport) {
	where := w.Party.Location()
	if where == LocationDungeon || where == LocationCombat || w.Current.Size != MapSide {
		return
	}
	// A town's view near its edge shows the far side (the original's wrap), which is not in sight.
	wraps := where == LocationSosaria || where == LocationAmbrosia
	var squares []Point
	for i, cell := range view.Cells {
		if cell.Base == ShapeVoid {
			continue
		}
		x, y := view.OriginX+i%ViewSize, view.OriginY+i/ViewSize
		if !wraps && (x < 0 || y < 0 || x >= MapSide || y >= MapSide) {
			continue
		}
		squares = append(squares, Point{w.Constrain(x), w.Constrain(y)})
	}
	w.AutoMap.MarkSquares(w.Current.ID, squares)
}
