package ui

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2/audio"
)

// Background music (music.ts). The original tracks (data/mac/Music/Song_*.mov)
// are QuickTime Music Architecture files: not audio, but a list of note
// events QuickTime's software synthesizer played. This reads the note list
// and renders it with a small synthesizer, one triangle (the first part) or
// square wave per note with a short attack and release, into a loop.
//
// File layout: the stco atom lists chunk offsets and stsz the sample sizes;
// concatenated, the samples are big-endian 32-bit music events:
//
//	000 d(24)                       rest      d = duration
//	001 ppppp nnnnnn vvvvvvv d(11)  note      part, pitch - 32, volume, duration
//	010 ppppp cccccccc v(16)        control   controller 7 = part volume (8.8 fixed)
//	011 ...                         marker
//	1001 p(12) n(16)                extended note, word 1: part, pitch
//	10 vvvvvvvv d(22)               extended note, word 2: volume, duration
//	1010 / 1011 ...                 extended control / knob (two words, ignored)
//	1111 p(12) l(16)                general event: l words long, skipped
//
// Durations are in the track's time scale (mdhd, usually 600 a second).

// MusicNote is one note of a tune.
type MusicNote struct {
	Time     float64 // seconds from the start of the tune
	Pitch    float64 // MIDI pitch
	Volume   int     // 0..127
	Duration float64 // seconds
	Part     int
}

// Tune is a note list and where it loops.
type Tune struct {
	Notes  []MusicNote
	Length float64
}

var qtContainers = map[string]bool{"moov": true, "trak": true, "mdia": true, "minf": true, "stbl": true}

// findAtom is the offset of the first atom of a type, at the top level or in containers; -1 if none.
func findAtom(b []byte, typ string, start, end int) int {
	be := binary.BigEndian
	for off := start; off+8 <= end; {
		size := int(be.Uint32(b[off:]))
		t := string(b[off+4 : off+8])
		if size == 0 {
			size = end - off
		}
		if size < 8 {
			break
		}
		if t == typ {
			return off
		}
		if qtContainers[t] {
			if in := findAtom(b, typ, off+8, min(end, off+size)); in >= 0 {
				return in
			}
		}
		off += size
	}
	return -1
}

// parseQuickTimeMusic decodes a QuickTime music file into a note list.
func parseQuickTimeMusic(b []byte) (*Tune, error) {
	be := binary.BigEndian
	u32 := func(off int) uint32 {
		if off < 0 || off+4 > len(b) {
			return 0
		}
		return be.Uint32(b[off:])
	}
	timeScale := 600.0
	if mdhd := findAtom(b, "mdhd", 0, len(b)); mdhd >= 0 && u32(mdhd+20) != 0 {
		timeScale = float64(u32(mdhd + 20))
	}
	stco, stsz := findAtom(b, "stco", 0, len(b)), findAtom(b, "stsz", 0, len(b))
	if stco < 0 || stsz < 0 {
		return nil, errors.New("not a QuickTime music file")
	}
	chunks := int(u32(stco + 12))
	fixed := int(u32(stsz + 12))
	samples := int(u32(stsz + 16))
	var words []uint32
	for i := range chunks {
		offset := int(u32(stco + 16 + i*4))
		size := fixed
		if size == 0 && i < samples {
			size = int(u32(stsz + 20 + i*4))
		}
		for p := offset; p+4 <= offset+size && p+4 <= len(b); p += 4 {
			words = append(words, be.Uint32(b[p:]))
		}
	}

	tune := &Tune{}
	partVolume := map[int]float64{}
	t := 0.0 // time-scale units
	push := func(part int, pitch float64, volume, duration int) {
		pv, ok := partVolume[part]
		if !ok {
			pv = 1
		}
		tune.Notes = append(tune.Notes, MusicNote{Time: t / timeScale, Part: part, Pitch: pitch,
			Volume: int(math.Round(float64(volume) * pv)), Duration: float64(duration) / timeScale})
	}
	for i := 0; i < len(words); i++ {
		w := words[i]
		switch nibble := w >> 28; {
		case nibble <= 1:
			t += float64(w & 0x00ffffff) // rest
		case nibble <= 3:
			push(int(w>>24&0x1f), float64(w>>18&0x3f+32), int(w>>11&0x7f), int(w&0x7ff))
		case nibble <= 5:
			if w>>16&0xff == 7 { // the part's volume, 8.8 fixed
				partVolume[int(w>>24&0x1f)] = math.Min(1, float64(w&0xffff>>8)/127)
			}
		case nibble == 9:
			var w2 uint32
			if i+1 < len(words) {
				i++
				w2 = words[i]
			}
			pitch := float64(w & 0xffff)
			if pitch > 255 {
				pitch /= 256
			}
			push(int(w>>16&0xfff), pitch, int(w2>>22&0xff), int(w2&0x3fffff))
		case nibble == 0xa || nibble == 0xb:
			i++
		case nibble == 0xf:
			i += max(1, int(w&0xffff)) - 1
		}
	}
	tune.Length = t / timeScale
	for _, n := range tune.Notes {
		tune.Length = math.Max(tune.Length, n.Time+n.Duration)
	}
	return tune, nil
}

// songFiles are the song files by track number (gSongNext); 9 and 12-15 do not exist.
var songFiles = map[int]string{1: "Song_1.mov", 2: "Song_2.mov", 3: "Song_3.mov", 4: "Song_4.mov",
	5: "Song_5.mov", 6: "Song_6.mov", 7: "Song_7.mov", 8: "Song_8.mov", 10: "Song_A.mov", 11: "Song_B.mov"}

// fullVolume is the music's gain at the full Music level.
const fullVolume = 0.35

// render synthesizes one loop of a tune, mono at the mixer's rate, at
// fullVolume. Notes that sound past the loop point wrap round to its start.
func render(t *Tune) []float32 {
	length := math.Max(t.Length, 0.5)
	n := int(length * sampleRate)
	out := make([]float32, n)
	for _, note := range t.Notes {
		freq := 440 * math.Pow(2, (note.Pitch-69)/12)
		peak := float64(note.Volume) / 127 * 0.5 * fullVolume
		dur := math.Max(0.05, note.Duration)
		start := int(note.Time * sampleRate)
		count := int(dur * sampleRate)
		phase := 0.0
		step := freq / sampleRate
		for i := range count {
			at := float64(i) / sampleRate
			env := 1.0
			switch {
			case at < 0.02:
				env = at / 0.02
			case at > dur-0.05:
				env = math.Max(0, (dur-at)/0.05)
			}
			var wave float64
			if note.Part == 0 { // triangle
				wave = 4*math.Abs(phase-0.5) - 1
			} else if phase < 0.5 {
				wave = 1
			} else {
				wave = -1
			}
			out[(start+i)%n] += float32(wave * env * peak)
			phase += step
			phase -= math.Floor(phase)
		}
	}
	return out
}

// MusicPlayer plays the track the game wants, looping, if music is on.
type MusicPlayer struct {
	mu      sync.Mutex
	ctx     *audio.Context
	data    string
	loops   map[int][]byte // rendered loops by track, nil when there is no file
	current int            // the track the game wants
	active  int            // the track playing or loading
	token   int
	player  *audio.Player
	on      bool
	level   int
	paused  bool
	hearing bool
	duckAt  time.Time
	duckLen float64
}

// NewMusicPlayer plays through ctx (nil: silent) the songs in data/mac/Music.
func NewMusicPlayer(ctx *audio.Context, data string) *MusicPlayer {
	return &MusicPlayer{ctx: ctx, data: data, loops: map[int][]byte{}, on: true, level: levels}
}

// Enabled says whether music is on.
func (m *MusicPlayer) Enabled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.on
}

// SetEnabled turns music on or off; on resumes the wanted track.
func (m *MusicPlayer) SetEnabled(on bool) {
	m.mu.Lock()
	m.on = on
	m.syncLocked()
	m.mu.Unlock()
}

// Level is the Music level, 0..levels.
func (m *MusicPlayer) Level() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.level
}

// SetLevel sets the Music level; at 0 nothing plays.
func (m *MusicPlayer) SetLevel(n int) {
	m.mu.Lock()
	m.level = max(0, min(levels, n))
	m.syncLocked()
	m.mu.Unlock()
}

// Play switches to a track (0 silence); remembered while music is off.
func (m *MusicPlayer) Play(track int) {
	m.mu.Lock()
	m.current = track
	m.syncLocked()
	m.mu.Unlock()
}

// Active is the track playing or loading, 0 while silent.
func (m *MusicPlayer) Active() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.active
}

// Pause holds the tune's place while the game is paused; Resume goes on from it.
func (m *MusicPlayer) Pause() { m.mu.Lock(); m.paused = true; m.runLocked(); m.mu.Unlock() }

// Resume undoes Pause.
func (m *MusicPlayer) Resume() { m.mu.Lock(); m.paused = false; m.runLocked(); m.mu.Unlock() }

// Hear plays the music over a pause while the Music level is being dialled.
func (m *MusicPlayer) Hear(on bool) { m.mu.Lock(); m.hearing = on; m.runLocked(); m.mu.Unlock() }

// Duck dips the music to a third under an effect of seconds, then back.
func (m *MusicPlayer) Duck(seconds float64) {
	m.mu.Lock()
	m.duckAt, m.duckLen = time.Now(), seconds
	m.mu.Unlock()
}

// volume is the gain now: the level, ducked under a long effect.
func (m *MusicPlayer) volumeLocked() float64 {
	v := levelGain(m.level)
	if m.duckLen > 0 {
		t := time.Since(m.duckAt).Seconds()
		switch {
		case t < 0.05:
			v *= 1 - (2.0/3)*t/0.05
		case t < math.Max(0.05, m.duckLen-0.25):
			v /= 3
		case t < m.duckLen+0.3:
			from := math.Max(0.05, m.duckLen-0.25)
			v *= 1.0/3 + (2.0/3)*(t-from)/(m.duckLen+0.3-from)
		default:
			m.duckLen = 0
		}
	}
	return v
}

// Update follows the duck; call once a frame.
func (m *MusicPlayer) Update() {
	m.mu.Lock()
	if m.player != nil {
		m.player.SetVolume(m.volumeLocked())
	}
	m.mu.Unlock()
}

func (m *MusicPlayer) runLocked() {
	if m.player == nil {
		return
	}
	if !m.paused || m.hearing {
		m.player.Play()
	} else {
		m.player.Pause()
	}
}

// syncLocked starts or stops so that what sounds is the wanted track, if music is on.
func (m *MusicPlayer) syncLocked() {
	wanted := 0
	if m.on && m.level > 0 && m.ctx != nil {
		wanted = m.current
	}
	if wanted == m.active {
		if m.player != nil {
			m.player.SetVolume(m.volumeLocked())
		}
		return
	}
	if m.player != nil {
		m.player.Close()
		m.player = nil
	}
	m.token++
	m.active = wanted
	if wanted == 0 {
		return
	}
	token := m.token
	go func() {
		pcm := m.loop(wanted)
		m.mu.Lock()
		defer m.mu.Unlock()
		if pcm == nil || token != m.token {
			return
		}
		p, err := m.ctx.NewPlayer(audio.NewInfiniteLoop(bytes.NewReader(pcm), int64(len(pcm))))
		if err != nil {
			return
		}
		m.player = p
		p.SetVolume(m.volumeLocked())
		m.runLocked()
	}()
}

// loop is a track rendered as 16-bit stereo, cached.
func (m *MusicPlayer) loop(track int) []byte {
	m.mu.Lock()
	pcm, ok := m.loops[track]
	m.mu.Unlock()
	if ok {
		return pcm
	}
	if file, ok := songFiles[track]; ok {
		if b, err := os.ReadFile(filepath.Join(m.data, "mac", "Music", file)); err == nil {
			if tune, err := parseQuickTimeMusic(b); err == nil {
				pcm = pcmBytes(render(tune), 1)
			}
		}
	}
	m.mu.Lock()
	m.loops[track] = pcm
	m.mu.Unlock()
	return pcm
}
