// SPDX-License-Identifier: Unlicense OR MIT

// Package ui renders the button: a black screen, one centered circular
// power button that toggles red (off) / green (on) when clicked.
package ui

import (
	"image/color"
	"math"

	"gioui.org/app"
	"gioui.org/f32"
	"gioui.org/gesture"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/onlyactions/the-button/internal/state"
)

var (
	offColor = color.NRGBA{R: 0xE5, G: 0x39, B: 0x35, A: 0xFF} // red
	onColor  = color.NRGBA{R: 0x2E, G: 0x7D, B: 0x32, A: 0xFF} // green
	symColor = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF} // white
	bgColor  = color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0xFF} // black
)

// Title is the main window's title; the desktop tray uses it to find
// and raise the window.
const Title = "the button"

// Run drives w's event loop until the window is destroyed. Any state
// change — from this window, the tray, an Android tile or widget —
// redraws the window.
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
}

// New returns a button bound to st.
func New(st *state.State) *Button {
	return &Button{st: st}
}

// Layout draws one frame: a black screen with one centered circular
// power button that toggles red/green when clicked.
func (b *Button) Layout(gtx layout.Context) layout.Dimensions {
	for {
		ev, ok := b.click.Update(gtx.Source)
		if !ok {
			break
		}
		if ev.Kind == gesture.KindClick {
			b.st.Toggle()
		}
	}

	paint.Fill(gtx.Ops, bgColor)

	sz := gtx.Constraints.Max
	if sz.X <= 0 || sz.Y <= 0 {
		return layout.Dimensions{Size: sz}
	}
	r := float32(min(sz.X, sz.Y)) * 0.40
	if b.click.Pressed() {
		r *= 0.93 // subtle tactile feedback while held down
	}
	c := f32.Pt(float32(sz.X)/2, float32(sz.Y)/2)

	// The circle. Its outline doubles as the click area, so the button
	// responds exactly to taps on the circle itself.
	disc := clip.Outline{Path: circlePath(gtx.Ops, c, r)}.Op()
	defer disc.Push(gtx.Ops).Pop()
	paint.FillShape(gtx.Ops, b.color(), disc)
	b.click.Add(gtx.Ops)

	// The power symbol in the center.
	drawPowerSymbol(gtx.Ops, c, r*0.46, r*0.13, symColor)

	return layout.Dimensions{Size: sz}
}

func (b *Button) color() color.NRGBA {
	if b.st.IsOn() {
		return onColor
	}
	return offColor
}

// circlePath returns a full circle of radius r around c.
func circlePath(ops *op.Ops, c f32.Point, r float32) clip.PathSpec {
	var p clip.Path
	p.Begin(ops)
	p.MoveTo(f32.Pt(c.X+r, c.Y))
	p.Arc(f32.Pt(-r, 0), f32.Pt(-r, 0), 2*math.Pi)
	return p.End()
}

// drawPowerSymbol draws the power symbol: a circle broken at the
// top, with a vertical line from the center up to the top.
func drawPowerSymbol(ops *op.Ops, c f32.Point, r, w float32, col color.NRGBA) {
	const gapHalf = 35 * math.Pi / 180 // 70° opening at the top
	start := -math.Pi/2 + gapHalf
	sweep := 2*math.Pi - 2*gapHalf

	sx, sy := math.Cos(start), math.Sin(start)
	sp := f32.Pt(c.X+r*float32(sx), c.Y+r*float32(sy))

	var p clip.Path
	p.Begin(ops)
	p.MoveTo(sp)
	f := f32.Pt(c.X-sp.X, c.Y-sp.Y) // focus = center, relative to pen
	p.Arc(f, f, float32(sweep))
	paint.FillShape(ops, col, clip.Stroke{Path: p.End(), Width: w}.Op())

	var q clip.Path
	q.Begin(ops)
	q.MoveTo(c)
	q.LineTo(f32.Pt(c.X, c.Y-r))
	paint.FillShape(ops, col, clip.Stroke{Path: q.End(), Width: w}.Op())
}
