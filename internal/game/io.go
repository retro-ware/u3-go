package game

// The boundary between the game logic and the user interface. Game code talks
// to the screen, keyboard and speaker only through IO, which the UI
// implements. The original was written in a blocking style (WaitKeyMouse()
// spun until a key arrived), the TypeScript port made every wait a Promise;
// here the game runs in its own goroutine and every IO method that waits
// simply blocks until the UI has an answer (see doc.go).

// Single-character key codes used throughout the game logic.
const (
	KeyLeft      = "\x1c"
	KeyRight     = "\x1d"
	KeyUp        = "\x1e"
	KeyDown      = "\x1f"
	KeyEnter     = "\r"
	KeyBackspace = "\b"
	KeyEscape    = "\x1b"
	KeySpace     = " "
	// Controller buttons (also produced by Enter/Escape/Z/X/C/V in controller mode).
	KeyA = "\x01"
	KeyB = "\x02"
	KeyX = "\x03"
	KeyY = "\x04"
	// The controller's system buttons, the pause button, and a window that loses focus.
	KeyPause = "\x05"
)

// MenuPlacement says where a controller-mode menu goes when the screen has
// text laid out for it (the title screens): its top row and title.
type MenuPlacement struct {
	Row    int
	Title  string
	Cursor int // item to highlight first
}

// MenuOption is one choice in a menu or a letter prompt.
type MenuOption struct {
	Key        string // the key the keyboard user presses
	Label      string // what controller users see
	Hidden     bool   // accepted from the keyboard but not shown in the menu
	Hint       string // a line shown under the menu while highlighted (title screens)
	HintColour string // the hint's colour (CSS-style name or #rgb); "" for plain
	Disabled   bool   // shown greyed and not selectable
	Colour     string // the label's colour; "" for plain
	Card       *Card  // shown over the party panel while highlighted
}

// CardTone is how a card's line reads.
type CardTone string

const (
	CardToneNone   CardTone = ""
	CardToneBetter CardTone = "better"
	CardToneWorse  CardTone = "worse"
	CardToneDim    CardTone = "dim"
)

// CardLine is one line of a Card.
type CardLine struct {
	Text string
	Tone CardTone
}

// Card is shown over the party panel while a list is open: an armament
// weighed against what the member has, or a spell.
type Card struct {
	Title string
	Lines []CardLine
}

// A card's width and height in letters.
const (
	CardWidth = 15
	CardRows  = 11
)

// CommandScope is where a top-level command is being read; it decides which
// command menu a controller sees.
type CommandScope string

const (
	ScopeField   CommandScope = "field"
	ScopeCombat  CommandScope = "combat"
	ScopeDungeon CommandScope = "dungeon"
)

// Music tracks, numbered as gSongNext was in the C source.
const (
	MusicNone         = 0
	MusicSosaria      = 1
	MusicTown         = 2
	MusicCastle       = 3
	MusicDungeon      = 4
	MusicCombat       = 5
	MusicShop         = 6
	MusicExodusCastle = 7
	MusicLordBritish  = 8
	MusicShrine       = 10
	MusicAmbrosia     = 11
)

// SpellFlash is where a spell's flash lands.
type SpellFlash string

const (
	FlashCaster SpellFlash = "caster"
	FlashTarget SpellFlash = "target"
	FlashWindow SpellFlash = "window"
)

// Echo says what ChooseOption prints of the answer.
type Echo string

const (
	EchoNone Echo = "none" // nothing
	EchoKey  Echo = "key"  // the key
	EchoLine Echo = "line" // the key and a newline
)

// IO is what the UI implements. Methods that wait block the game goroutine.
type IO interface {
	// Print prints to the scrolling message area; "\n" starts a new line. (UPrintWin)
	Print(text string)
	// PrintMessage prints entry n (1-based) of the Messages string table. (UPrintMessage)
	PrintMessage(n int)
	// Prompt draws the command prompt on the bottom line. (DrawPrompt)
	Prompt()

	// WaitKey waits for the next key press. (WaitKeyMouse)
	WaitKey() string
	// WaitKeyOrTimeout waits for a key, or returns "" after ms milliseconds.
	WaitKeyOrTimeout(ms int) string
	// FlushKeys discards queued key presses. (FlushEvents)
	FlushKeys()
	// QueueKeys replaces the scripted keys, read before any real input as
	// Apple II keys whatever the input mode. Auto-combat types a member's
	// turn this way. (Macro[] / AddMacro)
	QueueKeys(keys []string)

	// WaitCommand reads the next top-level command: a movement key, a
	// command letter, or "" when timeoutMs passes with no input (the turn
	// then passes). timeoutMs 0 waits for ever.
	WaitCommand(scope CommandScope, timeoutMs int) string
	// ChooseMember answers a "whom?" prompt: 1..4, or 0 when cancelled. It
	// echoes the answer. only (0-based members) limits a controller's menu;
	// nil lists everyone. (GetChar)
	ChooseMember(only []int) int
	// ChooseDirection answers a "Direction-" prompt: a movement key, the
	// space bar when allowNone, or "" when cancelled.
	ChooseDirection(allowNone, allowDiagonal bool) string
	// ChooseOption picks one option: its key, or "" when cancelled.
	ChooseOption(options []MenuOption, echo Echo, place *MenuPlacement) string
	// ShowSettings shows the Settings menu until closed.
	ShowSettings()
	// ShowPause shows the Pause menu (the game stops, timers and music with
	// it) until Resume. It returns a command key chosen there to run now
	// ("Q", quit and save), or "".
	ShowPause() string
	// ShowJournal shows the quest journal over the map. No turn passes.
	ShowJournal()
	// ShowMap shows the map of what the party has seen (worldmap.go) until closed.
	ShowMap()
	// ChooseFromList picks from a list shown as a menu in every input mode
	// (the title and party screens): the option's key, or "" when cancelled.
	ChooseFromList(options []MenuOption, place *MenuPlacement) string
	// AllocatePoints shares total points among the labels, each between min
	// and max: the values, or nil when cancelled.
	AllocatePoints(labels []string, total, min, max int, place *MenuPlacement) []int
	// InputText reads a line of text at the cursor, echoing it. words offers
	// ready-made answers to a controller. (UInputText)
	InputText(maxChars int, numbersOnly bool, words []string) string

	// Sound plays an effect by name (the Sound* constants). (PlaySoundFile)
	Sound(name string)
	// Music selects the background track (gSongNext); may do nothing.
	Music(track int)

	// RedrawMap redraws the viewport: map, combat arena or dungeon. (DrawMap)
	RedrawMap()
	// FlashTiles briefly inverts the viewport. (InverseTiles)
	FlashTiles()
	// FlashSpell flashes a spell being cast and plays its sound. (Flashriek)
	FlashSpell(kind SpellFlash, member int, sound string)
	// FlashMember briefly highlights one member's stats box. (InverseChar)
	FlashMember(member int)
	// HighlightMember keeps a member's box highlighted (combat turn marker). (InverseChnum)
	HighlightMember(member int, on bool)
	// UpdateStats redraws the stats boxes if anything changed. (ShowChars)
	UpdateStats(force bool)
	// ShowWind redraws the wind indicator. (ShowWind)
	ShowWind()
	// ShowMoons redraws the moon phases. (DrawMoonGateStuff)
	ShowMoons()
	// ClearTiles blacks out the viewport. (ClearTiles)
	ClearTiles()

	// ShowImage shows a full-viewport picture: "Fountain", "Rod", "Shrine", "TimeLord". (ImageDisplay)
	ShowImage(name string)
	// ShowMiniMap draws the whole map small until a key is pressed. (DrawMiniMap)
	ShowMiniMap()
	// ShowMiniDungeon draws the dungeon level as a map until a key. (DrawMiniDng)
	ShowMiniDungeon()
	// PlayEnding plays the end-of-game flash and scrolling text.
	PlayEnding()

	// Pause waits ms milliseconds while animations run. (IdleUntil)
	Pause(ms int)

	// ShowTitle draws the plain border and the Exodus picture.
	ShowTitle()
	// ShowGameFrame draws the in-game border with the stats boxes.
	ShowGameFrame()
	// ClearBottom blacks out the lower title screen. (ClearBottom)
	ClearBottom()
	// TextAt draws text at a cell position, outside the message area. (UPrint)
	TextAt(x, y int, text string)
	// CentreText draws text centred on the 40-column screen. (CenterMessage)
	CentreText(y int, text string)
	// InputTextAt reads a line of text at a cell position.
	InputTextAt(x, y, maxChars int, numbersOnly bool) string
}

// LetterOptions is one option per letter, labels from labels.
func LetterOptions(letters string, labels func(letter string) string) []MenuOption {
	var out []MenuOption
	for _, r := range letters {
		out = append(out, MenuOption{Key: string(r), Label: labels(string(r))})
	}
	return out
}

// YesNo asks yes or no; true for Y. The caller prints the newline.
func YesNo(io IO) bool {
	return io.ChooseOption([]MenuOption{{Key: "Y", Label: "Yes"}, {Key: "N", Label: "No"}}, EchoNone, nil) == "Y"
}

// InputNumber reads a number of up to maxDigits digits. (UInputNum)
func InputNumber(io IO, maxDigits int) int {
	text := io.InputText(maxDigits, true, nil)
	n := 0
	for i, c := range text {
		if c == '-' && i == 0 {
			continue
		}
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// Sound effect names.
const (
	SoundStep          = "Step"
	SoundHorseWalk     = "HorseWalk"
	SoundBump          = "Bump"
	SoundMountHorse    = "MountHorse"
	SoundHit           = "Hit"
	SoundAttack        = "Attack"
	SoundForceField    = "ForceField"
	SoundMoongate      = "Moongate"
	SoundSink          = "Sink"
	SoundShoot         = "Shoot"
	SoundError1        = "Error1"
	SoundError2        = "Error2"
	SoundDeathMale     = "DeathMale"
	SoundDeathFemale   = "DeathFemale"
	SoundBigDeath      = "BigDeath"
	SoundTorchIgnite   = "TorchIgnite"
	SoundCreak         = "Creak"
	SoundOuch          = "Ouch"
	SoundAlarm         = "Alarm"
	SoundCombatStart   = "CombatStart"
	SoundCombatVictory = "CombatVictory"
	SoundFailedSpell   = "FailedSpell"
	SoundImmolate      = "Immolate"
	SoundDownwards     = "Downwards"
	SoundUpwards       = "Upwards"
	SoundInvocation    = "Invocation"
	SoundHeal          = "Heal"
	SoundMiscSpell     = "MiscSpell"
	SoundMonsterSpell  = "MonsterSpell"
	SoundShrine        = "Shrine"
	SoundLBLevelRise   = "LBLevelRise"
	SoundExpLevelUp    = "ExpLevelUp"
)

// SoundSwish are the four swish variants.
var SoundSwish = []string{"Swish1", "Swish2", "Swish3", "Swish4"}

// DeathSound is the death sound for a character's sex letter.
func DeathSound(sex byte) string {
	if sex == 'F' {
		return SoundDeathFemale
	}
	return SoundDeathMale
}
