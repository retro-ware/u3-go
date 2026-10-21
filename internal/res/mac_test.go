package res

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"reflect"
	"strconv"
	"testing"
)

const refDir = "../../ref/ultima3"

type bundle map[string]map[string]json.RawMessage

func loadBundle(t *testing.T) (bundle, map[string]map[int][]byte, map[int]string) {
	b, err := os.ReadFile(refDir + "/web/public/data/resources.json")
	if err != nil {
		t.Skip("ref/ missing")
	}
	var j bundle
	if err := json.Unmarshal(b, &j); err != nil {
		t.Fatal(err)
	}
	data := map[string]map[int][]byte{}
	names := map[int]string{}
	for k, sec := range j {
		if k == "strings" {
			continue
		}
		data[k] = map[int][]byte{}
		for id, raw := range sec {
			var e struct{ Name, Data string }
			json.Unmarshal(raw, &e)
			n, _ := strconv.Atoi(id)
			data[k][n], _ = base64.StdEncoding.DecodeString(e.Data)
			if k == "maps" {
				names[n] = e.Name
			}
		}
	}
	return j, data, names
}

func TestLoadMac(t *testing.T) {
	j, want, names := loadBundle(t)
	r, err := LoadMac(refDir + "/Lairware/Resources/English.lproj")
	if err != nil {
		t.Fatal(err)
	}
	eq := func(what string, got, want any) {
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s differs", what)
		}
	}
	eq("maps", r.Maps, want["maps"])
	eq("monsters", r.Monsters, want["monsters"])
	eq("talk", r.Talk, want["talk"])
	eq("combat", r.Combat, want["combat"])
	eq("names", r.MapNames, names)
	eq("party", r.DefaultParty, want["party"][500])
	eq("roster", r.DefaultRoster, want["roster"][500])
	m, _ := miscFrom(want["misc"])
	eq("misc", r.Misc, m)
	strs := map[string][]string{}
	for k, raw := range j["strings"] {
		var s []string
		json.Unmarshal(raw, &s)
		strs[k] = s
	}
	eq("strings", r.Strings, strs)
}

func TestPCTables(t *testing.T) {
	r, err := LoadMac(refDir + "/Lairware/Resources/English.lproj")
	if err != nil {
		t.Skip("ref/ missing")
	}
	if !reflect.DeepEqual(r.Misc, pcMisc) || !reflect.DeepEqual(r.DefaultParty, pcParty) ||
		!reflect.DeepEqual(r.MapNames, pcMapNames) || !reflect.DeepEqual(r.Strings, pcStrings) ||
		!reflect.DeepEqual(r.Maps[Sosaria][4097:], pcWhirlpool) {
		t.Error("pctables.go differs from the LairWare data")
	}
}
