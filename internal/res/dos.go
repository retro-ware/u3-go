package res

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// pcMaps names the PC release's map files by Mac resource ID.
var pcMaps = map[int]string{
	400: "BRITISH", 401: "EXODUS", 402: "LCB", 403: "MOON", 404: "YEW", 405: "MONTOR-E", 406: "MONTOR-W",
	407: "GREY", 408: "DAWN", 409: "DEVIL", 410: "FAWN", 411: "DEATH",
	412: "M", 413: "FIRE", 414: "TIME", 415: "P", 416: "PERINIAN", 417: "MINE", 418: "DARDIN",
	420: "SOSARIA", 421: "AMBROSIA",
}

// pcArenas names the combat arenas: CONS 400-408 are CNFLCT-*.ULT, 410-413
// the shrine, fountain, time and brand pictures.
var pcArenas = map[int]string{
	400: "CNFLCT-A.ULT", 401: "CNFLCT-B.ULT", 402: "CNFLCT-C.ULT", 403: "CNFLCT-F.ULT", 404: "CNFLCT-G.ULT",
	405: "CNFLCT-M.ULT", 406: "CNFLCT-Q.ULT", 407: "CNFLCT-R.ULT", 408: "CNFLCT-S.ULT",
	410: "SHRINE.IMG", 411: "FOUNTAIN.IMG", 412: "TIME.IMG", 413: "BRAND.IMG",
}

// A PC town or outdoor map is 64x64 tiles, the talk block, then the monster
// table; a dungeon is eight 16x16 levels then a smaller talk block.
const (
	townSize, townTalk, townMonsters = 64 * 64, 384, 168
	dungeonSize, dungeonTalk         = 8 * 16 * 16, 144
	tableSize                        = 256 // the Mac's MONS and TLKS size
	arenaTiles                       = 11 * 11
	macLineBreak, macHighBit         = 255, 128 // Apple II text
)

// LoadDOS reads the PC release's files from dir. What the PC keeps in its
// executable comes from pctables.go.
func LoadDOS(dir string) (*Resources, error) {
	read := func(name string) ([]byte, error) {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			// Files on disk images may be lower case.
			b, err = os.ReadFile(filepath.Join(dir, strings.ToLower(name)))
		}
		return b, err
	}
	r := &Resources{
		Maps: map[int][]byte{}, Monsters: map[int][]byte{}, Talk: map[int][]byte{}, Combat: map[int][]byte{},
		MapNames: pcMapNames, Misc: pcMisc, DefaultParty: pcParty, Strings: pcStrings,
	}
	for id, name := range pcMaps {
		b, err := read(name + ".ULT")
		if err != nil {
			return nil, err
		}
		if id >= FirstDungeon && id <= LastDungeon {
			if len(b) < dungeonSize+dungeonTalk {
				return nil, fmt.Errorf("%s.ULT: short dungeon", name)
			}
			r.Maps[id] = b[:dungeonSize]
			r.Talk[id] = pcTalk(b[dungeonSize : dungeonSize+dungeonTalk])
			continue
		}
		if len(b) < townSize+townTalk+townMonsters {
			return nil, fmt.Errorf("%s.ULT: short map", name)
		}
		m := append([]byte{64}, b[:townSize]...)
		if id == Sosaria {
			m = append(m, pcWhirlpool...)
		} else {
			r.Talk[id] = pcTalk(b[townSize : townSize+townTalk])
		}
		r.Maps[id] = m
		r.Monsters[id] = pad(b[townSize+townTalk : townSize+townTalk+townMonsters])
	}
	for id, name := range pcArenas {
		b, err := read(name)
		if err != nil {
			return nil, err
		}
		if len(b) < arenaTiles {
			return nil, fmt.Errorf("%s: short arena", name)
		}
		r.Combat[id] = pcArena(b)
	}
	roster, err := read("ROSTER.ULT")
	if err != nil {
		return nil, err
	}
	r.DefaultRoster = roster
	return r, nil
}

func pad(b []byte) []byte {
	return append(bytes.Clone(b), make([]byte, tableSize-len(b))...)
}

// pcArena converts an arena to the Mac's tile numbering, which is twice the
// PC's: the 121 tiles, the tiles under the eight monsters (at 144) and under
// the four members (at 168).
func pcArena(b []byte) []byte {
	b = bytes.Clone(b)
	double := func(from, to int) {
		for i := from; i < to && i < len(b); i++ {
			b[i] *= 2
		}
	}
	double(0, arenaTiles)
	double(144, 152)
	double(168, 172)
	return b
}

// pcTalk converts a PC talk block (a table of little-endian offsets, its
// own length first, then NUL-terminated lower-case ASCII with \n line breaks)
// to the Mac's: each line NUL-led, upper case with the high bit set and 255 for
// line breaks.
func pcTalk(b []byte) []byte {
	out := []byte{0}
	n := int(b[0]) | int(b[1])<<8
	for i := 0; i < n/2 && 2*i+1 < len(b); i++ {
		at := int(b[2*i]) | int(b[2*i+1])<<8
		for ; at < len(b) && b[at] != 0 && len(out) < tableSize-1; at++ {
			switch c := b[at]; {
			case c == '\n':
				out = append(out, macLineBreak)
			case c == '`':
				out = append(out, '\''|macHighBit)
			default:
				out = append(out, bytes.ToUpper([]byte{c})[0]|macHighBit)
			}
		}
		out = append(out, 0)
	}
	return pad(out[:min(len(out), tableSize)])
}
