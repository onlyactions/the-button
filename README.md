# the button

Only a button. It is a circle, with a power symbol. Click it: it
turns from red (off) to green (on) and back. That is all.

Built with [Gio](https://gioui.org) — a pure Go GUI, one codebase for
every platform: Linux, Windows, Android and the Web (WebAssembly).
There is intentionally no macOS build.

## Install

Grab a build from [Releases](../../releases):

| File | Platform |
| --- | --- |
| `the-button-linux-amd64.tar.gz` | Linux x86_64, plain binary |
| `the-button-linux-x86_64.AppImage` | Linux AppImage |
| `the-button-windows-amd64.zip` | Windows x86_64 |
| `the-button-windows-arm64.zip` | Windows ARM64 |
| `the-button.apk` | Android (debug-signed — allow "unknown sources") |
| `the-button-web.zip` | any device with a browser |

### Linux notes

The AppImage bundles nothing: it always uses your system's graphics
stack (system EGL/GL + your driver), which avoids the classic
bundled-libEGL vs. host-driver conflicts. On a machine with a broken
GPU driver, force software rendering:

    LIBGL_ALWAYS_SOFTWARE=1 ./the-button-linux-x86_64.AppImage

### Web notes

Unzip and serve the folder over http (browsers refuse `file://` for
WebAssembly):

    python3 -m http.server 8000
    # then open http://localhost:8000

## Build from source

    go build .                                        # Linux (needs EGL/wayland/xkb dev headers)
    CGO_ENABLED=0 GOOS=windows go build -o the-button.exe .   # Windows
    GOOS=js GOARCH=wasm go build -o web/main.wasm .            # Web (needs an index.html)

Android APK (needs Android SDK + NDK):

    go install gioui.org/cmd/gogio@v0.10.3
    gogio -target android -minsdk 24 -icon icon.png \
        -appid com.onlyactions.thebutton -o the-button.apk .

Every push builds every platform and publishes the artifacts to
GitHub Releases (see `.github/workflows/release.yml`).
