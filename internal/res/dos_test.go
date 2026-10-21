package res

import (
	"bytes"
	"os"
	"testing"
)

// TestLoadDOS compares the PC release (a folder of its files named by
// U3_PC_DIR) with the LairWare data where the two games agree; DATA.md lists
// where they do not.
func TestLoadDOS(t *testing.T) {
	dir := os.Getenv("U3_PC_DIR")
	if dir == "" {
		t.Skip("U3_PC_DIR not set")
	}
	pc, err := LoadDOS(dir)
	if err != nil {
		t.Fatal(err)
	}
	mac, err := LoadMac(refDir + "/Lairware/Resources/English.lproj")
	if err != nil {
		t.Skip("ref/ missing")
	}
	// LairWare's map fixes: a townsperson in Yew, two in Ambrosia.
	macOnly := map[int]bool{404: true, 421: true}
	for id, m := range mac.Maps {
		// Sosaria is saved over in play: a played copy has its monsters moved.
		if !macOnly[id] && id != Sosaria && !bytes.Equal(pc.Maps[id], m) {
			t.Errorf("map %d differs", id)
		}
	}
	for id, m := range mac.Monsters {
		if !macOnly[id] && id != Sosaria && !bytes.Equal(pc.Monsters[id], m) {
			t.Errorf("monsters %d differ", id)
		}
	}
	for id, m := range mac.Combat {
		if n := min(len(m), arenaTiles); !bytes.Equal(pc.Combat[id][:n], m[:n]) {
			t.Errorf("arena %d differs", id)
		}
		if id < 410 && !bytes.Equal(pc.Combat[id][128:172], m[128:172]) {
			t.Errorf("arena %d placements differ", id)
		}
	}
	// Talk: the first line of LCB agrees; most others differ in punctuation.
	if got, want := pc.Talk[402][:20], mac.Talk[402][:20]; !bytes.Equal(got, want) {
		t.Errorf("LCB talk %x, want %x", got, want)
	}
}

func TestLooksLikeMap(t *testing.T) {
	if !looksLikeMap(make([]byte, townSize)) || looksLikeMap(bytes.Repeat([]byte{0x55}, townSize)) {
		t.Error("looksLikeMap")
	}
}
