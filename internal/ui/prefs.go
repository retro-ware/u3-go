package ui

import (
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"

	"github.com/retro-ware/u3-go/internal/game"
)

// The settings kept between runs (main.ts's localStorage prefs), as
// save/settings.json. Anything unreadable falls back to the default.

const prefsFile = "settings.json"

// Prefs are the remembered settings.
type Prefs struct {
	InputMode    string          `json:"inputMode"`
	Tiles        string          `json:"tiles"`
	Scanlines    bool            `json:"scanlines"`
	AutoCombat   bool            `json:"autoCombat"`
	PoisonKills  bool            `json:"poisonKills"`
	Starvation   game.Starvation `json:"starvation"`
	BalancedXP   bool            `json:"balancedXp"`
	Timer        game.Timer      `json:"timer"`
	Sound        bool            `json:"sound"`
	SoundSet     string          `json:"soundSet"`
	EffectsLevel any             `json:"effectsLevel"`
	Music        bool            `json:"music"`
	MusicLevel   any             `json:"musicLevel"`
	DungeonMap   game.MapMode    `json:"dungeonMap"`
}

func defaultPrefs() Prefs {
	return Prefs{InputMode: "controller", Tiles: "Standard", Starvation: game.StarvationMild, BalancedXP: true,
		Timer: game.TimerFast, Sound: true, SoundSet: "Standard", EffectsLevel: levels, Music: true, MusicLevel: levels,
		DungeonMap: game.MapModeOff}
}

// LoadPrefs reads the settings in dir, checking each.
func LoadPrefs(dir string) Prefs {
	p := defaultPrefs()
	if b, err := os.ReadFile(filepath.Join(dir, prefsFile)); err == nil {
		if json.Unmarshal(b, &p) != nil {
			p = defaultPrefs()
		}
	}
	d := defaultPrefs()
	if indexOf(TileSets, p.Tiles) < 0 {
		p.Tiles = d.Tiles
	}
	if p.InputMode != "keyboard" {
		p.InputMode = d.InputMode
	}
	if indexOf(game.MapModes, p.DungeonMap) < 0 {
		p.DungeonMap = d.DungeonMap
	}
	if indexOf(game.StarvationModes, p.Starvation) < 0 {
		p.Starvation = d.Starvation
	}
	if indexOf(game.TimerModes, p.Timer) < 0 {
		p.Timer = d.Timer
	}
	if indexOf(SoundSets, p.SoundSet) < 0 {
		p.SoundSet = d.SoundSet
	}
	p.EffectsLevel = validLevel(p.EffectsLevel, levels)
	p.MusicLevel = validLevel(p.MusicLevel, levels)
	return p
}

// Save writes the settings into dir.
func (p Prefs) Save(dir string) error {
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return game.DirStore(dir).Store(prefsFile, append(b, '\n'))
}

// Apply sets the settings on the world, screen and players.
func (p Prefs) Apply(w *game.World, s *Screen, sounds *SoundPlayer, music *MusicPlayer) {
	w.AutoCombat, w.PoisonKills, w.Starvation = p.AutoCombat, p.PoisonKills, p.Starvation
	w.BalancedXP, w.Timer, w.SoundEnabled, w.MapMode = p.BalancedXP, p.Timer, p.Sound, p.DungeonMap
	s.InputMode, s.Scanlines = p.InputMode, p.Scanlines
	sounds.Set, sounds.Level = p.SoundSet, p.EffectsLevel.(int)
	music.SetLevel(p.MusicLevel.(int))
	music.SetEnabled(p.Music)
}

// CurrentPrefs reads the settings back from what is in use; tiles is the set to remember.
func CurrentPrefs(w *game.World, s *Screen, sounds *SoundPlayer, music *MusicPlayer, tiles string) Prefs {
	return Prefs{InputMode: s.InputMode, Tiles: tiles, Scanlines: s.Scanlines, AutoCombat: w.AutoCombat,
		PoisonKills: w.PoisonKills, Starvation: w.Starvation, BalancedXP: w.BalancedXP, Timer: w.Timer,
		Sound: w.SoundEnabled, SoundSet: sounds.Set, EffectsLevel: sounds.Level, Music: music.Enabled(),
		MusicLevel: music.Level(), DungeonMap: w.MapMode}
}

// Snapshot paints the screen as it stands and writes it as a PNG (the -shot check).
func (s *Screen) Snapshot(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.viewDirty = true
	s.frameLocked(s.lastFrame + animationTick)
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(f, s.c.RGBA); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
