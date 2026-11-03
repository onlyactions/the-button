// SPDX-License-Identifier: Unlicense OR MIT

// Package gfx rasterizes the button icon — a filled circle with the
// power symbol — for tray icons, launcher icons, the quick-settings
// tile and the home-screen widget. It uses the exact geometry of the
// on-screen button in thebutton/internal/ui.
package gfx

import (
	"image"
	"image/color"
	"math"
)

// Colors matching the on-screen button.
var (
	OffColor = color.NRGBA{R: 0xE5, G: 0x39, B: 0x35, A: 0xFF} // red
	OnColor  = color.NRGBA{R: 0x2E, G: 0x7D, B: 0x32, A: 0xFF} // green
	SymColor = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF} // white
)

// Icon returns a size×size icon of the button in the given state.
// circleFrac is the circle radius as a fraction of size/2: 1.0
// touches the edges; ~0.9 suits launcher icons, ~0.45 small tray
// icons. The area outside the circle is transparent.
func Icon(size int, on bool, circleFrac float64) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	bg := OffColor
	if on {
		bg = OnColor
	}
	c := float64(size) / 2
	R := c * circleFrac
	// 3×3 supersampling keeps the edges smooth down to 24px icons.
	const ss = 3
	step := 1.0 / ss
	off := (step - 1) / 2 // center of first sample: -0.5 + step/2
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			var hit, glyph int
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					px := float64(x) + 0.5 + off + float64(sx)*step
					py := float64(y) + 0.5 + off + float64(sy)*step
					in, g := pixel(px, py, c, R)
					if in {
						hit++
						if g {
							glyph++
						}
					}
				}
			}
			if hit == 0 {
				continue
			}
			if glyph >= hit-glyph {
				img.SetNRGBA(x, y, SymColor)
			} else {
				img.SetNRGBA(x, y, bg)
			}
		}
	}
	return img
}

// pixel reports whether (x, y) lies inside the circle and, if so,
// whether it is part of the white power symbol. Geometry mirrors
// ui.drawPowerSymbol: a ring of radius 0.46·R and stroke 0.13·R with
// a 70° gap at the top, plus a vertical bar from the center to the
// top of the ring.
func pixel(x, y, c, R float64) (inCircle, onGlyph bool) {
	dx, dy := x-c, y-c
	d := math.Hypot(dx, dy)
	if d > R {
		return false, false
	}
	glyphR := R * 0.46
	stroke := R * 0.13
	// Ring with the gap at the top.
	if math.Abs(d-glyphR) <= stroke/2 {
		ang := math.Atan2(dy, dx) // up is -π/2
		const gapHalf = 35 * math.Pi / 180
		if math.Abs(ang+math.Pi/2) > gapHalf {
			return true, true
		}
	}
	// Vertical bar from the center up to the ring's top.
	if math.Abs(dx) <= stroke/2 && dy <= 0 && dy >= -glyphR {
		return true, true
	}
	return true, false
}

// BGRA returns the icon as 32bpp BGRA (the byte layout Windows
// bitmaps want): each uint32 packs 0xAARRGGBB on little-endian.
func BGRA(img *image.NRGBA) []uint32 {
	b := img.Bounds()
	out := make([]uint32, b.Dx()*b.Dy())
	i := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := img.NRGBAAt(x, y)
			out[i] = uint32(c.A)<<24 | uint32(c.R)<<16 | uint32(c.G)<<8 | uint32(c.B)
			i++
		}
	}
	return out
}

// ARGB returns the icon as big-endian ARGB32 bytes (the layout the
// StatusNotifierItem IconPixmap property wants): A, R, G, B per pixel.
func ARGB(img *image.NRGBA) []byte {
	b := img.Bounds()
	out := make([]byte, 0, b.Dx()*b.Dy()*4)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := img.NRGBAAt(x, y)
			out = append(out, c.A, c.R, c.G, c.B)
		}
	}
	return out
}
