// SPDX-License-Identifier: Unlicense OR MIT

// Package tray puts a toggle for the button in the desktop's system
// tray: a Shell_NotifyIcon icon on Windows, a StatusNotifierItem on
// D-Bus on Linux. Platforms without tray support are no-ops.
//
// Clicking the tray icon (or choosing "toggle" from its menu) flips
// the same shared state as the main window; the icon recolors
// red/green to match.
package tray

import "github.com/onlyactions/the-button/internal/state"

// Run installs the tray icon and blocks until the tray is dismissed
// (usually the process ends first). Run it in a goroutine. It never
// fails the app: on any tray problem it just returns.
//
// windowTitle names the main window so the tray's "show" entry can
// raise it where the platform supports that.
func Run(st *state.State, windowTitle string) {
	run(st, windowTitle)
}
