package game

import (
	"slices"
	"testing"
)

func TestPlayerRecordsReadShippedCharacters(t *testing.T) {
	r := loadTestResources(t)
	roster := NewRoster(slices.Clone(r.DefaultRoster))
	p := roster.Get(0)
	if !p.Exists() || p.Name() != "Tatiana" || p.Sex() != 'F' || p.Race() != 'E' || p.ClassLetter() != 'T' || p.Status() != 'G' {
		t.Fatalf("got %q %c %c %c %c", p.Name(), p.Sex(), p.Race(), p.ClassLetter(), p.Status())
	}
	if p.HitPoints() != 100 || p.MaxHitPoints() != 100 || p.Food() != 150 || p.Gold() != 150 || p.Level() != 1 {
		t.Fatal("numbers wrong")
	}
	if roster.Get(1).Name() != "Roderic" {
		t.Fatal(roster.Get(1).Name())
	}
}

func TestPlayerHitPointsAcrossByteBoundary(t *testing.T) {
	p := NewPlayerRecord(make([]byte, 64))
	p.Bytes[28], p.Bytes[29] = 1, 44 // max 300
	p.SetHitPoints(255)
	p.AddHitPoints(10)
	if p.HitPoints() != 265 {
		t.Fatal(p.HitPoints())
	}
	p.AddHitPoints(1000)
	if p.HitPoints() != 300 {
		t.Fatal(p.HitPoints())
	}
	if p.SubtractHitPoints(299) || p.HitPoints() != 1 {
		t.Fatal("299")
	}
	p.SetStatus('G')
	if !p.SubtractHitPoints(1) || p.Status() != 'D' || p.Alive() {
		t.Fatal("death")
	}
}

func TestStatusLetter(t *testing.T) {
	r := loadTestResources(t)
	member := func(status byte, level, maxHP int) *PlayerRecord {
		p := NewRoster(slices.Clone(r.DefaultRoster)).Get(0)
		p.SetStatus(status)
		p.Bytes[30] = byte(level - 1)
		p.Bytes[28], p.Bytes[29] = byte(maxHP/256), byte(maxHP%256)
		return p
	}
	due := member('G', 3, 100)
	if !LevelUpDue(due) || StatusLetter(due) != 'L' {
		t.Fatal("due")
	}
	fine := member('G', 1, 200)
	if LevelUpDue(fine) || StatusLetter(fine) != 'G' {
		t.Fatal("fine")
	}
	pd := member('P', 3, 100)
	if !LevelUpDue(pd) || StatusLetter(pd) != 'P' {
		t.Fatal("poisoned")
	}
	if StatusLetter(member('D', 3, 100)) != 'D' || StatusLetter(member('A', 3, 100)) != 'A' {
		t.Fatal("D/A")
	}
}

func TestPartyRecordDefault(t *testing.T) {
	r := loadTestResources(t)
	p := NewParty(slices.Clone(r.DefaultParty))
	if p.Shape() != 0x7e || p.Size() != 4 || p.Location() != 0 || p.SurfaceX() != 42 || p.SurfaceY() != 20 ||
		p.MemberSlot(0) != 0 || p.MemberSlot(3) != 3 || !p.Formed() || p.ExodusDestroyed() {
		t.Fatal("default party")
	}
}

func TestPartyMoveCounterCarries(t *testing.T) {
	p := NewParty(make([]byte, 64))
	p.SetSize(4)
	for range 30 {
		p.IncrementMoves()
	}
	if p.Moves() != 120 || p.Bytes[10] != 20 || p.Bytes[11] != 1 {
		t.Fatal(p.Moves())
	}
}

func TestPartyBagPoolsAndDeals(t *testing.T) {
	w := newWorld(t)
	if w.Party.Weapons(1) != 0 || w.Member(0).Bytes[48] != 1 || w.Member(0).Bytes[49] != 0 {
		t.Fatal("new game pool")
	}
	w.Member(0).Bytes[48+6] = 2
	w.Member(1).Bytes[48+6] = 1
	w.Member(1).Bytes[48] = 6
	w.Member(2).Bytes[40+3] = 1
	w.PoolGear()
	if w.Party.Weapons(6) != 2 || w.Party.Armour(3) != 1 || w.Member(1).Bytes[48] != 6 || w.Member(1).Bytes[48+6] != 0 {
		t.Fatal("pool")
	}
	w.SplitGear()
	if w.Member(1).Bytes[48+6] < 1 {
		t.Fatal("in hand")
	}
	swords := 0
	for m := range 4 {
		swords += int(w.Member(m).Bytes[48+6])
	}
	if swords != 3 || w.Party.Weapons(6) != 0 {
		t.Fatal(swords)
	}
	for m := range 4 {
		if w.Member(m).Bytes[40+3] == 1 && !w.CanUse(w.Member(m), false, 3) {
			t.Fatal("chain to someone who can't")
		}
	}
}
