package main

import (
	"fmt"
	"net/netip"
	"strings"
	"time"
)

// ---------------------------------------------------------------- Nibble secure
//
// Nibble plus sensitive data, for testing data masking (pgTestBuilder):
//   - customer_notes, employee_notes: free text notes without a foreign key
//   - persons: BSN as natural primary key (valid 11-check), name, birth date,
//     gender, email, phone, IBAN. Referenced by customer_contacts.person_bsn and
//     employees.bsn (also UNIQUE there); vehicles.driver_bsn refers to
//     employees(bsn), an FK to a unique key that is not the primary key.
//   - IBAN (valid mod-97) on customers, suppliers and employees
//   - credit card numbers (valid Luhn), NL license plates, IP addresses (inet)
//   - password hashes, JSONB with personal data, free text with personal data
//
// All values are generated; any resemblance to real people or accounts is chance.

var nlBanks = []string{"ABNA", "INGB", "RABO", "SNSB", "TRIO", "BUNQ", "ASNB", "KNAB"}
var plateLetters = "BDFGHJKLNPRSTVXZ"
var carBrands = []string{"Volkswagen", "Toyota", "Kia", "Peugeot", "Renault", "Skoda", "Tesla", "Volvo"}
var userAgents = []string{
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 14_5) Safari/605.1.15",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/126.0 Safari/537.36",
	"Mozilla/5.0 (X11; Linux x86_64) Firefox/127.0",
	"pgTestBuilder-demo-client/1.0",
}
var genders = []string{"F", "M", "M", "F", "X"}

// bsnFor gives a unique valid BSN (11-check) for index i (1-based).
// Each index owns two 8-digit bases; at least one of them always has a valid
// check digit, so the BSNs are unique.
func bsnFor(i int) string {
	base := 10000000 + 2*i
	for {
		s := fmt.Sprintf("%08d", base)
		sum := 0
		for j := 0; j < 8; j++ {
			sum += int(s[j]-'0') * (9 - j)
		}
		if d := sum % 11; d < 10 {
			return s + fmt.Sprint(d)
		}
		base++
	}
}

// ibanNL generates a valid Dutch IBAN (mod-97).
func ibanNL(g *Gen) string {
	bank := g.Pick(nlBanks)
	acct := fmt.Sprintf("%010d", g.r.Int64N(10_000_000_000))
	mod := 0
	for _, c := range bank + acct + "NL00" {
		if c >= 'A' && c <= 'Z' {
			mod = (mod*100 + int(c-'A') + 10) % 97
		} else {
			mod = (mod*10 + int(c-'0')) % 97
		}
	}
	return fmt.Sprintf("NL%02d%s%s", 98-mod, bank, acct)
}

// cardNumber generates a 16-digit test card number with a valid Luhn check digit.
func cardNumber(g *Gen) string {
	d := []byte("4")
	for i := 0; i < 14; i++ {
		d = append(d, byte('0'+g.Int(0, 9)))
	}
	sum := 0
	for i := len(d) - 1; i >= 0; i-- { // positions that are doubled once the check digit is added
		n := int(d[i] - '0')
		if (len(d)-1-i)%2 == 0 {
			if n *= 2; n > 9 {
				n -= 9
			}
		}
		sum += n
	}
	return string(append(d, byte('0'+(10-sum%10)%10)))
}

func plateNL(g *Gen) string {
	l := func() byte { return plateLetters[g.Int(0, len(plateLetters)-1)] }
	switch g.Int(0, 2) {
	case 0:
		return fmt.Sprintf("%c%c-%03d-%c", l(), l(), g.Int(0, 999), l()) // sidecode 8
	case 1:
		return fmt.Sprintf("%02d-%c%c%c-%d", g.Int(1, 99), l(), l(), l(), g.Int(0, 9)) // sidecode 9
	default:
		return fmt.Sprintf("%d-%c%c%c-%02d", g.Int(1, 9), l(), l(), l(), g.Int(0, 99)) // sidecode 10
	}
}

func ipFor(g *Gen) netip.Prefix {
	var a netip.Addr
	switch g.Int(0, 3) {
	case 0:
		a = netip.AddrFrom4([4]byte{10, byte(g.Int(0, 255)), byte(g.Int(0, 255)), byte(g.Int(1, 254))})
	case 1:
		a = netip.AddrFrom4([4]byte{192, 168, byte(g.Int(0, 255)), byte(g.Int(1, 254))})
	case 2:
		a = netip.AddrFrom4([4]byte{byte(g.Int(11, 223)), byte(g.Int(0, 255)), byte(g.Int(0, 255)), byte(g.Int(1, 254))})
	default:
		a = netip.AddrFrom16([16]byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, byte(g.Int(0, 255)), byte(g.Int(1, 255))})
	}
	return netip.PrefixFrom(a, a.BitLen())
}

// customerNote writes a free text note: about half without personal data, the
// rest with a phone number, email, IBAN, address, BSN or several of them; some
// span several lines.
func customerNote(g *Gen, k, nPersons int) string {
	f, l := g.FirstName(), g.LastName()
	addr := g.PostalAddress
	switch g.Int(0, 11) {
	case 0:
		return "Customer prefers delivery on Tuesdays."
	case 1:
		return fmt.Sprintf("Discount of %d%% agreed for the next order.", g.Int(2, 15))
	case 2:
		return "Invoice questions go to the finance department."
	case 3:
		return fmt.Sprintf("Visited on %s, no follow-up needed.", g.Date(2023, 2026).Format("2006-01-02"))
	case 4:
		return "Prefers contact by email, not by phone."
	case 5:
		return fmt.Sprintf("Called %s %s on %s about the late delivery.", f, l, g.Phone())
	case 6:
		return fmt.Sprintf("New contact person: %s %s, %s.", f, l, Email(f, l, k, "klant.example"))
	case 7:
		return fmt.Sprintf("Refunds to %s (account holder %s %s).", g.IBAN(), f, l)
	case 8:
		return fmt.Sprintf("Moved to %s as of %s.", addr(), g.Date(2024, 2026).Format("2 January 2006"))
	case 9:
		return fmt.Sprintf("Identity checked, %s %s.", g.NatIDLabel(), g.NatID(g.Int(1, nPersons)))
	case 10:
		return fmt.Sprintf("Complaint about order %d.\nCustomer: %s %s, phone %s\nPlease call back before Friday.",
			g.Int(100000, 999999), f, l, g.Phone())
	default:
		return fmt.Sprintf("Address check:\n%s\nEmail: %s\nIBAN: %s",
			addr(), Email(f, l, k, "mail.example"), g.IBAN())
	}
}

// employeeNote writes an HR note: about half without personal data, the rest
// with a phone number, email, IBAN, address or several of them; some span
// several lines.
func employeeNote(g *Gen, k int) string {
	f, l := g.FirstName(), g.LastName()
	addr := g.PostalAddress
	switch g.Int(0, 11) {
	case 0:
		return "Annual review done, objectives met."
	case 1:
		return fmt.Sprintf("Leave approved: %d days in %s.", g.Int(2, 15), g.Date(2025, 2026).Format("January 2006"))
	case 2:
		return "Completed the safety training."
	case 3:
		return fmt.Sprintf("Works from home on %s.", g.Pick([]string{"Mondays", "Wednesdays", "Fridays"}))
	case 4:
		return "Requested a new laptop."
	case 5:
		return fmt.Sprintf("Sick leave reported by phone (%s) on %s.", g.Phone(), g.Date(2024, 2026).Format("2006-01-02"))
	case 6:
		return fmt.Sprintf("Private email changed to %s.", Email(f, l, k, "prive.example"))
	case 7:
		return fmt.Sprintf("Salary to be paid into %s from next month.", g.IBAN())
	case 8:
		return fmt.Sprintf("Change of address: %s.", addr())
	case 9:
		return fmt.Sprintf("Emergency contact: %s %s, %s.", f, l, g.Phone())
	case 10:
		return fmt.Sprintf("Exit interview planned.\nForwarding address: %s\nPrivate phone: %s", addr(), g.Phone())
	default:
		return fmt.Sprintf("Personnel file updated:\nEmail: %s\nIBAN: %s\nPartner: %s %s",
			Email(f, l, k, "mail.example"), g.IBAN(), f, l)
	}
}

func findTable(t *Template, name string) *Table {
	for i := range t.Tables {
		if t.Tables[i].Name == name {
			return &t.Tables[i]
		}
	}
	panic("nibble secure: no table " + name)
}

// extend adds columns to a Nibble table; extra returns their values for row k.
func extend(tb *Table, ddl string, cols []string, extra func(g *Gen, k int) []any) {
	tb.DDL += ",\n    " + ddl
	tb.Cols = append(tb.Cols, cols...)
	orig := tb.Row
	tb.Row = func(g *Gen, k int) []any { return append(orig(g, k), extra(g, k)...) }
}

func nibbleSecure() *Template {
	t := nibble()
	t.Name = "Nibble secure"
	t.Description = "Nibble + sensitive data for masking tests: BSN (also as natural key), IBAN, birth dates, credit cards, license plates, IP addresses, password hashes, personal data in JSONB and free text."

	contacts, employees := findTable(t, "customer_contacts"), findTable(t, "employees")
	customers := findTable(t, "customers")
	nContact, nEmp, nCust := contacts.Rows, employees.Rows, customers.Rows

	// persons: one per customer contact (1..nContact) and per employee (after them)
	persons := Table{
		Name: "persons",
		DDL: `bsn        char(9) PRIMARY KEY CHECK (bsn ~ '^[0-9]{9}$'),
    first_name text NOT NULL,
    last_name  text NOT NULL,
    birth_date date NOT NULL,
    gender     text,
    email      text,
    phone      text,
    iban       text`,
		Cols: []string{"bsn", "first_name", "last_name", "birth_date", "gender", "email", "phone", "iban"},
		Rows: func(n int) int { return nContact(n) + nEmp(n) },
		Row: func(g *Gen, k int) []any {
			f, l := g.FirstName(), g.LastName()
			return []any{g.NatID(k), f, l, g.Date(1945, 2006), g.Pick(genders), Email(f, l, k, "person.example"), g.Phone(), g.IBAN()}
		},
	}
	t.Tables = append([]Table{persons}, t.Tables...)
	contacts, employees, customers = findTable(t, "customer_contacts"), findTable(t, "employees"), findTable(t, "customers")

	extend(contacts, `person_bsn char(9) NOT NULL,
    details    jsonb`, []string{"person_bsn", "details"}, func(g *Gen, k int) []any {
		f, l := g.FirstName(), g.LastName()
		details := map[string]any{
			"contact":    map[string]any{"email": Email(f, l, k, "private.example"), "phone": g.Phone()},
			"newsletter": g.Chance(0.4),
			"language":   g.Pick([]string{"nl", "nl", "en", "de"}),
		}
		return []any{g.NatID(k), details}
	})
	contacts.FKs = append(contacts.FKs, fkx("customer_contacts", "customer_contacts_person_fk", "person_bsn", "persons", "bsn", ""))
	contacts.Indexes = append(contacts.Indexes, idx("customer_contacts", "person_bsn"))

	extend(employees, `bsn           char(9) NOT NULL UNIQUE,
    iban          text,
    password_hash text`, []string{"bsn", "iban", "password_hash"}, func(g *Gen, k int) []any {
		return []any{g.NatID(nContact(g.N) + k), g.IBAN(), fmt.Sprintf("$2b$12$%022x%031x", g.r.Uint64(), g.r.Uint64())}
	})
	employees.FKs = append(employees.FKs, fkx("employees", "employees_person_fk", "bsn", "persons", "bsn", ""))

	extend(customers, `iban text`, []string{"iban"}, func(g *Gen, k int) []any {
		if g.Chance(0.8) {
			return []any{g.IBAN()}
		}
		return []any{nil}
	})
	extend(findTable(t, "suppliers"), `iban text NOT NULL`, []string{"iban"}, func(g *Gen, k int) []any {
		return []any{g.IBAN()}
	})

	// free text with personal data inside
	comments := findTable(t, "ticket_comments")
	origRow := comments.Row
	comments.Row = func(g *Gen, k int) []any {
		r := origRow(g, k)
		f, l := g.FirstName(), g.LastName()
		if r[3] != nil { // written by the customer's contact
			r[4] = fmt.Sprintf("Hello, this is %s %s. Please call me back on %s or mail %s. A refund can go to %s.",
				f, l, g.Phone(), Email(f, l, k, "mail.example"), g.IBAN())
		} else {
			r[4] = fmt.Sprintf("Called the customer on %s. Confirmed the address %s.", g.Phone(), g.PostalAddress())
		}
		return r
	}

	t.Tables = append(t.Tables,
		// No foreign key on purpose: customer_id refers to customers(id) only by
		// convention (a case for a virtual FK in pgTestBuilder). All values exist.
		Table{
			Name: "customer_notes",
			DDL: `note_id     integer PRIMARY KEY,
    customer_id integer NOT NULL,
    notes       text NOT NULL`,
			Indexes: []string{idx("customer_notes", "customer_id")},
			Cols:    []string{"note_id", "customer_id", "notes"},
			Rows:    func(n int) int { return nCust(n) },
			Row: func(g *Gen, k int) []any {
				return []any{k, g.Int(1, nCust(g.N)), customerNote(g, k, nContact(g.N)+nEmp(g.N))}
			},
		},
		// The same for employees: employee_id refers to employees(id) by convention only.
		Table{
			Name: "employee_notes",
			DDL: `note_id     integer PRIMARY KEY,
    employee_id integer NOT NULL,
    notes       text NOT NULL`,
			Indexes: []string{idx("employee_notes", "employee_id")},
			Cols:    []string{"note_id", "employee_id", "notes"},
			Rows:    func(n int) int { return nEmp(n) },
			Row: func(g *Gen, k int) []any {
				return []any{k, g.Int(1, nEmp(g.N)), employeeNote(g, k)}
			},
		},
		Table{
			Name: "payment_cards", HasID: true,
			DDL: idCol + `,
    customer_id integer NOT NULL,
    card_number text NOT NULL,
    holder_name text NOT NULL,
    expires     date NOT NULL`,
			FKs:     []string{fkx("payment_cards", "payment_cards_customer_id_fk", "customer_id", "customers", "id", "ON DELETE CASCADE")},
			Indexes: []string{idx("payment_cards", "customer_id")},
			Cols:    []string{"id", "customer_id", "card_number", "holder_name", "expires"},
			Rows:    func(n int) int { return max(10, nCust(n)/2) },
			Row: func(g *Gen, k int) []any {
				exp := time.Date(2026+g.Int(0, 4), time.Month(g.Int(1, 12)), 1, 0, 0, 0, 0, time.UTC)
				return []any{k, g.Int(1, nCust(g.N)), cardNumber(g), strings.ToUpper(g.FirstName() + " " + g.LastName()), exp}
			},
		},
		Table{
			Name: "vehicles", HasID: true,
			DDL: idCol + `,
    driver_bsn    char(9) NOT NULL,
    license_plate text NOT NULL,
    brand         text NOT NULL`,
			// FK to employees(bsn): a unique key that is not the primary key
			FKs:     []string{fkx("vehicles", "vehicles_driver_fk", "driver_bsn", "employees", "bsn", "")},
			Indexes: []string{idx("vehicles", "driver_bsn")},
			Cols:    []string{"id", "driver_bsn", "license_plate", "brand"},
			Rows:    func(n int) int { return max(5, nEmp(n)/2) },
			Row: func(g *Gen, k int) []any {
				return []any{k, g.NatID(nContact(g.N) + g.Int(1, nEmp(g.N))), g.Plate(), g.Pick(carBrands)}
			},
		},
		Table{
			Name: "login_events", HasID: true,
			DDL: idCol + `,
    employee_id  integer NOT NULL,
    ip_address   inet NOT NULL,
    user_agent   text,
    logged_in_at timestamptz NOT NULL`,
			FKs:     []string{fkx("login_events", "login_events_employee_id_fk", "employee_id", "employees", "id", "ON DELETE CASCADE")},
			Indexes: []string{idx("login_events", "employee_id")},
			Cols:    []string{"id", "employee_id", "ip_address", "user_agent", "logged_in_at"},
			Rows:    func(n int) int { return 5 * nEmp(n) },
			Row: func(g *Gen, k int) []any {
				return []any{k, g.Int(1, nEmp(g.N)), ipFor(g), g.Pick(userAgents), g.Timestamp(2024, 2026)}
			},
		},
	)
	return t
}
