#!/bin/sh
# Build the AppImage around a Linux build of "the button".
#
# Deliberately bundles NOTHING: no GL, no EGL, no X11 libraries. The app
# always uses the system graphics driver, which is the sane default and
# avoids the classic bundled-libEGL vs. host-driver conflicts
# (eglGetDisplay failures and friends).
#
# usage: sh packaging/appimage.sh [binary] [output.AppImage]
set -eu

BIN="${1:-the-button}"
OUT="${2:-the-button-linux-x86_64.AppImage}"
HERE="$(cd "$(dirname "$0")/.." && pwd)"
WORK="$(mktemp -d)"
APPDIR="$WORK/AppDir"
trap 'rm -rf "$WORK"' EXIT

mkdir -p "$APPDIR/usr/bin"
cp "$HERE/$BIN" "$APPDIR/usr/bin/the-button"
chmod 755 "$APPDIR/usr/bin/the-button"
cp "$HERE/icon.png" "$APPDIR/the-button.png"
ln -sf the-button.png "$APPDIR/.DirIcon"

cat > "$APPDIR/the-button.desktop" <<'EOF'
[Desktop Entry]
Type=Application
Name=the button
Comment=A button. It is a circle.
Exec=the-button
Icon=the-button
Terminal=false
Categories=Utility;
EOF

cat > "$APPDIR/AppRun" <<'EOF'
#!/bin/sh
HERE="$(dirname "$(readlink -f "$0")")"
exec "$HERE/usr/bin/the-button" "$@"
EOF
chmod 755 "$APPDIR/AppRun"

TOOL="$WORK/appimagetool"
curl -fsSL -o "$TOOL" https://github.com/AppImage/AppImageKit/releases/download/continuous/appimagetool-x86_64.AppImage
chmod +x "$TOOL"
ARCH=x86_64 "$TOOL" --appimage-extract-and-run --no-appstream "$APPDIR" "$OUT"

echo "built $OUT"
