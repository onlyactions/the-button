// SPDX-License-Identifier: Unlicense OR MIT

// Animation plumbing for the button: one-shot spans, easings and a
// spring for the press physics. Everything is driven by the frame
// clock (gtx.Now) so any moment is reproducible — including in tests.
package ui

import "time"

// span is a one-shot animation anchored to a moment in time.
type span struct {
	start time.Time
	dur   time.Duration
}

// begin returns a span starting at t.
func begin(t time.Time, dur time.Duration) span {
	return span{start: t, dur: dur}
}

// frac returns the span's progress at t, clamped to [0, 1]. Returns 0
// before the span starts (and for zero-duration spans that have not
// elapsed yet).
func (s span) frac(t time.Time) float32 {
	if s.start.IsZero() || t.Before(s.start) {
		return 0
	}
	if s.dur <= 0 {
		return 1
	}
	el := t.Sub(s.start)
	if el >= s.dur {
		return 1
	}
	return float32(el) / float32(s.dur)
}

// done reports whether the span has finished at t.
func (s span) done(t time.Time) bool {
	return !s.start.IsZero() && !t.Before(s.start.Add(s.dur))
}

// end returns the time the span finishes.
func (s span) end() time.Time {
	return s.start.Add(s.dur)
}

// Easing curves.

func clamp01(v float32) float32 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func easeOutCubic(t float32) float32 {
	t = clamp01(t)
	f := 1 - t
	return 1 - f*f*f
}

func easeInOutCubic(t float32) float32 {
	t = clamp01(t)
	if t < 0.5 {
		return 4 * t * t * t
	}
	f := 2*t - 2
	return 1 + f*f*f/2
}

// easeOutBack overshoots slightly past 1 and settles back — used for
// the glow bloom so the button feels like it "pops" alight.
func easeOutBack(t float32) float32 {
	t = clamp01(t)
	const k = 1.28
	f := t - 1
	return 1 + (k+1)*f*f*f + k*f*f
}

// spring is a damped harmonic oscillator, integrated with
// semi-implicit Euler at a fixed substep. It gives the button its
// tactile press/release physics: press sinks quickly, release pops
// back with a small natural overshoot.
type spring struct {
	x, v     float32 // position (scale) and velocity
	target   float32
	stiff    float32
	damping  float32
	substep  time.Duration
	settling bool // ever been stepped; used to treat x==target as at-rest
}

func newSpring(stiff, damping float32) spring {
	return spring{x: 1, target: 1, stiff: stiff, damping: damping, substep: 4 * time.Millisecond}
}

// step advances the spring by dt (clamped to 34ms to survive hiccups).
func (s *spring) step(dt time.Duration) {
	if dt <= 0 {
		return
	}
	if dt > 34*time.Millisecond {
		dt = 34 * time.Millisecond
	}
	h := float32(dt) / float32(s.substep)
	n := int(float32(dt) / float32(s.substep))
	if float32(n) < h {
		n++
	}
	if n > 64 {
		n = 64
	}
	d := float32(s.substep) / 1e9
	for i := 0; i < n; i++ {
		a := s.stiff*(s.target-s.x) - s.damping*s.v
		s.v += a * d
		s.x += s.v * d
	}
	s.settling = true
}

// moving reports whether the spring still needs frames.
func (s *spring) moving() bool {
	const eps = 0.0006
	return s.settling && (abs(s.x-s.target) > eps || abs(s.v) > eps*60)
}

func abs(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
