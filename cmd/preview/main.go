//go:build js && wasm

// SPDX-License-Identifier: Unlicense OR MIT

// Command preview renders the button's key moments — launch, rest,
// hover, press, release, both toggle directions — with the real Gio
// renderer at scripted clock times, and hands labeled PNGs to the host
// page (see scripts/preview_run.py). It exists so the button's look
// and feel can be judged from real pixels: stills, filmstrips and
// contact sheets.
package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"syscall/js"
	"time"

	"gioui.org/f32"
	"gioui.org/gpu/headless"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/onlyactions/the-button/internal/state"
	"github.com/onlyactions/the-button/internal/ui"
)

// S is the square render size: big enough to judge quality, small
// enough to stay quick. The real window is 320x320 logical, usually
// 1.5-2x that in device pixels, so 720 is representative.
const S = 720

var (
	epoch  = time.Unix(0, 0)
	center = f32.Pt(float32(S)/2, float32(S)/2)
)

// session is one button driven by a scripted timeline. Frame times
// must be monotonically increasing (the spring integrates the delta).
type session struct {
	st *state.State
	b  *ui.Button
	rt *input.Router
}

func newSession(on bool) *session {
	st := state.New()
	if on {
		st.Toggle()
	}
	return &session{st: st, b: ui.New(st), rt: &input.Router{}}
}

// step advances the session to epoch+at, delivering evs to the input
// router, and returns the laid-out frame's operations. The GPU is not
// touched: the button's animation state lives entirely in Go, so a
// frame can be stepped without rendering it.
func (s *session) step(at time.Duration, evs ...pointer.Event) *op.Ops {
	for i := range evs {
		evs[i].Time = at
		s.rt.Queue(evs[i])
	}
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Source:      s.rt.Source(),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(S, S)),
		Now:         epoch.Add(at),
	}
	s.b.Layout(gtx)
	s.rt.Frame(&ops)
	return &ops
}

// snap renders an op list and returns the image. Every snapshot gets
// a fresh headless window: the js/WebGL backend accumulates subtle
// stencil residue across frames on a shared window, which would
// contaminate long sequences.
func snap(ops *op.Ops) *image.RGBA {
	w, err := headless.NewWindow(S, S)
	if err != nil {
		panic(err)
	}
	defer w.Release()
	if err := w.Frame(ops); err != nil {
		panic(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, S, S))
	if err := w.Screenshot(img); err != nil {
		panic(err)
	}
	return img
}

// frame steps to epoch+at and renders it — shorthand for step+snap.
func (s *session) frame(at time.Duration, evs ...pointer.Event) *image.RGBA {
	return snap(s.step(at, evs...))
}

// mouse builds a mouse pointer event at the button's center.
func mouse(kind pointer.Kind, buttons pointer.Buttons) pointer.Event {
	return pointer.Event{
		Kind:      kind,
		Source:    pointer.Mouse,
		PointerID: 1,
		Buttons:   buttons,
		Position:  center,
	}
}

func main() {
	if err := run(); err != nil {
		js.Global().Set("__err", err.Error())
		js.Global().Set("__done", true)
	}
}

func run() error {
	type shot struct {
		name string
		img  *image.RGBA
	}
	var shots []shot
	capture := func(name string, img *image.RGBA) {
		shots = append(shots, shot{name, img})
	}

	// 1. Launch: the first second of the off state (draw-in of glyph,
	//    scale-up, glow bloom, fade from black).
	{
		s := newSession(false)
		capture("intro-120ms", s.frame(120*time.Millisecond))
		capture("intro-320ms", s.frame(320*time.Millisecond))
		capture("intro-560ms", s.frame(560*time.Millisecond))
		capture("intro-800ms", s.frame(800*time.Millisecond))
		capture("off-rest", s.frame(2200*time.Millisecond))
	}

	// 2. On at rest: the breathing halo at its peak and its dip.
	{
		s := newSession(true)
		s.step(900 * time.Millisecond)
		capture("on-peak", s.frame(1570*time.Millisecond))
		capture("on-dim", s.frame(3270*time.Millisecond))
		capture("on-rest", s.frame(5000*time.Millisecond))
	}

	// 3. Hover: the glow leans toward the cursor. (The router
	//    synthesizes Enter from a Move inside the hit area.)
	{
		s := newSession(false)
		s.step(400 * time.Millisecond) // t0; intro ends ~1120
		s.step(3200 * time.Millisecond)
		s.step(3210*time.Millisecond, mouse(pointer.Move, 0))
		capture("off-hover", s.frame(3360*time.Millisecond))
	}

	// 4. Press and hold: the button sinks into the surface.
	{
		s := newSession(false)
		s.step(400 * time.Millisecond) // t0; intro ends ~1120
		s.step(3200 * time.Millisecond)
		s.step(3210*time.Millisecond, mouse(pointer.Move, 0))
		s.step(3300*time.Millisecond, mouse(pointer.Press, pointer.ButtonPrimary))
		capture("off-press-40ms", s.frame(3340*time.Millisecond))
		capture("off-press-120ms", s.frame(3420*time.Millisecond))
		capture("off-press-300ms", s.frame(3600*time.Millisecond))
	}

	// 5. Release while off: toggle on — color crossfade, ripple, pop.
	{
		s := newSession(false)
		s.step(400 * time.Millisecond) // t0; intro ends ~1120
		s.step(3200 * time.Millisecond)
		s.step(3210*time.Millisecond, mouse(pointer.Move, 0))
		s.step(3300*time.Millisecond, mouse(pointer.Press, pointer.ButtonPrimary))
		s.step(3380*time.Millisecond, mouse(pointer.Release, pointer.ButtonPrimary))
		for _, m := range []int{40, 90, 150, 230, 330, 470, 700} {
			capture(fmt.Sprintf("to-on-%dms", m), s.frame(time.Duration(3380+m)*time.Millisecond))
		}
	}

	// 6. Release while on: toggle off.
	{
		s := newSession(true)
		s.step(400 * time.Millisecond) // t0; intro ends ~1120
		s.step(2000 * time.Millisecond)
		s.step(2010*time.Millisecond, mouse(pointer.Move, 0))
		s.step(2100*time.Millisecond, mouse(pointer.Press, pointer.ButtonPrimary))
		s.step(2180*time.Millisecond, mouse(pointer.Release, pointer.ButtonPrimary))
		for _, m := range []int{40, 90, 150, 230, 330, 470, 700} {
			capture(fmt.Sprintf("to-off-%dms", m), s.frame(time.Duration(2180+m)*time.Millisecond))
		}
	}

	// Hand everything to the host page.
	arr := js.Global().Get("__previews")
	for _, sh := range shots {
		var buf bytes.Buffer
		if err := png.Encode(&buf, sh.img); err != nil {
			return err
		}
		url := "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
		arr.Call("push", js.ValueOf(map[string]any{"name": sh.name, "url": url}))
	}
	js.Global().Set("__done", true)
	return nil
}
