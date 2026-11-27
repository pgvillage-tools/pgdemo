# PostgreSQL Demo Schema Builder (Go + Fyne)

Desktop app that creates a demo schema in a PostgreSQL database.

## Choices in the app

- **Help → About**: version, the Nibbler animation and the current connection (on macOS in the menu bar).
- **Connection**: a login popup at startup asks for host, port, database, user, password and SSL mode. Click **Change connection...** at the top of the window to switch to a different database later.
- **Schema name**: for example `demo`.
- **Schema type**:
  - Webshop: categories, products, customers, orders, order_items
  - HR: departments, employees (with managers), projects, employee_projects
  - Library: authors, books, members, loans
  - School: teachers, courses, students, enrollments
  - Nibble: 36 tables and 81 foreign keys, built to test partial (percentage) schema copies.
    It has composite and natural keys, FKs to UNIQUE (non-PK) columns, 9 self references, a cycle
    (departments <-> employees, deferrable), M:N junctions, CASCADE / SET NULL, skewed data,
    a polymorphic table without FKs (attachments) and a table without relations (app_settings).
    The view `fk_graph` lists every foreign key in the schema. Code: `nibble.go`.
  - Nibble mini: a smaller Nibble (11 tables) for testing the table class **Parent only** in
    pgTestBuilder. Its centre is `addresses`: a customer's address book (`addresses.customer_id`)
    that `orders` (delivery address), `shipments`, `invoices`, `employees`, `suppliers` and
    `warehouses` also point to. 80% of the orders go to an address of their own customer, 20% to
    an address of another customer (a gift, a drop shipment); shipments go to the delivery
    address of their order, invoices to the customer's own address.
    - With `customers` as driving table and `addresses` on Follow, the selection goes down to the
      addresses of the driving customers and from there to every order, shipment and invoice to
      those addresses, also of other customers, and keeps growing. With `addresses` on Parent
      only, addresses only come as the address of selected rows, with their own parent (the
      owner).
    - Also on `addresses`: `created_by`, an employee id without a foreign key (a virtual FK on a
      table that is Parent only). Lookup: `departments`. No views. Code: `nibble_mini.go`.
  - Nibble secure: Nibble plus sensitive data, for testing data masking (pgTestBuilder). Use schema
    name `nibble_secure`. Adds `persons` with the BSN (valid 11-check) as natural primary key,
    referenced by `customer_contacts.person_bsn` and `employees.bsn` (UNIQUE), and `vehicles.driver_bsn`
    referencing `employees(bsn)`. Also IBAN (valid mod-97) on customers, suppliers, employees and
    persons, birth dates, password hashes, `payment_cards` (valid Luhn numbers), `vehicles` (Dutch
    license plates), `login_events` (inet IP addresses, IPv4 and IPv6), personal data in JSONB
    (`customer_contacts.details`) and in free text (`ticket_comments.body`). `customer_notes`
    (note_id, customer_id, notes) and `employee_notes` (note_id, employee_id, notes) have free
    text notes, some over several lines, and **no foreign key**: customer_id / employee_id refer
    to customers(id) / employees(id) by convention only, a case for virtual FKs in pgTestBuilder
    (it suggests them). All values are generated.
    Code: `nibble_secure.go`.
  - Nibble secure history: Nibble secure (the same data for the same seed and size) plus five
    **history tables** with the earlier versions of the rows with personal data: `his_persons`,
    `his_customers`, `his_customer_contacts`, `his_employees` and `his_suppliers`. The current
    version stays in the original table; a `his_` table has the versions before it (0 to 3 per
    row, 1.5 on average), each with `version_no`, `valid_from`, `valid_to` (the day the next
    version started, no gaps, all before 2025) and `change_reason` (marriage, moved, new bank
    account, promotion, ...). They hold personal data that is no longer in the current table: old
    names, e-mail addresses, phone numbers, IBANs, addresses and salaries. The `his_` tables are
    **standalone: no foreign keys**. They refer to the current rows by value only (`bsn`,
    `customer_id`, `contact_id`, `employee_id`, `supplier_id`, and `changed_by` to an employee),
    with the same BSN, customer, employee and supplier numbers as the current rows.
    Also as BE / DE / UK (see below). Code: `nibble_history.go`.
  - Nibble enterprise: **three schemas** of one application, with foreign keys between them.
    You enter one name (e.g. `nibent`) and get:
    - `nibent_core`: Nibble secure, unchanged (the same data as Nibble secure for the same seed and size);
    - `nibent_hr` (18 tables): job titles, salary scales, contracts, position history, leave types,
      balances and requests, absences with **medical notes** (health data), **attendance** (one row
      per employee per working day of 2025), overtime, **payslips** for every month of 2025 with
      BSN and IBAN, payslip lines, emergency contacts, ID documents, performance reviews, trainings,
      and `hr_notes` (no FK: employee_id refers to `nibent_core.employees(id)` by convention);
    - `nibent_fleet` (13 tables): lease companies, car models, lease categories (per salary scale),
      cars (unique licence plates and VINs), lease contracts, car assignments, fuel cards and fuel
      transactions (weekly), the **trip log** (about 200 trips per car, with addresses), damages,
      traffic fines (no FK: `license_plate` refers to `cars(license_plate)` by convention; about 5%
      are plates not in the fleet), maintenance and private-use declarations with BSN.

    Foreign keys between the schemas: hr → core (employees, departments, `persons(bsn)` as natural
    key), fleet → core (employees, vehicles, `persons(bsn)`), fleet → hr (salary scales, contracts)
    and hr → fleet (`payslip_lines.lease_contract_id`), so hr and fleet refer to each other.
    The data is consistent across the schemas: employees whose number ends in 1, 4 or 7 drive a
    lease car (30%), their lease category matches the salary scale of their HR contract, and their
    payslips have a "private use company car" line pointing to their lease contract. On a payslip,
    net = salary − pension − wage tax + travel + other allowances.
    Size: about 142 rows per unit (Medium: 142,000; size 1,000,000: about 142 million, of which
    52 million in the core, 74 million in hr and 16 million in fleet).
    Drop existing schema drops all three. Code: `nibble_enterprise.go`.
  - PostGIS municipality: the geography of a fictional Dutch municipality (Nibbelstad, 9 × 9 km)
    in RD New (**EPSG:28992**, metres). Needs **PostGIS 3** in the database (made for 3.4).
    - Tables with geometry: `districts` (6 × 6 grid of neighbourhood polygons), `streets`
      (linestrings with name, type and speed limit), `buildings` (footprint polygons with year of
      construction, use and floors), `addresses` (points with street, number, postcode),
      `parcels` (cadastral parcels with parcel number, area and owner), `parks` and `water`
      (polygons), `sensors` (air quality, noise and traffic points) and `gps_tracks` (routes of
      municipal vehicles as linestrings). Without geometry: `owners` (persons with BSN, name,
      birth date, e-mail, phone, IBAN), `sensor_readings` (48 hourly values per sensor with a
      daily rhythm) and `vehicles` (licence plate, driver).
    - The geometry is random but always valid and inside the municipality. Every address lies
      inside its building, every building inside its parcel and inside the district of its
      `district_id`, so `ST_Within` / `ST_Contains` find real matches.
    - Normal foreign keys (building → district, street; address → building; parcel → building,
      owner; reading → sensor; track → vehicle) and a **GiST index** on every geometry column.
      Views: `district_overview` (buildings, addresses and sensors per district with
      `ST_Contains`), `address_nearest_sensor` (the nearest active sensor per address with the
      `<->` operator) and `vehicle_kilometres` (`ST_Length` of the tracks).
    - pgdemo checks first that PostGIS is installed in the database (and is 3.0 or newer), and
      uses the schema it is installed in. If it is missing, nothing is created and the message
      says what to do: a superuser runs `CREATE EXTENSION postgis;` in that database, or the
      server first needs the PostGIS package. NL only. Code: `postgis.go`.
  - Nibble secure BE / DE / UK, Nibble secure history BE / DE / UK and Nibble enterprise BE / DE / UK: the same tables, keys and rows
    as the Dutch templates, with the personal data of another country and **local column names**
    for the personal data columns (table names stay English). Code: `locale.go`.
    - Values: names, streets, postcodes, towns, mobile numbers, IBAN (valid mod-97, and for BE the
      national check), licence plates (unique ones for the fleet) and the national identification
      number instead of the BSN, unique and valid: BE rijksregisternummer (11 digits, mod-97 check),
      DE Steuer-ID (11 digits, ISO 7064 check digit, one digit twice), UK National Insurance number
      (AB123456C). The ID columns are `char(11)` (BE, DE) or `char(9)` (UK) with a matching CHECK.
    - Column names: DE German (`steuer_id`, `vorname`, `nachname`, `strasse`, `plz`, `ort`,
      `kennzeichen`, `notizen`, ...); UK British (`ni_number`, `forename`, `surname`,
      `address_line_1`, `town`, `registration_plate`, ...); BE mixed like many Belgian systems:
      the customer side (addresses, customers, contacts, payment cards, notes) and the fleet schema
      in French (`prenom`, `nom_de_famille`, `rue`, `code_postal`, `localite`, `numero_national`,
      `plaque`, ...), the employees, persons and HR in Dutch (`voornaam`, `achternaam`,
      `rijksregisternummer`, `nummerplaat`, `opmerkingen`, ...). So `customer_contacts.numero_national`
      refers to `persons.rijksregisternummer`.
    - How: the tables are created and loaded with the original names; at the end of the
      transaction `ALTER TABLE ... RENAME COLUMN` gives the local names. PostgreSQL renames the
      columns in the foreign keys, indexes and views along. Preview SQL lists the renames at the end.
    - The free text (notes, descriptions) stays English, with the local values inside.
    - The Dutch templates are unchanged: the same data for the same seed and size.
- **Drop existing schema** (asks for confirmation; `public` is never dropped).
- **Foreign keys / Indexes / Example views**: on or off.
- **Insert demo data**: yes or no. Size can be Small (100), Medium (1,000), Large (10,000) or Custom (up to 10,000,000). The number of rows per template and size: see "Sizes and row counts".
- **Seed**: leave empty for random data. Enter a number to get the same data every time.
- **Preview SQL**: shows the DDL before you run anything.

Everything runs in one transaction. If there's an error or you click Cancel, nothing is left behind.
Data is loaded with `COPY`, which is fast. Foreign keys and indexes are added after the load.
Data is written with `COPY ... WITH (FREEZE)` (text format): every table is created in the same
transaction that fills it, so PostgreSQL can store the rows already frozen. No VACUUM pass is
needed afterwards, and the first queries on the new data (for example a pgTestBuilder selection)
are not slowed down by PostgreSQL doing that work on the fly. Every table is analyzed before the
commit. Keys and indexes are built with more memory (`maintenance_work_mem` 1GB, `work_mem`
256MB) and up to 4 parallel workers per index, for this transaction only. While the server processes rows, pgdemo already generates the next ones (same data for
the same seed). The log shows per table where the time went, e.g.
`Inserted 90000000 rows into sales_order_lines in 3m28s (generating 1m10s, waiting for the server 2m15s)`:
much "generating" means pgdemo (this computer) is the bottleneck, much "waiting" the server.

## Sizes and row counts

The number of rows depends only on the template and the size, never on the seed: a different
seed gives other values, but always the same counts. The window shows the total for the chosen
template and size under the size, and `pgdemo create ... --preview` prints it per table.

Total rows over all tables:

| Template | Small (100) | Medium (1,000) | Large (10,000) | 100,000 | 1,000,000 | 10,000,000 | Rows per unit |
|---|---:|---:|---:|---:|---:|---:|---:|
| Webshop | 1,360 | 13,510 | 135,010 | 1,350,010 | 13,500,010 | 135,000,010 | 13.5 |
| HR | 318 | 3,108 | 31,008 | 310,008 | 3,100,008 | 31,000,008 | 3.1 |
| Library | 370 | 3,700 | 37,000 | 370,000 | 3,700,000 | 37,000,000 | 3.7 |
| School | 515 | 5,150 | 51,500 | 515,000 | 5,150,000 | 51,500,000 | 5.15 |
| Nibble | 5,395 | 47,645 | 471,545 | 4,710,545 | 47,100,545 | 471,000,545 | 47.1 |
| Nibble mini | 2,145 | 20,770 | 207,520 | 2,075,020 | 20,750,020 | 207,500,020 | 20.8 |
| Nibble secure | 5,970 | 52,645 | 521,545 | 5,210,545 | 52,100,545 | 521,000,545 | 52.1 |
| Nibble secure history | 6,825 | 60,818 | 603,295 | 6,028,045 | 60,275,545 | 602,750,545 | 60.3 |
| Nibble enterprise | 19,543 | 142,415 | 1,418,102 | 14,174,972 | 141,743,672 | 1,417,430,672 | 141.7 |
| PostGIS municipality | 1,691 | 7,136 | 71,036 | 710,036 | 7,100,036 | 71,000,036 | 7.1 |

- From Large upwards the total is almost exactly size × rows per unit. At Small it is a little
  higher, because some tables have a minimum (e.g. at least 30 employees) and the lookup tables
  (currencies, countries, departments, ...) have a fixed number of rows.
- The country versions (BE, DE, UK) have exactly the same counts as the Dutch templates.
- To aim at a total: size ≈ wanted rows ÷ rows per unit (e.g. Nibble secure with about
  10 million rows: size 10,000,000 ÷ 52.1 ≈ 192,000).

## Command line

`pgdemo create` makes a demo schema without the window, for example from a script or a
scheduled job that rebuilds a test database. It does exactly what Create does in the window,
in one transaction: on an error or an interrupt (Ctrl-C, `kill`) nothing is left behind.

```bash
pgdemo create --host db1 --database lab --user robert \
       --template nibble-secure --schema nibble_secure --size 1000 --seed 42 --drop
```

- `--template` (required): a name from `pgdemo templates`, e.g. `nibble-secure-history-de`, or
  the name as in the window (`"Nibble secure history DE"`); not case sensitive.
- `--schema` (required): the schema name; for Nibble enterprise the base name (`nibent` gives
  `nibent_core`, `nibent_hr`, `nibent_fleet`). The same rules as in the window.
- `--size`: `small` (100), `medium` (1000, the default), `large` (10000) or a number up to
  10,000,000. `--seed`: a number gives the same data every time; without it a random seed is
  used and shown in the output, so the run can be repeated.
- `--drop`: drop the schema(s) first if they exist (CASCADE). Without it an existing schema stops
  the run and nothing is changed. Schema `public` is never dropped.
- `--no-data` (structure only), `--no-fks`, `--no-indexes`, `--no-views`: as the check boxes.
- `--preview`: print the SQL, as *Preview SQL* in the window, and stop (no connection).
- Connection: `--host`, `--port`, `--database`, `--user`, `--sslmode`. Without them the
  environment variables `PGHOST`, `PGPORT`, `PGDATABASE`, `PGUSER`, `PGSSLMODE` are used, else the
  window's defaults (localhost, 5432, postgres, postgres, prefer).
- Password: only from the PostgreSQL password file, as psql and pg_dump use it: `PGPASSFILE`,
  else `~/.pgpass` (Mac, Linux; must be private, `chmod 600`) or
  `%APPDATA%\postgresql\pgpass.conf` (Windows). One line per connection,
  `host:port:database:user:password`; `*` matches anything, `#` starts a comment. If the line is
  missing, pgdemo names the connection it needs.

The output has one timestamped line per step (connected, seed, tables created and loaded,
keys, indexes, done); redirect it to keep it.

**Exit codes**

| Code | Meaning |
|------|---------|
| 0 | done |
| 1 | creating failed or was stopped (also: the schema exists and `--drop` was not given); rolled back, nothing was left behind |
| 2 | arguments, password file, connection, or a missing extension (PostGIS for PostGIS municipality); nothing was done |

**Where the program is.** Mac: `/Applications/pgdemo.app/Contents/MacOS/pgdemo`; Linux
(`install.sh`): `~/.local/bin/pgdemo` or `/usr/local/bin/pgdemo`; Windows: `pgdemo.exe` in the
unzipped folder. On Windows the output appears in the command prompt; pgdemo is a window program
there, so in a batch file use `start /wait "" pgdemo.exe create ...` to wait for it and get the
exit code in `%ERRORLEVEL%`. `pgdemo help` lists the options and `pgdemo version` shows the version.

Example for cron (Linux, Mac), every working day at 06:00 a fresh `nibble_secure`:
```
0 6 * * 1-5  /usr/local/bin/pgdemo create --host db1 --database lab --template nibble-secure --schema nibble_secure --seed 42 --drop >>$HOME/pgdemo.log 2>&1
```

## Requirements (macOS)

1. Go 1.22 or newer: https://go.dev/dl
2. Xcode command line tools (Fyne needs a C compiler): `xcode-select --install`

## Build and run

```bash
cd pgdemo
go mod tidy        # downloads fyne.io/fyne/v2 and github.com/jackc/pgx/v5
go run .
```

The first build takes a few minutes because Fyne compiles C code.

To build a standalone app:

```bash
go install fyne.io/tools/cmd/fyne@latest
fyne package --name "PG Demo"   # creates PG Demo.app
```

## Files

| File           | Contents                                              |
|----------------|-------------------------------------------------------|
| `main.go`      | Starts the app, or the command line (`create`)        |
| `cli.go`       | Command line: `create`, `templates`, `help`, `version` |
| `cli_pg.go`    | The database functions the command line uses          |
| `options.go`   | Checks shared by the window and the command line      |
| `passfile.go`  | The PostgreSQL password file (.pgpass) for the command line |
| `console_windows.go` | Windows: output of the command line in the command prompt |
| `ui.go`        | Fyne window, options, log, progress, dialogs          |
| `ui_about.go`  | Help → About: icon, version, the Nibbler animation, connection |
| `internal/nibbler/nibbler.go` | The 8-bit Nibbler animation of the About window (the same as in QNibbler and pgTestBuilder) |
| `db.go`        | Connection, transaction, extension check, CREATE/COPY/constraints |
| `templates.go` | The schema types (tables, DDL, views, row generators) |
| `nibble.go`    | The Nibble schema type (relationship-heavy)           |
| `nibble_mini.go` | Nibble mini: a smaller Nibble for testing Parent only |
| `nibble_secure.go` | Nibble secure: Nibble plus sensitive data         |
| `nibble_history.go` | Nibble secure history: Nibble secure plus `his_` tables |
| `nibble_enterprise.go` | Nibble enterprise: core + HR + fleet, three schemas |
| `postgis.go`   | PostGIS municipality: geometry in EPSG:28992 (needs PostGIS 3) |
| `locale.go`    | Country versions (BE, DE, UK): values, IDs, column renames |
| `version.go`   | Version (set by the package script) and the app icon (`Icon.png`) |
| `build/package.sh` | Builds the distribution packages (see below) |
| `copytext.go`  | COPY ... WITH (FREEZE) in text format, generating while sending |
| `gen.go`       | Random data helpers and name lists                    |

## Distribution packages

On the Mac, in the pgdemo folder:

```
bash build/package.sh            # version from build/VERSION, mac + windows + linux
bash build/package.sh 1.1.0 mac  # another version, only some targets
```

Output in `dist/`:

| Package | Contents |
|---|---|
| `pgdemo-<version>-macos-universal.zip` | `pgdemo.app` (Apple silicon + Intel), ad-hoc signed |
| `pgdemo-<version>-windows-amd64.zip` | `pgdemo.exe` |
| `pgdemo-<version>-linux-amd64.tar.xz` | binary, icon, desktop entry, `install.sh` |

Each package has `docs/README.md` and `docs/FIRST_START.txt` (how to open an unsigned app the
first time). Requirements are the same as for pgTestBuilder: Xcode command line tools for the
Mac; MinGW-w64 (`brew install mingw-w64`) or Podman + fyne-cross for Windows; Podman +
fyne-cross for Linux (`podman machine start` first). The script builds on the local disk, so the
`._` files on the stick do not end up in the packages.

## Adding your own schema type

Add a function in `templates.go` that returns a `*Template`, the same way `webshop()` does,
and add it to the `Templates` list. Parent tables must come before child tables.
For more than one schema, list the suffixes in `Template.Schemas` (e.g. `"hr"`), set
`Table.Schema` to the suffix, and use `{s_hr}` next to `{s}` in SQL (see `fks` and `idxs`).

The app saves the connection settings, but never the password.
