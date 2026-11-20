package main

import (
	"fmt"
	"time"
)

// Nibble secure history: Nibble secure plus history tables (his_<table>) with the
// earlier versions of the rows that hold personal data. The current version stays
// in the original table; his_<table> has the versions before it, each with
// valid_from / valid_to.
//
// The his_ tables are standalone on purpose: no foreign keys. They refer to the
// current rows only by value (bsn, customer_id, contact_id, employee_id,
// supplier_id), and they keep personal data that is no longer in the current
// table (old names, addresses, IBANs, salaries).
//
// The his_ tables come after all Nibble secure tables, so the Nibble secure
// tables get the same data as in Nibble secure for the same seed and size.

// histCount is the number of earlier versions of parent p (1-based): 0, 1, 2 or 3.
// Every block of four parents has one of each, in a rotating order, so a block
// has 6 history rows and a history row can be found without counting.
func histCount(p int) int {
	b := uint64((p - 1) / 4)
	return ((p-1)%4 + int(nbMix(b*0x4157+11)%4)) % 4
}

// histRows is the number of history rows for parents 1..parents.
func histRows(parents int) int {
	full := parents / 4
	n := 6 * full
	for p := 4*full + 1; p <= parents; p++ {
		n += histCount(p)
	}
	return n
}

// histAt gives history row k (1-based): parent p and version v (1 = oldest).
func histAt(k int) (p, v int) {
	b, o := (k-1)/6, (k-1)%6
	for i := 1; i <= 4; i++ {
		p = 4*b + i
		c := histCount(p)
		if o < c {
			return p, o + 1
		}
		o -= c
	}
	panic(fmt.Sprintf("histAt: no history row %d", k))
}

// histOpt returns f() with chance p, otherwise NULL (as optInt in nibble.go).
func histOpt(g *Gen, p float64, f func() int) any {
	if g.Chance(p) {
		return f()
	}
	return nil
}

var histStart = time.Date(2012, 1, 1, 0, 0, 0, 0, time.UTC)

// histPeriod is the period of version v of parent p in table salt: valid_from
// and valid_to (the day the next version started). The versions of one parent
// follow each other without gaps; all of them end before 2025.
func histPeriod(salt uint64, p, v int) (from, to time.Time) {
	h := nbMix(salt<<40 ^ uint64(p))
	from = histStart.AddDate(0, 0, int(h%1460)) // start in 2012-2015
	for i := 1; ; i++ {
		to = from.AddDate(0, 0, 120+int(nbMix(h+uint64(i))%1000)) // 4 months to 3 years
		if i == v {
			return from, to
		}
		from = to
	}
}

var histPersonReasons = []string{"marriage", "divorce", "moved", "new bank account", "new phone number", "new e-mail address", "correction"}
var histCustomerReasons = []string{"name change", "merger", "moved", "new bank account", "correction"}
var histContactReasons = []string{"marriage", "new phone number", "new e-mail address", "correction"}
var histEmployeeReasons = []string{"promotion", "salary increase", "transfer", "moved", "marriage", "new bank account"}
var histSupplierReasons = []string{"name change", "takeover", "moved", "new bank account"}

func nibbleSecureHistory() *Template {
	t := nibbleSecure()
	t.Name = "Nibble secure history"
	t.Description = "Nibble secure plus history tables his_persons, his_customers, his_customer_contacts, his_employees " +
		"and his_suppliers: earlier versions of the rows with personal data (old names, addresses, IBANs, salaries), " +
		"with valid_from / valid_to and no foreign keys."

	nPers := findTable(t, "persons").Rows
	nCust := findTable(t, "customers").Rows
	nContact := findTable(t, "customer_contacts").Rows
	nEmp := findTable(t, "employees").Rows
	nSupp := findTable(t, "suppliers").Rows
	nAddr := findTable(t, "addresses").Rows
	nDept := findTable(t, "departments").Rows
	hist := func(parents func(int) int) func(int) int {
		return func(n int) int { return histRows(parents(n)) }
	}
	// columns at the end of every his_ table
	const period = `version_no    integer NOT NULL,
    valid_from    date NOT NULL,
    valid_to      date NOT NULL,
    change_reason text NOT NULL,
    CHECK (valid_to > valid_from)`

	t.Tables = append(t.Tables,
		Table{
			Name: "his_persons", HasID: true,
			DDL: idCol + `,
    bsn           char(9) NOT NULL CHECK (bsn ~ '^[0-9]{9}$'),
    first_name    text NOT NULL,
    last_name     text NOT NULL,
    email         text,
    phone         text,
    iban          text,
    ` + period,
			Indexes: []string{idx("his_persons", "bsn")},
			Cols: []string{"id", "bsn", "first_name", "last_name", "email", "phone", "iban",
				"version_no", "valid_from", "valid_to", "change_reason"},
			Rows: hist(nPers),
			Row: func(g *Gen, k int) []any {
				p, v := histAt(k)
				from, to := histPeriod(1, p, v)
				f, l := g.FirstName(), g.LastName()
				return []any{k, g.NatID(p), f, l, Email(f, l, k, "person.example"), g.Phone(), g.IBAN(),
					v, from, to, g.Pick(histPersonReasons)}
			},
		},
		Table{
			Name: "his_customers", HasID: true,
			DDL: idCol + `,
    customer_id        integer NOT NULL,
    customer_number    text NOT NULL,
    name               text NOT NULL,
    iban               text,
    billing_address_id integer NOT NULL,
    changed_by         integer,
    ` + period,
			Indexes: []string{idx("his_customers", "customer_id")},
			Cols: []string{"id", "customer_id", "customer_number", "name", "iban", "billing_address_id", "changed_by",
				"version_no", "valid_from", "valid_to", "change_reason"},
			Rows: hist(nCust),
			Row: func(g *Gen, k int) []any {
				p, v := histAt(k)
				from, to := histPeriod(2, p, v)
				var iban any
				if g.Chance(0.8) {
					iban = g.IBAN()
				}
				return []any{k, p, fmt.Sprintf("C%07d", p), g.LastName() + " " + g.CompanySuffix(), iban,
					g.Int(1, nAddr(g.N)), histOpt(g, 0.7, func() int { return g.Int(1, nEmp(g.N)) }),
					v, from, to, g.Pick(histCustomerReasons)}
			},
		},
		Table{
			Name: "his_customer_contacts", HasID: true,
			DDL: idCol + `,
    contact_id    integer NOT NULL,
    customer_id   integer NOT NULL,
    first_name    text NOT NULL,
    last_name     text NOT NULL,
    email         text NOT NULL,
    phone         text,
    ` + period,
			Indexes: []string{idx("his_customer_contacts", "contact_id")},
			Cols: []string{"id", "contact_id", "customer_id", "first_name", "last_name", "email", "phone",
				"version_no", "valid_from", "valid_to", "change_reason"},
			Rows: hist(nContact),
			Row: func(g *Gen, k int) []any {
				p, v := histAt(k)
				from, to := histPeriod(3, p, v)
				f, l := g.FirstName(), g.LastName()
				// contacts 2c-1 and 2c belong to customer c, as in customer_contacts
				return []any{k, p, (p-1)/2 + 1, f, l, Email(f, l, k, "customer.example"), g.Phone(),
					v, from, to, g.Pick(histContactReasons)}
			},
		},
		Table{
			Name: "his_employees", HasID: true,
			DDL: idCol + `,
    employee_id     integer NOT NULL,
    employee_number text NOT NULL,
    bsn             char(9) NOT NULL CHECK (bsn ~ '^[0-9]{9}$'),
    last_name       text NOT NULL,
    email           text NOT NULL,
    job_title       text NOT NULL,
    salary          numeric(10,2) NOT NULL,
    department_id   integer NOT NULL,
    address_id      integer,
    iban            text,
    changed_by      integer,
    ` + period,
			Indexes: []string{idx("his_employees", "employee_id"), idx("his_employees", "bsn")},
			Cols: []string{"id", "employee_id", "employee_number", "bsn", "last_name", "email", "job_title", "salary",
				"department_id", "address_id", "iban", "changed_by", "version_no", "valid_from", "valid_to", "change_reason"},
			Rows: hist(nEmp),
			Row: func(g *Gen, k int) []any {
				p, v := histAt(k)
				from, to := histPeriod(4, p, v)
				f, l := g.FirstName(), g.LastName()
				// older versions earn less: about 6% per version back
				salary := g.Money(2800, 9500) * (1 - 0.06*float64(histCount(p)-v+1))
				return []any{k, p, fmt.Sprintf("E%05d", p), g.NatID(nContact(g.N) + p), l,
					Email(f, l, k, "nibble.example"), g.Pick(jobTitles), round2(salary),
					g.Int(1, nDept(g.N)), histOpt(g, 0.8, func() int { return g.Int(1, nAddr(g.N)) }), g.IBAN(),
					histOpt(g, 0.9, func() int { return g.Int(1, nEmp(g.N)) }),
					v, from, to, g.Pick(histEmployeeReasons)}
			},
		},
		Table{
			Name: "his_suppliers", HasID: true,
			DDL: idCol + `,
    supplier_id     integer NOT NULL,
    supplier_number text NOT NULL,
    name            text NOT NULL,
    iban            text NOT NULL,
    address_id      integer NOT NULL,
    ` + period,
			Indexes: []string{idx("his_suppliers", "supplier_id")},
			Cols: []string{"id", "supplier_id", "supplier_number", "name", "iban", "address_id",
				"version_no", "valid_from", "valid_to", "change_reason"},
			Rows: hist(nSupp),
			Row: func(g *Gen, k int) []any {
				p, v := histAt(k)
				from, to := histPeriod(5, p, v)
				return []any{k, p, fmt.Sprintf("S%05d", p), g.Pick(nbSupplierWords) + " " + g.Pick(nbSupplierNouns),
					g.IBAN(), g.Int(1, nAddr(g.N)), v, from, to, g.Pick(histSupplierReasons)}
			},
		},
	)
	return t
}
