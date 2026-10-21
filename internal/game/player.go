package game

// A 64-byte character record; the roster holds 20 back to back (ROST, 1280 bytes):
//
//	0-13   name, NUL padded, high bit sometimes set
//	14     marks and cards bit flags (0x10 Mark of Force, 0x20 Mark of Fire ...)
//	15     torches
//	16     0xFF while in the party
//	17     status letter: G good, P poisoned, D dead, A ashes
//	18-21  strength, dexterity, intelligence, wisdom
//	22-24  race, class and sex letters
//	25     magic points
//	26-29  hit points and maximum, big endian
//	30     level - 1;  31 experience within the level, 0..99
//	32-34  food as three base-100 digits; 35-36 gold, big endian
//	37-39  gems, keys, powders
//	40     armour worn; 41-47 armour counts
//	48     weapon readied; 49-63 weapon counts
//
// The C Player[21][65] array was 1-based; this port uses 0-based slots.

const (
	PlayerRecordSize = 64
	RosterSize       = 20
)

// Byte offsets into a player record.
const (
	PlayerOffsetName           = 0
	PlayerOffsetMarks          = 14
	PlayerOffsetTorches        = 15
	PlayerOffsetInParty        = 16
	PlayerOffsetStatus         = 17
	PlayerOffsetStrength       = 18
	PlayerOffsetDexterity      = 19
	PlayerOffsetIntelligence   = 20
	PlayerOffsetWisdom         = 21
	PlayerOffsetRace           = 22
	PlayerOffsetClass          = 23
	PlayerOffsetSex            = 24
	PlayerOffsetMana           = 25
	PlayerOffsetHitPointsHi    = 26
	PlayerOffsetHitPointsLo    = 27
	PlayerOffsetMaxHitPointsHi = 28
	PlayerOffsetMaxHitPointsLo = 29
	PlayerOffsetLevel          = 30
	PlayerOffsetExperience     = 31
	PlayerOffsetFoodHundreds   = 32
	PlayerOffsetFoodUnits      = 33
	PlayerOffsetFoodFraction   = 34
	PlayerOffsetGoldHi         = 35
	PlayerOffsetGoldLo         = 36
	PlayerOffsetGems           = 37
	PlayerOffsetKeys           = 38
	PlayerOffsetPowders        = 39
	PlayerOffsetArmourWorn     = 40
	PlayerOffsetArmourCounts   = 41
	PlayerOffsetWeaponReadied  = 48
	PlayerOffsetWeaponCounts   = 49
)

// PlayerRecord is a view over one character record.
type PlayerRecord struct {
	Bytes []byte
}

// NewPlayerRecord wraps 64 bytes.
func NewPlayerRecord(b []byte) *PlayerRecord {
	if len(b) != PlayerRecordSize {
		panic("player record must be 64 bytes")
	}
	return &PlayerRecord{Bytes: b}
}

// Exists is false for an empty roster slot (zero first name byte).
func (p *PlayerRecord) Exists() bool { return p.Bytes[PlayerOffsetName] != 0 }

// Name is the name, high bits stripped.
func (p *PlayerRecord) Name() string {
	var s []byte
	for i := 0; i < 13 && p.Bytes[i] != 0; i++ {
		s = append(s, p.Bytes[i]&0x7f)
	}
	return string(s)
}

// SetName stores up to 13 characters.
func (p *PlayerRecord) SetName(v string) {
	clear(p.Bytes[:14])
	for i := 0; i < min(13, len(v)); i++ {
		p.Bytes[i] = v[i] & 0x7f
	}
}

func (p *PlayerRecord) Status() byte      { return p.Bytes[PlayerOffsetStatus] }
func (p *PlayerRecord) SetStatus(s byte)  { p.Bytes[PlayerOffsetStatus] = s }
func (p *PlayerRecord) InParty() bool     { return p.Bytes[PlayerOffsetInParty] != 0 }
func (p *PlayerRecord) Sex() byte         { return p.Bytes[PlayerOffsetSex] }
func (p *PlayerRecord) Race() byte        { return p.Bytes[PlayerOffsetRace] }
func (p *PlayerRecord) ClassLetter() byte { return p.Bytes[PlayerOffsetClass] }
func (p *PlayerRecord) Strength() int     { return int(p.Bytes[PlayerOffsetStrength]) }
func (p *PlayerRecord) Dexterity() int    { return int(p.Bytes[PlayerOffsetDexterity]) }
func (p *PlayerRecord) Intelligence() int { return int(p.Bytes[PlayerOffsetIntelligence]) }
func (p *PlayerRecord) Wisdom() int       { return int(p.Bytes[PlayerOffsetWisdom]) }
func (p *PlayerRecord) Mana() int         { return int(p.Bytes[PlayerOffsetMana]) }
func (p *PlayerRecord) SetMana(v int)     { p.Bytes[PlayerOffsetMana] = byte(v) }
func (p *PlayerRecord) Torches() int      { return int(p.Bytes[PlayerOffsetTorches]) }
func (p *PlayerRecord) SetTorches(v int)  { p.Bytes[PlayerOffsetTorches] = byte(v) }
func (p *PlayerRecord) Marks() int        { return int(p.Bytes[PlayerOffsetMarks]) }
func (p *PlayerRecord) Experience() int   { return int(p.Bytes[PlayerOffsetExperience]) }
func (p *PlayerRecord) Gems() int         { return int(p.Bytes[PlayerOffsetGems]) }
func (p *PlayerRecord) Keys() int         { return int(p.Bytes[PlayerOffsetKeys]) }
func (p *PlayerRecord) Powders() int      { return int(p.Bytes[PlayerOffsetPowders]) }

// Level is stored as level - 1.
func (p *PlayerRecord) Level() int { return int(p.Bytes[PlayerOffsetLevel]) + 1 }

// Alive means Good or Poisoned. Mirrors CheckAlive().
func (p *PlayerRecord) Alive() bool { s := p.Status(); return s == 'G' || s == 'P' }

// SetInParty marks the character as a party member (0xFF) or not.
func (p *PlayerRecord) SetInParty(v bool) {
	p.Bytes[PlayerOffsetInParty] = 0
	if v {
		p.Bytes[PlayerOffsetInParty] = 0xff
	}
}

func be16(b []byte) int       { return int(b[0])<<8 | int(b[1]) }
func putBE16(b []byte, v int) { b[0], b[1] = byte(v/256), byte(v%256) }

func (p *PlayerRecord) HitPoints() int     { return be16(p.Bytes[PlayerOffsetHitPointsHi:]) }
func (p *PlayerRecord) SetHitPoints(v int) { putBE16(p.Bytes[PlayerOffsetHitPointsHi:], v) }
func (p *PlayerRecord) MaxHitPoints() int  { return be16(p.Bytes[PlayerOffsetMaxHitPointsHi:]) }
func (p *PlayerRecord) Gold() int          { return be16(p.Bytes[PlayerOffsetGoldHi:]) }
func (p *PlayerRecord) SetGold(v int)      { putBE16(p.Bytes[PlayerOffsetGoldHi:], v) }

// Food is the hundreds and units digits; the fraction is not shown.
func (p *PlayerRecord) Food() int {
	return int(p.Bytes[PlayerOffsetFoodHundreds])*100 + int(p.Bytes[PlayerOffsetFoodUnits])
}

// AddHitPoints mirrors HPAdd(): heal, capped at the maximum.
func (p *PlayerRecord) AddHitPoints(amount int) {
	p.SetHitPoints(min(p.MaxHitPoints(), p.HitPoints()+amount))
}

// SubtractHitPoints mirrors HPSubtract(). It reports whether the character
// died; the caller plays the death sound.
func (p *PlayerRecord) SubtractHitPoints(amount int) bool {
	hp := p.HitPoints() - amount
	if hp < 1 {
		p.SetHitPoints(0)
		p.SetStatus('D')
		return true
	}
	p.SetHitPoints(hp)
	return false
}

// Roster is the 20 records in one buffer, so saving is a single copy.
type Roster struct {
	Bytes   []byte
	Players []*PlayerRecord
}

// NewRoster wraps 1280 bytes.
func NewRoster(b []byte) *Roster {
	if len(b) != PlayerRecordSize*RosterSize {
		panic("roster must be 1280 bytes")
	}
	r := &Roster{Bytes: b}
	for i := range RosterSize {
		r.Players = append(r.Players, NewPlayerRecord(b[i*PlayerRecordSize:(i+1)*PlayerRecordSize]))
	}
	return r
}

// Get is the record in 0-based slot.
func (r *Roster) Get(slot int) *PlayerRecord { return r.Players[slot] }

// StatusLetter is the letter drawn after a member's name by the sets that
// show one: G, P, D, A, and L where Lord British owes a level.
func StatusLetter(p *PlayerRecord) byte {
	if p.Status() == 'G' && LevelUpDue(p) {
		return 'L'
	}
	return p.Status()
}

// LevelUpDue is Lord British's test for raising a member: level at or above
// maximum hit points / 100, not yet 2500, and past level 5 only with the Mark of Kings.
func LevelUpDue(p *PlayerRecord) bool {
	level := p.Level() - 1
	hpmax := p.MaxHitPoints()
	if hpmax%100 == 50 {
		hpmax -= 50
	}
	hpmax /= 100
	if level < hpmax || hpmax >= 25 {
		return false
	}
	return !(hpmax > 4 && p.Marks()&0x80 == 0)
}
