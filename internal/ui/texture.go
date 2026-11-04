// SPDX-License-Identifier: Unlicense OR MIT

// Everything static about the button's face is baked in software with
// float64 math and ordered dithering: the candy disc (four-stop
// gradient, specular hotspot, rim light, grounding shade) and the
// power glyph (bloom, emboss shadow, white strokes — drawn from an
// analytic distance field). GPU-side gradients and stroked paths are
// avoided entirely: on some WebGL/SwiftShader pipelines their
// anti-aliasing coverage steps visibly, and two-stop gradients band.
// The baked textures are regenerated only while the palette blend
// actually changes (during the 220ms color crossfade); the expanding
// toggle ripple is a tiny per-frame bake.
package ui

import (
	"image"
	"image/color"
	"math"
)

// texN is the resolution of the disc and glyph textures. The disc
// spans up to ~520 device pixels (~660 on a 4K desktop), so a 320px
// texture is upscaled at most ~2x — nothing in it has detail finer
// than its own gradients, and the glyph edge is anti-aliased in the
// bake.
const texN = 320

// bayer8 is the classic 8x8 ordered-dither matrix; it breaks up the
// 8-bit quantization steps that would otherwise band.
var bayer8 = [64]uint8{
	0, 32, 8, 40, 2, 34, 10, 42,
	48, 16, 56, 24, 50, 18, 58, 26,
	12, 44, 4, 36, 14, 46, 6, 38,
	60, 28, 52, 20, 62, 30, 54, 22,
	3, 35, 11, 43, 1, 33, 9, 41,
	51, 19, 59, 27, 49, 17, 57, 25,
	15, 47, 7, 39, 13, 45, 5, 37,
	63, 31, 55, 23, 61, 29, 53, 21,
}

var (
	texPal    palette
	texDisc   *image.RGBA
	texGlyph  *image.RGBA
	ringCache *image.RGBA
	ringKey   ringKeyT
)

// discImage returns the baked disc texture for pal, reusing the
// previous bake while the palette has not moved.
func discImage(pal palette) *image.RGBA {
	if texDisc == nil || !palClose(texPal, pal) {
		bakeTextures(pal)
	}
	return texDisc
}

// glyphImage returns the baked glyph texture for pal.
func glyphImage(pal palette) *image.RGBA {
	if texGlyph == nil || !palClose(texPal, pal) {
		bakeTextures(pal)
	}
	return texGlyph
}

func palClose(a, b palette) bool {
	near := func(x, y uint8) bool {
		d := int(x) - int(y)
		return d < 2 && d > -2
	}
	for _, c := range [][2]color.NRGBA{
		{a.top, b.top}, {a.mid, b.mid}, {a.bottom, b.bottom},
		{a.deep, b.deep}, {a.spec, b.spec}, {a.glow, b.glow},
	} {
		if !near(c[0].R, c[1].R) || !near(c[0].G, c[1].G) || !near(c[0].B, c[1].B) {
			return false
		}
	}
	return true
}

// bakeTextures renders the disc and glyph textures for pal in one
// pass over a shared coordinate frame: u,v in [-1,1] across the disc,
// d the distance from the center, ang the screen angle (atan2 with y
// down, so -pi/2 is the top).
func bakeTextures(pal palette) {
	n := texN
	disc := image.NewRGBA(image.Rect(0, 0, n, n))
	glyph := image.NewRGBA(image.Rect(0, 0, n, n))
	dp, gp := disc.Pix, glyph.Pix

	top := nrgbaF(pal.top)
	mid := nrgbaF(pal.mid)
	bot := nrgbaF(pal.bottom)
	deep := nrgbaF(pal.deep)
	spec := nrgbaF(pal.spec)
	glow := nrgbaF(pal.glow)

	half := float64(n-1) / 2

	for y := 0; y < n; y++ {
		v := 2*float64(y)/float64(n-1) - 1
		for x := 0; x < n; x++ {
			u := 2*float64(x)/float64(n-1) - 1
			d := math.Hypot(u, v)
			dith := (float64(bayer8[(y&7)*8+(x&7)])+0.5)/64 - 0.5

			// ---------------- the disc texture ----------------

			// Edge coverage across ~1.25 texels: the silhouette stays
			// smooth under upscaling.
			a := (1 - d) * half / 1.25
			if a > 1 {
				a = 1
			}
			if a > 0 {
				col := grad4(v, top, mid, bot, deep)

				// Specular: a broad sheen plus a small crisp hotspot
				// above-left of center - the light source that also drives
				// the rim.
				sp := 0.34*math.Exp(-((u+0.42)*(u+0.42)+(v+0.46)*(v+0.46))/(2*0.36*0.36)) +
					0.62*math.Exp(-((u+0.30)*(u+0.30)+(v+0.36)*(v+0.36))/(2*0.09*0.09))
				col = screen(col, spec, clampf(sp*float64(pal.specGain)))

				ang := math.Atan2(v, u)

				// Rim light along the upper-left edge.
				rim := smoothstep(0.78, 0.985, d) * sq(clampf(math.Cos(ang+2.4)))
				col = screen(col, top, 0.34*rim)
				// A faint trace of the button's own glow bleeding over
				// the opposite rim.
				rim2 := smoothstep(0.82, 0.99, d) * sq(clampf(math.Cos(ang-0.75)))
				col = screen(col, glow, 0.12*rim2)

				// Bounce: the lit room throws the button's own color
				// back at the lower rim.
				bounce := smoothstep(0.80, 1.0, d) * sq(clampf(math.Sin(ang-0.15)))
				col = screen(col, glow, 0.20*bounce)

				// Grounding: the surface beneath steals some light from
				// the bottom edge.
				low := smoothstep(0.74, 1.0, d) * sq(sq(clampf(math.Sin(ang))))
				col[0] *= 1 - 0.16*low
				col[1] *= 1 - 0.16*low
				col[2] *= 1 - 0.16*low

				// Inner luminance: when on, the body itself glows.
				core := float64(pal.coreGlow) * math.Exp(-d*d/(2*0.55*0.55))
				col = screen(col, glow, core)

				// Inner vignette: a touch of spherical volume.
				vig := 1 - 0.085*smoothstep(0.45, 1.0, d)
				col[0] *= vig
				col[1] *= vig
				col[2] *= vig

				o := (y*n + x) * 4
				dp[o+0] = quant8(col[0]*a, dith)
				dp[o+1] = quant8(col[1]*a, dith)
				dp[o+2] = quant8(col[2]*a, dith)
				dp[o+3] = quant8(a, dith)
			}

			// ---------------- the glyph texture ----------------

			gd := glyphDist(u, v)
			ed := glyphDist(u, v-0.011) // emboss copy, pressed slightly down
			hd := glyphDist(u, v+0.013) // engraved highlight, just above
			// White strokes with ~1.1 texels of edge.
			ga := clampf(-gd * half / 1.1)
			// Emboss shadow: the same shape, softer.
			ea := clampf(-ed*half/4.0) * 0.15
			// Engraved highlight on the upper lip.
			hla := clampf(-hd*half/2.0) * 0.16
			// Bloom: the lit body shining through its own symbol.
			bl := math.Exp(-math.Max(0, gd) * math.Max(0, gd) / (2 * 0.09 * 0.09))

			// Composite straight-alpha, premultiplied once at the end:
			// bloom adds the glow color, emboss darkens toward black,
			// the white strokes cover everything beneath.
			ba := clampf(bl * float64(pal.glyphGlow))
			col := [3]float64{glow[0], glow[1], glow[2]}
			alpha := ba
			gw := ga * 0.965
			ea2 := ea * (1 - gw)
			col[0] *= 1 - ea2
			col[1] *= 1 - ea2
			col[2] *= 1 - ea2
			alpha = alpha*(1-ea2) + ea2
			hla2 := hla * (1 - gw)
			col[0] = col[0]*(1-hla2) + hla2
			col[1] = col[1]*(1-hla2) + hla2
			col[2] = col[2]*(1-hla2) + hla2
			alpha = alpha*(1-hla2) + hla2
			col[0] = col[0]*(1-gw) + gw
			col[1] = col[1]*(1-gw) + gw
			col[2] = col[2]*(1-gw) + gw
			alpha = alpha*(1-gw) + gw

			if alpha > 0 {
				o := (y*n + x) * 4
				gp[o+0] = quant8(col[0]*alpha, dith)
				gp[o+1] = quant8(col[1]*alpha, dith)
				gp[o+2] = quant8(col[2]*alpha, dith)
				gp[o+3] = quant8(alpha, dith)
			}
		}
	}

	texPal = pal
	texDisc = disc
	texGlyph = glyph
}

// glyphDist is the signed distance to the power symbol in disc
// coordinates: the ring (with its top gap) and the vertical bar,
// both of half-width hw.
func glyphDist(u, v float64) float64 {
	const (
		rr  = ringR
		hw  = ringStroke / 2
		bar = barTip
	)

	d := math.Hypot(u, v)
	ang := math.Atan2(v, u)

	// The ring, excluding the gap sector at the top.
	rd := math.Inf(1)
	if math.Abs(ang+math.Pi/2) >= float64(gapHalf) {
		rd = math.Abs(d-rr) - hw
	}
	// The round caps closing the ring at the gap edges.
	for _, a := range []float64{-math.Pi/2 - float64(gapHalf), -math.Pi/2 + float64(gapHalf)} {
		cu, cv := rr*math.Cos(a), rr*math.Sin(a)
		rd = math.Min(rd, math.Hypot(u-cu, v-cv)-hw)
	}

	// The bar: a capsule from just below center up to barTip.
	var dv float64
	switch {
	case v > 0:
		dv = v
	case v < -bar:
		dv = v + bar
	}
	bd := math.Hypot(u, dv) - hw*1.06

	return math.Min(rd, bd)
}

// ---- the ripple ring ----

type ringKeyT struct {
	inner, alpha int // quantized inner radius fraction + alpha
	col          color.NRGBA
}

// ringImage bakes (and caches) a soft annulus for the toggle ripple:
// full at the inner radius, feathered outward, tinted col at alpha.
// inner is the ring's inner radius as a fraction of the texture's
// half-size; the outer edge is always the texture edge.
func ringImage(inner, alpha float32, col color.NRGBA) *image.RGBA {
	q := ringKeyT{
		inner: int(inner * 96),
		alpha: int(alpha * 96),
		col:   col,
	}
	if ringCache != nil && ringKey == q {
		return ringCache
	}

	const n = 192
	img := image.NewRGBA(image.Rect(0, 0, n, n))
	px := img.Pix
	half := float64(n-1) / 2
	cf := nrgbaF(col)
	for y := 0; y < n; y++ {
		v := 2*float64(y)/float64(n-1) - 1
		for x := 0; x < n; x++ {
			u := 2*float64(x)/float64(n-1) - 1
			d := math.Hypot(u, v)
			// A true annulus: nothing inside the hole, a ~2-texel
			// rise at its edge, then a power falloff to nothing at
			// the texture edge.
			t := (d - float64(inner)) / math.Max(1e-6, 1-float64(inner))
			rise := clampf((d - float64(inner)) * half / 2.0)
			fall := math.Pow(1-clampf(t), 1.2)
			m := rise * fall
			a := m * float64(alpha)
			if a <= 0 {
				continue
			}
			dith := (float64(bayer8[(y&7)*8+(x&7)])+0.5)/64 - 0.5
			o := (y*n + x) * 4
			px[o+0] = quant8(cf[0]*a, dith)
			px[o+1] = quant8(cf[1]*a, dith)
			px[o+2] = quant8(cf[2]*a, dith)
			px[o+3] = quant8(a, dith)
		}
	}
	ringKey = q
	ringCache = img
	return img
}

func quant8(v, dith float64) uint8 {
	q := v*255 + dith*1.4
	if q < 0 {
		return 0
	}
	if q > 255 {
		return 255
	}
	return uint8(q + 0.5)
}

func nrgbaF(c color.NRGBA) [3]float64 {
	return [3]float64{float64(c.R) / 255, float64(c.G) / 255, float64(c.B) / 255}
}

// grad4 interpolates a four-stop vertical gradient at v (-1..1) with
// smooth ease between stops.
func grad4(v float64, a, b, c, d [3]float64) [3]float64 {
	// Stops: -1, -0.18, 0.52, 1.
	var out [3]float64
	for i := range out {
		var x float64
		switch {
		case v <= -0.18:
			x = mixv(a[i], b[i], smoothstep(-1, -0.18, v))
		case v <= 0.52:
			x = mixv(b[i], c[i], smoothstep(-0.18, 0.52, v))
		default:
			x = mixv(c[i], d[i], smoothstep(0.52, 1, v))
		}
		out[i] = x
	}
	return out
}

func mixv(a, b, t float64) float64 { return a + (b-a)*t }

// screen blends b over a with the "screen" transfer at strength t:
// brightening without saturating to mud.
func screen(a, b [3]float64, t float64) [3]float64 {
	if t <= 0 {
		return a
	}
	if t > 1 {
		t = 1
	}
	var out [3]float64
	for i := range a {
		s := 1 - (1-a[i])*(1-b[i]) // full screen blend
		out[i] = a[i] + (s-a[i])*t // partial strength
	}
	return out
}

func smoothstep(e0, e1, x float64) float64 {
	t := (x - e0) / (e1 - e0)
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	return t * t * (3 - 2*t)
}

func clampf(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}

func sq(x float64) float64 { return x * x }
