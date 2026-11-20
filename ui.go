package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

var sizeOptions = []string{"Small (100)", "Medium (1,000)", "Large (10,000)", "Custom"}
var sizeValues = map[string]int{"Small (100)": 100, "Medium (1,000)": 1000, "Large (10,000)": 10000}

// maxCustomSize and schemaNameRe: see options.go

type appUI struct {
	a fyne.App
	w fyne.Window

	host, port, dbname, user, pass *widget.Entry
	ssl                            *widget.Select

	schema   *widget.Entry
	tmpl     *widget.Select
	tmplDesc *widget.Label

	drop, fks, idx, views, data *widget.Check
	size                        *widget.RadioGroup
	custom, seed                *widget.Entry
	estimate                    *widget.Label

	progress *widget.ProgressBar
	status   *widget.Label
	logLines []string
	logList  *widget.List

	connLabel *widget.Label
	connected bool
	lastGood  ConnSettings // last settings that connected successfully

	connBtn, previewBtn, createBtn, cancelBtn *widget.Button
	cancel                                    context.CancelFunc
}

func newUI(a fyne.App, w fyne.Window) *appUI { return &appUI{a: a, w: w} }

func (u *appUI) build() fyne.CanvasObject {
	p := u.a.Preferences()

	// --- connection
	u.host = entry(p.StringWithFallback("host", "localhost"))
	u.port = entry(p.StringWithFallback("port", "5432"))
	u.dbname = entry(p.StringWithFallback("database", "postgres"))
	u.user = entry(p.StringWithFallback("user", "postgres"))
	u.pass = widget.NewPasswordEntry()
	u.ssl = widget.NewSelect([]string{"disable", "prefer", "require", "verify-full"}, nil)
	u.ssl.SetSelected(p.StringWithFallback("sslmode", "prefer"))

	// The connection fields live in the login dialog (see showLogin);
	// the main page only shows a compact status bar.
	u.connLabel = widget.NewLabel("Not connected")
	u.connBtn = widget.NewButton("Change connection...", func() { u.showLogin(false) })
	connBar := container.NewHBox(
		widget.NewLabelWithStyle("Connection:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		u.connLabel, layout.NewSpacer(), u.connBtn,
	)

	// --- schema options
	u.schema = entry(p.StringWithFallback("schema", "demo"))
	u.schema.OnChanged = func(string) { u.updateEstimate() } // extra schema names follow
	u.tmplDesc = widget.NewLabel("")
	u.tmplDesc.Wrapping = fyne.TextWrapWord
	u.estimate = widget.NewLabel("")
	u.estimate.Wrapping = fyne.TextWrapWord
	u.tmpl = widget.NewSelect(TemplateNames(), func(name string) {
		if t := TemplateByName(name); t != nil {
			u.tmplDesc.SetText(t.Description)
		}
		u.updateEstimate()
	})

	u.drop = widget.NewCheck("Drop existing schema (CASCADE)", nil)
	u.fks = widget.NewCheck("Foreign keys", nil)
	u.idx = widget.NewCheck("Indexes", nil)
	u.views = widget.NewCheck("Example views", nil)
	u.fks.SetChecked(true)
	u.idx.SetChecked(true)
	u.views.SetChecked(true)

	// --- data options
	u.custom = widget.NewEntry()
	u.custom.SetPlaceHolder("e.g. 50000")
	u.custom.OnChanged = func(string) { u.updateEstimate() }
	u.seed = widget.NewEntry()
	u.seed.SetPlaceHolder("empty = random, a number = same data every time")
	u.size = widget.NewRadioGroup(sizeOptions, func(s string) {
		if s == "Custom" && u.data.Checked {
			u.custom.Enable()
		} else {
			u.custom.Disable()
		}
		u.updateEstimate()
	})
	u.data = widget.NewCheck("Insert demo data", func(on bool) {
		if on {
			u.size.Enable()
			u.seed.Enable()
		} else {
			u.size.Disable()
			u.seed.Disable()
		}
		u.size.OnChanged(u.size.Selected)
	})
	u.size.SetSelected(sizeOptions[1])
	u.data.SetChecked(true)
	u.tmpl.SetSelected(Templates[0].Name)

	optCard := widget.NewCard("Demo schema", "", container.NewVBox(
		widget.NewForm(
			widget.NewFormItem("Schema name", u.schema),
			widget.NewFormItem("Schema type", u.tmpl),
		),
		u.tmplDesc,
		widget.NewSeparator(),
		u.drop, u.fks, u.idx, u.views,
		widget.NewSeparator(),
		u.data,
		u.size,
		widget.NewForm(
			widget.NewFormItem("Custom size", u.custom),
			widget.NewFormItem("Seed", u.seed),
		),
		u.estimate,
	))

	// --- log + actions
	u.logList = widget.NewList(
		func() int { return len(u.logLines) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(i widget.ListItemID, o fyne.CanvasObject) { o.(*widget.Label).SetText(u.logLines[i]) },
	)
	copyBtn := widget.NewButton("Copy", func() {
		u.w.Clipboard().SetContent(strings.Join(u.logLines, "\n"))
		u.status.SetText(fmt.Sprintf("Copied %d log lines to the clipboard", len(u.logLines)))
	})
	exportBtn := widget.NewButton("Export log...", func() {
		lines := append([]string(nil), u.logLines...)
		d := dialog.NewFileSave(func(wc fyne.URIWriteCloser, err error) {
			if err != nil || wc == nil {
				return
			}
			defer wc.Close()
			if _, err := wc.Write([]byte(strings.Join(lines, "\n") + "\n")); err != nil {
				dialog.ShowError(err, u.w)
				return
			}
			u.status.SetText(fmt.Sprintf("Exported %d log lines to %s", len(lines), wc.URI().Path()))
		}, u.w)
		d.SetFileName("pgdemo_log_" + time.Now().Format("2006-01-02_1504") + ".txt")
		d.Show()
	})
	clearBtn := widget.NewButton("Clear", func() {
		dialog.ShowConfirm("Clear log",
			fmt.Sprintf("Remove all %d log lines?\nThis cannot be undone: use Export log... first if you want to keep them.", len(u.logLines)),
			func(ok bool) {
				if !ok {
					return
				}
				u.logLines = nil
				u.logList.Refresh()
				u.status.SetText("Log cleared")
			}, u.w)
	})
	logCard := widget.NewCard("Log", "", container.NewBorder(nil, container.NewHBox(layout.NewSpacer(), copyBtn, exportBtn, clearBtn), nil, nil, u.logList))

	u.progress = widget.NewProgressBar()
	u.status = widget.NewLabel("Ready")
	u.previewBtn = widget.NewButton("Preview SQL", u.onPreview)
	u.createBtn = widget.NewButton("Create demo schema", u.onCreate)
	u.createBtn.Importance = widget.HighImportance
	u.cancelBtn = widget.NewButton("Cancel", func() {
		if u.cancel != nil {
			u.cancel()
		}
	})
	u.cancelBtn.Disable()

	left := container.NewVScroll(optCard)
	split := container.NewHSplit(left, logCard)
	split.Offset = 0.45

	bottom := container.NewVBox(
		u.progress,
		container.NewHBox(u.status, layout.NewSpacer(), u.previewBtn, u.cancelBtn, u.createBtn),
	)
	top := container.NewVBox(connBar, widget.NewSeparator())
	return container.NewBorder(top, bottom, nil, nil, split)
}

func entry(text string) *widget.Entry {
	e := widget.NewEntry()
	e.SetText(text)
	return e
}

// log is safe to call from any goroutine.
func (u *appUI) log(msg string) {
	line := time.Now().Format("15:04:05") + "  " + msg
	fyne.Do(func() {
		u.logLines = append(u.logLines, strings.Split(line, "\n")...)
		u.logList.Refresh()
		u.logList.ScrollToBottom()
	})
}

func (u *appUI) conn() ConnSettings {
	return ConnSettings{
		Host:     strings.TrimSpace(u.host.Text),
		Port:     strings.TrimSpace(u.port.Text),
		Database: strings.TrimSpace(u.dbname.Text),
		User:     strings.TrimSpace(u.user.Text),
		Password: u.pass.Text,
		SSLMode:  u.ssl.Selected,
	}
}

func (u *appUI) dataSize() (int, error) {
	if v, ok := sizeValues[u.size.Selected]; ok {
		return v, nil
	}
	n, err := strconv.Atoi(strings.TrimSpace(u.custom.Text))
	if err != nil || n < 1 || n > maxCustomSize {
		return 0, fmt.Errorf("custom size must be a number between 1 and %d", maxCustomSize)
	}
	return n, nil
}

func (u *appUI) updateEstimate() {
	if u.estimate == nil || u.data == nil {
		return
	}
	t := TemplateByName(u.tmpl.Selected)
	if t == nil || !u.data.Checked {
		u.estimate.SetText("No data will be inserted.")
		return
	}
	n, err := u.dataSize()
	if err != nil {
		u.estimate.SetText(err.Error())
		return
	}
	est := EstimateRows(t, n)
	if len(t.Schemas) > 0 {
		o := Options{Schema: strings.TrimSpace(u.schema.Text), Template: t}
		est = "Schemas: " + strings.Join(o.SchemaNames(), ", ") + "\n" + est
	}
	u.estimate.SetText(est)
}

func (u *appUI) options() (Options, error) {
	o := Options{
		Conn:         u.conn(),
		Schema:       strings.TrimSpace(u.schema.Text),
		Template:     TemplateByName(u.tmpl.Selected),
		DropExisting: u.drop.Checked,
		ForeignKeys:  u.fks.Checked,
		Indexes:      u.idx.Checked,
		Views:        u.views.Checked,
		Data:         u.data.Checked,
	}
	if err := o.Validate(); err != nil { // options.go, shared with the command line
		return o, err
	}
	if o.Data {
		n, err := u.dataSize()
		if err != nil {
			return o, err
		}
		o.Size = n
		if s := strings.TrimSpace(u.seed.Text); s != "" {
			v, err := strconv.ParseUint(s, 10, 64)
			if err != nil {
				return o, errors.New("seed must be a positive whole number")
			}
			o.Seed = v
		} else {
			o.Seed = rand.Uint64()
		}
	} else {
		o.Size = 100 // only used for the preview text
	}
	return o, nil
}

func (u *appUI) savePrefs() {
	p := u.a.Preferences()
	c := u.conn()
	p.SetString("host", c.Host)
	p.SetString("port", c.Port)
	p.SetString("database", c.Database)
	p.SetString("user", c.User)
	p.SetString("sslmode", c.SSLMode)
	p.SetString("schema", strings.TrimSpace(u.schema.Text))
	// The password is deliberately not stored.
}

// showLogin opens the connection popup. At startup, Cancel quits the app;
// later on, Cancel keeps the current connection settings.
func (u *appUI) showLogin(startup bool) {
	var d *dialog.FormDialog
	d = dialog.NewForm("Database connection", "Connect", u.cancelLabel(startup),
		[]*widget.FormItem{
			widget.NewFormItem("Host", u.host),
			widget.NewFormItem("Port", u.port),
			widget.NewFormItem("Database", u.dbname),
			widget.NewFormItem("User", u.user),
			widget.NewFormItem("Password", u.pass),
			widget.NewFormItem("SSL mode", u.ssl),
		},
		func(ok bool) {
			if !ok {
				if startup {
					u.a.Quit()
					return
				}
				if u.connected {
					u.setConn(u.lastGood)
				}
				return
			}
			u.connect(func() { u.showLogin(startup) })
		}, u.w)
	u.pass.OnSubmitted = func(string) { d.Submit() }
	d.Resize(fyne.NewSize(450, 360))
	d.Show()
	if u.pass.Text == "" {
		u.w.Canvas().Focus(u.pass)
	}
}

func (u *appUI) cancelLabel(startup bool) string {
	if startup {
		return "Quit"
	}
	return "Cancel"
}

// connect tests the settings from the login dialog. On failure it shows the
// error and calls retry, which reopens the login dialog.
func (u *appUI) connect(retry func()) {
	c := u.conn()
	u.log(fmt.Sprintf("Connecting to %s:%s/%s ...", c.Host, c.Port, c.Database))

	bar := widget.NewProgressBarInfinite()
	wait := dialog.NewCustomWithoutButtons("Connecting...",
		container.NewVBox(widget.NewLabel(fmt.Sprintf("%s@%s:%s/%s", c.User, c.Host, c.Port, c.Database)), bar), u.w)
	wait.Show()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		v, err := TestConnection(ctx, c)
		fyne.Do(func() {
			wait.Hide()
			if err != nil {
				u.log("Connection failed: " + err.Error())
				e := dialog.NewError(fmt.Errorf("connection failed:\n%v", err), u.w)
				e.SetOnClosed(retry)
				e.Show()
				return
			}
			u.connected = true
			u.lastGood = c
			u.savePrefs()
			u.connLabel.SetText(fmt.Sprintf("%s@%s:%s/%s  (sslmode=%s)", c.User, c.Host, c.Port, c.Database, c.SSLMode))
			u.log("Connected: " + v)
		})
	}()
}

func (u *appUI) setConn(c ConnSettings) {
	u.host.SetText(c.Host)
	u.port.SetText(c.Port)
	u.dbname.SetText(c.Database)
	u.user.SetText(c.User)
	u.pass.SetText(c.Password)
	u.ssl.SetSelected(c.SSLMode)
}

func (u *appUI) onPreview() {
	o, err := u.options()
	if err != nil {
		dialog.ShowError(err, u.w)
		return
	}
	sql := PreviewSQL(o)
	box := widget.NewMultiLineEntry()
	box.SetText(sql)
	box.TextStyle = fyne.TextStyle{Monospace: true}
	box.Wrapping = fyne.TextWrapOff
	copyBtn := widget.NewButton("Copy to clipboard", func() { u.w.Clipboard().SetContent(sql) })
	d := dialog.NewCustom("SQL preview", "Close", container.NewBorder(nil, copyBtn, nil, nil, box), u.w)
	d.Resize(fyne.NewSize(850, 600))
	d.Show()
}

func (u *appUI) onCreate() {
	if !u.connected {
		u.showLogin(false)
		return
	}
	o, err := u.options()
	if err != nil {
		dialog.ShowError(err, u.w)
		return
	}
	if o.DropExisting {
		dialog.ShowConfirm("Drop schema?",
			fmt.Sprintf("If schema %s exists in database %q it will be dropped,\nincluding ALL objects and data in it.\n\nContinue?",
				quoteList(o.SchemaNames()), o.Conn.Database),
			func(ok bool) {
				if ok {
					u.start(o)
				}
			}, u.w)
		return
	}
	u.start(o)
}

func (u *appUI) setBusy(busy bool) {
	for _, b := range []*widget.Button{u.createBtn, u.previewBtn, u.connBtn} {
		if busy {
			b.Disable()
		} else {
			b.Enable()
		}
	}
	if busy {
		u.cancelBtn.Enable()
	} else {
		u.cancelBtn.Disable()
	}
}

func (u *appUI) start(o Options) {
	u.savePrefs()
	u.setBusy(true)
	u.progress.SetValue(0)
	u.log(fmt.Sprintf("=== Creating %s schema %s (seed %d) ===", o.Template.Name, quoteList(o.SchemaNames()), o.Seed))

	ctx, cancel := context.WithCancel(context.Background())
	u.cancel = cancel

	rep := Reporter{
		Progress: func(frac float64, status string) {
			fyne.Do(func() {
				u.progress.SetValue(frac)
				u.status.SetText(status)
			})
		},
		Log: u.log,
	}

	go func() {
		defer cancel()
		err := Run(ctx, o, rep)
		fyne.Do(func() {
			u.setBusy(false)
			u.cancel = nil
			switch {
			case errors.Is(err, context.Canceled):
				u.status.SetText("Cancelled")
				u.log("Cancelled - all changes were rolled back.")
			case err != nil:
				u.status.SetText("Failed")
				u.log("ERROR: " + err.Error())
				u.log("All changes were rolled back.")
				dialog.ShowError(err, u.w)
			default:
				u.progress.SetValue(1)
				u.status.SetText("Finished")
				dialog.ShowInformation("Done", fmt.Sprintf("Schema %s is ready.", quoteList(o.SchemaNames())), u.w)
			}
		})
	}()
}

// quoteList gives "a", "a, b" or "a, b and c" with each name quoted.
func quoteList(names []string) string {
	q := make([]string, len(names))
	for i, n := range names {
		q[i] = strconv.Quote(n)
	}
	if len(q) <= 1 {
		return strings.Join(q, "")
	}
	return strings.Join(q[:len(q)-1], ", ") + " and " + q[len(q)-1]
}
