package game

import (
	"encoding/json"
	"strings"
	"testing"
)

// savedCopy is a save as it comes back from storage: through JSON.
func savedCopy(t *testing.T, w *World) SaveData {
	t.Helper()
	b, err := json.Marshal(Serialize(w))
	if err != nil {
		t.Fatal(err)
	}
	var d SaveData
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatal(err)
	}
	return d
}

func TestSaveRoundTrip(t *testing.T) {
	a := newWorld(t)
	a.X, a.Y = 10, 11
	a.Member(0).SetGold(999)
	a.PutXYVal(MapValueFrigate, 3, 4)
	a.MoonPhase = [2]int{2, 6}
	a.WindDirection = 3
	data := savedCopy(t, a)
	b := NewWorld(a.Resources, 0)
	if !Restore(b, data) {
		t.Fatal("restore")
	}
	if b.X != 10 || b.Y != 11 || b.Member(0).Gold() != 999 || b.Member(0).Name() != a.Member(0).Name() ||
		b.GetXYVal(3, 4) != MapValueFrigate || b.MoonPhase != [2]int{2, 6} || b.WindDirection != 3 || b.Party.Location() != LocationSosaria {
		t.Fatal("mismatch")
	}
}

func TestSaveInTownIsSurfaceSquare(t *testing.T) {
	a := newWorld(t)
	a.ReturnX, a.ReturnY = 46, 19
	a.Party.SetLocation(LocationTown)
	a.EnterMap(402)
	a.X, a.Y = 5, 32
	b := NewWorld(a.Resources, 0)
	if !Restore(b, Serialize(a)) || b.Party.Location() != LocationSosaria || b.X != 46 || b.Y != 19 || b.Current != b.Surface {
		t.Fatal("town save")
	}
}

func TestSaveRejectsOtherVersion(t *testing.T) {
	a := newWorld(t)
	data := Serialize(a)
	data.Version = 99
	if Restore(NewWorld(a.Resources, 0), data) {
		t.Fatal("accepted v99")
	}
}

func TestSaveMigratesV2Bags(t *testing.T) {
	a := newWorld(t)
	a.Member(0).Bytes[48+6] = 2
	a.Member(0).Bytes[48] = 6
	a.Member(1).Bytes[40+3] = 1
	data := savedCopy(t, a)
	data.Version = 2
	b := NewWorld(a.Resources, 0)
	if !Restore(b, data) || b.Party.Weapons(6) != 1 || b.Member(0).Bytes[48] != 6 || b.Member(0).Bytes[48+6] != 0 || b.Party.Armour(3) != 1 {
		t.Fatal("v2")
	}
}

func TestSaveMigratesV3Supplies(t *testing.T) {
	a := newWorld(t)
	a.Member(0).Bytes[37] = 2
	a.Member(1).Bytes[38] = 1
	a.Member(2).SetTorches(4)
	data := savedCopy(t, a)
	data.Version = 3
	b := NewWorld(a.Resources, 0)
	if !Restore(b, data) || b.Party.Gems() != 2 || b.Party.Keys() != 1 || b.Party.Torches() != 4 || b.Member(2).Torches() != 0 {
		t.Fatal("v3")
	}
}

func TestSaveCarriesTheMap(t *testing.T) {
	w := newWorld(t)
	w.AutoMap.MarkSquares(MapIDSosaria, []Point{{10, 10}})
	w.AutoMap.Mark(412, 0, 3, 3)
	w.AutoMap.Visit("shrine:1,2")
	data := savedCopy(t, w)
	w.AutoMap.MarkSquares(MapIDSosaria, []Point{{40, 40}})
	other := newWorld(t)
	other.AutoMap.MarkSquares(MapIDSosaria, []Point{{1, 1}})
	if !Restore(other, data) {
		t.Fatal("restore")
	}
	m := other.AutoMap
	if !m.SquareSeen(MapIDSosaria, 10, 10) || !m.IsSeen(412, 0, 3, 3) || !m.Visited("shrine:1,2") ||
		m.SquareSeen(MapIDSosaria, 40, 40) || m.SquareSeen(MapIDSosaria, 1, 1) {
		t.Fatal("map")
	}
}

func TestDirStoreRoundTrip(t *testing.T) {
	s := DirStore(t.TempDir())
	w := newWorld(t)
	w.Party.SetGold(777)
	if !WriteSave(s, w) {
		t.Fatal("write")
	}
	b := newWorld(t)
	if !ReadSave(s, b) || b.Party.Gold() != 777 {
		t.Fatal("read")
	}
	ClearSave(s)
	if ReadSave(s, b) {
		t.Fatal("cleared")
	}
}

func TestExportImportRoundTrip(t *testing.T) {
	w := newWorld(t)
	w.Party.SetGold(4321)
	w.X, w.Y = 12, 34
	w.AutoMap.MarkSquares(MapIDSosaria, []Point{{12, 34}})
	parsed, err := ParseExport(ExportGame(w))
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Automap == nil || parsed.Save.Automap == nil || *parsed.Automap != *parsed.Save.Automap {
		t.Fatal("automap beside the save")
	}
	other := newWorld(t)
	other.AutoMap.MarkSquares(MapIDSosaria, []Point{{1, 1}})
	if !RestoreExported(other, parsed) || other.Party.Gold() != 4321 || other.X != 12 || other.Y != 34 ||
		!other.AutoMap.SquareSeen(MapIDSosaria, 12, 34) || other.AutoMap.SquareSeen(MapIDSosaria, 1, 1) {
		t.Fatal("import")
	}
}

func TestExportOlderMapBesideSave(t *testing.T) {
	w := newWorld(t)
	w.AutoMap.MarkSquares(MapIDSosaria, []Point{{5, 6}})
	m := w.AutoMap.ToText()
	save := savedCopy(t, w)
	save.Automap = nil
	other := newWorld(t)
	if !RestoreExported(other, &ExportedGame{Save: &save, Automap: &m}) || !other.AutoMap.SquareSeen(MapIDSosaria, 5, 6) {
		t.Fatal("beside")
	}
	if !RestoreExported(other, &ExportedGame{Save: &save}) || other.AutoMap.SquareSeen(MapIDSosaria, 5, 6) {
		t.Fatal("neither")
	}
}

func TestParseExportRefuses(t *testing.T) {
	for text, want := range map[string]string{
		"hello":                       "not JSON",
		`{"game":"other","format":1}`: "not an Ultima III game",
		`{"game":"ultima3","format":2,"save":{}}`:            "newer",
		`{"game":"ultima3","format":1,"save":{"version":1}}`: "incomplete",
	} {
		if _, err := ParseExport(text); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v, want %s", text, err, want)
		}
	}
}
