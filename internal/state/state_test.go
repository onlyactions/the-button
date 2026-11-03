// SPDX-License-Identifier: Unlicense OR MIT

package state

import (
	"path/filepath"
	"testing"
)

func TestToggleAndListen(t *testing.T) {
	s := New()
	if s.IsOn() {
		t.Fatal("new state should be off")
	}
	var got []bool
	cancel := s.OnChange(func(on bool) { got = append(got, on) })
	if s.Toggle() != true || !s.IsOn() {
		t.Fatal("first toggle should turn on")
	}
	if s.Toggle() != false || s.IsOn() {
		t.Fatal("second toggle should turn off")
	}
	cancel()
	if s.Toggle() != true {
		t.Fatal("third toggle should turn on")
	}
	if len(got) != 2 || got[0] != true || got[1] != false {
		t.Fatalf("listener got %v, want [true false]", got)
	}
}

func TestPersist(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	a := New()
	if err := a.SetFile(path); err != nil {
		t.Fatal(err)
	}
	a.Toggle() // on

	// A fresh state over the same file starts on.
	b := New()
	if err := b.SetFile(path); err != nil {
		t.Fatal(err)
	}
	if !b.IsOn() {
		t.Fatal("persisted state should be on")
	}

	// A missing file is not an error and means off.
	c := New()
	if err := c.SetFile(filepath.Join(dir, "nope.json")); err != nil {
		t.Fatal(err)
	}
	if c.IsOn() {
		t.Fatal("missing file should mean off")
	}
}
