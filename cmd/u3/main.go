// Command u3 runs Ultima III from either release's data.
//
//	u3 -import SRC [-data DIR]  copy the game files out of SRC (LairWare's .dmg, app or
//	                            folder; the PC release's disk image, zip or files) into DIR,
//	                            write DIR/COVER.PNG and exit
package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	data := flag.String("data", "data", "directory with the imported game files")
	imp := flag.String("import", "", "import the game files from this folder/zip/disk image/DMG into -data and exit")
	flag.Parse()
	if *imp != "" {
		if err := importGame(*imp, *data); err != nil {
			fmt.Fprintln(os.Stderr, "import:", err)
			os.Exit(1)
		}
		return
	}
}
