//go:build linux

// SPDX-License-Identifier: Unlicense OR MIT

package main

// Renders the button offscreen (software EGL) and saves PNGs so the
// visuals can be verified without a display.

import (
	"image"
	"image/png"
	"os"
	"testing"

	"gioui.org/gpu/headless"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/onlyactions/the-button/internal/state"
	"github.com/onlyactions/the-button/internal/ui"
)

func renderState(t *testing.T, w *headless.Window, on bool, path string) {
	const S = 600
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(S, S)),
	}
	st := state.New()
	if on {
		st.Toggle()
	}
	ui.New(st).Layout(gtx)
	if err := w.Frame(&ops); err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, S, S))
	if err := w.Screenshot(img); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func TestRenderScreenshots(t *testing.T) {
	const S = 600
	w, err := headless.NewWindow(S, S)
	if err != nil {
		t.Skipf("no EGL available: %v", err)
	}
	defer w.Release()
	renderState(t, w, false, "/tmp/button-off.png")
	renderState(t, w, true, "/tmp/button-on.png")
}
