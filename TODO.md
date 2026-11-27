# TODO – pgdemo

Status of 2026-11-26. pgTestBuilder has its own TODO (`pgTestBuilder/TODO.md` on the stick).

## Open

None.

**Next release: pgdemo 1.0.5** (`build/VERSION` is 1.0.5, work started 2026-11-26;
1.0.4 is released in `releases/pgdemo/1.0.4/`). In it so far: nothing yet.

**Released: pgdemo 1.0.4** (frozen 2026-11-26, `releases/pgdemo/1.0.4/`, see
`RELEASE-1.0.4.md`). Packages built by Robert on 2026-11-26 and in the release folder with
checksums (all OK). Checked: the Linux package unpacks without warnings and has no macOS file
attributes; Info.plist and FIRST_START.txt say 1.0.4 and FIRST_START.txt lists Nibble mini and
PostGIS municipality; the README in the packages has both templates and "Sizes and row counts";
all three programs contain the two templates, the PostGIS check, the About window and the
version 1.0.4 as a value of their own.
**Still to check on a database (known points of 1.0.4):** PostGIS municipality on the new
server with PostGIS 3.4; Nibble mini with "Parent only" in pgTestBuilder. Contents of 1.0.4:
- **README: "Sizes and row counts"** (2026-11-20): total rows per template for Small, Medium,
  Large, 100,000, 1,000,000 and 10,000,000, and the rows per unit of size; the counts depend only
  on template and size, not on the seed; country versions the same as the Dutch templates.
  Numbers generated from the template code (not typed).
- **New template "PostGIS municipality"** (built 2026-11-25, `postgis.go`; NL only; for PostGIS
  3.4, later also 3.6.2): a fictional municipality (Nibbelstad, 9 × 9 km) in RD New
  (EPSG:28992): `districts`, `streets`, `buildings`, `addresses`, `owners` (BSN), `parcels`,
  `parks`, `water`, `sensors`, `sensor_readings`, `vehicles`, `gps_tracks`; GiST indexes; views
  `district_overview`, `address_nearest_sensor`, `vehicle_kilometres`. Random but valid
  geometry inside the municipality; address in its building, building in its parcel and
  district. About 7.1 rows per unit (Medium 7,136).
  New in `db.go`: `Template.Extensions`; before anything is made, pgdemo checks that the
  extension is installed in the database (and PostGIS is 3.0 or newer), logs its version and
  schema and puts that schema on the search path of the transaction; otherwise it stops with
  what to do (CREATE EXTENSION by a superuser, or the PostGIS package on the server). Command
  line: exit code 2 for a missing extension. Preview: a note that PostGIS is needed.
  Tested in the sandbox with Go 1.24 (pgx and Fyne replaced by compile-only stand-ins, so
  `db.go` is type-checked): valid EWKT, closed simple rings, everything inside the municipality,
  the containments, foreign key values, unique parcel numbers / plates / BSN, readings in range,
  the extension check (installed, other schema, available but not installed, not on the
  server, too old), CLI exit code 2 and the preview; the other templates unchanged.
  Still to check by Robert on the new server with PostGIS 3.4: `go build`, create the
  template (window and command line), the views, and once without PostGIS (the message).
- **New template "Nibble mini"** (built 2026-11-26, `nibble_mini.go`; agreed design): a smaller
  Nibble (11 tables) for testing the table class "Parent only" in pgTestBuilder (1.4.1).
  `addresses` is a customer's address book (`customer_id`) that orders (delivery address),
  shipments, invoices, employees, suppliers and warehouses also point to; 80% of the orders to
  an own address, 20% to another customer's address; shipments to the order's delivery address,
  invoices to the customer's own address. On `addresses` also `created_by` (employee id, no
  foreign key: a virtual FK on a Parent only table). About 20.8 rows per unit (Medium 20,770). NL only. Adjusted the same day on Robert's
  request: without the view `address_usage` and the tables `payments`, `regions` and
  `countries` (and so without `addresses.region_id`); 14 -> 11 tables. Later that day also
  without `addresses.previous_address_id` (the self reference).
  Tested in the sandbox: every foreign key value exists, unique columns, the share of orders to
  another customer's address (~19% at Medium), shipments and invoices to the right address,
  no foreign key on `created_by`; the other templates unchanged.
  To check by Robert: `go build`, create it, then in pgTestBuilder customers driving with
  addresses on Follow and on Parent only.
- **About window** (built 2026-11-26; as in QNibbler and pgTestBuilder): menu Help → About pgdemo
  (macOS menu bar; a menu row on Windows and Linux). Icon, "pgdemo <version>", "Nibble-IT – demo
  schemas with generated data for PostgreSQL", the Nibbler animation
  (`internal/nibbler/nibbler.go` copied unchanged from pgTestBuilder), "Coded with love by
  Robert de Laat – www.nibble-it.nl", and in small print: the connection and server (or "Not
  connected"), the number of templates, the command line, "the connection is remembered without
  the password". New: `ui_about.go`; `ui.go` keeps the server version of the connection;
  `main.go` sets the menu. Checked in the sandbox: the animation package builds (also for
  Windows) and draws its frames; `ui_about.go` uses the same Fyne calls as pgTestBuilder's
  (both Fyne 2.8.2) but could not be compiled here (no Fyne).
  Tested by Robert 2026-11-26: builds, works and looks good.
  The source archive includes the folder `internal/` (done for 1.0.4; keep it in later freezes).

**Released: pgdemo 1.0.3** (frozen 2026-11-19, `releases/pgdemo/1.0.3/`, see
`RELEASE-1.0.3.md`). Packages built by Robert on 2026-11-19 and in the release folder with
checksums (all OK). Checked: the Linux package unpacks without warnings and has no macOS file
attributes; Info.plist and FIRST_START.txt say 1.0.3 and FIRST_START.txt mentions the command
line; the docs have the README section "Command line"; all three programs contain the command
line and the version 1.0.3 as a value of their own. Contents of 1.0.3:
- **Command line** (built 2026-11-19): `pgdemo create --template <template> --schema <name>`
  with `--size`, `--seed`, `--drop`, `--no-data`, `--no-fks`, `--no-indexes`, `--no-views`,
  `--preview` (SQL only) and the connection (`--host`, `--port`, `--database`, `--user`,
  `--sslmode`, else PGHOST etc., else the window's defaults); `pgdemo templates`, `help`,
  `version`. Password only from .pgpass (as pgTestBuilder). Exit codes 0 done, 1 failed or
  stopped (rolled back; also an existing schema without `--drop`), 2 arguments, password file or
  connection (nothing done). Output: timestamped lines on stdout; Windows writes to the command
  prompt (`start /wait` in batch files). Files: `cli.go`, `cli_pg.go`, `options.go` (the name
  checks, now shared with the window), `passfile.go` (from pgTestBuilder), `console_*.go`;
  `main.go` and `ui.go` changed; README section "Command line".
  Tested in the sandbox with Go 1.24 without pgx / Fyne (fake database): arguments, templates,
  sizes, seed, preview, .pgpass (missing, not private, no line), connection error, existing
  schema, failure, interrupt, exit codes, the shared name checks; also compiled for Windows and
  macOS. Not compiled: `ui.go` and `main.go` (need Fyne).
  Changed after Robert's finding (2026-11-19): any argument means the command line, never the
  window; options without a command or an unknown command give a message, the usage lines and
  exit code 2.
  Tested by Robert 2026-11-19 on the Mac against 192.168.178.140 (PostgreSQL 18.6): `go build`,
  `templates`, `--preview` (also without a password line), create Webshop (exit 0), the same
  again (exit 1, "use --drop"), with `--drop` (exit 0), database without a .pgpass line
  (exit 2), options without `create` (message, no window), and the window: Create with schema
  name `Demo` gives the name message.

## Parked

- **More countries** (parked 2026-11-18: not needed now, maybe in the future)
  E.g. FR, ES, SE, in the same way as BE, DE and UK (names, addresses, phone numbers, IBAN,
  plates, national ID, local column names).

- **Oracle version as a separate program** (idea, discussed 2026-11-19; may or may not be built)
  A demo schema builder for Oracle, as its own program (e.g. "orademo") with its own releases,
  not a database choice in pgdemo. Its tables and columns may differ from pgdemo's: own Oracle
  templates in Oracle style (VARCHAR2, NUMBER, JSON), no translation of the PostgreSQL DDL.
  Reuse: the data generators (names, addresses, valid national IDs, IBAN, plates, BE/DE/UK,
  seed and size) and the Fyne window; this needs the generator code in its own Go package
  (now everything is `package main`). Points of attention: driver go-ora (pure Go, no Oracle
  client; godror needs Instant Client and cgo); DDL commits at once in Oracle, so no "one
  transaction" and the program must clean up after an error; an empty string is NULL; a schema
  is a user (DBA rights to create one, or load into an existing schema); no COPY, so bulk
  inserts with array binding (slower). Start with one small template end to end. Test database:
  Oracle Database 23ai Free in Podman.

## Releases

**Released: pgdemo 1.0.1** (frozen 2026-11-18, `releases/pgdemo/1.0.1/`, see
`RELEASE-1.0.1.md`). Packages built by Robert on 2026-11-18 and in the release folder with
checksums (all OK). Checked: the Linux package unpacks without warnings and has no macOS file
attributes; the packages say 1.0.1 (Info.plist, FIRST_START.txt).
**Known fault:** the Linux and Windows packages show "1.0.2 (dev build)" in the window title
(Linux checked by Robert 2026-11-18). fyne-cross does not pass `-ldflags "-X main.version=..."`
on to the Go build, so those programs take `build/VERSION` of the folder they were built from.
The macOS package is right (native go build). Same fault in 1.0.0 (title "dev").

**Released: pgdemo 1.0.2** (frozen 2026-11-19, `releases/pgdemo/1.0.2/`, see
`RELEASE-1.0.2.md`). Packages built by Robert on 2026-11-19 and in the release folder with
checksums (all OK). Checked: the Linux package unpacks without warnings and has no macOS file
attributes; Info.plist and FIRST_START.txt say 1.0.2 and list Nibble secure history; the
Linux and Windows programs now contain the version 1.0.2 as a value of their own (in 1.0.1
only the build/VERSION text was in them), so the version fix is in. Window title checked by
Robert 2026-11-19 on Windows (ARM VM): "PostgreSQL Demo Schema Builder 1.0.2", no "(dev build)".
Linux is built the same way (fyne-cross).
Contents of 1.0.2:
- `build/package.sh`: for the fyne-cross builds (Linux, Windows) the version is written into
  the copied `version.go` (`var version = "<version>"`) instead of relying on `-ldflags`.
  Tested on a copy of `version.go` and in the 1.0.2 packages (see above).
- **New template "Nibble secure history"** (2026-11-18, `nibble_history.go`, also BE / DE / UK):
  Nibble secure (same data for the same seed and size) plus `his_persons`, `his_customers`,
  `his_customer_contacts`, `his_employees`, `his_suppliers` with earlier versions (0-3 per row,
  `version_no`, `valid_from`, `valid_to`, `change_reason`); standalone, no foreign keys.
  Tested in the sandbox with Go 1.24 (template code only, no database): same Nibble secure data,
  history rows match the current BSN / numbers, periods without gaps, local column names.
  Tested by Robert 2026-11-18: built and run on the Mac (`go run .`), schema created on the
  server with the 5 `his_` tables, no foreign keys on them (checked in pg_constraint).
  On purpose: the versions of one row are not consistent with each other or with the current row
  (every version has its own random values); the `his_` tables are for demos, e.g. ignoring
  tables in pgTestBuilder (decided by Robert 2026-11-18).

## Done and tested

- **Release pgdemo 1.0.4** (2026-11-26): templates PostGIS municipality and Nibble mini, the
  About window, README "Sizes and row counts"; sources, packages and checksums in
  `releases/pgdemo/1.0.4/`.
- **Release pgdemo 1.0.3** (2026-11-19): the command line (`pgdemo create`, `templates`, `help`,
  `version`); sources, packages and checksums in `releases/pgdemo/1.0.3/`.
- **Release pgdemo 1.0.2** (2026-11-19): template Nibble secure history (BE / DE / UK), the
  version in the Linux and Windows packages; sources, packages and checksums in
  `releases/pgdemo/1.0.2/`.
- **Release pgdemo 1.0.1** (2026-11-18): "(dev build)" title for plain builds, Linux package
  without macOS file attributes; sources, packages and checksums in `releases/pgdemo/1.0.1/`.
- **Release pgdemo 1.0.0** (2026-11-17): sources, packages and checksums in
  `releases/pgdemo/1.0.0/` (moved there 2026-11-18 from `releases/1.0.0/`, which now holds only
  pgTestBuilder 1.0.0; `RELEASE-1.0.0.md` is in both, unchanged, and still names the old paths).
- **Project folder tidied** (2026-11-18): removed `main.go.bak`, `ui.go.bak`, the old compiled
  `pgdemo` (16 Nov) and the outdated `README.pdf` (10 Nov).
