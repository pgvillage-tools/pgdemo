#!/bin/bash
# pgdemo (PostgreSQL Demo Schema Builder): build installable packages.
#
#   bash build/package.sh                 # version from build/VERSION, all targets
#   bash build/package.sh 1.2.0           # this version, all targets
#   bash build/package.sh 1.2.0 mac       # only some targets: mac, windows, linux
#
# Output in dist/:
#   pgdemo-<version>-macos-universal.zip   pgdemo.app (Apple silicon + Intel)
#   pgdemo-<version>-windows-amd64.zip     pgdemo.exe
#   pgdemo-<version>-linux-amd64.tar.xz    binary, icon, desktop entry, install.sh
#
# What each target needs (the script checks and says what is missing):
#   mac      a Mac with Go and the Xcode command line tools (xcode-select --install)
#   windows  on a Mac: MinGW-w64 (brew install mingw-w64)
#            or Docker/Podman with fyne-cross; on Windows: Git Bash + MinGW-w64 gcc
#   linux    on Linux: gcc and the X11/OpenGL headers
#            (Debian/Ubuntu: sudo apt install gcc libgl1-mesa-dev xorg-dev)
#            on a Mac: Docker/Podman with fyne-cross
#            (go install github.com/fyne-io/fyne-cross@latest)
#
# Bash 3.2 compatible (the bash that comes with macOS).

set -u
cd "$(dirname "$0")/.." || exit 1
ROOT=$(pwd)

APP=pgdemo
APP_ID=nl.nibble-it.pgdemo
MIN_MACOS=11.0

VERSION=""
case "${1:-}" in
	[0-9]*) VERSION=$1; shift ;;
esac
if [ -z "$VERSION" ] && [ -f build/VERSION ]; then
	VERSION=$(tr -d ' \t\r\n' < build/VERSION)
fi
VERSION=${VERSION:-0.0.0}
case "$VERSION" in
	[0-9]*.[0-9]*.[0-9]*) ;;
	*) echo "Version must look like 1.2.3 (got '$VERSION')"; exit 1 ;;
esac
TARGETS=${*:-mac windows linux}
BUILD_NO=$(date +%Y%m%d%H%M)

DIST="$ROOT/dist"
mkdir -p "$DIST"
# build in a temporary folder on the local disk: codesign and the archives
# must not see the "._" files macOS writes on a USB stick (exFAT/FAT)
WORK=$(mktemp -d "${TMPDIR:-/tmp}/pgdemo-package.XXXXXX") || exit 1
trap 'rm -rf "$WORK"' EXIT

HOST=$(uname -s)
LDFLAGS="-s -w -X main.version=$VERSION"
DOCS="README.md"
OK=""
SKIPPED=""
FAILED=""

say()  { printf '\n=== %s\n' "$*"; }
have() { command -v "$1" >/dev/null 2>&1; }
ok()   { OK="$OK\n  $1"; }
skip() { SKIPPED="$SKIPPED\n  $1: $2"; echo "Skipped: $2"; }
fail() { FAILED="$FAILED\n  $1: $2"; echo "FAILED: $2"; }

if ! have go; then
	echo "Go is not installed (https://go.dev/dl/)."
	exit 1
fi
echo "pgdemo $VERSION (build $BUILD_NO), $(go version)"

# copy the documentation into a package folder
copy_docs() {
	mkdir -p "$1/docs"
	for f in $DOCS; do
		[ -f "$f" ] && cp "$f" "$1/docs/"
	done
	cat > "$1/docs/FIRST_START.txt" <<EOF
pgdemo $VERSION - PostgreSQL Demo Schema Builder

Creates demo schemas with generated data in a PostgreSQL database
(Webshop, HR, Library, School, Nibble, Nibble secure, Nibble secure history,
Nibble enterprise, and their country versions BE, DE and UK).
The connection details are asked when the app starts; see docs/README.md.
Everything runs in one transaction: on an error or cancel nothing is left behind.
Without the window: pgdemo create --template <template> --schema <name> ...
(pgdemo help; docs/README.md, "Command line").
EOF
}

# ---------------------------------------------------------------- macOS
build_mac() {
	say "macOS (Apple silicon + Intel)"
	if [ "$HOST" != "Darwin" ]; then
		skip mac "macOS packages can only be built on a Mac"
		return
	fi
	for t in clang lipo sips iconutil codesign ditto; do
		if ! have $t; then
			skip mac "$t not found; install the Xcode command line tools: xcode-select --install"
			return
		fi
	done
	local out="$WORK/mac"
	mkdir -p "$out"
	for arch in arm64 amd64; do
		echo "Compiling darwin/$arch ..."
		if ! CGO_ENABLED=1 GOOS=darwin GOARCH=$arch \
			MACOSX_DEPLOYMENT_TARGET=$MIN_MACOS \
			CGO_CFLAGS="-mmacosx-version-min=$MIN_MACOS" \
			CGO_LDFLAGS="-mmacosx-version-min=$MIN_MACOS" \
			go build -trimpath -ldflags "$LDFLAGS" -o "$out/$APP-$arch" . ; then
			fail mac "compiling for darwin/$arch failed (see above)"
			return
		fi
	done
	local app="$out/$APP.app"
	mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources"
	lipo -create -output "$app/Contents/MacOS/$APP" "$out/$APP-arm64" "$out/$APP-amd64" || { fail mac "lipo failed"; return; }

	# icon
	local iset="$out/icon.iconset"
	mkdir -p "$iset"
	for s in 16 32 128 256 512; do
		sips -z $s $s Icon.png --out "$iset/icon_${s}x${s}.png" >/dev/null
		sips -z $((s * 2)) $((s * 2)) Icon.png --out "$iset/icon_${s}x${s}@2x.png" >/dev/null
	done
	iconutil -c icns "$iset" -o "$app/Contents/Resources/icon.icns" || { fail mac "iconutil failed"; return; }

	cat > "$app/Contents/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleName</key><string>$APP</string>
	<key>CFBundleDisplayName</key><string>$APP</string>
	<key>CFBundleExecutable</key><string>$APP</string>
	<key>CFBundleIdentifier</key><string>$APP_ID</string>
	<key>CFBundleIconFile</key><string>icon.icns</string>
	<key>CFBundlePackageType</key><string>APPL</string>
	<key>CFBundleShortVersionString</key><string>$VERSION</string>
	<key>CFBundleVersion</key><string>$BUILD_NO</string>
	<key>CFBundleInfoDictionaryVersion</key><string>6.0</string>
	<key>LSMinimumSystemVersion</key><string>$MIN_MACOS</string>
	<key>LSApplicationCategoryType</key><string>public.app-category.developer-tools</string>
	<key>NSHighResolutionCapable</key><true/>
	<key>NSHumanReadableCopyright</key><string>Nibble-IT</string>
</dict>
</plist>
EOF
	# ad-hoc signature: required on Apple silicon; no Developer ID
	codesign --force --deep --sign - "$app" || { fail mac "codesign failed"; return; }

	local pkg="$out/pkg"
	mkdir -p "$pkg"
	mv "$app" "$pkg/"
	copy_docs "$pkg"
	cat >> "$pkg/docs/FIRST_START.txt" <<'EOF'

macOS: the app is not signed with an Apple Developer ID. The first time,
macOS refuses to open it. Open System Settings > Privacy & Security and click
"Open Anyway" next to pgdemo (or, in Terminal:
xattr -dr com.apple.quarantine /Applications/pgdemo.app).
EOF
	local zipf="$DIST/$APP-$VERSION-macos-universal.zip"
	rm -f "$zipf"
	(cd "$pkg" && COPYFILE_DISABLE=1 ditto -c -k --norsrc . "$zipf") || { fail mac "zip failed"; return; }
	lipo -info "$pkg/$APP.app/Contents/MacOS/$APP"
	ok "$(basename "$zipf")"
}

# ---------------------------------------------------------------- Windows
build_windows() {
	say "Windows 64-bit"
	local out="$WORK/windows/$APP"
	mkdir -p "$out"
	local exe="$out/$APP.exe"
	if [ "$HOST" = "Darwin" ] || [ "$HOST" = "Linux" ]; then
		if have x86_64-w64-mingw32-gcc; then
			echo "Compiling windows/amd64 with MinGW-w64 ..."
			if ! CGO_ENABLED=1 GOOS=windows GOARCH=amd64 \
				CC=x86_64-w64-mingw32-gcc CXX=x86_64-w64-mingw32-g++ \
				go build -trimpath -ldflags "$LDFLAGS -H=windowsgui" -o "$exe" . ; then
				fail windows "compiling for windows/amd64 failed (see above)"
				return
			fi
		elif cross_engine >/dev/null; then
			fyne_cross windows "$exe" || return
		else
			skip windows "no MinGW-w64 (brew install mingw-w64) and fyne-cross cannot run: $CROSS_WHY"
			return
		fi
	elif have gcc; then
		# on Windows itself (Git Bash / MSYS2 with a MinGW-w64 gcc)
		echo "Compiling windows/amd64 ..."
		if ! CGO_ENABLED=1 go build -trimpath -ldflags "$LDFLAGS -H=windowsgui" -o "$exe" . ; then
			fail windows "compiling for windows/amd64 failed (see above)"
			return
		fi
	else
		skip windows "on Windows this needs a MinGW-w64 gcc (e.g. MSYS2) in the PATH"
		return
	fi
	copy_docs "$out"
	cat >> "$out/docs/FIRST_START.txt" <<'EOF'

Windows: unzip the folder anywhere and start pgdemo.exe. Windows may
show "Windows protected your PC" the first time (the program is not signed):
click "More info" and "Run anyway". pgdemo needs OpenGL 2.1; in a
virtual machine or over Remote Desktop without a graphics driver, put Mesa's
opengl32.dll next to pgdemo.exe.
EOF
	local zipf="$DIST/$APP-$VERSION-windows-amd64.zip"
	rm -f "$zipf"
	(cd "$WORK/windows" && zip -qrX "$zipf" "$APP") || { fail windows "zip failed"; return; }
	ok "$(basename "$zipf")"
}

# ---------------------------------------------------------------- Linux
build_linux() {
	say "Linux 64-bit (x86_64)"
	local out="$WORK/linux/$APP"
	mkdir -p "$out"
	local bin="$out/$APP"
	if [ "$HOST" = "Linux" ] && [ "$(uname -m)" = "x86_64" ]; then
		echo "Compiling linux/amd64 ..."
		if ! CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$LDFLAGS" -o "$bin" . ; then
			fail linux "compiling failed; are gcc and the X11/OpenGL headers installed? (sudo apt install gcc libgl1-mesa-dev xorg-dev)"
			return
		fi
	elif cross_engine >/dev/null; then
		fyne_cross linux "$bin" || return
	else
		skip linux "needs a Linux x86_64 machine, or fyne-cross on this Mac: $CROSS_WHY"
		return
	fi
	cp Icon.png "$out/$APP.png"
	cat > "$out/$APP.desktop" <<EOF
[Desktop Entry]
Type=Application
Name=$APP
Comment=Demo schemas with generated data for PostgreSQL
Exec=$APP
Icon=$APP
Terminal=false
Categories=Development;Database;
EOF
	cat > "$out/install.sh" <<'EOF'
#!/bin/sh
# Installs pgdemo for this user (~/.local) or, with sudo, for everyone (/usr/local).
set -e
cd "$(dirname "$0")"
if [ "$(id -u)" = 0 ]; then P=/usr/local; else P="$HOME/.local"; fi
install -Dm755 pgdemo "$P/bin/pgdemo"
install -Dm644 pgdemo.png "$P/share/icons/hicolor/512x512/apps/pgdemo.png"
install -Dm644 pgdemo.desktop "$P/share/applications/pgdemo.desktop"
echo "Installed in $P (start: pgdemo, or from the applications menu)."
EOF
	chmod +x "$out/install.sh" "$bin"
	copy_docs "$out"
	local tarf="$DIST/$APP-$VERSION-linux-amd64.tar.xz"
	rm -f "$tarf"
	# no macOS or SELinux file attributes in the archive: GNU tar on Linux would warn
	# "Ignoring unknown extended header keyword" for each of them when unpacking
	local tarflags=""
	if tar --version 2>/dev/null | grep -qi bsdtar; then
		tarflags="--no-xattrs --no-mac-metadata --no-acls" # macOS tar
	elif tar --no-xattrs --version >/dev/null 2>&1; then
		tarflags="--no-xattrs" # GNU tar
	fi
	if ! (cd "$WORK/linux" && COPYFILE_DISABLE=1 tar $tarflags -cJf "$tarf" "$APP"); then
		fail linux "tar failed"
		return
	fi
	ok "$(basename "$tarf")"
}

# ---------------------------------------------------------------- fyne-cross (Docker/Podman)
# cross_engine prints docker or podman when fyne-cross can run; otherwise it
# returns 1 and CROSS_WHY says what is missing.
CROSS_WHY=""
cross_engine() {
	if ! have docker && ! have podman; then
		CROSS_WHY="no Docker or Podman"
		return 1
	fi
	if have docker && docker info >/dev/null 2>&1; then
		ENGINE=docker
	elif have podman && podman info >/dev/null 2>&1; then
		ENGINE=podman
	elif have podman; then
		CROSS_WHY="Podman is installed but its virtual machine is not running: podman machine init (once), then podman machine start"
		return 1
	else
		CROSS_WHY="Docker is installed but not running (start Docker Desktop)"
		return 1
	fi
	if ! have fyne-cross; then
		if [ -x "$(go env GOPATH)/bin/fyne-cross" ]; then
			PATH="$PATH:$(go env GOPATH)/bin"
		else
			CROSS_WHY="fyne-cross is not installed: go install github.com/fyne-io/fyne-cross@latest"
			return 1
		fi
	fi
	echo $ENGINE
}

# fyne_cross <windows|linux> <output file>
#
# The containers only see folders under the home folder (Podman/Docker on a
# Mac share /Users, not /Volumes), so the sources are copied to
# ~/.cache/pgdemo-cross first.
fyne_cross() {
	local os=$1 dest=$2 engine src
	engine=$(cross_engine) || { fail "$os" "$CROSS_WHY"; return 1; }
	src="$HOME/.cache/pgdemo-cross/$APP"
	rm -rf "$src" && mkdir -p "$src" || { fail "$os" "cannot create $src"; return 1; }
	if ! (cd "$ROOT" && COPYFILE_DISABLE=1 tar -cf - --exclude '._*' --exclude '.DS_Store' \
		--exclude './dist' --exclude './fyne-cross' --exclude '*.bak' --exclude './README.pdf' .) | (cd "$src" && tar -xf -); then
		fail "$os" "copying the sources to $src failed"
		return 1
	fi
	# fyne-cross does not pass -ldflags "-X main.version=..." on to the Go build, so
	# the program would show "<build/VERSION> (dev build)": set the version in the copy
	if ! sed "s/^var version = \"dev\"\$/var version = \"$VERSION\"/" "$src/version.go" > "$src/version.go.new" ||
		! grep -q "^var version = \"$VERSION\"\$" "$src/version.go.new"; then
		fail "$os" "cannot set version $VERSION in $src/version.go"
		return 1
	fi
	mv "$src/version.go.new" "$src/version.go"
	echo "Compiling $os/amd64 with fyne-cross ($engine) in $src ..."
	echo "(the first run downloads the build image, about 1-2 GB; on Apple silicon the build runs emulated and is slow)"
	if ! (cd "$src" && fyne-cross "$os" -engine "$engine" -arch=amd64 -app-id "$APP_ID" -icon Icon.png \
		-name "$APP" -app-version "$VERSION" -ldflags "-X=main.version=$VERSION" -env GOTOOLCHAIN=auto); then
		fail "$os" "fyne-cross failed (see above)"
		return 1
	fi
	local b="$src/fyne-cross/bin/$os-amd64/$APP"
	[ "$os" = windows ] && b="$b.exe"
	if [ ! -f "$b" ]; then
		fail "$os" "fyne-cross did not produce $b"
		return 1
	fi
	cp "$b" "$dest"
}

for t in $TARGETS; do
	case "$t" in
		mac | macos | darwin) build_mac ;;
		windows | win) build_windows ;;
		linux) build_linux ;;
		*) echo "Unknown target '$t' (use mac, windows, linux)"; exit 1 ;;
	esac
done

say "Summary for pgdemo $VERSION"
[ -n "$OK" ] && printf "Built in dist/:%b\n" "$OK"
[ -n "$SKIPPED" ] && printf "Skipped:%b\n" "$SKIPPED"
[ -n "$FAILED" ] && printf "Failed:%b\n" "$FAILED"
[ -z "$FAILED" ]
