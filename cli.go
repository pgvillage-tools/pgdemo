package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Command line: create a demo schema without the window, for example from a
// script or a scheduled job.
//
//	pgdemo create --template <template> --schema <name> [options]
//
// It does exactly what Create does in the window. The password comes from the
// PostgreSQL password file (.pgpass). See README.md, "Command line".

// Exit codes.
const (
	exitOK     = 0
	exitFailed = 1 // creating failed or was stopped; rolled back, nothing was left behind
	exitSetup  = 2 // arguments, password file, connection or a missing extension: nothing was done
)

// isCommandLine: any argument means the command line, never the window; a
// mistyped command gives a message. Without arguments the window opens.
func isCommandLine(args []string) bool {
	return len(args) > 0
}

// runCommandLine handles the command line and returns the exit code.
func runCommandLine(args []string) int {
	attachConsole()
	return cliMain(args, os.Stdout, os.Stderr)
}

func cliMain(args []string, stdout, stderr io.Writer) int {
	switch args[0] {
	case "version", "--version":
		fmt.Fprintln(stdout, "pgdemo "+version)
		return exitOK
	case "templates":
		for _, t := range Templates {
			fmt.Fprintf(stdout, "%-28s %s\n", templateKey(t.Name), t.Name)
		}
		return exitOK
	case "create":
		return cliCreate(args[1:], stdout, stderr)
	case "help", "-h", "-help", "--help":
		fmt.Fprint(stdout, cliUsage)
		return exitOK
	}
	if strings.HasPrefix(args[0], "-") {
		fmt.Fprintf(stderr, "%s: options come after a command, e.g. pgdemo create %s\n", args[0], strings.Join(args, " "))
	} else {
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
	}
	fmt.Fprint(stderr, "usage: pgdemo create --template <template> --schema <name> [options]\n"+
		"       pgdemo templates | help | version\n(pgdemo help for all options)\n")
	return exitSetup
}

const cliUsage = `pgdemo - PostgreSQL Demo Schema Builder

Without arguments the program opens its window. On the command line:

  pgdemo create --template <template> --schema <name> [options]
  pgdemo templates        lists the templates
  pgdemo version          shows the version

create makes the demo schema as Create does in the window, in one transaction:
on an error or an interrupt nothing is left behind.

  --template   template, as listed by "pgdemo templates" (e.g. nibble-secure
               or "Nibble secure"; not case sensitive)
  --schema     schema name (Nibble enterprise: the base name, e.g. nibent)
  --size       small (100), medium (1000, default), large (10000) or a number
               up to 10,000,000
  --seed       a number: the same data every time; default a random seed
               (shown in the output, so a run can be repeated)
  --drop       drop the schema(s) first if they exist (CASCADE); without it,
               an existing schema stops the run
  --no-data    structure only;  --no-fks, --no-indexes, --no-views leave those out
  --preview    print the SQL and stop (no connection, no password needed)

Connection (default: the environment variable, else the window's default):
  --host       PGHOST, else localhost
  --port       PGPORT, else 5432
  --database   PGDATABASE, else postgres
  --user       PGUSER, else postgres
  --sslmode    PGSSLMODE, else prefer (disable, prefer, require, verify-full)

The password comes from the PostgreSQL password file: PGPASSFILE, else ~/.pgpass
(Mac, Linux; chmod 600) or %APPDATA%\postgresql\pgpass.conf (Windows), one line
per connection: host:port:database:user:password

Exit codes:
  0  done
  1  creating failed or was stopped; rolled back, nothing was left behind
  2  arguments, password file, connection or a missing extension (PostGIS);
     nothing was done

Example:
  pgdemo create --host db1 --database lab --template nibble-secure \
         --schema nibble_secure --size 1000 --seed 42 --drop
`

// cliBackend is the database work of create; tests replace it (see cli_pg.go).
type cliBackend struct {
	test func(ctx context.Context, c ConnSettings) (string, error)
	run  func(ctx context.Context, o Options, r Reporter) error
}

// templateKey is the command-line name of a template: "Nibble secure history DE"
// -> "nibble-secure-history-de".
func templateKey(name string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(name)), " ", "-")
}

// findTemplate accepts the key or the name, not case sensitive.
func findTemplate(s string) *Template {
	k := templateKey(strings.ReplaceAll(s, "_", "-"))
	for _, t := range Templates {
		if templateKey(t.Name) == k {
			return t
		}
	}
	return nil
}

// parseSize: small, medium, large or a number.
func parseSize(s string) (int, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "small":
		return 100, nil
	case "medium":
		return 1000, nil
	case "large":
		return 10000, nil
	}
	n, err := strconv.Atoi(strings.ReplaceAll(strings.TrimSpace(s), "_", ""))
	if err != nil || n < 1 || n > maxCustomSize {
		return 0, fmt.Errorf("--size must be small, medium, large or a number between 1 and %d", maxCustomSize)
	}
	return n, nil
}

func envOr(name, def string) string {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		return v
	}
	return def
}

func cliCreate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("create", flag.ContinueOnError)
	fs.SetOutput(stderr)
	tmplArg := fs.String("template", "", "template")
	schema := fs.String("schema", "", "schema name")
	sizeArg := fs.String("size", "medium", "size")
	seedArg := fs.String("seed", "", "seed")
	drop := fs.Bool("drop", false, "drop existing schema(s)")
	noData := fs.Bool("no-data", false, "structure only")
	noFKs := fs.Bool("no-fks", false, "no foreign keys")
	noIdx := fs.Bool("no-indexes", false, "no indexes")
	noViews := fs.Bool("no-views", false, "no example views")
	preview := fs.Bool("preview", false, "print the SQL and stop")
	host := fs.String("host", envOr("PGHOST", "localhost"), "host")
	port := fs.String("port", envOr("PGPORT", "5432"), "port")
	db := fs.String("database", envOr("PGDATABASE", "postgres"), "database")
	user := fs.String("user", envOr("PGUSER", "postgres"), "user")
	ssl := fs.String("sslmode", envOr("PGSSLMODE", "prefer"), "sslmode")
	usage := func(format string, a ...any) int {
		if format != "" {
			fmt.Fprintf(stderr, format+"\n", a...)
		}
		fmt.Fprint(stderr, "usage: pgdemo create --template <template> --schema <name> [options]\n(pgdemo help for more)\n")
		return exitSetup
	}
	if err := fs.Parse(args); err != nil {
		return usage("")
	}
	if fs.NArg() > 0 {
		return usage("unexpected argument %q", fs.Arg(0))
	}
	if *tmplArg == "" || *schema == "" {
		return usage("--template and --schema are required")
	}

	o := Options{
		Conn: ConnSettings{Host: strings.TrimSpace(*host), Port: strings.TrimSpace(*port),
			Database: strings.TrimSpace(*db), User: strings.TrimSpace(*user), SSLMode: strings.TrimSpace(*ssl)},
		Schema:       strings.TrimSpace(*schema),
		Template:     findTemplate(*tmplArg),
		DropExisting: *drop,
		ForeignKeys:  !*noFKs,
		Indexes:      !*noIdx,
		Views:        !*noViews,
		Data:         !*noData,
		Size:         100, // only used for the preview text when there is no data
	}
	if o.Template == nil {
		return usage("unknown template %q: see pgdemo templates", *tmplArg)
	}
	switch o.Conn.SSLMode {
	case "disable", "allow", "prefer", "require", "verify-ca", "verify-full":
	default:
		return usage("unknown --sslmode %q (disable, prefer, require, verify-full)", o.Conn.SSLMode)
	}
	if o.Data {
		n, err := parseSize(*sizeArg)
		if err != nil {
			return usage("%v", err)
		}
		o.Size = n
		if s := strings.TrimSpace(*seedArg); s != "" {
			v, err := strconv.ParseUint(s, 10, 64)
			if err != nil {
				return usage("--seed must be a positive whole number")
			}
			o.Seed = v
		} else {
			o.Seed = rand.Uint64() % 1_000_000_000
		}
	}
	if err := o.Validate(); err != nil {
		return usage("%v", err)
	}

	if *preview {
		fmt.Fprint(stdout, PreviewSQL(o))
		return exitOK
	}
	return cliRun(o, stdout)
}

// say writes a timestamped line.
func say(w io.Writer, format string, a ...any) {
	fmt.Fprintf(w, "%s  %s\n", time.Now().Format("15:04:05"), fmt.Sprintf(format, a...))
}

func cliRun(o Options, out io.Writer) int {
	fail := func(code int, format string, a ...any) int {
		say(out, "STOPPED: "+format, a...)
		say(out, "Exit code %d: %s", code, exitText(code))
		return code
	}
	say(out, "pgdemo %s: create %s schema %s in %s", version, o.Template.Name,
		strings.Join(o.SchemaNames(), ", "), connText(o.Conn))

	entries, err := readPassfile(passfilePath())
	if err != nil {
		return fail(exitSetup, "%v", err)
	}
	pw, ok := findPassword(entries, o.Conn)
	if !ok {
		return fail(exitSetup, "no line in %s for %s:%s/%s user %s (host:port:database:user:password)",
			passfilePath(), o.Conn.Host, portOr5432(o.Conn.Port), o.Conn.Database, o.Conn.User)
	}
	o.Conn.Password = pw

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	tctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	v, err := cliDB.test(tctx, o.Conn)
	cancel()
	if err != nil {
		return fail(exitSetup, "connection failed: %v", err)
	}
	say(out, "Connected: %s", v)
	if o.Data {
		say(out, "Size %d, seed %d (the same seed and size give the same data)", o.Size, o.Seed)
	} else {
		say(out, "Structure only, no data")
	}
	if o.DropExisting {
		say(out, "--drop: existing schema(s) %s are dropped first", strings.Join(o.SchemaNames(), ", "))
	}

	start := time.Now()
	err = cliDB.run(ctx, o, Reporter{
		Progress: func(float64, string) {},
		Log:      func(msg string) { say(out, "%s", msg) },
	})
	switch {
	case err != nil && (ctx.Err() != nil || errors.Is(err, context.Canceled)):
		return fail(exitFailed, "stopped (interrupt); everything was rolled back")
	case errors.As(err, new(*ExtensionError)):
		return fail(exitSetup, "%v; nothing was created", err)
	case err != nil && strings.Contains(err.Error(), "Drop existing schema"):
		// the window's wording; on the command line it is --drop
		msg, _, _ := strings.Cut(err.Error(), ":")
		return fail(exitFailed, "%s: use --drop to replace it, or another --schema; nothing was changed", msg)
	case err != nil:
		return fail(exitFailed, "%v; everything was rolled back", err)
	}
	say(out, "Schema %s is ready (%s)", strings.Join(o.SchemaNames(), ", "), time.Since(start).Round(time.Millisecond))
	say(out, "Exit code 0: %s", exitText(exitOK))
	return exitOK
}

func connText(c ConnSettings) string {
	return fmt.Sprintf("%s@%s:%s/%s (sslmode=%s)", c.User, c.Host, portOr5432(c.Port), c.Database, c.SSLMode)
}

func exitText(code int) string {
	switch code {
	case exitOK:
		return "done"
	case exitFailed:
		return "creating failed or was stopped; nothing was left behind"
	}
	return "arguments, password file, connection or a missing extension; nothing was done"
}

func portOr5432(p string) string {
	if p == "" {
		return "5432"
	}
	return p
}
