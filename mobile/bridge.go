// SPDX-License-Identifier: Unlicense OR MIT

// Package bridge is the gomobile-bound API surface for the Android
// app (built with `gomobile bind`). The Gio window, the quick-settings
// tile and the home-screen widget all toggle the same shared,
// persisted state through this package.
//
// Java-side usage (com.onlyactions.thebutton):
//
//	Bridge.setup(getFilesDir().getAbsolutePath())
//	Bridge.isOn()
//	Bridge.toggle()
//	Bridge.setListener(listener)   // listener.stateChanged(on)
//	Bridge.startView()             // from the activity, after Gio.init
package bridge

import (
	"log"
	"path/filepath"
	"sync"

	"gioui.org/app"

	"github.com/onlyactions/the-button/internal/state"
	"github.com/onlyactions/the-button/internal/ui"
)

var (
	st = state.New()

	mu       sync.Mutex
	listener Listener
	start    sync.Once
)

func init() {
	// Forward every state change to the Java side (the tile and the
	// widget re-render themselves). gobind handles cross-thread
	// JNI calls.
	st.OnChange(func(on bool) {
		mu.Lock()
		l := listener
		mu.Unlock()
		if l != nil {
			l.StateChanged(on)
		}
	})
}

// Listener is implemented on the Java side (Sync.java) to update the
// quick-settings tile and home-screen widget when the state changes
// from another surface.
type Listener interface {
	StateChanged(on bool)
}

// Setup points the state at the app's private files directory so it
// survives process restarts. Call it from every Android entry point
// (activity, tile service, widget) before touching the state.
func Setup(filesDir string) {
	_ = st.SetFile(filepath.Join(filesDir, "state.json"))
}

// IsOn reports whether the button is on.
func IsOn() bool { return st.IsOn() }

// Toggle flips the button and returns its new state.
func Toggle() bool { return st.Toggle() }

// SetListener registers (or, with nil, removes) the Java-side change
// listener.
func SetListener(l Listener) {
	mu.Lock()
	defer mu.Unlock()
	listener = l
}

// StartView starts the Gio window. It is idempotent; the window
// attaches to whatever GioView the activity creates and survives
// activity re-creation (a single window serves any number of
// successive views).
func StartView() {
	start.Do(func() {
		go func() {
			w := new(app.Window)
			w.Option(app.Title(ui.Title), app.Size(320, 320))
			if err := ui.Run(w, st); err != nil {
				log.Printf("the button: gio window: %v", err)
			}
		}()
	})
}
