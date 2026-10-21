package ui

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"io/fs"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/mp3"

	"github.com/retro-ware/u3-go/assets"
)

// Sound effects (sound.ts), from one of two sets:
//
//	Standard  the port's chip-tune effects, embedded (assets/sounds),
//	          balanced against each other.
//	Lairware  the original Mac effects from the import (data/mac/Sounds,
//	          IMA ADPCM WAVs and one MP3), recorded at very different
//	          levels: a gain table brings them into line.
//
// Effects are decoded once and cached. Two mixing rules keep repeats from
// piling up: the same effect is not started again within a few dozen
// milliseconds, and an effect longer than half a second is not restarted
// while it still sounds. A long effect ducks the music under it. In the
// Standard set the effects a turn repeats play at a slightly different
// pitch each time, so no two footsteps are the same.

// SoundSets are the effect sets the Settings menu offers.
var SoundSets = []string{"Standard", "Lairware"}

// sampleRate is the mixer's rate.
const sampleRate = 44100

// fileFor is the file a set plays for a game sound name: the Standard set
// has one swing, varied at play time, where the Mac had four.
func fileFor(set, name string) string {
	if set == "Standard" && len(name) == 6 && name[:5] == "Swish" {
		return "Swish"
	}
	return name
}

// levels: a level is 0, off, to levels, the game's own loudness.
const levels = 10

// levelGain is a level's gain, on a curve (the square) so each step sounds like one.
func levelGain(level int) float64 {
	l := float64(max(0, min(levels, level))) / levels
	return l * l
}

// levelLabel is a level as Settings shows it: Off, or 10% to 100%.
func levelLabel(level int) string {
	if level <= 0 {
		return "Off"
	}
	return strconv.Itoa(min(levels, level)*10) + "%"
}

// validLevel is a remembered level, or fallback when it is not a whole step.
func validLevel(level any, fallback int) int {
	if f, ok := level.(float64); ok && f == math.Trunc(f) && f >= 0 && f <= levels {
		return int(f)
	}
	if n, ok := level.(int); ok && n >= 0 && n <= levels {
		return n
	}
	return fallback
}

const (
	minInterval     = 70 * time.Millisecond // the same effect starts at most this often
	longSound       = 0.5                   // seconds: not restarted while it plays
	duckSound       = 1.0                   // seconds: ducks the music
	detuneSemitones = 1.0
)

// varied are the Standard effects a turn repeats, detuned each play.
var varied = map[string]bool{"Step": true, "HorseWalk": true, "Bump": true, "Hit": true, "Attack": true,
	"Swish1": true, "Swish2": true, "Swish3": true, "Swish4": true, "Ouch": true, "Shoot": true, "Error1": true, "Error2": true}

// lairwareGainDB pulls the Mac files to targets by kind: what a turn
// repeats far below the one-off jingles.
var lairwareGainDB = map[string]float64{
	"Step": -9, "HorseWalk": 0, "Bump": -9, "Error1": -8, "Error2": -6, "Attack": -6,
	"Swish1": 1, "Swish2": -1, "Swish3": -3, "Swish4": -5, "Hit": -8, "Ouch": -5, "Shoot": -3,
	"DeathMale": -4, "DeathFemale": -10, "BigDeath": -8, "CombatStart": -7, "CombatVictory": -9,
	"Alarm": -5, "Creak": -2, "TorchIgnite": -6, "Sink": -5, "Moongate": -9, "MountHorse": -9,
	"Upwards": -7, "Downwards": -7, "ForceField": -8, "FailedSpell": -7, "Immolate": -9,
	"MiscSpell": -10, "MonsterSpell": -5, "Invocation": -4, "Heal": -11, "Shrine": -13,
	"LBLevelRise": -10, "ExpLevelUp": -6,
}

// clip is a decoded effect: mono samples at the mixer's rate.
type clip struct {
	samples []float32
}

func (c *clip) seconds() float64 { return float64(len(c.samples)) / sampleRate }

// SoundPlayer plays effects. Its output is pluggable, for the tests.
type SoundPlayer struct {
	mu        sync.Mutex
	Enabled   bool
	Set       string
	Level     int
	data      string // the import, for the Lairware set
	cache     map[string]*clip
	lastStart map[string]time.Duration
	ends      map[string]time.Duration

	now    func() time.Duration
	random func() float64
	load   func(set, file string) *clip
	output func(c *clip, gain, rate float64) // starts a sound
	onLong func(seconds float64)             // a long effect started: the music ducks
}

// NewSoundPlayer plays through ctx (nil: silent) from the embedded set and the import in data.
func NewSoundPlayer(ctx *audio.Context, data string) *SoundPlayer {
	start := time.Now()
	p := &SoundPlayer{Enabled: true, Set: "Standard", Level: levels, data: data,
		cache: map[string]*clip{}, lastStart: map[string]time.Duration{}, ends: map[string]time.Duration{},
		now: func() time.Duration { return time.Since(start) }, random: rand.Float64}
	p.load = p.loadFile
	p.output = func(c *clip, gain, rate float64) {
		if ctx != nil {
			ctx.NewPlayerFromBytes(pcmBytes(resample(c.samples, rate), gain)).Play()
		}
	}
	return p
}

func (p *SoundPlayer) loadFile(set, file string) *clip {
	var b []byte
	var err error
	if set == "Standard" {
		b, err = fs.ReadFile(assets.FS, "sounds/"+file+".wav")
	} else {
		dir := filepath.Join(p.data, "mac", "Sounds")
		if b, err = os.ReadFile(filepath.Join(dir, file+".wav")); err != nil {
			if mp, err2 := os.ReadFile(filepath.Join(dir, file+".mp3")); err2 == nil {
				return decodeMP3(mp)
			}
		}
	}
	if err != nil {
		return nil
	}
	samples, rate, err := decodeWAV(b)
	if err != nil {
		return nil
	}
	return &clip{resample(samples, float64(rate)/sampleRate)}
}

// Play plays a named effect from the current set. Missing files are silent.
func (p *SoundPlayer) Play(name string) {
	p.mu.Lock()
	if !p.Enabled || p.Level <= 0 {
		p.mu.Unlock()
		return
	}
	set := p.Set
	key := set + "/" + fileFor(set, name)
	t := p.now()
	if last, ok := p.lastStart[key]; ok && t-last < minInterval {
		p.mu.Unlock()
		return
	}
	p.lastStart[key] = t
	c, ok := p.cache[key]
	if !ok {
		c = p.load(set, fileFor(set, name))
		p.cache[key] = c
	}
	if c == nil {
		p.mu.Unlock()
		return
	}
	length := c.seconds()
	if length >= longSound && p.ends[key] > t {
		p.mu.Unlock()
		return
	}
	rate := 1.0
	if set == "Standard" && varied[name] {
		rate = math.Pow(2, (p.random()*2-1)*detuneSemitones/12)
	}
	gain := levelGain(p.Level)
	if set == "Lairware" {
		gain *= math.Pow(10, lairwareGainDB[name]/20)
	}
	if length >= longSound {
		p.ends[key] = t + time.Duration(length*float64(time.Second))
	}
	out, onLong := p.output, p.onLong
	p.mu.Unlock()
	out(c, gain, rate)
	if length >= duckSound && onLong != nil {
		onLong(length)
	}
}

// resample plays samples at rate times their speed (linear interpolation).
func resample(in []float32, rate float64) []float32 {
	if rate == 1 || len(in) == 0 {
		return in
	}
	n := int(float64(len(in)) / rate)
	out := make([]float32, n)
	for i := range out {
		pos := float64(i) * rate
		j := int(pos)
		f := float32(pos - float64(j))
		a := in[min(j, len(in)-1)]
		b := in[min(j+1, len(in)-1)]
		out[i] = a + (b-a)*f
	}
	return out
}

// pcmBytes is mono samples as the 16-bit stereo little-endian stream Ebiten plays.
func pcmBytes(s []float32, gain float64) []byte {
	out := make([]byte, len(s)*4)
	for i, v := range s {
		x := int16(max(-32768, min(32767, float64(v)*gain*32767)))
		binary.LittleEndian.PutUint16(out[i*4:], uint16(x))
		binary.LittleEndian.PutUint16(out[i*4+2:], uint16(x))
	}
	return out
}

// decodeWAV reads a RIFF WAVE of 8- or 16-bit PCM or IMA ADPCM, mixed to
// mono: the samples (-1..1) and their rate.
func decodeWAV(b []byte) ([]float32, int, error) {
	le := binary.LittleEndian
	if len(b) < 12 || string(b[:4]) != "RIFF" || string(b[8:12]) != "WAVE" {
		return nil, 0, errors.New("not a WAVE file")
	}
	var format, channels, bits, blockAlign int
	var rate int
	var data []byte
	for p := 12; p+8 <= len(b); {
		id, size := string(b[p:p+4]), int(le.Uint32(b[p+4:]))
		body := b[p+8 : min(len(b), p+8+size)]
		switch id {
		case "fmt ":
			if len(body) < 16 {
				return nil, 0, errors.New("short fmt chunk")
			}
			format, channels = int(le.Uint16(body)), int(le.Uint16(body[2:]))
			rate, blockAlign, bits = int(le.Uint32(body[4:])), int(le.Uint16(body[12:])), int(le.Uint16(body[14:]))
		case "data":
			data = body
		}
		p += 8 + size + size&1
	}
	if channels < 1 || rate <= 0 || data == nil {
		return nil, 0, errors.New("bad WAVE file")
	}
	var out []float32
	switch {
	case format == 1 && bits == 16:
		for i := 0; i+2*channels <= len(data); i += 2 * channels {
			var sum float32
			for c := range channels {
				sum += float32(int16(le.Uint16(data[i+2*c:]))) / 32768
			}
			out = append(out, sum/float32(channels))
		}
	case format == 1 && bits == 8:
		for i := 0; i+channels <= len(data); i += channels {
			var sum float32
			for c := range channels {
				sum += (float32(data[i+c]) - 128) / 128
			}
			out = append(out, sum/float32(channels))
		}
	case format == 0x11 && bits == 4 && channels == 1:
		out = decodeIMA(data, blockAlign)
	default:
		return nil, 0, errors.New("unsupported WAVE encoding")
	}
	return out, rate, nil
}

var imaSteps = [89]int{7, 8, 9, 10, 11, 12, 13, 14, 16, 17, 19, 21, 23, 25, 28, 31, 34, 37, 41, 45,
	50, 55, 60, 66, 73, 80, 88, 97, 107, 118, 130, 143, 157, 173, 190, 209, 230, 253, 279, 307,
	337, 371, 408, 449, 494, 544, 598, 658, 724, 796, 876, 963, 1060, 1166, 1282, 1411, 1552, 1707,
	1878, 2066, 2272, 2499, 2749, 3024, 3327, 3660, 4026, 4428, 4871, 5358, 5894, 6484, 7132, 7845,
	8630, 9493, 10442, 11487, 12635, 13899, 15289, 16818, 18500, 20350, 22385, 24623, 27086, 29794, 32767}

var imaIndex = [16]int{-1, -1, -1, -1, 2, 4, 6, 8, -1, -1, -1, -1, 2, 4, 6, 8}

// decodeIMA decodes mono IMA ADPCM: blocks of a 4-byte header (the first
// sample and the step index) then two samples a byte, low nibble first.
func decodeIMA(data []byte, blockAlign int) []float32 {
	if blockAlign < 5 {
		blockAlign = len(data)
	}
	var out []float32
	for b := 0; b+4 <= len(data); b += blockAlign {
		block := data[b:min(len(data), b+blockAlign)]
		pred := int(int16(binary.LittleEndian.Uint16(block)))
		index := min(88, max(0, int(block[2])))
		out = append(out, float32(pred)/32768)
		for _, by := range block[4:] {
			for _, nib := range [2]int{int(by & 15), int(by >> 4)} {
				step := imaSteps[index]
				diff := step >> 3
				if nib&4 != 0 {
					diff += step
				}
				if nib&2 != 0 {
					diff += step >> 1
				}
				if nib&1 != 0 {
					diff += step >> 2
				}
				if nib&8 != 0 {
					pred -= diff
				} else {
					pred += diff
				}
				pred = max(-32768, min(32767, pred))
				index = max(0, min(88, index+imaIndex[nib]))
				out = append(out, float32(pred)/32768)
			}
		}
	}
	return out
}

// decodeMP3 decodes an MP3 (pure Go) to mono at the mixer's rate.
func decodeMP3(b []byte) *clip {
	s, err := mp3.DecodeWithSampleRate(sampleRate, bytes.NewReader(b))
	if err != nil {
		return nil
	}
	pcm, err := io.ReadAll(s) // 16-bit stereo
	if err != nil {
		return nil
	}
	out := make([]float32, len(pcm)/4)
	for i := range out {
		l := int16(binary.LittleEndian.Uint16(pcm[i*4:]))
		r := int16(binary.LittleEndian.Uint16(pcm[i*4+2:]))
		out[i] = (float32(l) + float32(r)) / 65536
	}
	return &clip{out}
}

// OnLong sets what hears of a long effect starting (the music ducks under it).
func (p *SoundPlayer) OnLong(f func(seconds float64)) { p.onLong = f }
