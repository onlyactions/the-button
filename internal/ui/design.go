// SPDX-License-Identifier: Unlicense OR MIT

// The look of the button: palette and painting primitives. Every
// shape is a circle, an ellipse or a baked texture, so the whole
// design is expressed with a handful of tiny helpers.
package ui

import (
	"image"
	"image/color"
	"math"

	"gioui.org/f32"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
)

// palette is everything that changes between red (off) and green
// (on).
type palette struct {
	top, mid, bottom, deep color.NRGBA // the disc's vertical gradient stops
	spec                   color.NRGBA // specular hotspot tint
	glow                   color.NRGBA // the halo and the room light
	room                   float32     // how strongly the button lights the room
	haloMul                float32     // halo size: off is tight, on is wide
	haloGain               float32     // halo brightness
	coreGlow               float32     // inner luminance of the body
	glyphGlow              float32     // how much the body shines through the glyph
	specGain               float32     // specular intensity
	shadow                 float32     // shadow strength: strong when off, soft when lit
}

var (
	// Off: a dormant ember — dark saturated body, dim tight halo,
	// grounded by a firm shadow.
	palOff = palette{
		top:       color.NRGBA{R: 0xFF, G: 0x6A, B: 0x45, A: 0xFF},
		mid:       color.NRGBA{R: 0xE4, G: 0x2E, B: 0x1D, A: 0xFF},
		bottom:    color.NRGBA{R: 0xB8, G: 0x14, B: 0x09, A: 0xFF},
		deep:      color.NRGBA{R: 0x7C, G: 0x0C, B: 0x04, A: 0xFF},
		spec:      color.NRGBA{R: 0xFF, G: 0xE4, B: 0xD3, A: 0xFF},
		glow:      color.NRGBA{R: 0xFF, G: 0x45, B: 0x26, A: 0xFF},
		room:      0.040,
		haloMul:   0.92,
		haloGain:  0.36,
		coreGlow:  0.0,
		glyphGlow: 0.22,
		specGain:  0.90,
		shadow:    1.25,
	}
	// On: energized — luminous body, wide breathing halo, the room
	// lit, shadows washed out by its own light.
	palOn = palette{
		top:       color.NRGBA{R: 0x78, G: 0xE2, B: 0x8E, A: 0xFF},
		mid:       color.NRGBA{R: 0x1F, G: 0xAC, B: 0x56, A: 0xFF},
		bottom:    color.NRGBA{R: 0x0C, G: 0x8F, B: 0x45, A: 0xFF},
		deep:      color.NRGBA{R: 0x05, G: 0x63, B: 0x31, A: 0xFF},
		spec:      color.NRGBA{R: 0xF0, G: 0xFF, B: 0xEA, A: 0xFF},
		glow:      color.NRGBA{R: 0x36, G: 0xE8, B: 0x84, A: 0xFF},
		room:      0.075,
		haloMul:   1.08,
		haloGain:  1.0,
		coreGlow:  0.10,
		glyphGlow: 0.55,
		specGain:  1.15,
		shadow:    0.72,
	}

	bgTop    = color.NRGBA{R: 0x17, G: 0x18, B: 0x1D, A: 0xFF}
	bgBottom = color.NRGBA{R: 0x08, G: 0x09, B: 0x0C, A: 0xFF}

	glyphColor = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xF6}
)

// lerpPal crossfades two palettes. The blend is lifted through a
// luminous midpoint: at the halfway point every color is pushed
// toward light, so the toggle flashes softly instead of squelching
// through muddy brown.
func lerpPal(a, b palette, f float32) palette {
	f = clamp01(f)
	lift := 0.52 * float32(math.Sin(math.Pi*float64(f)))
	liftColor := func(c color.NRGBA) color.NRGBA {
		return color.NRGBA{
			R: lerpU8(c.R, 0xFF, lift),
			G: lerpU8(c.G, 0xFF, lift),
			B: lerpU8(c.B, 0xFF, lift),
			A: c.A,
		}
	}
	return palette{
		top:       liftColor(lerpColor(a.top, b.top, f)),
		mid:       liftColor(lerpColor(a.mid, b.mid, f)),
		bottom:    liftColor(lerpColor(a.bottom, b.bottom, f)),
		deep:      liftColor(lerpColor(a.deep, b.deep, f)),
		spec:      lerpColor(a.spec, b.spec, f),
		glow:      liftColor(lerpColor(a.glow, b.glow, f)),
		room:      a.room + (b.room-a.room)*f,
		haloMul:   a.haloMul + (b.haloMul-a.haloMul)*f,
		haloGain:  a.haloGain + (b.haloGain-a.haloGain)*f,
		coreGlow:  a.coreGlow + (b.coreGlow-a.coreGlow)*f,
		glyphGlow: a.glyphGlow + (b.glyphGlow-a.glyphGlow)*f,
		specGain:  a.specGain + (b.specGain-a.specGain)*f,
		shadow:    a.shadow + (b.shadow-a.shadow)*f,
	}
}

func lerpColor(a, b color.NRGBA, f float32) color.NRGBA {
	return color.NRGBA{
		R: lerpU8(a.R, b.R, f),
		G: lerpU8(a.G, b.G, f),
		B: lerpU8(a.B, b.B, f),
		A: lerpU8(a.A, b.A, f),
	}
}

func lerpU8(a, b uint8, f float32) uint8 {
	return uint8(float32(a) + (float32(b)-float32(a))*f)
}

func lerp(a, b, f float32) float32 {
	return a + (b-a)*f
}

// withAlpha returns c at the given opacity (0..1).
func withAlpha(c color.NRGBA, f float32) color.NRGBA {
	f = clamp01(f)
	return color.NRGBA{R: c.R, G: c.G, B: c.B, A: uint8(clamp01(float32(c.A)/255*f) * 255)}
}

// pt returns the point on a circle of radius r around c at screen
// angle a (radians; y grows downwards, so -pi/2 is the top).
func pt(c f32.Point, r, a float32) f32.Point {
	return f32.Pt(c.X+r*float32(math.Cos(float64(a))), c.Y+r*float32(math.Sin(float64(a))))
}

// fillCircle paints a solid circle.
func fillCircle(ops *op.Ops, c f32.Point, r float32, col color.NRGBA) {
	if r <= 0 || col.A == 0 {
		return
	}
	paint.FillShape(ops, col, clip.Outline{Path: circlePath(ops, c, r)}.Op())
}

// circlePath returns a full circle of radius r around c.
func circlePath(ops *op.Ops, c f32.Point, r float32) clip.PathSpec {
	var p clip.Path
	p.Begin(ops)
	p.MoveTo(f32.Pt(c.X+r, c.Y))
	p.Arc(f32.Pt(-r, 0), f32.Pt(-r, 0), 2*math.Pi)
	return p.End()
}

// ellipseGlow paints a soft elliptical glow: a unit-circle glow
// scaled to the ellipse's radii and moved to c. Uses only filled
// circles, so the anisotropic scale cannot distort any stroke.
func ellipseGlow(ops *op.Ops, c f32.Point, rx, ry, peak float32, layers int, k float32, col color.NRGBA) {
	if rx <= 0 || ry <= 0 || peak <= 0 || col.A == 0 {
		return
	}
	defer op.Affine(f32.AffineId().Scale(f32.Point{}, f32.Pt(rx, ry)).Offset(c)).Push(ops).Pop()
	glow(ops, f32.Pt(0, 0), 0.5, 1.0, peak, layers, k, col)
}

// glow paints a soft radial glow as a stack of concentric circles.
// The weights telescope — circle i carries C(t_i) - C(t_i+1) of the
// falloff C(t) = (1-t)^k — so the stacked alpha hits the desired
// profile exactly at every circle boundary and interpolates linearly
// in between. Steps are O(1/N^2): no visible rings, where naive
// value-proportional weights would step O(1/N).
func glow(ops *op.Ops, c f32.Point, r0, r1, peak float32, layers int, k float32, col color.NRGBA) {
	if peak <= 0 || r1 <= r0 || layers <= 0 {
		return
	}
	fall := func(t float64) float64 {
		return math.Pow(1-t, float64(k))
	}
	for i := 0; i < layers; i++ {
		t0 := float64(i) / float64(layers)
		t1 := float64(i+1) / float64(layers)
		a := (fall(t0) - fall(t1)) * float64(peak)
		if a <= 0 {
			continue
		}
		r := lerp(r0, r1, float32((t0+t1)/2))
		fillCircle(ops, c, r, withAlpha(col, float32(a)))
	}
}

// fillScreen paints a full-canvas vertical gradient.
func fillScreen(ops *op.Ops, w, h int, top, bottom color.NRGBA) {
	st := clip.Rect{Max: image.Pt(w, h)}.Push(ops)
	paint.LinearGradientOp{
		Stop1:  f32.Pt(0, 0),
		Color1: top,
		Stop2:  f32.Pt(0, float32(h)),
		Color2: bottom,
	}.Add(ops)
	paint.PaintOp{}.Add(ops)
	st.Pop()
}
