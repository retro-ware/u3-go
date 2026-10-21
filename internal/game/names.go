package game

import "slices"

// Names is a stock of character names for the "random name" choice on the
// character creation screen. Nothing in the game depends on them.
var Names = []string{
	"Aldric", "Brenna", "Cedric", "Dara", "Elric", "Fenna", "Gareth", "Hilde", "Ivar", "Jorun",
	"Kael", "Liora", "Merrick", "Nessa", "Orin", "Petra", "Quill", "Rowan", "Sabra", "Torvin",
	"Ulla", "Varek", "Wren", "Xanthe", "Ysolde", "Zarek", "Ansel", "Bryn", "Corwin", "Delia",
	"Emrys", "Faelan", "Gwyn", "Halvard", "Isolde", "Jasper", "Keira", "Lorcan", "Maren", "Niall",
	"Oriel", "Pell", "Rhiannon", "Soren", "Tamsin", "Ulric", "Vesna", "Wystan", "Yorick", "Zelda",
}

// RandomName is a name from the list, avoiding taken (roster names) when it can.
func RandomName(random func() float64, taken []string) string {
	var free []string
	for _, n := range Names {
		if !slices.Contains(taken, n) {
			free = append(free, n)
		}
	}
	if len(free) == 0 {
		free = Names
	}
	return free[int(random()*float64(len(free)))%len(free)]
}
