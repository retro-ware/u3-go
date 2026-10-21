package game

// MonsterSlots is the number of monsters in a MONS table.
const MonsterSlots = 32

// Column offsets of the monster table (a struct of arrays: 32 monsters, six
// fields, each a 32-byte column).
const (
	monType      = 0   // map value of the monster (0 = slot unused)
	monTileUnder = 32  // the terrain the monster stands on (restored when it moves)
	monX         = 64  //
	monY         = 96  //
	monHP        = 128 // top two bits are the town behaviour; low nibble the NPC's dialogue index
	monVariant   = 160 // 0, 1 or 2: Orc / Goblin / Troll style variants
)

// Movement behaviour of town and castle NPCs, from the top two bits of the hp column.
const (
	BehaviourStationary = 0x00
	BehaviourWander     = 0x40
	BehaviourFollow     = 0x80
	BehaviourAttack     = 0xc0
)

// MonsterTable is a view over the 256-byte monster table (MONS resource, and
// the Monsters[256] array in the C source). On the surface the whole table is
// saved with the map so that wandering monsters persist.
type MonsterTable struct {
	Bytes []byte
}

// NewMonsterTable wraps 256 bytes.
func NewMonsterTable(b []byte) *MonsterTable {
	if len(b) != 256 {
		panic("monster table must be 256 bytes")
	}
	return &MonsterTable{Bytes: b}
}

func (t *MonsterTable) Type(i int) int          { return int(t.Bytes[monType+i]) }
func (t *MonsterTable) SetType(i, v int)        { t.Bytes[monType+i] = byte(v) }
func (t *MonsterTable) TileUnder(i int) int     { return int(t.Bytes[monTileUnder+i]) }
func (t *MonsterTable) SetTileUnder(i, v int)   { t.Bytes[monTileUnder+i] = byte(v) }
func (t *MonsterTable) X(i int) int             { return int(t.Bytes[monX+i]) }
func (t *MonsterTable) Y(i int) int             { return int(t.Bytes[monY+i]) }
func (t *MonsterTable) HP(i int) int            { return int(t.Bytes[monHP+i]) }
func (t *MonsterTable) SetHP(i, v int)          { t.Bytes[monHP+i] = byte(v) }
func (t *MonsterTable) Variant(i int) int       { return int(t.Bytes[monVariant+i]) }
func (t *MonsterTable) SetVariant(i, v int)     { t.Bytes[monVariant+i] = byte(v) }
func (t *MonsterTable) SetPosition(i, x, y int) { t.Bytes[monX+i], t.Bytes[monY+i] = byte(x), byte(y) }
func (t *MonsterTable) Clear(i int)             { t.SetType(i, 0) }
func (t *MonsterTable) CopyFrom(other []byte)   { copy(t.Bytes, other) }
func (t *MonsterTable) Behaviour(i int) int     { return t.HP(i) & 0xc0 } // Monsters[i + 128] & 0xC0
func (t *MonsterTable) TalkIndex(i int) int     { return t.HP(i) & 0x0f } // Monsters[i + 128] & 0x0F
func (t *MonsterTable) DisplayValue(i int) int  { return t.Type(i) + t.Variant(i) }

// At mirrors MonsterHere(): the monster at (x, y), or -1, searching from the top slot down.
func (t *MonsterTable) At(x, y int) int {
	for i := MonsterSlots - 1; i >= 0; i-- {
		if t.Type(i) != 0 && t.X(i) == x && t.Y(i) == y {
			return i
		}
	}
	return -1
}

// FreeSlot is the highest-numbered free slot, or -1 (SpawnMonster searches downward from 31).
func (t *MonsterTable) FreeSlot() int {
	for i := MonsterSlots - 1; i >= 0; i-- {
		if t.Type(i) == 0 {
			return i
		}
	}
	return -1
}
