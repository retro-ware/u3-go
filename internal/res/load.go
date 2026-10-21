package res

import (
	"errors"
	"path/filepath"
)

// Load reads an imported data directory: the LairWare files in dir/mac if
// present, else the PC release's in dir/pc. TileSets offers the graphics of both.
func Load(dir string) (*Resources, error) {
	r, macErr := LoadMac(filepath.Join(dir, "mac"))
	if macErr == nil {
		return r, nil
	}
	r, pcErr := LoadDOS(filepath.Join(dir, "pc"))
	if pcErr == nil {
		return r, nil
	}
	return nil, errors.Join(macErr, pcErr)
}
