// Command u3 runs Ultima III from either release's data.
//
//	u3                          play: reads ./data, keeps saves and settings in ./save
//	u3 -import SRC [-data DIR]  copy the game files out of SRC (LairWare's .dmg, app or
//	                            folder; the PC release's disk image, zip or files) into DIR,
//	                            write DIR/COVER.PNG and exit
//	u3 -shot FILE [-keys K]     headless check: play the scripted keys K (see
//	                            ui.ParseScript), then write the screen to FILE and exit
//	u3 -tiles FILE [-set NAME]  write a tile set's tiles to FILE and exit
package main

import (
	"errors"
	"flag"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"time"

	"github.com/hajimehoshi/ebiten/v2/audio"

	"github.com/retro-ware/u3-go/internal/game"
	"github.com/retro-ware/u3-go/internal/res"
	"github.com/retro-ware/u3-go/internal/ui"
)

func main() {
	data := flag.String("data", "data", "directory with the imported game files")
	saveDir := flag.String("save", "save", "directory for the saved game and settings")
	imp := flag.String("import", "", "import the game files from this folder/zip/disk image/DMG into -data and exit")
	shot := flag.String("shot", "", "headless: play -keys, write the screen to this PNG and exit")
	keys := flag.String("keys", "", "scripted keys for -shot: characters, and {up} {down} {left} {right} {enter} {esc} {space} {bs} {A} {B} {X} {Y} {pause} {wait}")
	tiles := flag.String("tiles", "", "write the tile set's tiles to this PNG and exit")
	set := flag.String("set", "", "tile set to use (default: the remembered one)")
	newGame := flag.Bool("new", false, "start without the saved game")
	flag.Parse()
	if *imp != "" {
		if err := importGame(*imp, *data); err != nil {
			fmt.Fprintln(os.Stderr, "import:", err)
			os.Exit(1)
		}
		return
	}
	if err := run(*data, *saveDir, *shot, *keys, *tiles, *set, *newGame); err != nil {
		fmt.Fprintln(os.Stderr, "u3:", err)
		os.Exit(1)
	}
}

func run(data, saveDir, shot, keys, tiles, set string, newGame bool) error {
	r, err := res.Load(data)
	if err != nil {
		return fmt.Errorf("no game files in %s (run u3 -import first): %w", data, err)
	}
	prefs := ui.LoadPrefs(saveDir)
	tileSet := prefs.Tiles
	if set != "" {
		tileSet = set
	}
	gfx, err := ui.LoadGraphics(tileSet, data)
	if err != nil {
		if gfx, err = ui.LoadGraphics("Standard", data); err != nil {
			return err
		}
	}
	if tiles != "" {
		return writePNG(tiles, gfx)
	}

	headless := shot != ""
	var ctx *audio.Context
	if !headless {
		ctx = audio.NewContext(44100)
	}
	w := game.NewWorld(r, 0)
	w.SetDiagonalMoves(false)
	store := game.DirStore(saveDir)
	if newGame || !game.ReadSave(store, w) {
		w.NewGame()
		w.AutoMap.Clear()
	}
	sounds := ui.NewSoundPlayer(ctx, data)
	music := ui.NewMusicPlayer(ctx, data)
	sounds.OnLong(music.Duck)

	var screen *ui.Screen
	var k *ui.Keyboard
	if headless {
		k = ui.NewScriptKeyboard(ui.ParseScript(keys), func() {
			if err := screen.Snapshot(shot); err != nil {
				fmt.Fprintln(os.Stderr, "u3: shot:", err)
				os.Exit(1)
			}
			os.Exit(0)
		})
	} else {
		k = ui.NewKeyboard()
	}
	screen = ui.NewScreen(32, gfx, data, k, sounds, music, w)
	prefs.Apply(w, screen, sounds, music)
	screen.TileSetName = gfx.Name
	remember := func() {
		if !headless {
			ui.CurrentPrefs(w, screen, sounds, music, screen.TileSetName).Save(saveDir)
		}
	}
	screen.OnSettingsChange, screen.OnModeChange = remember, remember
	w.OnMapModeChange, w.OnRulesChange, w.OnAutoCombatChange = remember, remember, remember

	app := ui.NewApp(screen, k, music)
	app.OnVolume = remember
	quit := func() {
		if headless {
			os.Exit(0)
		}
		app.Quit()
		select {} // the window closes; this goroutine goes with the process
	}
	play := func() {
		if !headless {
			app.BootHold(filepath.Join(data, "COVER.PNG"))
		}
		screen.Lock() // the game runs holding the screen, letting go while it waits
		g := game.NewGame(w, screen, game.GameOptions{
			Save: func(w *game.World) bool { return game.WriteSave(store, w) },
			Load: func(w *game.World) bool { return game.ReadSave(store, w) },
		})
		game.MainMenu(w, screen, g.Run, game.MenuOptions{
			Save:     func(w *game.World) { game.WriteSave(store, w) },
			Transfer: fileTransfer{w, store, saveDir},
			Quit:     quit,
		})
	}
	if headless {
		play()
		return nil
	}
	go play()
	return app.Run("Ultima III")
}

func writePNG(path string, gfx *ui.Graphics) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(f, ui.TileSheet(gfx, 32)); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// fileTransfer exports and imports the game as text files in the save
// directory; there is no clipboard without cgo.
type fileTransfer struct {
	w     *game.World
	store game.SaveStore
	dir   string
}

// importFile is where Import from a file looks.
const importFile = "import.json"

func (t fileTransfer) Export() string {
	if !t.w.Party.Formed() {
		return ""
	}
	return game.ExportGame(t.w)
}

func (t fileTransfer) Import(text string) string {
	e, err := game.ParseExport(text)
	if err != nil {
		return err.Error()
	}
	if !game.RestoreExported(t.w, e) {
		return "the save does not fit"
	}
	game.WriteSave(t.store, t.w)
	return ""
}

var errNoClipboard = errors.New("no clipboard")

func (fileTransfer) ClipboardWrite(string) error    { return errNoClipboard }
func (fileTransfer) ClipboardRead() (string, error) { return "", errNoClipboard }

// FileSave writes save/ultima3-YYYYMMDDHHMM.json.
func (t fileTransfer) FileSave(text string) error {
	return game.DirStore(t.dir).Store("ultima3-"+time.Now().Format("200601021504")+".json", []byte(text))
}

// FileLoad reads save/import.json.
func (t fileTransfer) FileLoad() (string, error) {
	b, err := os.ReadFile(filepath.Join(t.dir, importFile))
	return string(b), err
}
