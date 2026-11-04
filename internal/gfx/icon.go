// SPDX-License-Identifier: Unlicense OR MIT

// Package gfx rasterizes the button icon — a glossy circle with the
// power symbol — for tray icons, launcher icons, the quick-settings
// tile and the home-screen widget. The palette and glyph geometry
// mirror the on-screen button in thebutton/internal/ui.
package gfx

import (
	"image"
	"image/color"
	"math"
)

// Colors matching the on-screen button: the top of its candy gradient
// and the deep base.
var (
	OffTop   = color.NRGBA{R: 0xFF, G: 0x6A, B: 0x45, A: 0xFF}
	OffDeep  = color.NRGBA{R: 0xB8, G: 0x14, B: 0x09, A: 0xFF}
	OnTop    = color.NRGBA{R: 0x78, G: 0xE2, B: 0x8E, A: 0xFF}
	OnDeep   = color.NRGBA{R: 0x0A, G: 0x7F, B: 0x3C, A: 0xFF}
	SymColor = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
)

// Legacy aliases for the flat single-color look.
var (
	OffColor = OffDeep
	OnColor  = OnDeep
)

// Icon returns a size×size icon of the button in the given state,
// with the disc's vertical candy gradient. circleFrac is the disc
// radius as a fraction of size/2: 1.0 touches the edges; ~0.9 suits
// launcher icons, ~0.45 small tray icons. The area outside the circle
// is transparent.
func Icon(size int, on bool, circleFrac float64) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	top, deep := OffTop, OffDeep
	if on {
		top, deep = OnTop, OnDeep
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
			var shade, spec float64
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					px := float64(x) + 0.5 + off + float64(sx)*step
					py := float64(y) + 0.5 + off + float64(sy)*step
					in, g := pixel(px, py, c, R)
					if in {
						hit++
						shade += 0.5 - (py-c)/R/2 // 1 at the top, 0 at the bottom
						// A soft specular sheen in the upper-left,
						// mirroring the on-screen button's light.
						lx, ly := (px-c)/R+0.30, (py-c)/R+0.35
						spec += math.Exp(-(lx*lx + ly*ly) / (2 * 0.32 * 0.32))
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
				continue
			}
			t := shade / float64(hit)
			st := 0.62 * spec / float64(hit)
			if st > 1 {
				st = 1
			}
			if t > 1 {
				t = 1
			}
			img.SetNRGBA(x, y, color.NRGBA{
				R: lerp8(lerp8(deep.R, top.R, t), 0xFF, st),
				G: lerp8(lerp8(deep.G, top.G, t), 0xF2, st),
				B: lerp8(lerp8(deep.B, top.B, t), 0xE8, st),
				A: 0xFF,
			})
		}
	}
	return img
}

func lerp8(a, b uint8, t float64) uint8 {
	return uint8(float64(a) + (float64(b)-float64(a))*t)
}

// pixel reports whether (x, y) lies inside the circle and, if so,
// whether it is part of the white power symbol. Geometry mirrors the
// on-screen glyph: a ring of radius 0.42·R with a 48° gap at the top,
// plus a vertical bar from just below center up to the ring's inner
// edge. The stroke is chunkier than on screen so the symbol survives
// 16-24px tray sizes.
func pixel(x, y, c, R float64) (inCircle, onGlyph bool) {
	dx, dy := x-c, y-c
	d := math.Hypot(dx, dy)
	if d > R {
		return false, false
	}
	glyphR := R * 0.45
	stroke := R * 0.14
	// Ring with the gap at the top.
	if math.Abs(d-glyphR) <= stroke/2 {
		ang := math.Atan2(dy, dx) // up is -π/2
		const gapHalf = 24 * math.Pi / 180
		if math.Abs(ang+math.Pi/2) > gapHalf {
			return true, true
		}
	}
	// Vertical bar from just below center up to the ring's inner edge.
	if math.Abs(dx) <= stroke/2*1.12 && dy <= stroke/2*0.5 && dy >= -glyphR {
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
