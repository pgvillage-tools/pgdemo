package main

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type ConnSettings struct {
	Host, Port, Database, User, Password, SSLMode string
}

func (c ConnSettings) URL() string {
	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(c.User, c.Password),
		Host:   net.JoinHostPort(c.Host, c.Port),
		Path:   "/" + c.Database,
	}
	q := url.Values{}
	q.Set("sslmode", c.SSLMode)
	q.Set("application_name", "pgdemo")
	q.Set("connect_timeout", "10")
	u.RawQuery = q.Encode()
	return u.String()
}

func TestConnection(ctx context.Context, c ConnSettings) (string, error) {
	conn, err := pgx.Connect(ctx, c.URL())
	if err != nil {
		return "", err
	}
	defer conn.Close(context.Background())
	var v string
	err = conn.QueryRow(ctx, "SELECT version()").Scan(&v)
	return v, err
}

// Options holds every choice made in the GUI.
type Options struct {
	Conn         ConnSettings
	Schema       string
	Template     *Template
	DropExisting bool
	ForeignKeys  bool
	Indexes      bool
	Views        bool
	Data         bool
	Size         int    // base number of rows
	Seed         uint64 // same seed = same data
}

// Reporter lets Run talk to the GUI without knowing about Fyne.
type Reporter struct {
	Progress func(frac float64, status string) // frequent, for the progress bar
	Log      func(msg string)                  // important steps only
}

func (o Options) schemaIdent() string { return pgx.Identifier{o.SchemaName("")}.Sanitize() }

// SchemaName is the name of the main schema ("") or of an extra schema (suffix).
func (o Options) SchemaName(suffix string) string {
	if suffix == "" {
		if o.Template != nil && o.Template.MainSuffix != "" {
			return o.Schema + "_" + o.Template.MainSuffix
		}
		return o.Schema
	}
	return o.Schema + "_" + suffix
}

// SchemaNames lists every schema the template creates, the main one first.
func (o Options) SchemaNames() []string {
	names := []string{o.SchemaName("")}
	if o.Template != nil {
		for _, s := range o.Template.Schemas {
			names = append(names, o.SchemaName(s))
		}
	}
	return names
}

func identOf(name string) string { return pgx.Identifier{name}.Sanitize() }

func (o Options) fill(sql string) string {
	if o.Template != nil {
		for _, s := range o.Template.Schemas {
			sql = strings.ReplaceAll(sql, "{s_"+s+"}", identOf(o.SchemaName(s)))
		}
	}
	return strings.ReplaceAll(sql, "{s}", o.schemaIdent())
}

func (o Options) createStatements() []string {
	out := make([]string, 0, len(o.Template.Tables))
	for _, t := range o.Template.Tables {
		out = append(out, o.fill(fmt.Sprintf("CREATE TABLE %s.%s (\n    %s\n)", schemaRef(t.Schema), t.Name, t.DDL)))
	}
	return out
}

// postStatements run after the data load (faster than loading into constrained tables).
func (o Options) postStatements() []string {
	var out []string
	for _, t := range o.Template.Tables {
		if o.ForeignKeys {
			for _, s := range t.FKs {
				out = append(out, o.fill(s))
			}
		}
		if o.Indexes {
			for _, s := range t.Indexes {
				out = append(out, o.fill(s))
			}
		}
	}
	if o.Views {
		for _, s := range o.Template.Views {
			out = append(out, o.fill(s))
		}
	}
	return out
}

// renameStatements give the personal data columns their local names (country
// templates). They run last, so the foreign keys, indexes and views are made
// with the original names and PostgreSQL renames them along.
func (o Options) renameStatements() []string {
	var out []string
	for _, r := range o.Template.Renames {
		out = append(out, o.fill(fmt.Sprintf("ALTER TABLE %s.%s RENAME COLUMN %s TO %s",
			schemaRef(r.Schema), r.Table, r.Column, r.To)))
	}
	return out
}

// PreviewSQL returns the DDL that Run would execute (without the data).
func PreviewSQL(o Options) string {
	var b strings.Builder
	fmt.Fprintf(&b, "-- Demo schema: %s (%s)\n\n", strings.Join(o.SchemaNames(), ", "), o.Template.Name)
	for _, name := range o.SchemaNames() {
		if o.DropExisting {
			fmt.Fprintf(&b, "DROP SCHEMA IF EXISTS %s CASCADE;\n", identOf(name))
		}
		fmt.Fprintf(&b, "CREATE SCHEMA %s;\n\n", identOf(name))
	}
	for _, s := range o.createStatements() {
		b.WriteString(s + ";\n\n")
	}
	if o.Data {
		fmt.Fprintf(&b, "-- (data is loaded here with COPY: %s)\n\n",
			strings.ReplaceAll(EstimateRows(o.Template, o.Size), "\n", ", "))
	}
	for _, s := range o.postStatements() {
		b.WriteString(s + ";\n\n")
	}
	if rs := o.renameStatements(); len(rs) > 0 {
		b.WriteString("-- local column names\n")
		for _, s := range rs {
			b.WriteString(s + ";\n")
		}
	}
	return b.String()
}

// tableName is the table name for messages: with its schema when the template
// has more than one schema.
func (o Options) tableName(t Table) string {
	if len(o.Template.Schemas) == 0 {
		return t.Name
	}
	return o.SchemaName(t.Schema) + "." + t.Name
}

func firstLine(s string) string {
	s, _, _ = strings.Cut(s, "\n")
	return s
}

// Run creates the demo schema. Everything happens in one transaction:
// on an error or cancel nothing is left behind.
func Run(ctx context.Context, o Options, r Reporter) error {
	start := time.Now()
	creates := o.createStatements()
	post := o.postStatements()
	tables := o.Template.Tables

	renames := o.renameStatements()
	total := 1 + len(o.SchemaNames()) + len(creates) + len(post)
	if len(renames) > 0 {
		total++
	}
	if o.Data {
		total += 2 * len(tables)
	}
	done := 0
	step := func(msg string) {
		done++
		r.Progress(float64(done)/float64(total), msg)
		r.Log(msg)
	}
	r.Progress(0, "Connecting...")
	conn, err := pgx.Connect(ctx, o.Conn.URL())
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer conn.Close(context.Background())

	t, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer t.Rollback(context.Background()) // no-op after a successful commit
	run := func(sql string) error {
		if _, err := t.Exec(ctx, sql); err != nil {
			return fmt.Errorf("%s\n-> %w", firstLine(sql), err)
		}
		return nil
	}

	// More memory for index builds and foreign key checks, and more parallel
	// workers per index build (this transaction only, no special rights needed;
	// the server's own limits still apply).
	for _, set := range []string{
		"SET LOCAL maintenance_work_mem = '1GB'",
		"SET LOCAL work_mem = '256MB'",
		"SET LOCAL max_parallel_maintenance_workers = 4",
	} {
		if err := run(set); err != nil {
			return err
		}
	}

	for _, name := range o.SchemaNames() {
		var exists bool
		if err := t.QueryRow(ctx,
			"SELECT EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = $1)", name).Scan(&exists); err != nil {
			return err
		}
		if exists && !o.DropExisting {
			return fmt.Errorf("schema %q already exists: tick \"Drop existing schema\" or choose another name", name)
		}
		if exists {
			if err := run("DROP SCHEMA " + identOf(name) + " CASCADE"); err != nil {
				return err
			}
			r.Log("Dropped existing schema " + name)
		}
	}
	for _, name := range o.SchemaNames() {
		if err := run("CREATE SCHEMA " + identOf(name)); err != nil {
			return err
		}
		step("Created schema " + name)
	}

	for i, sql := range creates {
		if err := run(sql); err != nil {
			return err
		}
		step("Created table " + o.tableName(tables[i]))
	}

	if o.Data {
		g := NewGen(o.Size, o.Seed)
		g.loc = o.Template.Locale
		for _, tb := range tables {
			count := tb.Rows(o.Size)
			base := done
			st, err := copyFreeze(ctx, t, o.SchemaName(tb.Schema), tb.Name, tb.Cols, count,
				func(k int) []any { return tb.Row(g, k) },
				func(n int) {
					r.Progress((float64(base)+float64(n)/float64(count))/float64(total),
						fmt.Sprintf("%s: %d / %d rows", o.tableName(tb), n, count))
				})
			if err != nil {
				return fmt.Errorf("loading %s: %w", o.tableName(tb), err)
			}
			step(fmt.Sprintf("Inserted %d rows into %s in %s", count, o.tableName(tb), st))
		}
	}

	for _, sql := range post {
		if err := run(sql); err != nil {
			return err
		}
		step(firstLine(sql))
	}
	for _, sql := range renames {
		if err := run(sql); err != nil {
			return err
		}
	}
	if len(renames) > 0 {
		step(fmt.Sprintf("Gave %d personal data columns their local names", len(renames)))
	}

	if o.Data {
		for _, tb := range tables {
			full := identOf(o.SchemaName(tb.Schema)) + "." + tb.Name
			if tb.HasID {
				// Make the identity continue after the generated ids.
				_, err := t.Exec(ctx, fmt.Sprintf(
					"SELECT setval(pg_get_serial_sequence($1, 'id'), (SELECT coalesce(max(id), 0) + 1 FROM %s), false)", full), full)
				if err != nil {
					return fmt.Errorf("reset sequence %s: %w", o.tableName(tb), err)
				}
			}
			// Statistics for the planner. No VACUUM needed: COPY FREEZE wrote the
			// rows frozen and the pages all-visible.
			if err := run("ANALYZE " + full); err != nil {
				return err
			}
			step("Analyzed " + o.tableName(tb))
		}
	}

	if err := t.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	step(fmt.Sprintf("Done in %s", time.Since(start).Round(time.Millisecond)))
	return nil
}
