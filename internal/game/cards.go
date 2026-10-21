package game

import (
	"fmt"
	"slices"
	"strings"
)

// The cards a list shows over the party panel while one of its items is
// highlighted: a weapon or armour weighed for one member against what they
// have readied or wear, in the shops' Buy lists and in Ready and Wear. A
// spell's card is spells.go's.

// Shoots is the weapons that shoot: sling, bow, +2 bow, +4 bow.
var Shoots = []int{3, 5, 9, 13}

// HitChance is, in 100, how often a foe's blow lands on a member wearing
// armour index armour (combat.go: a roll of 0 to armour + 16 under 8).
func HitChance(armour int) int {
	return int(float64(800)/float64(armour+17) + 0.5)
}

// CardRow is a card line with label at its left and value at its right.
func CardRow(label, value string) string {
	return label + fmt.Sprintf("%*s", CardWidth-len(label), value)
}

// CardWrap wraps words to a card's width.
func CardWrap(text string) []string {
	var lines []string
	for _, word := range strings.Split(text, " ") {
		if last := len(lines) - 1; last >= 0 && len(lines[last])+1+len(word) <= CardWidth {
			lines[last] += " " + word
		} else {
			lines = append(lines, word)
		}
	}
	return lines
}

// GearName is a weapon's or armour's name as a card says it: hands and skin are what nothing is.
func GearName(w *World, isWeapon bool, index int) string {
	if index == 0 {
		if isWeapon {
			return "Hands"
		}
		return "Skin"
	}
	off := 16
	if isWeapon {
		off = 0
	}
	return w.Resources.Strings["WeaponsArmour"][off+index]
}

// cannotUse is "A Wizard cannot ready it", over the lines it takes.
func cannotUse(w *World, p *PlayerRecord, isWeapon bool) []string {
	career := "member"
	classes := w.Resources.Strings["Classes"]
	if i := slices.Index(w.Resources.Misc.Career, p.ClassLetter()); i >= 0 && i < len(classes) {
		career = classes[i]
	}
	article := "A"
	if strings.ContainsAny(career[:1], "AEIOU") {
		article = "An"
	}
	verb := "wear"
	if isWeapon {
		verb = "ready"
	}
	return CardWrap(fmt.Sprintf("%s %s cannot %s it", article, career, verb))
}

// CardFoot is what else the card says below the comparison: a shop's price
// (red where the party is short of it) when IsPrice, else the pack's count.
type CardFoot struct {
	IsPrice bool
	Price   int
	Held    int
}

// GearCard weighs a weapon or armour for member p against what they have
// readied or wear: a weapon's damage a hit (each step up the list is 3
// more) and its reach, or how often foes would hit in the armour; then the
// price or the pack's count.
func GearCard(w *World, p *PlayerRecord, isWeapon bool, index int, foot CardFoot) Card {
	base, _ := gearRange(isWeapon)
	inUse := int(p.Bytes[base])
	lines := []CardLine{{Text: p.Name(), Tone: CardToneDim}}
	tone := func(better, worse bool) CardTone {
		if better {
			return CardToneBetter
		}
		if worse {
			return CardToneWorse
		}
		return CardToneNone
	}
	switch {
	case index == inUse && isWeapon:
		lines = append(lines, CardLine{"In hand now", CardToneDim})
	case index == inUse:
		lines = append(lines, CardLine{"Worn now", CardToneDim})
	default:
		lines = append(lines, CardLine{"vs " + GearName(w, isWeapon, inUse), CardToneDim})
	}
	lines = append(lines, CardLine{})
	switch {
	case !w.CanUse(p, isWeapon, index):
		for _, t := range cannotUse(w, p, isWeapon) {
			lines = append(lines, CardLine{t, CardToneWorse})
		}
	case isWeapon:
		more := (index - inUse) * 3
		v := "same"
		if more > 0 {
			v = fmt.Sprintf("+%d", more)
		} else if more < 0 {
			v = fmt.Sprint(more)
		}
		lines = append(lines, CardLine{CardRow("Damage", v), tone(more > 0, more < 0)})
		shoots, shot := slices.Contains(Shoots, index), slices.Contains(Shoots, inUse)
		reach := "close"
		if shoots {
			reach = "afar"
		}
		lines = append(lines, CardLine{CardRow("Reach", reach), tone(shoots && !shot, shot && !shoots)})
	default:
		lines = append(lines, CardLine{CardRow("Foes hit", fmt.Sprintf("%d%%", HitChance(index))), tone(index > inUse, index < inUse)})
		if index != inUse {
			lines = append(lines, CardLine{CardRow("Now", fmt.Sprintf("%d%%", HitChance(inUse))), CardToneDim})
		}
	}
	lines = append(lines, CardLine{})
	if foot.IsPrice {
		lines = append(lines, CardLine{CardRow("Price", fmt.Sprintf("%dgp", foot.Price)), tone(false, w.Party.Gold() < foot.Price)})
	} else if index > 0 {
		lines = append(lines, CardLine{CardRow("In pack", fmt.Sprint(foot.Held)), CardToneDim})
	}
	return Card{Title: GearName(w, isWeapon, index), Lines: lines}
}
