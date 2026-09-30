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
	controls := flag.Bool("controls", false, "also require the painted dark control band and light controls")
	flag.Parse()
	if err := check(*path, *controls); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func check(path string, controls bool) error {
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
	if controls {
		dark, light, total := 0, 0, 0
		// The generated fixture fills this band with saturated colour. The
		// actual OSD darkens it and paints light icons/text on top. DOM geometry
		// alone cannot prove that a native video child has not covered the OSD.
		for y := b.Min.Y + b.Dy()*80/100; y < b.Min.Y+b.Dy()*97/100; y += 2 {
			for x := b.Min.X + b.Dx()/10; x < b.Min.X+b.Dx()*9/10; x += 2 {
				r, g, blue, _ := picture.At(x, y).RGBA()
				high, low := max(r, g, blue), min(r, g, blue)
				if high < 100*257 {
					dark++
				}
				if low > 130*257 && high-low < 35*257 {
					light++
				}
				total++
			}
		}
		if total == 0 || dark*100 < total*30 || light < 10 {
			return fmt.Errorf("%s: painted playback controls are missing (%d/%d dark pixels, %d light pixels)", path, dark, total, light)
		}
		fmt.Printf("PASS %s: painted control band and icons/text are visible\n", path)
	}
	return nil
}
