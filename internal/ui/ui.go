// SPDX-License-Identifier: Unlicense OR MIT

// Package ui renders the button.
//
// The design idea: the button is the only light in the room. A deep
// charcoal gradient behind, a soft halo around the disc, and a faint
// ambient wash of the button's own color on everything — so flipping
// the button changes the mood of the whole scene, not just the disc.
//
// Feel: a damped spring drives the press physics (pressing sinks the
// button, releasing pops it back with a small overshoot), the color
// crossfades over a fifth of a second inside an expanding ripple, the
// power glyph draws itself on launch, and while the button is on the
// glow breathes slowly — alive, but calm.
package ui

import (
	"image"
	"image/color"
	"math"
	"time"

	"gioui.org/app"
	"gioui.org/f32"
	"gioui.org/gesture"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/onlyactions/the-button/internal/state"
)

// Title is the main window's title; the desktop tray uses it to find
// and raise the window.
const Title = "the button"

// Timing of everything that moves.
const (
	introTotal      = 720 * time.Millisecond
	introBgDur      = 180 * time.Millisecond
	introScaleDelay = 80 * time.Millisecond
	introScaleDur   = 420 * time.Millisecond
	introGlowDelay  = 100 * time.Millisecond
	introGlowDur    = 420 * time.Millisecond
	introBarDelay   = 260 * time.Millisecond
	introBarDur     = 200 * time.Millisecond
	introArcDelay   = 380 * time.Millisecond
	introArcDur     = 320 * time.Millisecond

	colorDur  = 220 * time.Millisecond
	rippleDur = 440 * time.Millisecond
	hoverDur  = 140 * time.Millisecond

	breathePeriod = 3400 * time.Millisecond

	pressScale = 0.915
)

// Proportions, relative to the button radius r (or the window's
// smaller side for the room wash).
const (
	discFrac = 0.36

	// Ambient shadow: a wide dark breath under the disc.
	ambShadowOffset = 0.055
	ambShadow0      = 0.92
	ambShadow1      = 1.24
	ambShadowPeak   = 0.34

	// Contact shadow: the tight ellipse where the button meets the
	// surface — what makes it read as a physical object.
	contactY    = 0.965
	contactRX   = 0.64
	contactRY   = 0.10
	contactPeak = 0.42

	// Halo: three stacked bands — a hot rim hugging the edge, a mid
	// haze, and a wide bloom that reaches for the corners. Sizes are
	// scaled by the palette's haloMul (tight when off, wide when on).
	rimGlow0, rimGlow1, rimGlowPeak = 1.0, 1.09, 0.30
	midGlow0, midGlow1, midGlowPeak = 1.02, 1.44, 0.27
	wideGlow0, wideGlow1, widePeak  = 1.05, 1.75, 0.10

	roomExtent = 1.45

	ringR, ringStroke = 0.42, 0.095
	gapHalfDeg        = 24
	barTip            = 0.372

	ripple0, ripple1 = 0.95, 1.30
)

var (
	gapHalf          = float32(gapHalfDeg * math.Pi / 180)
	white            = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	transparentBlack = color.NRGBA{A: 0xFF}
)

// Run drives w's event loop until the window is destroyed. Any state
// change — from this window, the tray, an Android tile or widget —
// animates and redraws the window.
func Run(w *app.Window, st *state.State) error {
	cancel := st.OnChange(func(bool) { w.Invalidate() })
	defer cancel()
	b := New(st)
	var ops op.Ops
	for {
		switch e := w.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			b.Layout(gtx)
			e.Frame(gtx.Ops)
		}
	}
}

// Button is the power button UI bound to a state.
type Button struct {
	st    *state.State
	click gesture.Click

	firstFrame time.Time
	lastStep   time.Time

	spring spring // press scale

	palShow palette // palette as last painted
	palFrom palette // transition source
	shownOn bool    // state as last painted
	transAt time.Time

	hovered    bool
	hoverAt    time.Time
	breatheRef time.Time
}

// New returns a button bound to st.
func New(st *state.State) *Button {
	return &Button{
		st:      st,
		spring:  newSpring(900, 38),
		palShow: palOff,
		palFrom: palOff,
	}
}

// Layout draws one frame.
func (b *Button) Layout(gtx layout.Context) layout.Dimensions {
	now := gtx.Now
	if b.firstFrame.IsZero() {
		b.firstFrame = now
		b.lastStep = now
		b.breatheRef = now.Add(introTotal)
	}

	b.spring.step(now.Sub(b.lastStep))
	b.lastStep = now

	// Pointer events.
	for {
		ev, ok := b.click.Update(gtx.Source)
		if !ok {
			break
		}
		switch ev.Kind {
		case gesture.KindPress:
			b.spring.target = pressScale
		case gesture.KindClick:
			b.st.Toggle()
			b.spring.target = 1
		}
	}
	if h := b.click.Hovered(); h != b.hovered {
		b.hovered = h
		b.hoverAt = now
	}

	// Begin a color transition whenever the state differs from what is
	// on screen — this covers clicks, the tray, the tile and the widget
	// alike, and makes any toggle animate.
	if on := b.st.IsOn(); on != b.shownOn {
		b.palFrom = b.palShow
		b.shownOn = on
		b.transAt = now
	}

	introF := clamp01(float32(now.Sub(b.firstFrame)) / float32(introTotal))
	transF := easeInOutCubic(span{start: b.transAt, dur: colorDur}.frac(now))
	b.palShow = lerpPal(b.palFrom, palFor(b.shownOn), transF)

	sz := gtx.Constraints.Max
	if sz.X <= 0 || sz.Y <= 0 {
		return layout.Dimensions{Size: sz}
	}
	b.draw(gtx.Ops, sz, now, introF)

	// Keep frames coming while anything moves.
	if next, ok := b.nextFrame(now, introF); ok {
		gtx.Execute(op.InvalidateCmd{At: next})
	}
	return layout.Dimensions{Size: sz}
}

func palFor(on bool) palette {
	if on {
		return palOn
	}
	return palOff
}

// pressAmount is 0 at rest and 1 fully pressed.
func (b *Button) pressAmount() float32 {
	return clamp01((1 - b.spring.x) / (1 - pressScale))
}

// nextFrame reports when the next frame is needed.
func (b *Button) nextFrame(now time.Time, introF float32) (time.Time, bool) {
	if introF < 1 {
		return now, true // intro: continuous
	}
	if !b.transAt.IsZero() && now.Before(b.transAt.Add(rippleDur)) {
		return now, true // transition + ripple: continuous
	}
	if b.spring.moving() {
		return now, true
	}
	if !b.hoverAt.IsZero() && now.Before(b.hoverAt.Add(hoverDur)) {
		return now, true
	}
	if b.shownOn {
		return now.Add(33 * time.Millisecond), true // breathing, ~30fps
	}
	return time.Time{}, false
}

// draw paints one complete scene.
func (b *Button) draw(ops *op.Ops, sz image.Point, now time.Time, introF float32) {
	t0 := b.firstFrame
	minDim := float32(min(sz.X, sz.Y))
	c := f32.Pt(float32(sz.X)/2, float32(sz.Y)/2)
	r := minDim * discFrac

	pal := b.palShow
	press := b.pressAmount()
	glowL := b.glowLevel(now, introF)

	// Total scale: intro grow-in times press spring.
	introShadowF := easeOutCubic(span{t0.Add(introScaleDelay), introScaleDur}.frac(now))
	introS := 0.82 + 0.18*easeOutCubic(span{t0.Add(introScaleDelay), introScaleDur}.frac(now))
	s := b.spring.x * introS

	// Group alphas are folded straight into each glow's peak rather
	// than painted through opacity layers: fewer composited passes,
	// and it sidesteps layer-composition quirks on WebGL targets.

	// --- the room ---
	fillScreen(ops, sz.X, sz.Y, bgTop, bgBottom)

	// Ambient wash: the button is the only light in the room, so its
	// color reaches softly into every corner.
	glow(ops, c, r*0.35*s, minDim*roomExtent,
		pal.room*clamp01(glowLevelCap(glowL)), 26, 1.3, pal.glow)

	// --- the button ---
	// Ambient shadow: a wide dark breath, tightening as the button is
	// pressed into the surface.
	glow(ops, f32.Pt(c.X, c.Y+float32(ambShadowOffset)*r*s*(1-0.35*press)),
		r*ambShadow0*s, r*ambShadow1*s,
		ambShadowPeak*pal.shadow*(1+0.25*press)*introShadowF, 14, 2.2, transparentBlack)

	// Contact shadow: the tight dark ellipse where the button meets
	// the surface — this is what grounds it.
	ellipseGlow(ops, f32.Pt(c.X, c.Y+float32(contactY)*r*s),
		r*contactRX*s, r*contactRY*s,
		contactPeak*pal.shadow*(1+0.35*press)*introShadowF, 10, 1.6, transparentBlack)

	// Halo: three stacked bands — hot rim, mid haze, wide bloom —
	// breathing when on, tight and dim when off.
	hm, hg := pal.haloMul, pal.haloGain
	a := clamp01(glowL)
	glow(ops, c, r*rimGlow0*s, r*rimGlow1*hm*s, rimGlowPeak*hg*a, 14, 2.0, pal.glow)
	glow(ops, c, r*midGlow0*s, r*midGlow1*hm*s, midGlowPeak*hg*a, 18, 1.5, pal.glow)
	glow(ops, c, r*wideGlow0*s, r*wideGlow1*hm*s, widePeak*hg*a, 22, 1.05, pal.glow)

	// Ripple: an expanding ring born with every toggle — a small
	// per-frame bake, so its edges are as clean as the disc's. It
	// grows with a gentle quadratic ease so it stays on screen and
	// stays legible against the halo.
	if f := (span{start: b.transAt, dur: rippleDur}).frac(now); f > 0 && f < 1 {
		e := 1 - (1-f)*(1-f) // easeOutQuad
		// The band always starts just outside the disc's current
		// (spring-scaled) edge, so the whole ring stays visible.
		rOuter := r * lerp(1.14, ripple1, e) * s
		rInner := r * lerp(1.02, ripple1-0.13, e) * s
		inner := clamp01(rInner / rOuter)
		img := ringImage(inner, 0.85*(1-f), withAlpha(pal.glow, 1))
		drawTex(ops, img, c, rOuter)
	}

	// The disc: baked candy shading — gradient, specular, rim.
	drawTex(ops, discImage(pal), c, r*s)

	// Pressing draws the light inward: a soft darkening toward the
	// center that follows the spring.
	if press > 0.01 {
		glow(ops, c, r*0.82*s, r*1.0*s, 0.10*press, 10, 1.2, transparentBlack)
	}

	// The power glyph draws itself on launch: the bar grows upward
	// behind a rising rect, the ring sweeps in behind a widening
	// wedge. Both clips only exist during the intro; at rest the whole
	// glyph texture is drawn once.
	barF := easeOutCubic(span{t0.Add(introBarDelay), introBarDur}.frac(now))
	arcF := easeInOutCubic(span{t0.Add(introArcDelay), introArcDur}.frac(now))
	if barF >= 1 && arcF >= 1 {
		drawTex(ops, glyphImage(pal), c, r*s)
	} else {
		gtex := glyphImage(pal)
		if barF > 0 {
			tipY := c.Y - r*lerp(0.06, barTip, barF)*s
			// A rect just wide enough for the bar — a full-width
			// rect would also cut across the ring's top.
			bw := r * ringStroke * 0.9 * s
			st := clip.Rect{Min: image.Pt(int(c.X-bw), int(tipY)), Max: image.Pt(int(c.X+bw+.5), int(c.Y+r*0.10+.5))}.Push(ops)
			drawTex(ops, gtex, c, r*s)
			st.Pop()
		}
		if arcF > 0 {
			// The ring sweeps in symmetrically from the top gap,
			// both directions at once.
			a0 := -math.Pi/2 + float64(gapHalf)
			full := (2*math.Pi - 2*float64(gapHalf)) * float64(arcF) / 2
			rad := r * (ringR + ringStroke) * s
			st := wedgeClip(ops, c, rad, float32(a0), float32(full))
			drawTex(ops, gtex, c, r*s)
			st.Pop()
			st = wedgeClip(ops, c, rad, float32(-math.Pi/2-float64(gapHalf)), float32(-full))
			drawTex(ops, gtex, c, r*s)
			st.Pop()
		}
	}

	// Click target and cursor over the disc.
	b.click.Add(ops)
	pointer.CursorPointer.Add(ops)

	// First impression: the whole scene fades in from black.
	if f := easeOutCubic(span{t0, introBgDur}.frac(now)); f < 1 {
		paint.FillShape(ops, withAlpha(transparentBlack, 1-f), clip.Rect{Max: sz}.Op())
	}
}

// drawTex paints a square texture centered at c with the given
// radius (its half-size on screen), clipped to its own box. The clip
// must be pushed before the affine: transforms apply to subsequent
// clip ops as well, so the reverse order would scale the clip itself.
func drawTex(ops *op.Ops, img *image.RGBA, c f32.Point, rad float32) {
	if rad <= 0 {
		return
	}
	iop := paint.NewImageOp(img)
	iop.Filter = paint.FilterLinear
	n := float32(img.Bounds().Dx())
	sc := 2 * rad / n
	defer clip.Rect{Min: image.Pt(int(c.X-rad), int(c.Y-rad)), Max: image.Pt(int(c.X+rad+.5), int(c.Y+rad+.5))}.Push(ops).Pop()
	defer op.Affine(f32.AffineId().Scale(f32.Point{}, f32.Pt(sc, sc)).
		Offset(f32.Pt(c.X-rad, c.Y-rad))).Push(ops).Pop()
	iop.Add(ops)
	paint.PaintOp{}.Add(ops)
}

// wedgeClip builds a pie-slice clip starting at angle a0 sweeping
// clockwise — used only for the glyph's launch draw-in.
func wedgeClip(ops *op.Ops, c f32.Point, rad, a0, sweep float32) clip.Stack {
	var p clip.Path
	p.Begin(ops)
	p.MoveTo(c)
	p.LineTo(pt(c, rad, a0))
	f := f32.Pt(c.X-pt(c, rad, a0).X, c.Y-pt(c, rad, a0).Y)
	p.Arc(f, f, sweep)
	p.Close()
	return clip.Outline{Path: p.End()}.Op().Push(ops)
}

// glowLevel combines every influence on the halo's strength.
func (b *Button) glowLevel(now time.Time, introF float32) float32 {
	base := float32(0.58)
	if b.shownOn {
		if introF >= 1 {
			// Breathe between 0.80 and 1.0 — an opacity cannot
			// overshoot 1, so the pulse must live below it. The
			// phase starts at its peak to stay continuous with
			// the intro's bloom.
			ph := float64(now.Sub(b.breatheRef)) / float64(breathePeriod)
			base = 0.90 + 0.10*float32(math.Sin(2*math.Pi*ph+math.Pi/2))
		} else {
			base = 1.0
		}
	}
	if b.hovered {
		base += 0.13 * easeOutCubic(span{start: b.hoverAt, dur: hoverDur}.frac(now))
	}
	// A soft bloom overshoot right after a toggle: the light flares
	// and settles, instead of sliding linearly between levels.
	if f := (span{start: b.transAt, dur: rippleDur}).frac(now); f > 0 && f < 1 {
		base *= 1 + 0.15*float32(math.Sin(math.Pi*float64(f)))
	}
	base *= 1 - 0.30*b.pressAmount() // pressing draws light inward
	base *= easeOutBack(span{b.firstFrame.Add(introGlowDelay), introGlowDur}.frac(now))
	return clamp01(base)
}

// glowLevelCap softens the room wash at high glow levels.
func glowLevelCap(l float32) float32 {
	return 0.55 + 0.45*l
}

// strokeCircle paints a full stroked circle.
func strokeCircle(ops *op.Ops, c f32.Point, r, w float32, col color.NRGBA) {
	if r <= 0 || w <= 0 || col.A == 0 {
		return
	}
	var p clip.Path
	p.Begin(ops)
	p.MoveTo(f32.Pt(c.X+r, c.Y))
	p.Arc(f32.Pt(-r, 0), f32.Pt(-r, 0), 2*math.Pi)
	paint.FillShape(ops, col, clip.Stroke{Path: p.End(), Width: w}.Op())
}
