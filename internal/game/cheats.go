package game

import (
	"fmt"
	"strings"
)

// The cheat menu behind Y on the help screen (the TypeScript port's; none of
// this existed in the original). Each cheat is a small change to the world.

// Cheat is one entry of the menu.
type Cheat struct {
	Key   string
	Label string
	// Available says whether it makes sense now (Exit dungeon only in a dungeon).
	Available func(w *World) bool
	// Apply applies it and returns a one-line confirmation.
	Apply func(w *World, io IO) string
}

// Lord British's doorstep, where a new party starts.
const (
	homeX = 42
	homeY = 20
)

// leaveDungeon asks the dungeon loop to end; the game returns the party to
// ReturnX/Y, the dungeon's square unless a cheat moved it.
func leaveDungeon(w *World) { w.Dungeon.Exit = true }

func always(*World) bool { return true }

// eachMember runs f for every filled party position.
func eachMember(w *World, f func(p *PlayerRecord)) {
	for m := range 4 {
		if w.Party.MemberSlot(m) >= 0 {
			f(w.Member(m))
		}
	}
}

// Cheats is the menu in order.
var Cheats = []Cheat{
	{"R", "Full restore", always, func(w *World, _ IO) string {
		eachMember(w, func(p *PlayerRecord) {
			p.SetStatus('G')
			p.SetHitPoints(p.MaxHitPoints())
		})
		return "Everyone is whole again."
	}},
	{"L", "Raise every level", always, func(w *World, io IO) string {
		// What an audience with Lord British gives, to everyone at once:
		// a level's experience and a hundred more hit points.
		eachMember(w, func(p *PlayerRecord) {
			exp := min(9899, int(p.Bytes[30])*100+int(p.Bytes[31])+100)
			p.Bytes[30], p.Bytes[31] = byte(exp/100), byte(exp%100)
			newMax := min(9950, p.MaxHitPoints()+100)
			p.Bytes[28], p.Bytes[29] = byte(newMax/256), byte(newMax%256)
			p.SetHitPoints(newMax)
		})
		io.UpdateStats(false)
		return "Everyone is a level greater."
	}},
	{"H", "Go home (Lord British)", func(w *World) bool { return w.Party.Location() != LocationCombat }, func(w *World, io IO) string {
		switch {
		case w.Party.Location() == LocationDungeon:
			w.ReturnX, w.ReturnY = homeX, homeY
			leaveDungeon(w)
		case w.InTownOrCastle():
			w.ReturnX, w.ReturnY = homeX, homeY
			ExitToSurface(w, io, nil)
		default:
			w.X, w.Y = homeX, homeY
			w.Party.SetSurfaceX(homeX)
			w.Party.SetSurfaceY(homeY)
			w.Party.SetShape(0x7e) // on foot: a horse or ship stays behind
		}
		return "Back at the castle gate."
	}},
	{"X", "Exit dungeon", func(w *World) bool { return w.Party.Location() == LocationDungeon }, func(w *World, _ IO) string {
		leaveDungeon(w)
		return "Up and out."
	}},
	{"G", "Get 100 gold", always, func(w *World, _ IO) string {
		w.Party.SetGold(w.Party.Gold() + 100)
		return fmt.Sprintf("Gold: %d.", w.Party.Gold())
	}},
	{"F", "Get 100 food", always, func(w *World, _ IO) string {
		w.AddFood(100)
		return fmt.Sprintf("Food: %d.", w.Party.Food())
	}},
	{"M", "Get 10 gems", always, func(w *World, _ IO) string {
		w.Party.SetGems(w.Party.Gems() + 10)
		return fmt.Sprintf("Gems: %d.", w.Party.Gems())
	}},
	{"K", "Get 5 keys", always, func(w *World, _ IO) string {
		w.Party.SetKeys(w.Party.Keys() + 5)
		return fmt.Sprintf("Keys: %d.", w.Party.Keys())
	}},
	{"T", "Get 5 torches", always, func(w *World, _ IO) string {
		w.Party.SetTorches(w.Party.Torches() + 5)
		return fmt.Sprintf("Torches: %d.", w.Party.Torches())
	}},
	{"W", "Add word", always, func(w *World, io IO) string {
		// A word for Other learnt early, as if heard said (LearnWord), so a
		// controller's Other offers it. Only a word Other offers is kept.
		io.Print("\nWord: ")
		typed := strings.ToUpper(strings.TrimSpace(io.InputText(8, false, nil)))
		io.Print("\n")
		if typed == "" {
			return "No word added."
		}
		switch LearnWord(w, typed) {
		case "unknown":
			return "Other offers no word " + typed + "."
		case "known":
			return typed + " is known already."
		}
		JournalCheck(w, io)
		return "Known now: " + typed + ". Other offers it from now on."
	}},
}
