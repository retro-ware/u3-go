package game

import "time"

// Random is a small seedable pseudo-random generator (mulberry32), as the
// TypeScript port used in place of the Mac Toolbox Random(), so tests can be
// deterministic and draw the same numbers in the same order.
type Random struct {
	state uint32
}

// NewRandom seeds a generator; seed 0 means "from the clock".
func NewRandom(seed uint32) *Random {
	if seed == 0 {
		seed = uint32(time.Now().UnixMilli())
	}
	return &Random{state: seed}
}

// Next is a uniform float in [0, 1).
func (r *Random) Next() float64 {
	r.state += 0x6d2b79f5
	t := r.state
	t = (t ^ t>>15) * (t | 1)
	t ^= t + (t^t>>7)*(t|61)
	return float64(t^t>>14) / 4294967296
}

// Range is an integer in the inclusive range [low, high]. Mirrors RandNum() in UltimaMain.c.
func (r *Random) Range(low, high int) int {
	return low + int(r.Next()*float64(high-low+1))
}
