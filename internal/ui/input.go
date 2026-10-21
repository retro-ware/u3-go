package ui

import (
	"strings"
	"sync"
	"time"
)

// Keyboard input as a blocking queue (input.ts). The game goroutine asks
// for keys with Next; presses that arrive while nobody is waiting are
// queued (up to a small limit) so fast typing is not lost, as the
// original's event queue kept them. A queued press goes stale after a
// moment: presses made while the game was busy for longer than that (a
// combat starting, an animation) are dropped when it next asks, so they
// cannot pile up and play out later.
//
// Keys are the single-character codes of package game: arrows map to the
// Mac arrow codes 28..31; letters keep their case.

const (
	maxQueued  = 8
	staleKeyMs = 300 // a queued press older than this when the game asks is dropped
)

// Pause reasons: the window lost focus, or the Pause menu is open.
const (
	holdFocus = "focus"
	holdMenu  = "menu"
)

// Input sources: what the player last pressed decides what Settings offers.
const (
	sourceKeyboard = "keyboard"
	sourceGamepad  = "gamepad"
)

type queuedKey struct {
	key string
	at  time.Duration
}

// Keyboard is the key queue between the window (or a script) and the game.
type Keyboard struct {
	mu      sync.Mutex
	queue   []queuedKey
	wake    chan struct{}
	waiting bool
	holds   map[string]bool
	start   time.Time
	now     func() time.Duration

	pausedAt  time.Duration
	pausedFor time.Duration // time spent paused before the current pause

	lastSource string
	onPause    func(reason string) // the game paused
	onResume   func()              // it runs again
	onInput    func()              // any key from any source

	// Headless: keys come from script; when it runs out while the game
	// waits, onIdle is called (the -shot check takes its picture and exits).
	headless bool
	script   []string
	onIdle   func()
}

// NewKeyboard is an empty queue on the real clock.
func NewKeyboard() *Keyboard {
	k := &Keyboard{wake: make(chan struct{}, 1), holds: map[string]bool{}, start: time.Now(), lastSource: sourceKeyboard}
	k.now = func() time.Duration { return time.Since(k.start) }
	return k
}

// NewScriptKeyboard reads keys from a script instead of a window: tokens
// as in ParseScript. onIdle runs when the script is used up and the game
// waits for more.
func NewScriptKeyboard(script []string, onIdle func()) *Keyboard {
	k := NewKeyboard()
	k.headless, k.script, k.onIdle = true, script, onIdle
	return k
}

// scriptWait is the script token that makes the next timed wait time out.
const scriptWait = "{wait}"

// ParseScript splits a key script: plain characters are themselves, and
// {up} {down} {left} {right} {enter} {esc} {space} {bs} {A} {B} {X} {Y}
// {pause} name the special keys; {wait} lets a timed wait run out.
func ParseScript(s string) []string {
	names := map[string]string{"up": "\x1e", "down": "\x1f", "left": "\x1c", "right": "\x1d",
		"enter": "\r", "esc": "\x1b", "space": " ", "bs": "\b",
		"A": "\x01", "B": "\x02", "X": "\x03", "Y": "\x04", "pause": "\x05", "wait": scriptWait}
	var out []string
	for i := 0; i < len(s); i++ {
		if s[i] == '{' {
			if end := strings.IndexByte(s[i:], '}'); end > 0 {
				if k, ok := names[s[i+1:i+end]]; ok {
					out = append(out, k)
					i += end
					continue
				}
			}
		}
		out = append(out, s[i:i+1])
	}
	return out
}

// Paused is whether anything holds the game paused.
func (k *Keyboard) Paused() bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	return len(k.holds) > 0
}

// Pause holds the game for a reason; timed waits stand still meanwhile.
func (k *Keyboard) Pause(reason string) {
	k.mu.Lock()
	if k.holds[reason] {
		k.mu.Unlock()
		return
	}
	first := len(k.holds) == 0
	k.holds[reason] = true
	if first {
		k.pausedAt = k.now()
	}
	cb := k.onPause
	k.mu.Unlock()
	if first && cb != nil {
		cb(reason)
	}
}

// Resume lets go of one hold; the game runs once nothing holds it.
func (k *Keyboard) Resume(reason string) {
	k.mu.Lock()
	if !k.holds[reason] {
		k.mu.Unlock()
		return
	}
	delete(k.holds, reason)
	last := len(k.holds) == 0
	if last {
		k.pausedFor += k.now() - k.pausedAt
	}
	cb := k.onResume
	k.mu.Unlock()
	if last && cb != nil {
		cb()
	}
}

// ActiveTime is a clock (ms) that stands still while paused, as timed waits do.
func (k *Keyboard) ActiveTime() int {
	k.mu.Lock()
	defer k.mu.Unlock()
	return int(k.activeLocked() / time.Millisecond)
}

func (k *Keyboard) activeLocked() time.Duration {
	now := k.now()
	t := now - k.pausedFor
	if len(k.holds) > 0 {
		t -= now - k.pausedAt
	}
	return t
}

// Waiting is whether the game is blocked on a key with none queued.
func (k *Keyboard) Waiting() bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.waiting && len(k.queue) == 0
}

// takeQueued is the oldest queued press still fresh, dropping stale ones.
func (k *Keyboard) takeQueued() (string, bool) {
	fresh := k.now() - staleKeyMs*time.Millisecond
	for len(k.queue) > 0 && k.queue[0].at < fresh {
		k.queue = k.queue[1:]
	}
	if len(k.queue) == 0 {
		return "", false
	}
	key := k.queue[0].key
	k.queue = k.queue[1:]
	return key, true
}

// Next blocks for the next key press; with timeoutMs > 0 it gives up after
// that much unpaused time and returns "", false.
func (k *Keyboard) Next(timeoutMs int) (string, bool) {
	if k.headless {
		return k.nextScripted(timeoutMs)
	}
	k.mu.Lock()
	deadline := k.activeLocked() + time.Duration(timeoutMs)*time.Millisecond
	for {
		if key, ok := k.takeQueued(); ok {
			k.waiting = false
			k.mu.Unlock()
			return key, true
		}
		if timeoutMs > 0 && k.activeLocked() >= deadline {
			k.waiting = false
			k.mu.Unlock()
			return "", false
		}
		k.waiting = true
		k.mu.Unlock()
		if timeoutMs > 0 {
			// ponytail: polls the paused clock every 10ms rather than juggling timers.
			select {
			case <-k.wake:
			case <-time.After(10 * time.Millisecond):
			}
		} else {
			<-k.wake
		}
		k.mu.Lock()
	}
}

func (k *Keyboard) nextScripted(timeoutMs int) (string, bool) {
	k.mu.Lock()
	if len(k.script) == 0 {
		k.mu.Unlock()
		if k.onIdle != nil {
			k.onIdle() // does not return in the -shot check
		}
		select {} // nothing more will come
	}
	key := k.script[0]
	k.script = k.script[1:]
	k.mu.Unlock()
	if key == scriptWait {
		if timeoutMs > 0 {
			return "", false
		}
		return k.nextScripted(timeoutMs) // an untimed wait cannot run out
	}
	return key, true
}

// PeekWait consumes a {wait} at the head of the script, for Pause in headless runs.
func (k *Keyboard) scriptedPause() {
	k.mu.Lock()
	defer k.mu.Unlock()
	if len(k.script) > 0 && k.script[0] == scriptWait {
		k.script = k.script[1:]
	}
}

// Flush drops queued presses (after "INVALID MOVE!" so a held key does not repeat the bump).
func (k *Keyboard) Flush() {
	k.mu.Lock()
	k.queue = nil
	k.mu.Unlock()
}

// Push injects a key; source "" keeps the last one (a key the game gives itself).
func (k *Keyboard) Push(key, source string) {
	k.mu.Lock()
	if source != "" {
		k.lastSource = source
	}
	if k.waiting || len(k.queue) < maxQueued {
		k.queue = append(k.queue, queuedKey{key, k.now()})
	}
	cb := k.onInput
	k.mu.Unlock()
	select {
	case k.wake <- struct{}{}:
	default:
	}
	if cb != nil {
		cb()
	}
}

// LastSource is where the last press came from.
func (k *Keyboard) LastSource() string {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.lastSource
}
