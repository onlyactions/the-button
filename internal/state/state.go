// SPDX-License-Identifier: Unlicense OR MIT

// Package state holds the button's on/off state, persists it to disk,
// and fans out change notifications to any number of listeners
// (window, tray, Android tile, Android widget).
package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// State is the single source of truth for whether the button is on.
// It is safe for concurrent use.
type State struct {
	mu   sync.Mutex
	on   bool
	path string
	fns  map[int64]func(on bool)
	next int64
}

// New returns a state that starts in the off position.
func New() *State {
	return &State{fns: make(map[int64]func(on bool))}
}

// SetFile enables persistence: the state is loaded from path (if it
// exists) and written back on every change. Persistence is
// best-effort; a missing or corrupt file just means "off".
func (s *State) SetFile(path string) error {
	s.mu.Lock()
	s.path = path
	s.mu.Unlock()
	return s.load()
}

func (s *State) load() error {
	b, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var v struct {
		On bool `json:"on"`
	}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	s.mu.Lock()
	s.on = v.On
	s.mu.Unlock()
	return nil
}

// IsOn reports whether the button is on.
func (s *State) IsOn() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.on
}

// Toggle flips the button and returns its new position. All change
// listeners are notified after the state (and its file) is updated.
func (s *State) Toggle() bool {
	s.mu.Lock()
	on := !s.on
	s.on = on
	s.saveLocked()
	fns := make([]func(on bool), 0, len(s.fns))
	for _, f := range s.fns {
		fns = append(fns, f)
	}
	s.mu.Unlock()
	for _, f := range fns {
		f(on)
	}
	return on
}

// OnChange registers fn to be called on every state change and
// returns a function that unregisters it again.
func (s *State) OnChange(fn func(on bool)) (cancel func()) {
	if fn == nil {
		return func() {}
	}
	s.mu.Lock()
	id := s.next
	s.next++
	s.fns[id] = fn
	s.mu.Unlock()
	return func() {
		s.mu.Lock()
		delete(s.fns, id)
		s.mu.Unlock()
	}
}

// saveLocked persists the state; errors are deliberately ignored.
func (s *State) saveLocked() {
	if s.path == "" {
		return
	}
	b, err := json.Marshal(struct {
		On bool `json:"on"`
	}{s.on})
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(s.path), 0o755)
	tmp := s.path + ".tmp"
	if os.WriteFile(tmp, b, 0o644) != nil {
		return
	}
	_ = os.Rename(tmp, s.path)
}
