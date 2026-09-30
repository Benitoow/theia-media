// check-playback-picture catches a black compositor under a working OSD.
// Only use it with create-proof-media's coloured test pattern: it is not a
// heuristic for whether an arbitrary film is playing.
package main

import (
	"flag"
	"fmt"
	"image/png"
	"os"
)

func main() {
	path := flag.String("image", "", "captured window playing the generated colour fixture")
	flag.Parse()
	if err := check(*path); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func check(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	picture, err := png.Decode(f)
	if err != nil {
		return err
	}
	b := picture.Bounds()
	coloured, sampled := 0, 0
	// Stay away from window decorations and the controls at either edge.
	for y := b.Min.Y + b.Dy()/4; y < b.Min.Y+b.Dy()*3/4; y += 4 {
		for x := b.Min.X + b.Dx()/4; x < b.Min.X+b.Dx()*3/4; x += 4 {
			r, g, blue, _ := picture.At(x, y).RGBA()
			high, low := max(r, g, blue), min(r, g, blue)
			if high > 50*257 && high-low > 30*257 {
				coloured++
			}
			sampled++
		}
	}
	if sampled == 0 || coloured*100 < sampled*20 {
		return fmt.Errorf("%s: colour fixture is missing under the controls (%d/%d coloured pixels)", path, coloured, sampled)
	}
	fmt.Printf("PASS %s: generated film is visible (%d/%d coloured pixels)\n", path, coloured, sampled)
	return nil
}
