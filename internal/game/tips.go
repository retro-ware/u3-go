package game

import "math"

// Pointers for a new player, the TypeScript port's own: how the game works,
// without the quest's answers (those are the journal's). Tips, in the Pause
// menu, lists the topics; A opens one to its text, a page at a time.

// Tip is a topic: its heading and its paragraphs.
type Tip struct {
	Topic string
	Text  []string
}

// Tips are the topics in menu order.
var Tips = []Tip{
	{"First steps", []string{
		"A new party starts next to Lord British's castle and the town of Britain.",
		"Go visit Lord British first!",
		"Walk into townsfolk to talk. Notes are saved to the Journal.",
	}},
	{"Levels", []string{
		"Fights give experience. A level is earned every 100 XP.",
		"A character with an L or blue name can level up - go visit Lord British!",
		"Levels increase max hit points by 100.",
		"Abilities are raised at shrines, not by leveling.",
	}},
	{"Food", []string{
		"The party eats as it travels. F: shows remaining food.",
		"Grocers sell food at a gold piece a ration.",
		"Out of food, the party starves!",
		"Gameplay -> Starving sets the starvation penalty.",
	}},
	{"Healing", []string{
		"Members heal a little as the party travels.",
		"Healers cure poison, heal, raise the dead and restore ashes.",
		"There is a healer in Lord British's castle.",
		"With a member hurt, the command menu puts a caster's heal first.",
	}},
	{"Casters", []string{
		"A wizard's mana grows back to its intelligence, a cleric's to its wisdom.",
		"Paladins, larks and the other mixed classes get half.",
		"The spell list greys a spell if the caster lacks sufficient mana.",
		"Mana grows back as the party travels.",
	}},
	{"Chests", []string{
		"Defeated monsters leave treasure chests.",
		"About half are trapped with acid, poison, a bomb or gas!",
		"Safe chest, a cleric spell, opens one without springing a trap.",
	}},
	{"Guilds", []string{
		"Guilds sell keys, torches, powders and gems. Keys open locked doors.",
		"Powders can pause time in a combat!",
		"Gems can show a map of the area you are in.",
		"Grey is about 60 steps south-west of the castle and has a guild.",
	}},
	{"Dungeons", []string{
		"Dungeons are dark.",
		"A torch (Ignite) burns for 255 turns.",
		"The Light spell activates for 10 turns, Long light for 250.",
		"Klimb and Descend to activate ladders.",
	}},
	{"Fights", []string{
		"A fight has no way out - you can't run away!",
		"Gameplay -> Timer sets how long a turn waits before automatically passing.",
		"Auto combat plays the fight for you.",
		"If the whole party falls, you can reload or go back to Lord British.",
	}},
	{"Monsters", []string{
		"The spell Repel orcs also affects goblins and trolls.",
		"The spell Repel undead affects ghouls and zombies.",
		"Both Repel spells are free, but get one try a fight.",
		"Many monsters poison - stay near a healer until you can cure poison.",
	}},
	{"Ships and horses", []string{
		"No one sells ships.",
		"Attack a pirate ship from the shore!",
		"Board the frigate they leave.",
		"Horse dealers sell a horse for each member, 200 gold a head.",
	}},
	{"Stats", []string{
		"Strength, dexterity, intelligence and wisdom rise only at Ambrosia's shrines.",
		"Each stops at its race's limit.",
		"Gold paid past it is lost.",
	}},
	{"Saving", []string{
		"The game saves itself at every town, castle and dungeon door.",
		"Quit and save saves on the surface.",
		"Export game on the title screen to keep a copy as a file.",
	}},
}

// TipPages is a topic's text as pages of at most rows lines of width
// characters: its paragraphs wrapped, a blank line between them, and a
// paragraph that would not fit in the rest of a page starting the next (one
// longer than a page is cut where each page ends). wrap's max is math.MaxInt
// for no limit.
func TipPages(tip Tip, width, rows int, wrap func(text string, width, max int) []string) [][]string {
	pages := [][]string{{}}
	for _, paragraph := range tip.Text {
		lines := wrap(paragraph, width, math.MaxInt)
		page := &pages[len(pages)-1]
		if len(*page) > 0 && len(*page)+1+len(lines) > rows {
			pages = append(pages, []string{})
		} else if len(*page) > 0 {
			*page = append(*page, "")
		}
		for _, line := range lines {
			if len(pages[len(pages)-1]) == rows {
				pages = append(pages, []string{})
			}
			pages[len(pages)-1] = append(pages[len(pages)-1], line)
		}
	}
	return pages
}

// TipsNudge is said once, on a new game's first turn, in sixteen columns.
const TipsNudge = "New to Sosaria?\nSee Tips, in the\nPause menu.\n"
