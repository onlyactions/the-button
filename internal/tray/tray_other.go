//go:build !windows && (!linux || android)

// SPDX-License-Identifier: Unlicense OR MIT

package tray

import "github.com/onlyactions/the-button/internal/state"

func run(st *state.State, windowTitle string) {}
