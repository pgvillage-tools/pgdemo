package main

import (
	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"pgdemo/internal/nibbler"
)

// mainMenu: only Help → About (as in QNibbler and pgTestBuilder). On macOS it is
// in the menu bar at the top of the screen, on Windows and Linux a menu row in
// the window.
func (u *appUI) mainMenu() *fyne.MainMenu {
	return fyne.NewMainMenu(
		fyne.NewMenu("Help", fyne.NewMenuItem("About pgdemo", u.showAbout)),
	)
}

// showAbout: the icon, the name and version, the Nibbler eating a nibble (the
// 8-bit animation of QNibbler, internal/nibbler, running while the window is
// open), and the current connection.
func (u *appUI) showAbout() {
	icon := canvas.NewImageFromResource(appIcon)
	icon.FillMode = canvas.ImageFillContain
	icon.SetMinSize(fyne.NewSize(72, 72))
	title := widget.NewLabelWithStyle("pgdemo "+version, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	sub := widget.NewLabel("Nibble-IT – demo schemas with generated data for PostgreSQL")
	head := container.NewHBox(icon, container.NewVBox(layout.NewSpacer(), title, sub, layout.NewSpacer()))

	// the animation: scaled up with sharp pixels
	const scale = 4
	ink := theme.Color(theme.ColorNameForeground)
	scene := canvas.NewImageFromImage(nibbler.Frame(0, ink))
	scene.ScaleMode = canvas.ImageScalePixels
	scene.FillMode = canvas.ImageFillContain
	scene.SetMinSize(fyne.NewSize(nibbler.Width*scale, nibbler.Height*scale))
	anim := fyne.NewAnimation(time.Duration(nibbler.Loop*float64(time.Second)), func(f float32) {
		scene.Image = nibbler.Frame(float64(f)*nibbler.Loop, ink)
		scene.Refresh()
	})
	anim.Curve = fyne.AnimationLinear
	anim.RepeatCount = fyne.AnimationRepeatForever
	caption := widget.NewLabelWithStyle("8 bits = 1 byte  ·  4 bits = 1 nibble", fyne.TextAlignCenter, fyne.TextStyle{Italic: true})

	love := widget.NewLabelWithStyle("Coded with love by Robert de Laat – www.nibble-it.nl", fyne.TextAlignCenter, fyne.TextStyle{})

	// the details: readable, but smaller than the rest
	var lines []string
	if u.connected {
		c := u.lastGood
		lines = append(lines, fmt.Sprintf("Connection: %s@%s:%s/%s (sslmode=%s)", c.User, c.Host, c.Port, c.Database, c.SSLMode))
		if u.server != "" {
			server, _, _ := strings.Cut(u.server, " on ") // "PostgreSQL 18.6 on x86_64-pc-linux-gnu, ..."
			lines = append(lines, "Server: "+server)
		}
	} else {
		lines = append(lines, "Not connected")
	}
	lines = append(lines,
		fmt.Sprintf("Templates: %d (pgdemo templates on the command line lists them)", len(Templates)),
		"Command line: pgdemo create --template <template> --schema <name> (pgdemo help)",
		"The connection is remembered without the password.")
	info := container.NewVBox()
	small := fyne.CurrentApp().Settings().Theme().Size(theme.SizeNameCaptionText)
	for _, line := range lines {
		t := canvas.NewText(line, theme.Color(theme.ColorNameForeground))
		t.TextSize = small
		info.Add(t)
	}

	var d dialog.Dialog
	closeBtn := widget.NewButton("Close", func() { d.Hide() })
	closeBtn.Importance = widget.HighImportance
	content := container.NewVBox(head, scene, caption, love, widget.NewSeparator(), container.NewPadded(info),
		container.NewHBox(layout.NewSpacer(), closeBtn))
	cd := dialog.NewCustomWithoutButtons("About pgdemo", content, u.w)
	d = cd
	cd.SetOnClosed(anim.Stop)
	cd.Resize(fyne.NewSize(600, 0))
	cd.Show()
	anim.Start()
}
