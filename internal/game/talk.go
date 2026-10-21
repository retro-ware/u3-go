package game

// Speech mirrors Speak(): NPC dialogue. A TLKS resource is a run of NUL
// terminated strings; person selects the nth. Characters have the high bit
// set (Apple II text) and 0xFF marks a line break.
func Speech(talk []byte, person int) string {
	at := func(p int) byte {
		if p < len(talk) {
			return talk[p]
		}
		return 0
	}
	p := 0
	for person > 0 && p < 256 {
		for at(p) != 0 && p < 256 {
			p++
		}
		person--
		p++
	}
	var out []byte
	for p < 256 && at(p) != 0 {
		if at(p) == 0xff {
			out = append(out, '\n')
		} else {
			out = append(out, at(p)&0x7f)
		}
		p++
	}
	return string(out)
}
