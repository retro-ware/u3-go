package game

import (
	"fmt"
	"strings"
)

// Dungeons: a port of UltimaDngn.c. The party explores eight 16x16 levels in
// first person. RunDungeon is the dungeon's own command loop
// (DungeonStart()); BuildDungeonView decides what is visible (DrawDungeon()),
// for the UI to draw.

// Messages table entries the dungeon prints.
const (
	dungeonMsgPass         = 23
	dungeonMsgNoneLeft     = 67
	dungeonMsgDark         = 150
	dungeonMsgTimeLord     = 151
	dungeonMsgFountain     = 152
	dungeonMsgCant         = 153
	dungeonMsgYuck         = 154
	dungeonMsgWonderful    = 155
	dungeonMsgArgh         = 156
	dungeonMsgNice         = 157
	dungeonMsgStrangeWind  = 158
	dungeonMsgTrap         = 159
	dungeonMsgEvaded       = 160
	dungeonMsgRedHotRod    = 161
	dungeonMsgLeftAMark    = 162
	dungeonMsgGremlins     = 163
	dungeonMsgMistyWriting = 164
	dungeonMsgAdvance      = 165
	dungeonMsgRetreat      = 166
	dungeonMsgTurnRight    = 167
	dungeonMsgTurnLeft     = 168
	dungeonMsgDescend      = 169
	dungeonMsgKlimb        = 170
	dungeonMsgInvalidCmd   = 171
	dungeonMsgNotDngCmd    = 172
)

// dungeonIdlePassMs is the idle time before a turn passes by itself under the Fast timer.
const dungeonIdlePassMs = 6000

// Unit vectors for the four headings: north, east, south, west. (HeadX, HeadY)
var (
	HeadX = []int{0, 1, 0, -1}
	HeadY = []int{-1, 0, 1, 0}
)

func invalidCommand(io IO) {
	io.PrintMessage(dungeonMsgInvalidCmd)
	io.Sound(SoundError1)
}

func notDungeonCommand(io IO) {
	io.PrintMessage(dungeonMsgNotDngCmd)
	io.Sound(SoundError1)
}

func stepTo(world *World, io IO, heading, message int) {
	io.PrintMessage(message)
	xs := (world.X + HeadX[heading]) & 0x0f
	ys := (world.Y + HeadY[heading]) & 0x0f
	if world.GetXYDng(xs, ys) == DungeonCellWall {
		NoGo(io)
		return
	}
	world.X, world.Y = xs, ys
	io.RedrawMap()
}

// compassMsg is the compass direction messages by heading.
var compassMsg = []int{24, 26, 25, 27}

// StepCompass is a move in top-down mode (this port): the party turns to
// face the direction pressed, then steps. Inside a doorway, where turning is
// forbidden, only along the door works.
func StepCompass(world *World, io IO, heading int) {
	inDoorway := world.GetXYDng(world.X, world.Y) >= 0xa0
	if inDoorway && heading != world.Dungeon.Heading && heading != (world.Dungeon.Heading+2)&3 {
		io.PrintMessage(compassMsg[heading])
		NoGo(io)
		return
	}
	if !inDoorway {
		world.Dungeon.Heading = heading
	}
	stepTo(world, io, heading, compassMsg[heading])
}

// Forward mirrors Forward().
func Forward(world *World, io IO) { stepTo(world, io, world.Dungeon.Heading, dungeonMsgAdvance) }

// Retreat mirrors Retreat().
func Retreat(world *World, io IO) { stepTo(world, io, (world.Dungeon.Heading+2)&3, dungeonMsgRetreat) }

// Turn mirrors Right() / Left(). Turning is not possible inside a doorway.
func Turn(world *World, io IO, right bool) {
	if right {
		io.PrintMessage(dungeonMsgTurnRight)
	} else {
		io.PrintMessage(dungeonMsgTurnLeft)
	}
	if world.GetXYDng(world.X, world.Y) >= 0xa0 {
		NoGo(io)
		return
	}
	step := 3
	if right {
		step = 1
	}
	world.Dungeon.Heading = (world.Dungeon.Heading + step) & 3
	io.RedrawMap()
}

// Descend mirrors dDescend(): down a ladder.
func Descend(world *World, io IO) {
	io.PrintMessage(dungeonMsgDescend)
	cell := world.GetXYDng(world.X, world.Y)
	if cell > 127 || cell&DungeonCellLadderDown == 0 {
		invalidCommand(io)
		return
	}
	world.Dungeon.Level++
	io.RedrawMap()
}

// Klimb mirrors dKlimb(): up a ladder; from the top level this leaves the dungeon.
func Klimb(world *World, io IO) {
	io.PrintMessage(dungeonMsgKlimb)
	if world.GetXYDng(world.X, world.Y)&DungeonCellLadderUp == 0 {
		invalidCommand(io)
		return
	}
	world.Dungeon.Level--
	if world.Dungeon.Level >= 0 && world.Dungeon.Level < 8 {
		io.RedrawMap()
		return
	}
	world.Dungeon.Level = 0
	world.Dungeon.Exit = true
}

// peer mirrors dPeer(): a gem (the party's, this port) shows the level map.
func peer(world *World, io IO) {
	io.Print("Peer at gem!\n")
	if world.Party.Gems() < 1 {
		io.PrintMessage(dungeonMsgNoneLeft)
		return
	}
	world.Party.SetGems(world.Party.Gems() - 1)
	io.ShowMiniDungeon()
	io.ClearTiles()
}

// RunDungeon mirrors DungeonStart(0): play in the dungeon until the party
// leaves. The caller returns the party to Sosaria afterwards.
func RunDungeon(world *World, io IO) {
	d := &world.Dungeon
	d.Torch = 0
	d.Exit = false
	world.Music = MusicDungeon
	io.Music(MusicDungeon)

	for !d.Exit && !world.Done {
		CheckAllDead(world, io)
		if world.Resurrecting {
			return
		}
		io.ShowMoons()
		io.ShowWind()
		if d.Torch == 0 {
			io.PrintMessage(dungeonMsgDark)
			io.ClearTiles()
		} else {
			// With light, the auto-map learns the 3x3 around the party.
			world.AutoMap.Mark(world.Current.ID, d.Level, world.X, world.Y)
		}
		io.RedrawMap()
		io.Print(" ")
		io.Prompt()

		key := io.WaitCommand(ScopeDungeon, world.TimeLimit(dungeonIdlePassMs))
		if key == "" {
			key = KeySpace
		}
		if world.Done {
			return
		}
		switch {
		case key == KeyEscape || key == KeyPause:
			io.ShowPause() // the game stops where it stands; no turn passes
			continue
		case key == "J" || key == "j":
			io.ShowJournal() // no turn passes
			continue
		case key == ViewMapKey:
			io.ShowMap() // the level's auto-map, and Sosaria; no turn passes
			continue
		}
		before := fmt.Sprint(d.Level, world.X, world.Y)
		world.CommandCancelled = false
		dungeonDispatch(world, io, key)
		arrived := fmt.Sprint(d.Level, world.X, world.Y) != before
		if world.Resurrecting || d.Exit {
			return
		}
		if world.TakeCancelled() {
			continue // backed out at a prompt: no turn passes
		}

		// End of turn. (dungeonmech)
		if d.Level < 0 {
			d.Exit = true
			return
		}
		AgeChars(world, io)
		io.UpdateStats(false)
		if d.Torch > 0 {
			d.Torch--
		}

		cell := world.GetXYDng(world.X, world.Y)
		if cell == DungeonCellOpen {
			// Random encounters get likelier on deeper levels.
			if world.Rng.Range(0, 0x82+d.Level) < 128 {
				continue
			}
			typ := min(6, world.Rng.Range(0, d.Level+2)) + 0x18
			world.PutXYDng(DungeonCellChest, world.X, world.Y)
			Combat(world, io, typ*2, 0)
			io.ClearTiles()
			continue
		}
		encounter(world, io, cell, arrived)
	}
}

// ToggleMap is L: change the dungeon's view between first person, first
// person with a piece of the auto-map, and top down. A display change; the
// loop passes a turn anyway, as any dungeon command does.
func ToggleMap(world *World, io IO) {
	world.MapMode = NextMapMode(world.MapMode)
	io.Print("View:\n" + ViewNames[world.MapMode] + "\n")
	if world.OnMapModeChange != nil {
		world.OnMapModeChange()
	}
	io.RedrawMap()
}

// dungeonDispatch is the dungeon key switch. Letters not usable underground say so.
func dungeonDispatch(world *World, io IO, key string) {
	full := world.MapMode == MapModeFull
	switch strings.ToUpper(key) {
	case KeySpace:
		io.PrintMessage(dungeonMsgPass)
	// With the whole level on screen the arrows are compass moves; otherwise turn and advance.
	case KeyLeft, "4":
		if full {
			StepCompass(world, io, 3)
		} else {
			Turn(world, io, false)
		}
	case KeyRight, "6":
		if full {
			StepCompass(world, io, 1)
		} else {
			Turn(world, io, true)
		}
	case KeyUp, "8":
		if full {
			StepCompass(world, io, 0)
		} else {
			Forward(world, io)
		}
	case KeyDown, "2":
		if full {
			StepCompass(world, io, 2)
		} else {
			Retreat(world, io)
		}
	case "C":
		Cast(world, io, -1)
	case QuickCastKey:
		if !QuickHeal(world, io) {
			What2(io)
		}
	case SafeChestKey:
		if !QuickSafeChest(world, io) {
			What2(io)
		}
	case LightKey:
		if !QuickLight(world, io) {
			What2(io)
		}
	case "D":
		Descend(world, io)
	case "G":
		GetChest(world, io, 0, "command")
	case "I":
		IgniteTorch(world, io)
	case "K":
		Klimb(world, io)
	case "L":
		ToggleMap(world, io)
	case "M":
		ModifyOrder(world, io)
	case "N":
		NegateTime(world, io)
	case "O":
		OtherCommand(world, io, false)
	case "P":
		peer(world, io)
	case "R":
		ReadyWeapon(world, io, -1)
	case "V":
		Volume(world, io)
	case "W":
		WearArmour(world, io)
	case "Y":
		Yell(world, io)
	case "Z":
		Stats(world, io, -1)
	default:
		if len(key) == 1 && (key[0] >= 'A' && key[0] <= 'Z' || key[0] >= 'a' && key[0] <= 'z') {
			notDungeonCommand(io)
		} else {
			What2(io)
		}
	}
}

// encounter is what the party's cell does at the end of a turn (dngnotcombat).
// The Time Lord and the fountain speak only when the party has just arrived,
// so turning or waiting does not bring the prompt back; the rest keep the
// original's every-turn behaviour.
func encounter(world *World, io IO, cell int, arrived bool) {
	d := &world.Dungeon
	switch cell {
	case DungeonCellTimeLord:
		if !arrived {
			return
		}
		io.ShowImage("TimeLord")
		io.Music(MusicShrine)
		io.PrintMessage(dungeonMsgTimeLord)
		io.WaitKey()
		io.Print("\n")
		io.RedrawMap()
		io.Music(MusicDungeon)
		world.Journal.TimeLord = true
		HearClue(world, "TimeLord")
		JournalCheck(world, io)

	case DungeonCellFountain:
		if !arrived {
			return
		}
		io.ShowImage("Fountain")
		io.Music(MusicShrine)
		for {
			io.PrintMessage(dungeonMsgFountain)
			n := io.ChooseMember(nil)
			if n < 1 || n > 4 {
				break
			}
			if !world.MemberAlive(n - 1) {
				io.PrintMessage(dungeonMsgCant)
				io.Sound(SoundError1)
				continue
			}
			p := world.Member(n - 1)
			switch world.X & 0x03 {
			case 0: // poison
				p.SetStatus('P')
				io.PrintMessage(dungeonMsgYuck)
				io.FlashMember(n - 1)
				io.Sound(SoundHit)
			case 1: // heal
				p.SetHitPoints(p.MaxHitPoints())
				io.PrintMessage(dungeonMsgWonderful)
			case 2: // damage
				io.PrintMessage(dungeonMsgArgh)
				if p.SubtractHitPoints(25) {
					io.Sound(DeathSound(p.Sex()))
				}
				io.FlashTiles()
				io.FlashMember(n - 1)
				io.Sound(SoundHit)
			default: // cure
				io.PrintMessage(dungeonMsgNice)
				if p.Status() == 'P' {
					p.SetStatus('G')
				}
			}
			io.UpdateStats(false)
		}
		io.Music(MusicDungeon)
		io.RedrawMap()

	case DungeonCellWind:
		io.PrintMessage(dungeonMsgStrangeWind)
		d.Torch = 0

	case DungeonCellTrap:
		world.PutXYDng(0, world.X, world.Y)
		io.PrintMessage(dungeonMsgTrap)
		io.Sound(SoundStep)
		if StealDisarmFails(world, world.Member(0)) {
			io.PrintMessage(dungeonMsgEvaded)
			return
		}
		BombTrap(world, io)

	case DungeonCellMark:
		io.ShowImage("Rod")
		io.Music(MusicShrine)
		io.PrintMessage(dungeonMsgRedHotRod)
		n := io.ChooseMember(nil)
		if n >= 1 && n <= 4 {
			p := world.Member(n - 1)
			p.Bytes[14] |= 1 << ((world.X & 3) + 4)
			RodFound(world, world.Current.ID, world.X)
			io.FlashMember(n - 1)
			io.Sound(SoundHit)
			if p.SubtractHitPoints(50) {
				io.Sound(DeathSound(p.Sex()))
			}
			io.PrintMessage(dungeonMsgLeftAMark)
		}
		io.UpdateStats(false)
		io.Music(MusicDungeon)
		io.RedrawMap()
		JournalCheck(world, io)

	case DungeonCellGremlins:
		world.PutXYDng(0, world.X, world.Y)
		// Gremlins steal 100 food (from the party's pool in this port).
		world.Party.SetFoodHundredths(max(0, world.Party.FoodHundredths()-10000))
		io.Sound(SoundOuch)
		io.PrintMessage(dungeonMsgGremlins)
		io.UpdateStats(false)

	case DungeonCellWriting:
		io.PrintMessage(dungeonMsgMistyWriting)
		io.Print(Speech(world.Current.Talk, d.Level+1))
		io.Print("\n")
		if ReadWriting(world, world.Current.ID, d.Level) {
			JournalCheck(world, io)
		}
	}
}

// DungeonDrawKind is what a DungeonDrawOp draws.
type DungeonDrawKind string

const (
	DrawWall   DungeonDrawKind = "wall"
	DrawDoor   DungeonDrawKind = "door"
	DrawLadder DungeonDrawKind = "ladder"
	DrawChest  DungeonDrawKind = "chest"
)

// DungeonDrawOp is one thing to draw in the first-person view, in drawing order.
type DungeonDrawOp struct {
	Kind     DungeonDrawKind
	Location int  // view location 0..31
	Down     bool // ladders: down rather than up
}

// Relative positions of the 32 view locations before rotating for the
// heading: 0 the party's cell, 1 and 2 left and right, 3..5 one step ahead,
// 6..9 two steps, and so on. (offsetX, offsetY in DungeonBlock())
var (
	dungeonOffsetX = []int{0, -1, 1, -1, 0, 1, -2, -1, 1, 2, -2, -1, 0, 1, 2, -3, -2, -1, 1, 2, 3, -3, -2, -1, 0, 1, 2, 3, -2, -1, 1, 2}
	dungeonOffsetY = []int{0, 0, 0, -1, -1, -1, -1, -1, -1, -1, -2, -2, -2, -2, -2, -2, -2, -2, -2, -2, -2, -3, -3, -3, -3, -3, -3, -3, -3, -3, -3, -3}
)

// CellAtLocation is the dungeon cell at a view location, turned to face the heading.
func CellAtLocation(world *World, location int) int {
	dx, dy := dungeonOffsetX[location], dungeonOffsetY[location]
	for h := world.Dungeon.Heading; h > 0; h-- {
		dx, dy = -dy, dx
	}
	return world.GetXYDng(world.X+dx, world.Y+dy)
}

// BuildDungeonView mirrors DrawDungeon(): decide what is visible, walking
// the view outward and stopping each line of sight at the first wall. block
// reports 0 for open floor, 255 for a wall and 1 for a door directly ahead,
// like DungeonBlock().
func BuildDungeonView(world *World) []DungeonDrawOp {
	var ops []DungeonDrawOp
	block := func(location int) int {
		cell := CellAtLocation(world, location)
		if cell < 128 {
			if cell&DungeonCellLadderUp != 0 {
				ops = append(ops, DungeonDrawOp{DrawLadder, location, false})
			}
			if cell&DungeonCellLadderDown != 0 {
				ops = append(ops, DungeonDrawOp{DrawLadder, location, true})
			}
			if cell&DungeonCellChest != 0 {
				ops = append(ops, DungeonDrawOp{Kind: DrawChest, Location: location})
			}
			return 0
		}
		ops = append(ops, DungeonDrawOp{Kind: DrawWall, Location: location})
		if cell < 0xc0 {
			return 255
		}
		ops = append(ops, DungeonDrawOp{Kind: DrawDoor, Location: location})
		if location == 0 {
			return 1
		}
		return 255
	}
	// wall reports whether any of the locations, tried in order, blocks (as || short-circuits).
	wall := func(locations ...int) bool {
		for _, l := range locations {
			if block(l) > 127 {
				return true
			}
		}
		return false
	}

	here := block(0)
	if here == 0 {
		// Left side, then right side, one step at a time.
		if !wall(1, 3, 6, 10, 15) {
			block(21)
		}
		if !wall(2, 5, 9, 14, 20) {
			block(27)
		}
	}
	if (here == 0 || here == 1) && block(4) == 0 {
		if !wall(7, 11, 16) {
			block(22)
		}
		if !wall(8, 13, 19) {
			block(26)
		}
		if block(12) == 0 {
			if !wall(17, 23) {
				block(28)
			}
			if !wall(18) {
				block(25)
			}
			if !wall(24) {
				block(29)
				block(30)
			}
		}
	}
	return ops
}

// SecretMessage is the misty writing carved on the deepest levels, or "". (DrawSecretMessage)
func SecretMessage(world *World) string {
	d := world.Dungeon
	index := world.Current.ID - 412
	which := 0
	if d.Level == 6 && world.X == 1 && world.Y == 1 && d.Heading == 3 {
		which = index*3 + 1
	}
	if d.Level == 6 && world.X == 1 && world.Y == 15 && d.Heading == 2 {
		which = index*3 + 2
	}
	if d.Level == 7 && world.X == 15 && world.Y == 15 && d.Heading == 1 {
		which = index*3 + 3
	}
	if which == 0 {
		return ""
	}
	if m := world.Resources.Strings["Messages"]; which-1 < len(m) {
		return m[which-1]
	}
	return ""
}
