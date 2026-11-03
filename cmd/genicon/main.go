// SPDX-License-Identifier: Unlicense OR MIT

// Command genicon regenerates every icon asset — icon.png, the
// Android launcher icons, the widget drawables and the quick-settings
// tile drawables — from the same drawing code the app and the tray
// use. Run from the repository root: go run ./cmd/genicon
package main

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"

	"github.com/onlyactions/the-button/internal/gfx"
)

func main() {
	write := func(path string, size int, on bool, frac float64) {
		img := gfx.Icon(size, on, frac)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			fatal(err)
		}
		f, err := os.Create(path)
		if err != nil {
			fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, img); err != nil {
			fatal(err)
		}
		fmt.Println("wrote", path)
	}

	// Desktop / AppImage / web icon.
	write("icon.png", 512, false, 0.95)

	res := filepath.Join("android", "app", "src", "main", "res")

	// Launcher icons (red = resting state).
	densities := []struct {
		dir  string
		size int
	}{
		{"mipmap-mdpi", 48},
		{"mipmap-hdpi", 72},
		{"mipmap-xhdpi", 96},
		{"mipmap-xxhdpi", 144},
		{"mipmap-xxxhdpi", 192},
	}
	for _, d := range densities {
		write(filepath.Join(res, d.dir, "ic_launcher.png"), d.size, false, 0.9)
	}

	// Widget button drawables (nodpi: one scaled source is fine).
	write(filepath.Join(res, "drawable-nodpi", "widget_off.png"), 256, false, 0.95)
	write(filepath.Join(res, "drawable-nodpi", "widget_on.png"), 256, true, 0.95)

	// Quick-settings tile drawables.
	write(filepath.Join(res, "drawable-nodpi", "tile_off.png"), 64, false, 0.9)
	write(filepath.Join(res, "drawable-nodpi", "tile_on.png"), 64, true, 0.9)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "genicon:", err)
	os.Exit(1)
}
