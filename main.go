package main

import (
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
)

func main() {
	// With a command (create, templates, help, version) there is no window: see cli.go.
	if isCommandLine(os.Args[1:]) {
		os.Exit(runCommandLine(os.Args[1:]))
	}
	a := app.NewWithID("nl.nibble-it.pgdemo")
	a.SetIcon(appIcon)
	w := a.NewWindow("PostgreSQL Demo Schema Builder " + version)
	u := newUI(a, w)
	w.SetContent(u.build())
	w.SetMainMenu(u.mainMenu()) // Help → About (ui_about.go)
	w.Resize(fyne.NewSize(1100, 720))
	w.Show()
	u.showLogin(true) // connection popup on startup
	a.Run()
}
