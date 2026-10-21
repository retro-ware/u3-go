package game

import (
	"slices"
	"strings"
	"testing"
)

// shopMember makes member 0 a named member of career with weapon readied and armour worn.
func shopMember(w *World, career byte, weapon, armour int) *PlayerRecord {
	p := w.Member(0)
	p.SetName("Kaix")
	p.Bytes[23] = career
	p.Bytes[48] = byte(weapon)
	p.Bytes[40] = byte(armour)
	return p
}

// rows is a card's lines as text and tone, for comparing.
func rows(c Card) []string {
	var out []string
	for _, l := range c.Lines {
		if l.Tone != "" {
			out = append(out, l.Text+" ["+string(l.Tone)+"]")
		} else {
			out = append(out, l.Text)
		}
	}
	return out
}

func price(n int) CardFoot { return CardFoot{IsPrice: true, Price: n} }
func held(n int) CardFoot  { return CardFoot{Held: n} }

func wantRows(t *testing.T, got, want []string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
}

// menuIO records every menu ChooseOption is shown.
type menuIO struct {
	*FakeIO
	menus [][]MenuOption
}

func (m *menuIO) ChooseOption(options []MenuOption, echo Echo, place *MenuPlacement) string {
	m.menus = append(m.menus, options)
	return m.FakeIO.ChooseOption(options, echo, place)
}

func findOption(menu []MenuOption, key string) *MenuOption {
	for i := range menu {
		if menu[i].Key == key {
			return &menu[i]
		}
	}
	return nil
}

func TestHitChance(t *testing.T) {
	var got []int
	for a := range 7 {
		got = append(got, HitChance(a))
	}
	if !slices.Equal(got, []int{47, 44, 42, 40, 38, 36, 35}) {
		t.Fatal(got)
	}
}

func TestGearCardWeapon(t *testing.T) {
	w := newWorld(t)
	w.Party.SetGold(100)
	p := shopMember(w, 'F', 1, 1)
	mace := GearCard(w, p, true, 2, price(30))
	if mace.Title != "Mace" {
		t.Fatal(mace.Title)
	}
	wantRows(t, rows(mace), []string{"Kaix [dim]", "vs Dagger [dim]", "", "Damage       +3 [better]", "Reach     close", "", "Price      30gp"})
	p.Bytes[48] = 6
	wantRows(t, rows(GearCard(w, p, true, 4, price(125))), []string{
		"Kaix [dim]", "vs Sword [dim]", "", "Damage       -6 [worse]", "Reach     close", "", "Price     125gp [worse]",
	})
	wantRows(t, rows(GearCard(w, p, true, 6, price(200)))[:4], []string{"Kaix [dim]", "In hand now [dim]", "", "Damage     same"})
	p.Bytes[48] = 0
	if r := rows(GearCard(w, p, true, 1, price(5)))[1]; r != "vs Hands [dim]" {
		t.Fatal(r)
	}
}

func TestGearCardReach(t *testing.T) {
	w := newWorld(t)
	p := shopMember(w, 'F', 6, 1)
	wantRows(t, rows(GearCard(w, p, true, 3, held(1)))[3:5], []string{"Damage       -9 [worse]", "Reach      afar [better]"})
	p.Bytes[48] = 5
	wantRows(t, rows(GearCard(w, p, true, 6, held(1)))[3:5], []string{"Damage       +3 [better]", "Reach     close [worse]"})
}

func TestGearCardArmour(t *testing.T) {
	w := newWorld(t)
	p := shopMember(w, 'F', 1, 1)
	chain := GearCard(w, p, false, 3, price(0))
	if chain.Title != "Chain" {
		t.Fatal(chain.Title)
	}
	wantRows(t, rows(chain)[1:5], []string{"vs Cloth [dim]", "", "Foes hit    40% [better]", "Now         44% [dim]"})
	p.Bytes[40] = 4
	if r := rows(GearCard(w, p, false, 2, price(0)))[3]; r != "Foes hit    42% [worse]" {
		t.Fatal(r)
	}
	wantRows(t, rows(GearCard(w, p, false, 4, price(0)))[1:5], []string{"Worn now [dim]", "", "Foes hit    38%", ""})
}

func TestGearCardUnusable(t *testing.T) {
	w := newWorld(t)
	p := shopMember(w, 'W', 1, 1)
	wantRows(t, rows(GearCard(w, p, true, 2, held(1)))[3:5], []string{"A Wizard cannot [worse]", "ready it [worse]"})
	p.Bytes[23] = 'I'
	wantRows(t, rows(GearCard(w, p, false, 4, held(1)))[3:5], []string{"An Illusionist [worse]", "cannot wear it [worse]"})
}

func TestGearCardsFit(t *testing.T) {
	w := newWorld(t)
	w.Party.SetGold(0)
	var over []string
	check := func(c Card) {
		if len(c.Lines) > CardRows || len(c.Title) > CardWidth {
			over = append(over, c.Title)
		}
		for _, l := range c.Lines {
			if len(l.Text) > CardWidth {
				over = append(over, l.Text)
			}
		}
	}
	for _, career := range w.Resources.Misc.Career {
		p := shopMember(w, career, 0, 0)
		p.SetName("Abcdefghijklm")
		for h := range 16 {
			for ware := range 16 {
				p.Bytes[48] = byte(h)
				check(GearCard(w, p, true, ware, price(1250)))
				check(GearCard(w, p, true, ware, held(99)))
			}
		}
		for worn := range 8 {
			for ware := range 8 {
				p.Bytes[40] = byte(worn)
				check(GearCard(w, p, false, ware, price(2500)))
			}
		}
	}
	if len(over) > 0 {
		t.Fatal(over)
	}
}

func TestReadyCardsShowPack(t *testing.T) {
	w := newWorld(t)
	p := shopMember(w, 'F', 1, 1)
	w.Party.SetWeapons(6, 2)
	sword := findOption(OwnedItems(w, p, true, 'P'), "G")
	if sword == nil || sword.Card == nil || sword.Card.Title != "Sword" {
		t.Fatal(sword)
	}
	wantRows(t, rows(*sword.Card)[3:], []string{"Damage      +15 [better]", "Reach     close", "", "In pack       2 [dim]"})
	if slices.Contains(rows(*OwnedItems(w, p, true, 'P')[0].Card), "In pack       0 [dim]") {
		t.Fatal("hands shown as stock")
	}
}

func TestSpellCards(t *testing.T) {
	w := newWorld(t)
	w.Member(0).SetMana(10)
	fireball := SpellOption(w, 5, "F", 0)
	if fireball.Card == nil || fireball.Card.Title != "Fireball" {
		t.Fatal(fireball.Card)
	}
	wantRows(t, rows(*fireball.Card), []string{"FULGAR [dim]", "", "Cost         25", "Mana         10 [worse]", "", "A strong bolt", "at one foe."})
	if !fireball.Disabled {
		t.Fatal("affordable")
	}
	for spell := range 35 {
		c := SpellOption(w, spell, "A", 0).Card
		if len(c.Lines) > CardRows || len(c.Title) > CardWidth {
			t.Fatal(c.Title)
		}
		for _, l := range c.Lines {
			if len(l.Text) > CardWidth {
				t.Fatal(l.Text)
			}
		}
	}
}

func TestShopBuyListReweighed(t *testing.T) {
	w := newWorld(t)
	shopMember(w, 'F', 1, 1)
	w.Party.SetGold(500)
	w.Party.SetSurfaceX(0)
	io := &menuIO{FakeIO: newFakeIO(w.Resources)}
	io.Keys = []string{"N", "B", "C", "Y", KeyEscape} // buy a mace, ready it, and leave
	Shop(w, io, 3, 0)
	var buys [][]MenuOption
	for _, m := range io.menus {
		if o := findOption(m, "C"); o != nil && strings.Contains(o.Label, "Mace") {
			buys = append(buys, m)
		}
	}
	if len(buys) != 2 {
		t.Fatal(len(buys))
	}
	if r := rows(*findOption(buys[1], "C").Card)[1]; r != "In hand now [dim]" {
		t.Fatal(r)
	}
	wantRows(t, rows(*findOption(buys[1], "B").Card)[1:4], []string{"vs Mace [dim]", "", "Damage       -3 [worse]"})
	// Selling the dagger it put back in the bag: no weighing on the Sell list.
	io.menus = nil
	io.Keys = []string{"N", "S", KeyEscape}
	Shop(w, io, 3, 0)
	var sell []MenuOption
	for _, m := range io.menus {
		if o := findOption(m, "B"); o != nil && strings.Contains(o.Label, "Dagger") {
			sell = m
			break
		}
	}
	if sell == nil {
		t.Fatal("no sell list")
	}
	for _, o := range sell {
		if o.Card != nil {
			t.Fatal("weighed on the Sell list", o.Key)
		}
	}
}

func TestShopBuyListCards(t *testing.T) {
	w := newWorld(t)
	shopMember(w, 'F', 1, 1)
	w.Party.SetGold(500)
	w.Party.SetSurfaceX(0)
	io := &menuIO{FakeIO: newFakeIO(w.Resources)}
	io.Keys = []string{"N", "B", KeyEscape}
	Shop(w, io, 3, 0)
	for _, m := range io.menus {
		mace := findOption(m, "C")
		if mace == nil || !strings.Contains(mace.Label, "Mace") {
			continue
		}
		if !slices.Contains(rows(*mace.Card), "Damage       +3 [better]") {
			t.Fatal(rows(*mace.Card))
		}
		if r := rows(*findOption(m, "B").Card)[1]; r != "In hand now [dim]" {
			t.Fatal(r)
		}
		return
	}
	t.Fatal("no buy list")
}

func TestGuildMenuFits(t *testing.T) {
	w := newWorld(t)
	io := &menuIO{FakeIO: newFakeIO(w.Resources)}
	io.Keys = []string{"N"}
	Shop(w, io, 5, 0)
	for _, m := range io.menus {
		if torches := findOption(m, "T"); torches != nil {
			if torches.Label != "Torches x5 (30 gp)" {
				t.Fatal(torches.Label)
			}
			for _, o := range m {
				if len(o.Label) > 18 {
					t.Fatal(o.Label)
				}
			}
			return
		}
	}
	t.Fatal("no guild menu")
}
