package game

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

// Saving and restoring a game. The original wrote the party, roster and
// "current Sosaria" back into its resource file; here the same data becomes
// JSON (binary fields base64), the TypeScript port's format, kept as a file.
// As in the original, a game saved in a town resumes on Sosaria at the
// square the party entered from: towns are never persisted.

// SaveVersion: 2 pools gold and food in the party record, 3 weapons and
// armour, 4 gems, keys, powders and torches, 5 adds the journal, 6 the
// moongates come out of. Older saves are migrated on load.
const SaveVersion = 6

// SaveKey is the save's name in a SaveStore.
const SaveKey = "ultima3.save"

// SaveData is a saved game.
type SaveData struct {
	Version         int           `json:"version"`
	Party           string        `json:"party"`
	Roster          string        `json:"roster"`
	SurfaceTiles    string        `json:"surfaceTiles"`
	SurfaceMonsters string        `json:"surfaceMonsters"`
	Whirlpool       saveWhirlpool `json:"whirlpool"`
	X               int           `json:"x"`
	Y               int           `json:"y"`
	MoonPhase       [2]int        `json:"moonPhase"`
	MoonTimer       [2]int        `json:"moonTimer"`
	WindDirection   int           `json:"windDirection"`
	// Journal is the quest journal (version 5 on), raw so restore can check it field by field.
	Journal    json.RawMessage `json:"journal,omitempty"`
	Moongates  []int           `json:"moongates,omitempty"`  // version 6 on
	TipsNudged *bool           `json:"tipsNudged,omitempty"` // absent: from before Tips, so it has
	Automap    *string         `json:"automap,omitempty"`    // what the party has seen (AutoMap.ToText)
}

type saveWhirlpool struct {
	X  int `json:"x"`
	Y  int `json:"y"`
	DX int `json:"dx"`
	DY int `json:"dy"`
}

// restoreJournal is a journal from a save, checked field by field; anything missing or odd starts empty.
func restoreJournal(raw json.RawMessage) JournalState {
	out := EmptyJournal()
	var r map[string]any
	if json.Unmarshal(raw, &r) != nil || r == nil {
		return out
	}
	for _, f := range JournalFlags {
		*out.Flag(f) = r[f] == true
	}
	for name, dst := range map[string]*[]string{"clues": &out.Clues, "words": &out.Words, "goals": &out.Goals, "seen": &out.Seen} {
		if list, ok := r[name].([]any); ok {
			*dst = []string{}
			for _, s := range list {
				if s, ok := s.(string); ok {
					*dst = append(*dst, s)
				}
			}
		}
	}
	return out
}

var b64 = base64.StdEncoding

// Serialize captures the world. The position is the surface position even in a town.
func Serialize(w *World) SaveData {
	x, y := w.ReturnX, w.ReturnY
	if w.Party.Location() == LocationSosaria {
		x, y = w.X, w.Y
	}
	journal, _ := json.Marshal(w.Journal)
	nudged := w.TipsNudged
	automap := w.AutoMap.ToText()
	return SaveData{
		Version:         SaveVersion,
		Party:           b64.EncodeToString(w.Party.Bytes),
		Roster:          b64.EncodeToString(w.Roster.Bytes),
		SurfaceTiles:    b64.EncodeToString(w.Surface.Tiles),
		SurfaceMonsters: b64.EncodeToString(w.Surface.Monsters.Bytes),
		Whirlpool:       saveWhirlpool(w.Whirlpool),
		X:               x,
		Y:               y,
		MoonPhase:       w.MoonPhase,
		MoonTimer:       w.MoonTimer,
		WindDirection:   w.WindDirection,
		Journal:         journal,
		Moongates:       append([]int{}, w.GatesKnown...),
		TipsNudged:      &nudged,
		Automap:         &automap,
	}
}

// Restore restores a world from data; false, leaving the world untouched, if the data is unusable.
func Restore(w *World, data SaveData) bool {
	if data.Version < 1 || data.Version > SaveVersion {
		return false
	}
	party, err1 := b64.DecodeString(data.Party)
	roster, err2 := b64.DecodeString(data.Roster)
	tiles, err3 := b64.DecodeString(data.SurfaceTiles)
	monsters, err4 := b64.DecodeString(data.SurfaceMonsters)
	if errors.Join(err1, err2, err3, err4) != nil || len(party) != 64 || len(roster) != 1280 || len(monsters) != 256 {
		return false
	}
	surface := w.LoadMapState(MapIDSosaria)
	if len(tiles) != len(surface.Tiles) {
		return false
	}
	copy(w.Party.Bytes, party)
	copy(w.Roster.Bytes, roster)
	copy(surface.Tiles, tiles)
	BlockExodusApproach(surface, w.DiagonalMoves) // the save may have been made with the other setting
	copy(surface.Monsters.Bytes, monsters)
	w.Surface, w.Current = surface, surface
	w.Whirlpool = Whirlpool(data.Whirlpool)
	w.Party.SetLocation(LocationSosaria)
	w.X, w.ReturnX = data.X, data.X
	w.Y, w.ReturnY = data.Y, data.Y
	w.Party.SetSurfaceX(data.X)
	w.Party.SetSurfaceY(data.Y)
	w.MoonPhase, w.MoonTimer = data.MoonPhase, data.MoonTimer
	w.WindDirection = data.WindDirection
	if data.Version < 2 {
		w.PoolPurses() // members' purses become the party's
	}
	if data.Version < 3 {
		w.PoolGear() // members' bags become the party's
	}
	if data.Version < 4 {
		w.PoolSupplies() // and their gems, keys, powders and torches
	}
	w.Journal = restoreJournal(data.Journal)
	// A journal from before words and goals were recorded has them worked out.
	var j map[string]any
	if json.Unmarshal(data.Journal, &j) == nil && j != nil {
		if _, ok := j["words"].([]any); !ok {
			UpgradeJournal(w)
		}
	}
	w.GatesKnown = nil
	for _, g := range data.Moongates {
		if g >= 0 && g < 8 {
			w.GatesKnown = append(w.GatesKnown, g)
		}
	}
	w.TipsNudged = data.TipsNudged == nil || *data.TipsNudged
	if data.Automap != nil {
		w.AutoMap.FromText(*data.Automap)
	}
	return true
}

// SaveStore keeps saves by name: the game's localStorage.
type SaveStore interface {
	Load(name string) ([]byte, bool)
	Store(name string, data []byte) error
	Remove(name string)
}

// DirStore is a SaveStore of files in a directory.
type DirStore string

// Load reads a save file.
func (d DirStore) Load(name string) ([]byte, bool) {
	b, err := os.ReadFile(filepath.Join(string(d), name))
	return b, err == nil
}

// Store writes a save file atomically (a temporary file renamed into place).
func (d DirStore) Store(name string, data []byte) error {
	if err := os.MkdirAll(string(d), 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(string(d), name+".*")
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(f.Name(), filepath.Join(string(d), name))
	}
	if err != nil {
		os.Remove(f.Name())
	}
	return err
}

// Remove deletes a save file.
func (d DirStore) Remove(name string) { os.Remove(filepath.Join(string(d), name)) }

// WriteSave saves the world to the store; false if it could not.
func WriteSave(s SaveStore, w *World) bool {
	b, err := json.Marshal(Serialize(w))
	return err == nil && s.Store(SaveKey, b) == nil
}

// ReadSave restores the world from the store; false when there is no usable save.
func ReadSave(s SaveStore, w *World) bool {
	raw, ok := s.Load(SaveKey)
	if !ok {
		return false
	}
	var data SaveData
	if json.Unmarshal(raw, &data) != nil {
		return false
	}
	return Restore(w, data)
}

// ClearSave deletes the save.
func ClearSave(s SaveStore) { s.Remove(SaveKey) }

// ExportFormat is the version of ExportedGame.
const ExportFormat = 1

// ExportedGame is the save as one piece of JSON text, carried between machines.
type ExportedGame struct {
	Game     string    `json:"game"`
	Format   float64   `json:"format"`
	Exported string    `json:"exported"` // ISO 8601
	Save     *SaveData `json:"save"`
	Automap  *string   `json:"automap"` // also in the save; for a version from before it was
}

// ExportGame is the current game as text.
func ExportGame(w *World) string {
	save := Serialize(w)
	b, _ := json.Marshal(ExportedGame{"ultima3", ExportFormat, time.Now().UTC().Format("2006-01-02T15:04:05.000Z"), &save, save.Automap})
	return string(b)
}

// RestoreExported puts an exported game (ParseExport's) in place of the
// world's, with the map beside the save where the save has none.
func RestoreExported(w *World, e *ExportedGame) bool {
	if !Restore(w, *e.Save) {
		return false
	}
	if e.Save.Automap == nil {
		text := ""
		if e.Automap != nil {
			text = *e.Automap
		}
		w.AutoMap.FromText(text)
	}
	return true
}

// ParseExport reads exported text back. Its error says, in a few words fit
// for the screen, why the text is not an exported game. The save is checked
// for shape only; Restore checks its content.
func ParseExport(text string) (*ExportedGame, error) {
	var raw any
	if json.Unmarshal([]byte(text), &raw) != nil {
		return nil, errors.New("not JSON")
	}
	o, ok := raw.(map[string]any)
	if !ok {
		return nil, errors.New("not a game")
	}
	format, isNum := o["format"].(float64)
	if o["game"] != "ultima3" || !isNum {
		return nil, errors.New("not an Ultima III game")
	}
	if format > ExportFormat {
		return nil, errors.New("from a newer version")
	}
	sv, ok := o["save"].(map[string]any)
	if !ok {
		return nil, errors.New("no save inside")
	}
	if v, ok := sv["version"].(float64); !ok || v < 1 || v > SaveVersion {
		return nil, errors.New("save version unknown")
	}
	for _, k := range []string{"party", "roster", "surfaceTiles", "surfaceMonsters"} {
		if _, ok := sv[k].(string); !ok {
			return nil, errors.New("save incomplete")
		}
	}
	var e ExportedGame
	if json.Unmarshal([]byte(text), &e) != nil {
		return nil, errors.New("save incomplete")
	}
	if a, ok := o["automap"].(string); ok {
		e.Automap = &a
	} else {
		e.Automap = nil
	}
	return &e, nil
}
