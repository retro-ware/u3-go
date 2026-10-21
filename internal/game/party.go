package game

// The 64-byte party record (PRTY resource). The C code read it into
// Party[byte + 1], so its constants are 1-based; this file uses the
// resource's own 0-based offsets:
//
//	0      Party[1]      shape: 0x7E ranger, 0x14 horse, 0x16 frigate, 0x18 whirlpool
//	1      Party[2]      number of members (1-4)
//	2      Party[3]      location type (Location*)
//	3-4    Party[4..5]   surface X, Y (saved when entering somewhere)
//	6-9    Party[7..10]  1-based roster slot of members 1-4, 0 = empty
//	10-13  Party[11..14] move counter, four base-100 digits, least significant first
//	15     Party[16]     1 once Exodus has been destroyed
//	20-22  (port)        gold, 0..99999, low byte first
//	24-26  (port)        food in hundredths, low byte first
//	27     (port)        1 while the Modern/Classic choice is still to be made
//	32-46  (port)        weapons in the bag, by index 1..15
//	48-54  (port)        armour in the bag, by index 1..7
//	56-59  (port)        gems, keys, powders, torches
//
// Everything from byte 20 on is the TypeScript port's: gold, food, gear and
// supplies belong to the party rather than to each member.

const PartyRecordSize = 64

// Values of the location byte (Party[3]).
const (
	LocationSosaria  = 0x00
	LocationDungeon  = 0x01
	LocationTown     = 0x02
	LocationCastle   = 0x03
	LocationCombat   = 0x80
	LocationAmbrosia = 0xff
)

// Caps on the pooled gold and food (five digits on the status bar).
const (
	GoldMax = 99999
	FoodMax = 99999
)

// Party is a view over the party record.
type Party struct {
	Bytes []byte
}

// NewParty wraps a 64-byte record.
func NewParty(b []byte) *Party {
	if len(b) != PartyRecordSize {
		panic("party record must be 64 bytes")
	}
	return &Party{Bytes: b}
}

func (p *Party) Shape() int             { return int(p.Bytes[0]) }
func (p *Party) SetShape(v int)         { p.Bytes[0] = byte(v) }
func (p *Party) Size() int              { return int(p.Bytes[1]) }
func (p *Party) SetSize(v int)          { p.Bytes[1] = byte(v) }
func (p *Party) Location() int          { return int(p.Bytes[2]) }
func (p *Party) SetLocation(v int)      { p.Bytes[2] = byte(v) }
func (p *Party) SurfaceX() int          { return int(p.Bytes[3]) }
func (p *Party) SetSurfaceX(v int)      { p.Bytes[3] = byte(v) }
func (p *Party) SurfaceY() int          { return int(p.Bytes[4]) }
func (p *Party) SetSurfaceY(v int)      { p.Bytes[4] = byte(v) }
func (p *Party) ExodusDestroyed() bool  { return p.Bytes[15] == 1 }
func (p *Party) RulesPending() bool     { return p.Bytes[27] == 1 }
func (p *Party) SetRulesPending(v bool) { p.Bytes[27] = b2u(v) }

// MemberRosterNumber is Party[7 + member]: the 1-based roster slot, 0 when empty.
func (p *Party) MemberRosterNumber(member int) int { return int(p.Bytes[6+member]) }

// SetMemberRosterNumber sets a position's 1-based roster slot.
func (p *Party) SetMemberRosterNumber(member, n int) { p.Bytes[6+member] = byte(n) }

// MemberSlot is the 0-based roster slot of member 0..3, or -1 when empty.
func (p *Party) MemberSlot(member int) int { return p.MemberRosterNumber(member) - 1 }

// Formed is true when at least one member has been assigned.
func (p *Party) Formed() bool { return p.Bytes[6] != 0 }

// Moves is Party[11..14], four base-100 digits.
func (p *Party) Moves() int {
	b := p.Bytes
	return int(b[10]) + int(b[11])*100 + int(b[12])*10000 + int(b[13])*1000000
}

// IncrementMoves mirrors IncMoves(): add the party size, carrying between the
// base-100 digits and saturating at 99,999,999.
func (p *Party) IncrementMoves() {
	b := p.Bytes
	b[10] += byte(p.Size())
	if b[10] > 99 {
		b[10] -= 100
		b[11]++
		if b[11] > 99 {
			b[11] -= 100
			b[12]++
			if b[12] > 99 {
				b[12] -= 100
				b[13]++
				if b[13] > 99 {
					b[10], b[11], b[12], b[13] = 99, 99, 99, 99
				}
			}
		}
	}
}

func get24(b []byte) int    { return int(b[0]) | int(b[1])<<8 | int(b[2])<<16 }
func put24(b []byte, v int) { b[0], b[1], b[2] = byte(v), byte(v>>8), byte(v>>16) }

// Gold is the pooled gold, 0..99999.
func (p *Party) Gold() int { return get24(p.Bytes[20:]) }

// SetGold clamps to 0..GoldMax.
func (p *Party) SetGold(v int) { put24(p.Bytes[20:], clamp(v, 0, GoldMax)) }

// FoodHundredths is the food in hundredths of a ration.
func (p *Party) FoodHundredths() int { return get24(p.Bytes[24:]) }

// SetFoodHundredths clamps to 0..FoodMax*100+99.
func (p *Party) SetFoodHundredths(v int) { put24(p.Bytes[24:], clamp(v, 0, FoodMax*100+99)) }

// Food is whole rations.
func (p *Party) Food() int { return p.FoodHundredths() / 100 }

// SetFood sets whole rations, dropping any fraction.
func (p *Party) SetFood(v int) { p.SetFoodHundredths(clamp(v, 0, FoodMax) * 100) }

// Weapons is the bag's count of weapon index 1..15; 0 for hands.
func (p *Party) Weapons(index int) int {
	if index >= 1 && index <= 15 {
		return int(p.Bytes[31+index])
	}
	return 0
}

// SetWeapons sets a weapon count, 0..99.
func (p *Party) SetWeapons(index, n int) {
	if index >= 1 && index <= 15 {
		p.Bytes[31+index] = byte(clamp(n, 0, 99))
	}
}

// Armour is the bag's count of armour index 1..7; 0 for skin.
func (p *Party) Armour(index int) int {
	if index >= 1 && index <= 7 {
		return int(p.Bytes[47+index])
	}
	return 0
}

// SetArmour sets an armour count, 0..99.
func (p *Party) SetArmour(index, n int) {
	if index >= 1 && index <= 7 {
		p.Bytes[47+index] = byte(clamp(n, 0, 99))
	}
}

// Gear is what the bag holds of one kind: Gear(true, 6) is swords.
func (p *Party) Gear(isWeapon bool, index int) int {
	if isWeapon {
		return p.Weapons(index)
	}
	return p.Armour(index)
}

// SetGear sets a weapon or armour count.
func (p *Party) SetGear(isWeapon bool, index, n int) {
	if isWeapon {
		p.SetWeapons(index, n)
	} else {
		p.SetArmour(index, n)
	}
}

// ClearGear empties the bag.
func (p *Party) ClearGear() {
	clear(p.Bytes[32:47])
	clear(p.Bytes[48:55])
}

func (p *Party) Gems() int        { return int(p.Bytes[56]) }
func (p *Party) SetGems(v int)    { p.Bytes[56] = byte(clamp(v, 0, 99)) }
func (p *Party) Keys() int        { return int(p.Bytes[57]) }
func (p *Party) SetKeys(v int)    { p.Bytes[57] = byte(clamp(v, 0, 99)) }
func (p *Party) Powders() int     { return int(p.Bytes[58]) }
func (p *Party) SetPowders(v int) { p.Bytes[58] = byte(clamp(v, 0, 99)) }
func (p *Party) Torches() int     { return int(p.Bytes[59]) }
func (p *Party) SetTorches(v int) { p.Bytes[59] = byte(clamp(v, 0, 99)) }
func (p *Party) ClearSupplies()   { clear(p.Bytes[56:60]) }

func clamp(v, lo, hi int) int { return max(lo, min(hi, v)) }

func b2u(v bool) byte {
	if v {
		return 1
	}
	return 0
}
