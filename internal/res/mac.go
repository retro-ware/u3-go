package res

import (
	"encoding/binary"
	"errors"
	"fmt"
	"golang.org/x/text/encoding/charmap"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// macForks are where a LairWare install keeps its resource fork: the
// repository's English.lproj, or the 2.0 release's app Resources folder.
var macForks = []string{"MainResources.rsrc", "Ultima III.rsrc", "English.lproj/MainResources.rsrc"}

// LoadMac reads a LairWare install's resource fork and Strings/*.plist from
// dir (an English.lproj or Resources folder).
func LoadMac(dir string) (*Resources, error) {
	var fork []byte
	var err error
	for _, name := range macForks {
		if fork, err = os.ReadFile(filepath.Join(dir, name)); err == nil {
			break
		}
	}
	if err != nil {
		return nil, fmt.Errorf("no LairWare resource fork in %s", dir)
	}
	rs, err := parseFork(fork)
	if err != nil {
		return nil, err
	}
	r := &Resources{
		Maps: map[int][]byte{}, Monsters: map[int][]byte{}, Talk: map[int][]byte{}, Combat: map[int][]byte{},
		MapNames: map[int]string{}, Strings: map[string][]string{},
	}
	misc := map[int][]byte{}
	for _, x := range rs {
		switch x.typ {
		case "MAPS":
			r.Maps[x.id] = x.data
			r.MapNames[x.id] = x.name
		case "MONS":
			r.Monsters[x.id] = x.data
		case "TLKS":
			r.Talk[x.id] = x.data
		case "CONS":
			r.Combat[x.id] = x.data
		case "MISC":
			misc[x.id] = x.data
		case "PRTY":
			r.DefaultParty = x.data
		case "ROST":
			r.DefaultRoster = x.data
		}
	}
	if r.Misc, err = miscFrom(misc); err != nil {
		return nil, err
	}
	sdir := filepath.Join(dir, "Strings")
	if _, err := os.Stat(sdir); err != nil {
		sdir = filepath.Join(dir, "English.lproj", "Strings")
	}
	plists, _ := filepath.Glob(filepath.Join(sdir, "*.plist"))
	for _, p := range plists {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		r.Strings[strings.TrimSuffix(filepath.Base(p), ".plist")] = plistStrings(string(b))
	}
	// LairWare 2.0 has no MoreMessages: fill missing tables from the later release.
	for k, v := range pcStrings {
		if _, ok := r.Strings[k]; !ok {
			r.Strings[k] = v
		}
	}
	return r, nil
}

func miscFrom(m map[int][]byte) (Misc, error) {
	t := func(n, from, to int) []byte {
		b := m[BaseRes+n]
		if len(b) < to {
			return nil
		}
		return b[from:to]
	}
	x := Misc{
		MoonX: t(0, 0, 8), MoonY: t(0, 8, 16),
		Career: t(1, 0, 11), WeaponUse: t(2, 0, 11), ArmourUse: t(3, 0, 11),
		LocationX: t(4, 0, 32), LocationY: t(4, 32, 64),
		Experience: t(5, 0, 16),
	}
	if x.MoonX == nil || x.Career == nil || x.WeaponUse == nil || x.ArmourUse == nil || x.LocationX == nil || x.Experience == nil {
		return x, errors.New("MISC tables missing")
	}
	return x, nil
}

var plistString = regexp.MustCompile(`(?s)<string>(.*?)</string>`)

func plistStrings(xml string) []string {
	var out []string
	for _, m := range plistString.FindAllStringSubmatch(xml, -1) {
		out = append(out, html.UnescapeString(m[1]))
	}
	return out
}

type macRes struct {
	typ  string
	id   int
	name string
	data []byte
}

// parseFork reads a classic Mac OS resource fork (Inside Macintosh: More
// Macintosh Toolbox, "Resource Manager").
func parseFork(f []byte) (out []macRes, err error) {
	defer func() {
		if recover() != nil {
			err = errors.New("damaged resource fork")
		}
	}()
	be := binary.BigEndian
	dataOff := int(be.Uint32(f))
	mapOff := int(be.Uint32(f[4:]))
	types := mapOff + int(be.Uint16(f[mapOff+24:]))
	names := mapOff + int(be.Uint16(f[mapOff+26:]))
	p := types + 2
	for range int(be.Uint16(f[types:])) + 1 {
		typ := string(f[p : p+4])
		refs := types + int(be.Uint16(f[p+6:]))
		for range int(be.Uint16(f[p+4:])) + 1 {
			id := int(int16(be.Uint16(f[refs:])))
			nameOff := int16(be.Uint16(f[refs+2:]))
			start := dataOff + int(be.Uint32(f[refs+4:])&0xFFFFFF)
			n := int(be.Uint32(f[start:]))
			name := ""
			if nameOff != -1 {
				at := names + int(nameOff)
				name = macRoman(f[at+1 : at+1+int(f[at])])
			}
			out = append(out, macRes{typ, id, name, f[start+4 : start+4+n]})
			refs += 12
		}
		p += 8
	}
	return out, nil
}

// macRoman decodes Mac OS Roman text (the reference extractor kept the bytes
// as Latin-1, giving "DardinÕs Pit"), with curly apostrophes made straight
// for the games' ASCII fonts.
func macRoman(b []byte) string {
	s, _ := charmap.Macintosh.NewDecoder().Bytes(b)
	return strings.ReplaceAll(string(s), "’", "'")
}
