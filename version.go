package main

import (
	_ "embed"
	"strings"

	"fyne.io/fyne/v2"
)

// version is set when building a package:
//
//	go build -ldflags "-X main.version=1.0.0"
//
// (see build/package.sh). A plain go build or go run takes the number from
// build/VERSION and adds "(dev build)", so it is clear which version the code
// is and that it is not a released package.
var version = "dev"

//go:embed build/VERSION
var versionFile string

func init() {
	if version == "dev" {
		if v := strings.TrimSpace(versionFile); v != "" {
			version = v + " (dev build)"
		}
	}
}

// appIcon is the window and taskbar icon (Icon.png next to the sources).
//
//go:embed Icon.png
var appIconPNG []byte

var appIcon = fyne.NewStaticResource("Icon.png", appIconPNG)
