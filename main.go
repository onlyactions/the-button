// SPDX-License-Identifier: Unlicense OR MIT

// the button: a single circular power button. Nothing else.
//
// Click it: it turns from red (off) to green (on) and back.
//
// Desktop entry point: a Gio window plus a system tray icon
// (Windows: Shell_NotifyIcon; Linux: StatusNotifierItem over D-Bus)
// that toggles the same state. The state persists across restarts.
//
// The Android app lives in mobile/ (the gomobile-bound API) and
// android/ (the Gradle project); see those directories.
package main

import (
	"log"
	"os"
	"path/filepath"

	"gioui.org/app"

	"github.com/onlyactions/the-button/internal/state"
	"github.com/onlyactions/the-button/internal/tray"
	"github.com/onlyactions/the-button/internal/ui"
)

func main() {
	st := state.New()
	if dir, err := os.UserConfigDir(); err == nil {
		_ = st.SetFile(filepath.Join(dir, "the-button", "state.json"))
	}

	go func() {
		w := new(app.Window)
		w.Option(app.Title(ui.Title), app.Size(320, 320))
		if err := ui.Run(w, st); err != nil {
			log.Fatal(err)
		}
		// Closing the window quits the app (and with it the tray).
		os.Exit(0)
	}()

	go tray.Run(st, ui.Title)

	app.Main()
}
