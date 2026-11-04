# the button

Only a button. It is a circle, with a power symbol. Click it: it
turns from red (off) to green (on) and back. That is all.

Built with [Gio](https://gioui.org) — a pure Go GUI, one codebase for
every platform: Linux, Windows, Android and the Web (WebAssembly).
There is intentionally no macOS build.

## The design

The button is the only light in the room. Its face is a candy-gloss
disc — a four-stop vertical gradient with a specular hotspot, a rim
light along the upper-left edge and a grounding shade at the bottom —
baked in software with float precision and ordered dithering, so it
renders clean everywhere. Around it, a three-band halo (hot rim, mid
haze, wide bloom) breathes when the button is on, and the room itself
catches a wash of the button's color. The power glyph is engraved:
an emboss shadow beneath, a highlight on its upper lip, and a bloom
when the body is lit.

It feels physical: a damped spring drives the press (the button sinks
and its shadow tightens), releasing pops it back with a small
overshoot, and every toggle is announced by an expanding ripple and a
soft luminous flash — the color crossfade is lifted through light so
it never squelches through muddy brown. On launch the whole scene
fades from black while the glyph draws itself in: the bar first, then
the ring sweeping symmetrically from the top gap.

## Install

Grab a build from [Releases](../../releases):

| File | Platform |
| --- | --- |
| `the-button-linux-amd64.tar.gz` | Linux x86_64, plain binary |
| `the-button-linux-x86_64.AppImage` | Linux AppImage |
| `the-button-windows-amd64.zip` | Windows x86_64 |
| `the-button-windows-arm64.zip` | Windows ARM64 |
| `the-button.apk` | Android (signed; app + quick-settings tile + home-screen widget) |
| `the-button-web.zip` | any device with a browser |

### Linux notes

The AppImage bundles nothing: it always uses your system's graphics
stack (system EGL/GL + your driver), which avoids the classic
bundled-libEGL vs. host-driver conflicts. On a machine with a broken
GPU driver, force software rendering:

    LIBGL_ALWAYS_SOFTWARE=1 ./the-button-linux-x86_64.AppImage

### Android extras

The single on/off state is shared by four surfaces: the app window,
the quick-settings tile (pull down the notification shade and add
"the button"), the home-screen widget (long-press the desktop →
widgets → the button) and the desktop tray icon. Toggle it anywhere;
it flips everywhere.

### Web notes

Unzip and serve the folder over http (browsers refuse `file://` for
WebAssembly):

    python3 -m http.server 8000
    # then open http://localhost:8000

## Build from source

    go build .                                        # Linux (needs EGL/wayland/xkb dev headers)
    CGO_ENABLED=0 GOOS=windows go build -o the-button.exe .   # Windows
    GOOS=js GOARCH=wasm go build -o web/main.wasm .            # Web (needs an index.html)

Android APK (needs Android SDK + NDK + JDK 17):

    go tool gomobile bind -javapkg=com.onlyactions.thebutton \
        -target=android -androidapi 24 -o android/app/libs/thebutton.aar ./mobile
    cd android && gradle assembleRelease

`mobile/` is the Go library (`gomobile bind`) holding the state and the
Gio window; `android/` is the Gradle project around it (activity,
quick-settings tile, home-screen widget). The release APK is signed
with the committed throwaway keystore `android/app/thebutton.keystore`
(password `thebutton`) so consecutive CI builds upgrade in place.

Icons are generated from the same drawing code as the app itself:

    go run ./cmd/genicon

Every push builds every platform and publishes the artifacts to
GitHub Releases (see `.github/workflows/release.yml`).
