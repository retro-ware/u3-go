package game

// The three numbering schemes the original used for terrain and creatures:
//
//  1. Map value (0-255): a byte in a map. Bits 7..2 select the tile, bits 1..0
//     a monster variant (Orc / Goblin / Troll). Terrain values are multiples of 4.
//  2. Shape (0-254, even): a map value with the variant bits cleared, shifted
//     right by one. The 11x11 viewport holds shapes. One odd shape, 0x5D, is a
//     door drawn from the second frame of the letter "I".
//  3. Tile index (0-95): the position in the tile sheet, shape >> 1. 0-63 are
//     the Apple II tiles, 64-67 the shared party figures, 68-78 one figure per
//     class, 79 empty, 80-95 the monster variants.

// Map values for terrain and creatures.
const (
	MapValueWater       = 0x00
	MapValueGrass       = 0x04
	MapValueBrush       = 0x08
	MapValueForest      = 0x0c
	MapValueMountains   = 0x10
	MapValueDungeon     = 0x14
	MapValueTown        = 0x18
	MapValueCastle      = 0x1c
	MapValueFloor       = 0x20
	MapValueChest       = 0x24
	MapValueHorse       = 0x28
	MapValueFrigate     = 0x2c
	MapValueWhirlpool   = 0x30
	MapValueSerpent     = 0x34
	MapValueManOWar     = 0x38
	MapValuePirate      = 0x3c
	MapValueMerchant    = 0x40
	MapValueJester      = 0x44
	MapValueGuard       = 0x48
	MapValueLordBritish = 0x4c
	MapValueFighter     = 0x50
	MapValueCleric      = 0x54
	MapValueWizard      = 0x58
	MapValueThief       = 0x5c
	MapValueOrc         = 0x60
	MapValueSkeleton    = 0x64
	MapValueGiant       = 0x68
	MapValueDaemon      = 0x6c
	MapValuePincher     = 0x70
	MapValueDragon      = 0x74
	MapValueBalron      = 0x78
	MapValueExodus      = 0x7c
	MapValueForceField  = 0x80
	MapValueLava        = 0x84
	MapValueMoonGate    = 0x88
	MapValueWall        = 0x8c
	MapValueVoid        = 0x90
	MapValueWall2       = 0x94
	MapValueLetterA     = 0x98 // letters A..T occupy 0x98..0xE4 in steps of 4
	MapValueLetterI     = 0xb8 // doubles as a door
	MapValueLetterT     = 0xe4
	MapValueSnakeBottom = 0xe8
	MapValueSnakeTop    = 0xec
	MapValueMagicBall   = 0xf0
	MapValueFireBall    = 0xf4
	MapValueShrine      = 0xf8
	MapValueRanger      = 0xfc
)

// Shapes (map value / 2) the code compares against most often.
const (
	ShapeWater       = 0x00
	ShapeGrass       = 0x02
	ShapeBrush       = 0x04
	ShapeForest      = 0x06
	ShapeMountains   = 0x08
	ShapeDungeon     = 0x0a
	ShapeTown        = 0x0c
	ShapeCastle      = 0x0e
	ShapeFloor       = 0x10
	ShapeChest       = 0x12
	ShapeHorse       = 0x14
	ShapeFrigate     = 0x16
	ShapeWhirlpool   = 0x18
	ShapeSerpent     = 0x1a
	ShapeManOWar     = 0x1c
	ShapePirate      = 0x1e
	ShapeJester      = 0x22
	ShapeForceField  = 0x40
	ShapeLava        = 0x42
	ShapeMoonGate    = 0x44
	ShapeWall        = 0x46
	ShapeVoid        = 0x48
	ShapeDoor        = 0x5d // odd on purpose: the alternate frame of the letter "I"
	ShapeSnakeBottom = 0x74
	ShapeSnakeTop    = 0x76
	ShapeMagicBall   = 0x78
	ShapeFireBall    = 0x7a
	ShapeShrine      = 0x7c
	ShapeRanger      = 0x7e // the party marker on the surface
)

// MapValueToShape clears the variant bits and halves.
func MapValueToShape(value int) int { return (value & 0xfc) >> 1 }

// ShapeToTileIndex is the tile sheet position of a shape.
func ShapeToTileIndex(shape int) int { return shape >> 1 }

// MonsterVariantShape is the shape of a monster variant, which lives in its
// own rows of the tile sheet (DrawMap() in UltimaGraphics.c). Only map values
// 92..126 (Thief .. Exodus) have variants.
func MonsterVariantShape(value int) int {
	variant := value & 0x03
	if variant != 0 && value >= 92 && value <= 126 {
		return (((value>>2)-23)*2 + 79 + variant) * 2
	}
	return MapValueToShape(value)
}

// IsLetter is true for the letter tiles A..T (town signs; "I" may be a door).
func IsLetter(value int) bool { return value >= MapValueLetterA && value <= MapValueLetterT }

// Cells 68-78 hold one figure per class, in career-table order. A sheet
// without them shows the shared figure from ClassFallbackTile instead.
const (
	FirstClassTile = 68
	ClassCount     = 11
)

// ClassFallbackTile is the original's DetermineShape() grouping, Lark as the
// jester and Ranger as the party marker.
var ClassFallbackTile = []int{64, 65, 66, 67, 64, 64, 17, 66, 65, 66, 63}

// ClassTile is the tile of a class by career-table index; the party marker for an unknown class.
func ClassTile(career int) int {
	if career >= 0 && career < ClassCount {
		return FirstClassTile + career
	}
	return ShapeRanger >> 1
}

// IsClassTile says whether a tile index is one of the class figures.
func IsClassTile(index int) bool { return index >= FirstClassTile && index < FirstClassTile+ClassCount }

// ClassFallbackTileFor is the shared figure drawn for a class tile when the sheet has none.
func ClassFallbackTileFor(index int) int {
	if IsClassTile(index) {
		return ClassFallbackTile[index-FirstClassTile]
	}
	return index
}
