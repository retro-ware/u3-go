package ui

import (
	"image/color"
	"image/png"
	"math"
	"os"
	"sync/atomic"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/retro-ware/u3-go/internal/game"
)

// The window (main.ts, fullScreen.ts, bootHold.ts and the keyboard and
// gamepad halves of input.ts): an Ebiten game that turns key and pad
// presses into the keyboard queue, shows the Screen's canvas scaled to the
// window with optional scanlines, and owns the window-level keys: F11 or
// Alt-Enter full screen, Alt +/- the music level, Alt-V music on or off,
// Alt-Shift-6 the next tile set.

const (
	keyRepeatFirst = 400 * time.Millisecond
	keyRepeatEvery = 50 * time.Millisecond
	bootHoldTime   = 3 * time.Second
)

// App is the window round a Screen.
type App struct {
	Screen *Screen
	// OnVolume is called after a window key changed the music (to save it).
	OnVolume func()

	k       *Keyboard
	music   *MusicPlayer
	pads    GamepadReader
	img     *ebiten.Image
	focused bool
	quit    atomic.Bool
	start   time.Time

	boot     *ebiten.Image // the cover, shown until bootDone
	bootDone chan struct{}
}

// NewApp is the window for s, its keys going to k.
func NewApp(s *Screen, k *Keyboard, music *MusicPlayer) *App {
	return &App{Screen: s, k: k, music: music, focused: true, start: time.Now()}
}

// Quit closes the window at the next frame.
func (a *App) Quit() { a.quit.Store(true) }

// BootHold shows the cover (data/COVER.PNG) for up to three seconds before
// the game draws; any key skips it. Call from the game goroutine before it
// draws anything; it returns when the hold is over.
func (a *App) BootHold(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	img, err := png.Decode(f)
	f.Close()
	if err != nil {
		return
	}
	a.boot = ebiten.NewImageFromImage(img)
	a.bootDone = make(chan struct{})
	a.k.Next(int(bootHoldTime / time.Millisecond))
	close(a.bootDone)
}

func (a *App) booting() bool {
	if a.bootDone == nil {
		return false
	}
	select {
	case <-a.bootDone:
		a.boot, a.bootDone = nil, nil
		return false
	default:
		return true
	}
}

// Update reads the input once a frame.
func (a *App) Update() error {
	if a.quit.Load() {
		return ebiten.Termination
	}
	if f := ebiten.IsFocused(); f != a.focused {
		a.focused = f
		if f {
			a.k.Resume(holdFocus)
		} else {
			a.k.Pause(holdFocus)
		}
	}
	a.music.Update()
	alt := ebiten.IsKeyPressed(ebiten.KeyAlt)
	shift := ebiten.IsKeyPressed(ebiten.KeyShift)
	just := func(keys ...ebiten.Key) bool {
		for _, k := range keys {
			if inpututil.IsKeyJustPressed(k) {
				return true
			}
		}
		return false
	}
	switch {
	case just(ebiten.KeyF11) || (alt && just(ebiten.KeyEnter, ebiten.KeyNumpadEnter)):
		ebiten.SetFullscreen(!ebiten.IsFullscreen())
		return nil
	case alt && shift && just(ebiten.KeyDigit6):
		go a.Screen.NextTileSet() // waits for the game goroutine to let go of the screen
		return nil
	case alt && just(ebiten.KeyEqual, ebiten.KeyNumpadAdd):
		a.music.SetLevel(min(levels, a.music.Level()+1))
		a.volumeChanged()
		return nil
	case alt && just(ebiten.KeyMinus, ebiten.KeyNumpadSubtract):
		a.music.SetLevel(max(0, a.music.Level()-1))
		a.volumeChanged()
		return nil
	case alt && just(ebiten.KeyV):
		a.music.SetEnabled(!a.music.Enabled())
		a.volumeChanged()
		return nil
	}
	if !alt {
		a.readKeys()
	}
	a.readPads()
	return nil
}

func (a *App) volumeChanged() {
	if a.OnVolume != nil {
		a.OnVolume()
	}
}

// specialKeys are the keys that are not characters; the arrows and
// backspace repeat when held, as typed keys do.
var specialKeys = []struct {
	key    ebiten.Key
	code   string
	repeat bool
}{
	{ebiten.KeyArrowUp, game.KeyUp, true}, {ebiten.KeyArrowDown, game.KeyDown, true},
	{ebiten.KeyArrowLeft, game.KeyLeft, true}, {ebiten.KeyArrowRight, game.KeyRight, true},
	{ebiten.KeyBackspace, game.KeyBackspace, true}, {ebiten.KeyDelete, game.KeyBackspace, true},
	{ebiten.KeyEnter, game.KeyEnter, false}, {ebiten.KeyNumpadEnter, game.KeyEnter, false},
	{ebiten.KeyEscape, game.KeyEscape, false},
}

func (a *App) readKeys() {
	pressed := false
	for _, sk := range specialKeys {
		d := time.Duration(inpututil.KeyPressDuration(sk.key)) * time.Second / time.Duration(ebiten.TPS())
		if d == 0 {
			continue
		}
		first := inpututil.IsKeyJustPressed(sk.key)
		pressed = pressed || first
		if first || (sk.repeat && d >= keyRepeatFirst && (d-keyRepeatFirst)%keyRepeatEvery < time.Second/time.Duration(ebiten.TPS())) {
			a.push(sk.code)
		}
	}
	if len(inpututil.AppendJustPressedKeys(nil)) > 0 {
		pressed = true
	}
	controller := a.Screen.InputMode == "controller"
	for _, r := range ebiten.AppendInputChars(nil) {
		if r < ' ' || r > '~' {
			continue
		}
		ch := string(r)
		// A key standing in for a controller button never repeats, as a button does not.
		if controller && !pressed && isButton(controllerKeyFor(ch)) {
			continue
		}
		a.push(ch)
	}
}

func isButton(k string) bool {
	return k == game.KeyA || k == game.KeyB || k == game.KeyX || k == game.KeyY
}

// push queues a key; during the boot hold it is the one that skips it.
func (a *App) push(key string) { a.k.Push(key, sourceKeyboard) }

func (a *App) readPads() {
	ids := ebiten.AppendGamepadIDs(nil)
	if len(ids) == 0 {
		return
	}
	pads := map[int]PadState{}
	for _, id := range ids {
		if !ebiten.IsStandardGamepadLayoutAvailable(id) {
			continue
		}
		b := func(x ebiten.StandardGamepadButton) bool { return ebiten.IsStandardGamepadButtonPressed(id, x) }
		ax := ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisLeftStickHorizontal)
		ay := ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisLeftStickVertical)
		pads[int(id)] = PadState{
			A: b(ebiten.StandardGamepadButtonRightBottom), B: b(ebiten.StandardGamepadButtonRightRight),
			X: b(ebiten.StandardGamepadButtonRightLeft), Y: b(ebiten.StandardGamepadButtonRightTop),
			Pause: b(ebiten.StandardGamepadButtonCenterRight) || b(ebiten.StandardGamepadButtonCenterLeft),
			Up:    b(ebiten.StandardGamepadButtonLeftTop) || ay < -0.5, Down: b(ebiten.StandardGamepadButtonLeftBottom) || ay > 0.5,
			Left: b(ebiten.StandardGamepadButtonLeftLeft) || ax < -0.5, Right: b(ebiten.StandardGamepadButtonLeftRight) || ax > 0.5,
		}
	}
	a.pads.Poll(time.Since(a.start), pads, a.k, func() { go a.Screen.useControllerLocked() })
}

// Draw shows the canvas, scaled to fit the window, and the scanlines.
func (a *App) Draw(screen *ebiten.Image) {
	if a.img == nil {
		b := a.Screen.c.Bounds()
		a.img = ebiten.NewImage(b.Dx(), b.Dy())
	}
	src := a.img
	if a.booting() && a.boot != nil {
		src = a.boot
	} else if a.Screen.Frame() {
		a.img.WritePixels(a.Screen.Canvas().Pix)
		a.Screen.Release()
	}
	sw, sh := screen.Bounds().Dx(), screen.Bounds().Dy()
	iw, ih := src.Bounds().Dx(), src.Bounds().Dy()
	scale := min(float64(sw)/float64(iw), float64(sh)/float64(ih))
	w, h := float64(iw)*scale, float64(ih)*scale
	x0, y0 := (float64(sw)-w)/2, (float64(sh)-h)/2
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(x0, y0)
	screen.DrawImage(src, op)
	if a.Screen.Scanlines {
		for _, band := range scanlineBands(int(h)) {
			vector.FillRect(screen, float32(x0), float32(y0)+float32(band[0]), float32(w), float32(band[1]),
				scanlineShade, false)
		}
	}
}

// Layout uses the window's real pixels, so the scanlines fall on them.
func (a *App) Layout(w, h int) (int, int) { return w, h }

// LayoutF is the window in device pixels.
func (a *App) LayoutF(w, h float64) (float64, float64) {
	s := ebiten.Monitor().DeviceScaleFactor()
	return w * s, h * s
}

// Run opens the window and runs until it closes or Quit.
func (a *App) Run(title string) error {
	b := a.Screen.c.Bounds()
	ebiten.SetWindowTitle(title)
	ebiten.SetWindowSize(b.Dx(), b.Dy())
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	return ebiten.RunGame(a)
}

// useControllerLocked switches to controller mode from the window's side.
func (s *Screen) useControllerLocked() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.UseController()
}

// scanlineShade darkens a band to scanlineLevel.
var scanlineShade = color.RGBA{A: uint8(math.Round(255 * (1 - scanlineLevel)))}
