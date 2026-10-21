package ui

import (
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/retro-ware/u3-go/internal/game"
)

// testData is the imported Mac data, when the tests are pointed at it (U3_MAC_DATA).
func testData(t *testing.T) string {
	d := os.Getenv("U3_MAC_DATA")
	if d == "" {
		t.Skip("U3_MAC_DATA not set")
	}
	return d
}

type started struct {
	name       string
	gain, rate float64
}

func fakeSounds(lengths map[string]float64) (*SoundPlayer, *time.Duration, *[]started, *[]float64) {
	var now time.Duration
	var out []started
	var rolls []float64
	p := NewSoundPlayer(nil, "")
	p.now = func() time.Duration { return now }
	p.random = func() float64 {
		if len(rolls) == 0 {
			return 0.5
		}
		r := rolls[0]
		rolls = rolls[1:]
		return r
	}
	p.load = func(set, file string) *clip {
		l := lengths[set+"/"+file]
		if l == 0 {
			l = 0.1
		}
		return &clip{make([]float32, int(l*sampleRate))}
	}
	names := map[*clip]string{}
	orig := p.load
	p.load = func(set, file string) *clip { c := orig(set, file); names[c] = set + "/" + file; return c }
	p.output = func(c *clip, gain, rate float64) { out = append(out, started{names[c], gain, rate}) }
	return p, &now, &out, &rolls
}

func TestSoundSetsAndGain(t *testing.T) {
	p, now, out, _ := fakeSounds(nil)
	p.Play("Step")
	p.Play("Step") // too soon
	*now = 100 * time.Millisecond
	p.Play("Step")
	p.Set = "Lairware"
	*now = 200 * time.Millisecond
	p.Play("Step")
	if len(*out) != 3 || (*out)[2].name != "Lairware/Step" || (*out)[0].gain != 1 ||
		math.Abs((*out)[2].gain-math.Pow(10, -9.0/20)) > 1e-6 {
		t.Fatalf("%+v", *out)
	}
	p.Level = 5
	*now = time.Second
	p.Set = "Standard"
	p.Play("Bump")
	if g := (*out)[3].gain; math.Abs(g-0.25) > 1e-9 {
		t.Errorf("level 5 gain %v", g)
	}
	p.Level = 0
	*now = 2 * time.Second
	p.Play("Bump")
	if len(*out) != 4 {
		t.Error("played at Off")
	}
	if fileFor("Standard", "Swish3") != "Swish" || fileFor("Lairware", "Swish3") != "Swish3" {
		t.Error("swish alias")
	}
}

func TestLevels(t *testing.T) {
	for l, want := range map[int]string{0: "Off", 1: "10%", 6: "60%", 10: "100%"} {
		if levelLabel(l) != want {
			t.Errorf("%d: %s", l, levelLabel(l))
		}
	}
	if levelGain(levels) != 1 || levelGain(0) != 0 || validLevel(3.0, levels) != 3 {
		t.Error("levels")
	}
	for _, bad := range []any{11.0, -1.0, 2.5, "7", nil} {
		if validLevel(bad, levels) != levels {
			t.Errorf("%v accepted", bad)
		}
	}
}

func TestDetune(t *testing.T) {
	p, now, out, rolls := fakeSounds(nil)
	*rolls = []float64{0, 1, 0.5, 0}
	p.Play("Hit")
	*now = 100 * time.Millisecond
	p.Play("Hit")
	*now = 200 * time.Millisecond
	p.Play("CombatStart")
	p.Set = "Lairware"
	*now = 300 * time.Millisecond
	p.Play("Hit")
	want := []float64{math.Pow(2, -1.0/12), math.Pow(2, 1.0/12), 1, 1}
	for i, s := range *out {
		if math.Abs(s.rate-want[i]) > 1e-4 {
			t.Errorf("play %d rate %v want %v", i, s.rate, want[i])
		}
	}
}

func TestLongEffects(t *testing.T) {
	p, now, out, _ := fakeSounds(map[string]float64{"Standard/CombatStart": 2})
	var ducks []float64
	p.onLong = func(s float64) { ducks = append(ducks, s) }
	p.Play(game.SoundCombatStart)
	*now = 500 * time.Millisecond
	p.Play(game.SoundCombatStart)
	*now = 2500 * time.Millisecond
	p.Play(game.SoundCombatStart)
	if len(*out) != 2 || len(ducks) != 2 || math.Abs(ducks[0]-2) > 1e-3 {
		t.Errorf("%d plays, ducks %v", len(*out), ducks)
	}
}

func TestStandardSetComplete(t *testing.T) {
	names := append([]string{game.SoundStep, game.SoundHorseWalk, game.SoundBump, game.SoundMountHorse, game.SoundHit,
		game.SoundAttack, game.SoundForceField, game.SoundMoongate, game.SoundSink, game.SoundShoot, game.SoundError1,
		game.SoundError2, game.SoundDeathMale, game.SoundDeathFemale, game.SoundBigDeath, game.SoundTorchIgnite,
		game.SoundCreak, game.SoundOuch, game.SoundAlarm, game.SoundCombatStart, game.SoundCombatVictory,
		game.SoundFailedSpell, game.SoundImmolate, game.SoundDownwards, game.SoundUpwards, game.SoundInvocation,
		game.SoundHeal, game.SoundMiscSpell, game.SoundMonsterSpell, game.SoundShrine, game.SoundLBLevelRise,
		game.SoundExpLevelUp}, game.SoundSwish...)
	p := NewSoundPlayer(nil, "")
	for _, n := range names {
		if c := p.loadFile("Standard", fileFor("Standard", n)); c == nil || len(c.samples) == 0 {
			t.Errorf("Standard %s missing", n)
		}
	}
}

func TestLairwareSounds(t *testing.T) {
	data := testData(t)
	p := NewSoundPlayer(nil, data)
	for name := range lairwareGainDB {
		if c := p.loadFile("Lairware", name); c == nil || c.seconds() < 0.05 {
			t.Errorf("%s did not decode", name)
		}
	}
}

func TestQuickTimeMusic(t *testing.T) {
	data := testData(t)
	b, err := os.ReadFile(filepath.Join(data, "mac", "Music", "Song_1.mov"))
	if err != nil {
		t.Fatal(err)
	}
	tune, err := parseQuickTimeMusic(b)
	if err != nil {
		t.Fatal(err)
	}
	first := tune.Notes[0]
	if len(tune.Notes) <= 100 || first.Pitch != 48 || first.Volume != 64 || math.Abs(first.Duration-1) > 1e-3 ||
		math.Abs(first.Time-425.0/600) > 1e-3 || tune.Length <= 30 || tune.Length >= 300 {
		t.Fatalf("first %+v, %d notes, %v s", first, len(tune.Notes), tune.Length)
	}
	var long *MusicNote
	for i := range tune.Notes {
		if i > 0 && tune.Notes[i].Time < tune.Notes[i-1].Time {
			t.Fatal("notes out of order")
		}
		if tune.Notes[i].Duration > 5 && long == nil {
			long = &tune.Notes[i]
		}
	}
	if long == nil || long.Pitch != 55 || math.Abs(long.Duration-5097.0/600) > 1e-3 {
		t.Errorf("long note %+v", long)
	}
	for _, f := range songFiles {
		b, _ := os.ReadFile(filepath.Join(data, "mac", "Music", f))
		tune, err := parseQuickTimeMusic(b)
		if err != nil || len(tune.Notes) <= 10 {
			t.Fatalf("%s: %v", f, err)
		}
		for _, n := range tune.Notes {
			if n.Volume <= 0 || n.Volume > 127 || n.Pitch <= 20 || n.Pitch >= 110 {
				t.Fatalf("%s: %+v", f, n)
			}
		}
		if s := render(tune); len(s) == 0 {
			t.Fatal("empty render")
		}
	}
}

func TestMusicFollowsSettings(t *testing.T) {
	m := NewMusicPlayer(nil, "") // no audio: nothing becomes active
	m.Play(game.MusicSosaria)
	if m.Active() != 0 {
		t.Error("played without audio")
	}
	m.ctx = nil
}
