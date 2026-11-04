// SPDX-License-Identifier: Unlicense OR MIT

package ui

import (
	"image"
	"image/png"
	"os"
	"testing"
)

// TestDumpDiscTexture writes the baked off/on disc and glyph textures
// to /tmp so the shading math can be inspected without a renderer.
func TestDumpDiscTexture(t *testing.T) {
	for name, pal := range map[string]palette{"off": palOff, "on": palOn} {
		bakeTextures(pal)
		for _, part := range []struct {
			suffix string
			img    *image.RGBA
		}{{"disc", texDisc}, {"glyph", texGlyph}} {
			f, err := os.Create("/tmp/tex-" + name + "-" + part.suffix + ".png")
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			if err := png.Encode(f, part.img); err != nil {
				t.Fatal(err)
			}
		}
	}
}
