// Package res holds Ultima III's game data in the shape the engine uses: the
// LairWare Macintosh resources, numbered from BASERES = 400. Both importers
// (the LairWare Mac files and the PC release's .ULT files) produce it.
package res

const BaseRes = 400

// Map resource IDs, as LoadUltimaMap() in UltimaMisc.c numbers them.
const (
	LordBritishCastle = 400
	ExodusCastle      = 401
	FirstTown         = 402
	FirstDungeon      = 412
	LastDungeon       = 418
	Sosaria           = 420
	Ambrosia          = 421
)

// Misc holds the MISC 400-405 lookup tables (GetMiscStuff() in UltimaMisc.c).
type Misc struct {
	MoonX, MoonY         []byte // moongate positions on Sosaria by Trammel phase (8 each)
	Career               []byte // class letters F C W T P B L I D A R (11)
	WeaponUse, ArmourUse []byte // highest weapon/armour letter per class (11)
	LocationX, LocationY []byte // the enterable locations by map id - 400 (32 each, 0xFF unused)
	Experience           []byte // experience per monster tier (16)
}

// Resources is the whole game's data.
type Resources struct {
	Maps, Monsters, Talk, Combat map[int][]byte
	MapNames                     map[int]string
	Misc                         Misc
	DefaultParty, DefaultRoster  []byte
	Strings                      map[string][]string
}
