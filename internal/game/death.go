package game

// What happens when the whole party is dead: CheckAllDead() from
// UltimaMain.c. The Apple II resurrected the party at Lord British's castle;
// this port offers going back to the last save (the game saves at every
// door) or Lord British's mercy with nothing but daggers and cloth.

const (
	deathMsgAllOut       = 109
	deathMsgResurrecting = 249
)

// CheckAllDead announces a dead party, waits for a key and rebuilds the
// party on Sosaria. It sets World.Resurrecting so the combat and dungeon
// loops unwind to the main loop.
func CheckAllDead(w *World, io IO) {
	for m := range 4 {
		if w.MemberAlive(m) {
			return
		}
	}
	io.PrintMessage(deathMsgAllOut)
	w.Music = MusicNone
	io.Music(MusicNone)
	io.FlushKeys()
	io.WaitKey()

	if w.LoadLastSave != nil {
		choice := io.ChooseFromList([]MenuOption{
			{Key: "T", Label: "Try again from last save"},
			{Key: "F", Label: "Flee to Lord British, lose gear"},
		}, &MenuPlacement{Row: 9, Title: "All players out!"})
		if choice != "F" && w.LoadLastSave() {
			// Back on the surface where the save was made, everything since undone.
			w.Combat = nil
			w.TimeNegate = 0
			w.Music = MusicSosaria
			io.Music(MusicSosaria)
			io.Print("\n\n\n\n\n\n\n\n")
			io.ShowWind()
			io.UpdateStats(true)
			w.Resurrecting = true
			return
		}
	}

	io.PrintMessage(deathMsgResurrecting)
	io.Sound(SoundBigDeath)
	io.Pause(1500)

	// The party starts over with 150 gold and at least 100 food per member (pooled).
	w.Party.SetGold(150 * w.Party.Size())
	w.Party.SetFood(max(w.Party.Food(), 100*w.Party.Size()))
	for _, m := range w.memberIndices() {
		p := w.Member(m)
		clear(p.Bytes[35:64]) // gold (unused now), gems, keys, powders, armour, weapons
		p.SetTorches(0)
		p.Bytes[40] = 1 // cloth armour worn
		p.Bytes[48] = 1 // dagger in hand
		p.SetStatus('G')
		p.SetHitPoints(100)
	}
	w.Party.ClearGear() // the bag is lost too
	w.Party.ClearSupplies()
	w.Combat = nil
	w.Party.SetLocation(LocationSosaria)
	w.Party.SetShape(0x7e)
	w.X, w.ReturnX = 42, 42
	w.Y, w.ReturnY = 20, 20
	w.Party.SetSurfaceX(42)
	w.Party.SetSurfaceY(20)
	w.ResetSosaria()
	w.TimeNegate = 0
	w.Music = MusicSosaria
	io.Music(MusicSosaria)
	io.UpdateStats(true)
	io.Print("\n\n\n\n\n\n\n\n")
	io.Sound(SoundBigDeath)
	w.Resurrecting = true
}
