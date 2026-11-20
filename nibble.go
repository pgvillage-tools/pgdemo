package main

import (
	"fmt"
	"math"
	"time"
)

// ---------------------------------------------------------------- Nibble
//
// A relationship-heavy schema (36 tables) meant for testing tools that copy a
// percentage of the data while keeping every foreign key valid. On purpose it has:
//
//   - surrogate keys (identity) and natural keys (country/currency/unit codes)
//   - composite primary keys and composite foreign keys
//   - FKs to a UNIQUE constraint that is not the primary key
//     (price_list_items.product_sku -> products.sku,
//     sales_orders (contact_id, customer_id) -> customer_contacts (id, customer_id))
//   - self references: departments, employees (manager + mentor), customers,
//     product_categories, products, projects, invoices, tickets
//   - a self many-to-many: bill_of_materials (product -> component products)
//   - a cycle between two tables: departments.manager_id <-> employees.department_id
//     (DEFERRABLE INITIALLY DEFERRED)
//   - several FKs from one table to the same parent (addresses, currencies, products)
//   - nullable and NOT NULL FKs, ON DELETE CASCADE / SET NULL / NO ACTION
//   - skewed data: a few customers own most orders and tickets
//   - a polymorphic table without FKs (attachments) and a table with no relations (app_settings)
//
// Rows that refer to each other are generated with deterministic formulas, so all
// foreign keys (including composite ones) are valid for every size and seed.

var nbCurrencies = [][3]string{{"EUR", "Euro", "€"}, {"USD", "US Dollar", "$"}, {"GBP", "Pound Sterling", "£"}, {"SEK", "Swedish Krona", "kr"}}
var nbRates = []float64{1, 1.08, 0.86, 11.4} // per EUR

var nbCountries = [][3]string{
	{"NL", "Netherlands", "EUR"}, {"BE", "Belgium", "EUR"}, {"DE", "Germany", "EUR"}, {"FR", "France", "EUR"},
	{"ES", "Spain", "EUR"}, {"GB", "United Kingdom", "GBP"}, {"US", "United States", "USD"}, {"SE", "Sweden", "SEK"},
}
var nbCompass = []string{"North", "East", "South", "West"}

var nbUnits = [][2]string{{"pcs", "piece"}, {"kg", "kilogram"}, {"m", "metre"}, {"l", "litre"}, {"box", "box"}}

var nbDepartments = []struct {
	name   string
	parent int
}{
	{"Nibble", 0}, {"Sales", 1}, {"Operations", 1}, {"Finance", 1}, {"IT", 1}, {"HR", 1},
	{"Sales Netherlands", 2}, {"Sales Export", 2}, {"Warehouse", 3}, {"Purchasing", 3},
	{"Accounting", 4}, {"Payroll", 6}, {"Support", 5}, {"Development", 5},
}

var nbSegments = []struct {
	name     string
	discount float64
}{{"Consumer", 0}, {"Small business", 2.5}, {"Enterprise", 5}, {"Government", 3}, {"Partner", 10}}

var nbWarehouses = [][2]string{{"AMS", "Amsterdam"}, {"RTM", "Rotterdam"}, {"EIN", "Eindhoven"}, {"ANR", "Antwerp"}, {"DUS", "Düsseldorf"}, {"LYS", "Lyon"}}

var nbStreets = []string{"Kerkstraat", "Dorpsstraat", "Stationsweg", "Molenweg", "Schoolstraat", "Industrieweg", "Hoofdstraat", "Parallelweg", "Marktplein", "Havenkade"}
var nbCompanySuffix = []string{"B.V.", "Holding", "Groep", "& Zonen", "Trading", "Services", "Logistics", "Solutions"}
var nbSupplierWords = []string{"Global", "Euro", "Prime", "United", "Delta", "Nordic", "Atlas", "Core"}
var nbSupplierNouns = []string{"Parts", "Supplies", "Components", "Industries", "Wholesale", "Materials"}
var nbCarriers = []string{"PostNL", "DHL", "DPD", "UPS", "GLS"}
var nbPayMethods = []string{"bank transfer", "bank transfer", "iDEAL", "credit card", "direct debit"}
var nbPOStatus = []string{"draft", "sent", "confirmed", "received", "received", "closed"}
var nbTicketStatus = []string{"open", "in progress", "waiting", "resolved", "resolved", "closed", "closed"}
var nbPriorities = []string{"low", "normal", "normal", "normal", "high", "urgent"}
var nbProjectRoles = []string{"developer", "analyst", "tester", "consultant", "designer"}
var nbEntityTypes = []string{"customer", "sales_order", "ticket", "product"}
var nbSettings = [][2]string{
	{"company_name", "Nibble"}, {"default_currency", "EUR"}, {"invoice_payment_days", "30"},
	{"support_email", "support@nibble.example"}, {"schema_version", "1"},
}

// nbMix is splitmix64: a fast deterministic hash.
func nbMix(x uint64) uint64 {
	x += 0x9E3779B97F4A7C15
	x = (x ^ (x >> 30)) * 0xBF58476D1CE4E5B9
	x = (x ^ (x >> 27)) * 0x94D049BB133111EB
	return x ^ (x >> 31)
}

// nbSkew maps k to 1..n deterministically; low numbers are much more likely.
func nbSkew(k, n int, salt uint64) int {
	u := float64(nbMix(uint64(k)*0x100000001B3^salt)%1_000_000) / 1_000_000
	return min(n, int(float64(n)*u*u*u)+1)
}

// fkx builds a foreign key with free column lists and options.
func fkx(table, name, cols, ref, refCols, extra string) string {
	s := fmt.Sprintf("ALTER TABLE {s}.%s ADD CONSTRAINT %s FOREIGN KEY (%s) REFERENCES {s}.%s (%s)",
		table, name, cols, ref, refCols)
	if extra != "" {
		s += " " + extra
	}
	return s
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }

func nibble() *Template {
	// ---- row counts for base size n
	nCur := func(int) int { return len(nbCurrencies) }
	nCountry := func(int) int { return len(nbCountries) }
	nUnit := func(int) int { return len(nbUnits) }
	rateMonths := 36
	nRate := func(int) int { return len(nbCurrencies) * (len(nbCurrencies) - 1) * rateMonths }
	nRegion := func(int) int { return len(nbCountries) * len(nbCompass) }
	nAddr := func(n int) int { return max(60, 2*n) }
	nDept := func(int) int { return len(nbDepartments) }
	nEmp := func(n int) int { return max(30, n/5) }
	nSeg := func(int) int { return len(nbSegments) }
	nCust := func(n int) int { return max(20, n) }
	nContact := func(n int) int { return 2 * nCust(n) }
	nSupp := func(n int) int { return max(10, n/20) }
	nCat := func(int) int { return 3 * len(categoryNames) } // 10 main + 20 sub categories
	nProd := func(n int) int { return max(30, n/2) }
	nPS := func(n int) int { return 2 * nProd(n) }
	nBOM := func(n int) int { return 3 * (nProd(n) / 5) }
	nWh := func(int) int { return len(nbWarehouses) }
	nStock := func(n int) int { return 2 * nProd(n) }
	nPL := func(int) int { return len(nbCurrencies) }
	nPLI := func(n int) int { return nPL(n) * nProd(n) }
	nOrd := func(n int) int { return 3 * nCust(n) }
	nOL := func(n int) int { return 3 * nOrd(n) }
	nShip := func(n int) int { return nOrd(n) - nOrd(n)/4 }
	nSL := func(n int) int { return 3 * nShip(n) }
	nInv := nShip
	nIL := func(n int) int { return 3 * nInv(n) }
	nPay := func(n int) int { return nInv(n) - nInv(n)/5 }
	nPO := func(n int) int { return max(10, n/2) }
	nPOL := func(n int) int { return 2 * nPO(n) }
	nProj := func(n int) int { return max(10, n/20) }
	nPM := func(n int) int { return 4 * nProj(n) }
	nTS := func(n int) int { return 5 * nPM(n) }
	nTick := func(n int) int { return max(20, n/2) }
	nTC := func(n int) int { return 3 * nTick(n) }
	nAtt := func(n int) int { return max(20, n/2) }
	nSet := func(int) int { return len(nbSettings) }

	// ---- deterministic relations (keep composite FKs valid)
	orderCustomer := func(n, o int) int { return nbSkew(o, nCust(n), 1) }
	shipOrder := func(k int) int { return k + (k-1)/3 } // k-th order with id%4 != 0 (shipped/delivered)
	ticketCustomer := func(n, t int) int { return nbSkew(t, nCust(n), 2) }
	ticketContact := func(n, t int) int { return 2*ticketCustomer(n, t) - 1 + t%2 }
	prodSupplier := func(n, p, j int) int { return (p-1+j)%nSupp(n) + 1 }
	prodWarehouse := func(n, p, j int) int { return (p-1+2*j)%nWh(n) + 1 }
	poSupplier := func(n, po int) int { return (po-1)%nSupp(n) + 1 }
	projMember := func(n, p, j int) int { return ((p-1)*4+j)%nEmp(n) + 1 }
	sku := func(p int) string { return fmt.Sprintf("SKU-%06d", p) }
	optInt := func(g *Gen, p float64, f func() int) any {
		if g.Chance(p) {
			return f()
		}
		return nil
	}

	return &Template{
		Name:        "Nibble",
		Description: "36 tables, ~60 foreign keys: composite and natural keys, self references, a cycle, M:N junctions and skewed data. Built for testing partial (percentage) schema copies.",
		Tables: []Table{
			// ======================== reference data
			{
				Name: "currencies",
				DDL: `code   char(3) PRIMARY KEY,
    name   text NOT NULL,
    symbol text NOT NULL`,
				Cols: []string{"code", "name", "symbol"},
				Rows: nCur,
				Row: func(g *Gen, k int) []any {
					c := nbCurrencies[k-1]
					return []any{c[0], c[1], c[2]}
				},
			},
			{
				Name: "countries",
				DDL: `code          char(2) PRIMARY KEY,
    name          text NOT NULL UNIQUE,
    currency_code char(3) NOT NULL`,
				FKs:     []string{fkx("countries", "countries_currency_fk", "currency_code", "currencies", "code", "")},
				Indexes: []string{idx("countries", "currency_code")},
				Cols:    []string{"code", "name", "currency_code"},
				Rows:    nCountry,
				Row: func(g *Gen, k int) []any {
					c := nbCountries[k-1]
					return []any{c[0], c[1], c[2]}
				},
			},
			{
				Name: "units_of_measure",
				DDL: `code text PRIMARY KEY,
    name text NOT NULL`,
				Cols: []string{"code", "name"},
				Rows: nUnit,
				Row: func(g *Gen, k int) []any {
					u := nbUnits[k-1]
					return []any{u[0], u[1]}
				},
			},
			{
				Name: "exchange_rates",
				DDL: `from_currency char(3) NOT NULL,
    to_currency   char(3) NOT NULL,
    rate_date     date NOT NULL,
    rate          numeric(14,6) NOT NULL CHECK (rate > 0),
    PRIMARY KEY (from_currency, to_currency, rate_date),
    CHECK (from_currency <> to_currency)`,
				FKs: []string{
					fkx("exchange_rates", "exchange_rates_from_fk", "from_currency", "currencies", "code", ""),
					fkx("exchange_rates", "exchange_rates_to_fk", "to_currency", "currencies", "code", ""),
				},
				Indexes: []string{idx("exchange_rates", "to_currency")},
				Cols:    []string{"from_currency", "to_currency", "rate_date", "rate"},
				Rows:    nRate,
				Row: func(g *Gen, k int) []any {
					nc := len(nbCurrencies)
					pairs := nc * (nc - 1)
					i := k - 1
					pair, month := i%pairs, i/pairs
					f, t := pair/(nc-1), pair%(nc-1)
					if t >= f {
						t++
					}
					rate := nbRates[t] / nbRates[f] * (0.97 + 0.06*g.r.Float64())
					return []any{nbCurrencies[f][0], nbCurrencies[t][0],
						time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, month, 0),
						math.Round(rate*1e6) / 1e6}
				},
			},
			{
				Name: "regions", HasID: true,
				DDL: idCol + `,
    country_code char(2) NOT NULL,
    name         text NOT NULL,
    UNIQUE (country_code, name)`,
				FKs:  []string{fkx("regions", "regions_country_fk", "country_code", "countries", "code", "")},
				Cols: []string{"id", "country_code", "name"},
				Rows: nRegion,
				Row: func(g *Gen, k int) []any {
					c := nbCountries[(k-1)/len(nbCompass)]
					return []any{k, c[0], c[1] + " " + nbCompass[(k-1)%len(nbCompass)]}
				},
			},
			{
				Name: "addresses", HasID: true,
				DDL: idCol + `,
    street       text NOT NULL,
    house_number text NOT NULL,
    postal_code  text NOT NULL,
    city         text NOT NULL,
    region_id    integer,
    country_code char(2) NOT NULL`,
				FKs: []string{
					fk("addresses", "region_id", "regions"),
					fkx("addresses", "addresses_country_fk", "country_code", "countries", "code", ""),
				},
				Indexes: []string{idx("addresses", "region_id"), idx("addresses", "country_code")},
				Cols:    []string{"id", "street", "house_number", "postal_code", "city", "region_id", "country_code"},
				Rows:    nAddr,
				Row: func(g *Gen, k int) []any {
					r := g.Int(1, nRegion(g.N))
					country := nbCountries[(r-1)/len(nbCompass)][0]
					var region any = r
					if g.Chance(0.1) {
						region = nil // region is optional
					}
					postal := g.Postcode()
					return []any{k, g.Street(), fmt.Sprint(g.Int(1, 250)), postal, g.City(), region, country}
				},
			},

			// ======================== organisation (cycle: departments <-> employees)
			{
				Name: "departments", HasID: true,
				DDL: idCol + `,
    name        text NOT NULL UNIQUE,
    cost_center text NOT NULL UNIQUE,
    parent_id   integer,
    manager_id  integer,
    CHECK (parent_id <> id)`,
				FKs: []string{
					fk("departments", "parent_id", "departments"),
					fkx("departments", "departments_manager_id_fk", "manager_id", "employees", "id",
						"ON DELETE SET NULL DEFERRABLE INITIALLY DEFERRED"),
				},
				Indexes: []string{idx("departments", "parent_id"), idx("departments", "manager_id")},
				Cols:    []string{"id", "name", "cost_center", "parent_id", "manager_id"},
				Rows:    nDept,
				Row: func(g *Gen, k int) []any {
					d := nbDepartments[k-1]
					var parent any
					if d.parent > 0 {
						parent = d.parent
					}
					// employee k is the head of department k
					return []any{k, d.name, fmt.Sprintf("CC%03d", k*10), parent, k}
				},
			},
			{
				Name: "employees", HasID: true,
				DDL: idCol + `,
    employee_number text NOT NULL UNIQUE,
    first_name      text NOT NULL,
    last_name       text NOT NULL,
    email           text NOT NULL UNIQUE,
    hire_date       date NOT NULL,
    job_title       text NOT NULL,
    salary          numeric(10,2) NOT NULL,
    department_id   integer NOT NULL,
    manager_id      integer,
    mentor_id       integer,
    address_id      integer,
    CHECK (manager_id <> id),
    CHECK (mentor_id <> id)`,
				FKs: []string{
					fk("employees", "department_id", "departments"),
					fk("employees", "manager_id", "employees"),
					fk("employees", "mentor_id", "employees"),
					fkx("employees", "employees_address_id_fk", "address_id", "addresses", "id", "ON DELETE SET NULL"),
				},
				Indexes: []string{idx("employees", "department_id"), idx("employees", "manager_id"),
					idx("employees", "mentor_id"), idx("employees", "address_id")},
				Cols: []string{"id", "employee_number", "first_name", "last_name", "email", "hire_date", "job_title",
					"salary", "department_id", "manager_id", "mentor_id", "address_id"},
				Rows: nEmp,
				Row: func(g *Gen, k int) []any {
					f, l := g.FirstName(), g.LastName()
					var dept int
					var manager any
					title := g.Pick(jobTitles)
					if k <= nDept(g.N) {
						d := nbDepartments[k-1]
						dept, title = k, "Head of "+d.name
						if d.parent > 0 {
							manager = d.parent // head of the parent department
						}
					} else {
						dept = g.Int(2, nDept(g.N))
						manager = dept // reports to the head of the department
					}
					mentor := optInt(g, 0.3, func() int { return g.Int(1, max(1, k-1)) })
					if mentor == k {
						mentor = nil
					}
					addr := optInt(g, 0.8, func() int { return g.Int(1, nAddr(g.N)) })
					return []any{k, fmt.Sprintf("E%05d", k), f, l, Email(f, l, k, "nibble.example"), g.Date(2005, 2025),
						title, g.Money(2800, 9500), dept, manager, mentor, addr}
				},
			},

			// ======================== customers
			{
				Name: "customer_segments", HasID: true,
				DDL: idCol + `,
    name         text NOT NULL UNIQUE,
    discount_pct numeric(4,1) NOT NULL DEFAULT 0`,
				Cols: []string{"id", "name", "discount_pct"},
				Rows: nSeg,
				Row: func(g *Gen, k int) []any {
					s := nbSegments[k-1]
					return []any{k, s.name, s.discount}
				},
			},
			{
				Name: "customers", HasID: true,
				DDL: idCol + `,
    customer_number     text NOT NULL UNIQUE,
    name                text NOT NULL,
    segment_id          integer NOT NULL,
    parent_customer_id  integer,
    billing_address_id  integer NOT NULL,
    shipping_address_id integer,
    account_manager_id  integer,
    currency_code       char(3) NOT NULL,
    created_at          timestamptz NOT NULL DEFAULT now(),
    CHECK (parent_customer_id <> id)`,
				FKs: []string{
					fk("customers", "segment_id", "customer_segments"),
					fk("customers", "parent_customer_id", "customers"),
					fk("customers", "billing_address_id", "addresses"),
					fk("customers", "shipping_address_id", "addresses"),
					fkx("customers", "customers_account_manager_id_fk", "account_manager_id", "employees", "id", "ON DELETE SET NULL"),
					fkx("customers", "customers_currency_fk", "currency_code", "currencies", "code", ""),
				},
				Indexes: []string{idx("customers", "segment_id"), idx("customers", "parent_customer_id"),
					idx("customers", "billing_address_id"), idx("customers", "shipping_address_id"),
					idx("customers", "account_manager_id"), idx("customers", "currency_code")},
				Cols: []string{"id", "customer_number", "name", "segment_id", "parent_customer_id", "billing_address_id",
					"shipping_address_id", "account_manager_id", "currency_code", "created_at"},
				Rows: nCust,
				Row: func(g *Gen, k int) []any {
					var parent any
					if k > 1 && g.Chance(0.1) {
						parent = g.Int(1, k-1) // part of a customer group
					}
					cur := "EUR"
					if g.Chance(0.25) {
						cur = nbCurrencies[g.Int(1, len(nbCurrencies)-1)][0]
					}
					return []any{k, fmt.Sprintf("C%07d", k), g.LastName() + " " + g.CompanySuffix(),
						g.Int(1, nSeg(g.N)), parent, g.Int(1, nAddr(g.N)),
						optInt(g, 0.4, func() int { return g.Int(1, nAddr(g.N)) }),
						optInt(g, 0.7, func() int { return g.Int(1, nEmp(g.N)) }),
						cur, g.Timestamp(2015, 2025)}
				},
			},
			{
				Name: "customer_contacts", HasID: true,
				DDL: idCol + `,
    customer_id integer NOT NULL,
    first_name  text NOT NULL,
    last_name   text NOT NULL,
    email       text NOT NULL,
    phone       text,
    is_primary  boolean NOT NULL DEFAULT false,
    UNIQUE (id, customer_id)`,
				FKs:     []string{fkx("customer_contacts", "customer_contacts_customer_id_fk", "customer_id", "customers", "id", "ON DELETE CASCADE")},
				Indexes: []string{idx("customer_contacts", "customer_id")},
				Cols:    []string{"id", "customer_id", "first_name", "last_name", "email", "phone", "is_primary"},
				Rows:    nContact,
				Row: func(g *Gen, k int) []any {
					// contacts 2c-1 and 2c belong to customer c
					f, l := g.FirstName(), g.LastName()
					return []any{k, (k-1)/2 + 1, f, l, Email(f, l, k, "customer.example"), g.Phone(), k%2 == 1}
				},
			},

			// ======================== products & suppliers
			{
				Name: "suppliers", HasID: true,
				DDL: idCol + `,
    supplier_number    text NOT NULL UNIQUE,
    name               text NOT NULL,
    address_id         integer NOT NULL,
    currency_code      char(3) NOT NULL,
    payment_terms_days integer NOT NULL DEFAULT 30`,
				FKs: []string{
					fk("suppliers", "address_id", "addresses"),
					fkx("suppliers", "suppliers_currency_fk", "currency_code", "currencies", "code", ""),
				},
				Indexes: []string{idx("suppliers", "address_id"), idx("suppliers", "currency_code")},
				Cols:    []string{"id", "supplier_number", "name", "address_id", "currency_code", "payment_terms_days"},
				Rows:    nSupp,
				Row: func(g *Gen, k int) []any {
					return []any{k, fmt.Sprintf("S%05d", k), g.Pick(nbSupplierWords) + " " + g.Pick(nbSupplierNouns),
						g.Int(1, nAddr(g.N)), nbCurrencies[g.Int(0, len(nbCurrencies)-1)][0], []int{14, 30, 30, 60}[g.Int(0, 3)]}
				},
			},
			{
				Name: "product_categories", HasID: true,
				DDL: idCol + `,
    name      text NOT NULL UNIQUE,
    parent_id integer`,
				FKs:     []string{fk("product_categories", "parent_id", "product_categories")},
				Indexes: []string{idx("product_categories", "parent_id")},
				Cols:    []string{"id", "name", "parent_id"},
				Rows:    nCat,
				Row: func(g *Gen, k int) []any {
					nm := len(categoryNames)
					if k <= nm {
						return []any{k, categoryNames[k-1], nil}
					}
					p := (k-nm-1)/2 + 1
					return []any{k, categoryNames[p-1] + " " + []string{"Basic", "Premium"}[(k-nm-1)%2], p}
				},
			},
			{
				Name: "products", HasID: true,
				DDL: idCol + `,
    sku            text NOT NULL UNIQUE,
    name           text NOT NULL,
    category_id    integer NOT NULL,
    uom_code       text NOT NULL,
    list_price     numeric(10,2) NOT NULL CHECK (list_price >= 0),
    replaced_by_id integer,
    created_at     timestamptz NOT NULL DEFAULT now(),
    CHECK (replaced_by_id <> id)`,
				FKs: []string{
					fk("products", "category_id", "product_categories"),
					fkx("products", "products_uom_fk", "uom_code", "units_of_measure", "code", ""),
					fk("products", "replaced_by_id", "products"),
				},
				Indexes: []string{idx("products", "category_id"), idx("products", "uom_code"), idx("products", "replaced_by_id")},
				Cols:    []string{"id", "sku", "name", "category_id", "uom_code", "list_price", "replaced_by_id", "created_at"},
				Rows:    nProd,
				Row: func(g *Gen, k int) []any {
					price := g.Money(1, 900)
					g.prices = append(g.prices, price)
					var repl any
					if k < nProd(g.N) && g.Chance(0.05) {
						repl = k + 1 // successor product
					}
					name := fmt.Sprintf("%s %s %d", g.Pick(productAdjectives), g.Pick(productNouns), g.Int(100, 999))
					return []any{k, sku(k), name, g.Int(len(categoryNames)+1, nCat(g.N)), nbUnits[g.Int(0, len(nbUnits)-1)][0],
						price, repl, g.Timestamp(2018, 2025)}
				},
			},
			{
				Name: "product_suppliers",
				DDL: `product_id     integer NOT NULL,
    supplier_id    integer NOT NULL,
    supplier_sku   text NOT NULL,
    cost_price     numeric(10,2) NOT NULL,
    lead_time_days integer NOT NULL,
    PRIMARY KEY (product_id, supplier_id)`,
				FKs: []string{
					fkx("product_suppliers", "product_suppliers_product_id_fk", "product_id", "products", "id", "ON DELETE CASCADE"),
					fk("product_suppliers", "supplier_id", "suppliers"),
				},
				Indexes: []string{idx("product_suppliers", "supplier_id")},
				Cols:    []string{"product_id", "supplier_id", "supplier_sku", "cost_price", "lead_time_days"},
				Rows:    nPS,
				Row: func(g *Gen, k int) []any {
					p, j := (k-1)/2+1, (k-1)%2
					s := prodSupplier(g.N, p, j)
					return []any{p, s, fmt.Sprintf("X%d-%d", s, p), round2(g.prices[p-1] * (0.5 + 0.3*g.r.Float64())), g.Int(2, 60)}
				},
			},
			{
				Name: "bill_of_materials",
				DDL: `parent_product_id    integer NOT NULL,
    component_product_id integer NOT NULL,
    quantity             numeric(10,3) NOT NULL CHECK (quantity > 0),
    PRIMARY KEY (parent_product_id, component_product_id),
    CHECK (parent_product_id <> component_product_id)`,
				FKs: []string{
					fkx("bill_of_materials", "bom_parent_fk", "parent_product_id", "products", "id", "ON DELETE CASCADE"),
					fkx("bill_of_materials", "bom_component_fk", "component_product_id", "products", "id", ""),
				},
				Indexes: []string{idx("bill_of_materials", "component_product_id")},
				Cols:    []string{"parent_product_id", "component_product_id", "quantity"},
				Rows:    nBOM,
				Row: func(g *Gen, k int) []any {
					// every 5th product is an assembly of the 3 products after it
					p := ((k-1)/3 + 1) * 5
					c := (p+(k-1)%3)%nProd(g.N) + 1
					return []any{p, c, float64(g.Int(1, 20)) / 2}
				},
			},
			{
				Name: "warehouses", HasID: true,
				DDL: idCol + `,
    code       text NOT NULL UNIQUE,
    name       text NOT NULL,
    address_id integer NOT NULL,
    manager_id integer`,
				FKs: []string{
					fk("warehouses", "address_id", "addresses"),
					fkx("warehouses", "warehouses_manager_id_fk", "manager_id", "employees", "id", "ON DELETE SET NULL"),
				},
				Indexes: []string{idx("warehouses", "address_id"), idx("warehouses", "manager_id")},
				Cols:    []string{"id", "code", "name", "address_id", "manager_id"},
				Rows:    nWh,
				Row: func(g *Gen, k int) []any {
					w := nbWarehouses[k-1]
					return []any{k, w[0], "Warehouse " + w[1], k, g.Int(1, nEmp(g.N))}
				},
			},
			{
				Name: "stock_levels",
				DDL: `warehouse_id  integer NOT NULL,
    product_id    integer NOT NULL,
    qty_on_hand   integer NOT NULL,
    reorder_level integer NOT NULL,
    PRIMARY KEY (warehouse_id, product_id)`,
				FKs: []string{
					fk("stock_levels", "warehouse_id", "warehouses"),
					fkx("stock_levels", "stock_levels_product_id_fk", "product_id", "products", "id", "ON DELETE CASCADE"),
				},
				Indexes: []string{idx("stock_levels", "product_id")},
				Cols:    []string{"warehouse_id", "product_id", "qty_on_hand", "reorder_level"},
				Rows:    nStock,
				Row: func(g *Gen, k int) []any {
					p, j := (k-1)/2+1, (k-1)%2
					return []any{prodWarehouse(g.N, p, j), p, g.Int(0, 2000), g.Int(5, 100)}
				},
			},
			{
				Name: "price_lists", HasID: true,
				DDL: idCol + `,
    name          text NOT NULL UNIQUE,
    currency_code char(3) NOT NULL,
    valid_from    date NOT NULL,
    valid_to      date`,
				FKs:  []string{fkx("price_lists", "price_lists_currency_fk", "currency_code", "currencies", "code", "")},
				Cols: []string{"id", "name", "currency_code", "valid_from", "valid_to"},
				Rows: nPL,
				Row: func(g *Gen, k int) []any {
					c := nbCurrencies[k-1][0]
					return []any{k, "Standard " + c, c, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), nil}
				},
			},
			{
				Name: "price_list_items",
				DDL: `price_list_id integer NOT NULL,
    product_sku   text NOT NULL,
    price         numeric(12,2) NOT NULL,
    PRIMARY KEY (price_list_id, product_sku)`,
				FKs: []string{
					fkx("price_list_items", "price_list_items_price_list_id_fk", "price_list_id", "price_lists", "id", "ON DELETE CASCADE"),
					// references a UNIQUE column, not the primary key
					fkx("price_list_items", "price_list_items_sku_fk", "product_sku", "products", "sku", "ON UPDATE CASCADE ON DELETE CASCADE"),
				},
				Indexes: []string{idx("price_list_items", "product_sku")},
				Cols:    []string{"price_list_id", "product_sku", "price"},
				Rows:    nPLI,
				Row: func(g *Gen, k int) []any {
					np := nProd(g.N)
					pl, p := (k-1)/np+1, (k-1)%np+1
					return []any{pl, sku(p), round2(g.prices[p-1] * nbRates[pl-1])}
				},
			},

			// ======================== sales
			{
				Name: "sales_orders", HasID: true,
				DDL: idCol + `,
    order_number       text NOT NULL UNIQUE,
    customer_id        integer NOT NULL,
    contact_id         integer,
    sales_rep_id       integer,
    price_list_id      integer NOT NULL,
    currency_code      char(3) NOT NULL,
    ship_to_address_id integer NOT NULL,
    order_date         timestamptz NOT NULL DEFAULT now(),
    status             text NOT NULL CHECK (status IN ('new','paid','shipped','delivered','cancelled'))`,
				FKs: []string{
					fk("sales_orders", "customer_id", "customers"),
					// the contact must belong to the same customer
					fkx("sales_orders", "sales_orders_contact_fk", "contact_id, customer_id", "customer_contacts", "id, customer_id", ""),
					fkx("sales_orders", "sales_orders_sales_rep_id_fk", "sales_rep_id", "employees", "id", "ON DELETE SET NULL"),
					fk("sales_orders", "price_list_id", "price_lists"),
					fkx("sales_orders", "sales_orders_currency_fk", "currency_code", "currencies", "code", ""),
					fk("sales_orders", "ship_to_address_id", "addresses"),
				},
				Indexes: []string{idx("sales_orders", "customer_id"), idx("sales_orders", "contact_id"),
					idx("sales_orders", "sales_rep_id"), idx("sales_orders", "price_list_id"),
					idx("sales_orders", "ship_to_address_id"), idx("sales_orders", "order_date")},
				Cols: []string{"id", "order_number", "customer_id", "contact_id", "sales_rep_id", "price_list_id",
					"currency_code", "ship_to_address_id", "order_date", "status"},
				Rows: nOrd,
				Row: func(g *Gen, k int) []any {
					c := orderCustomer(g.N, k)
					contact := optInt(g, 0.8, func() int { return 2*c - 1 + g.Int(0, 1) })
					pl := g.Int(1, nPL(g.N))
					status := []string{"shipped", "delivered", "delivered"}[g.Int(0, 2)]
					if k%4 == 0 {
						status = []string{"new", "paid", "cancelled"}[g.Int(0, 2)]
					}
					return []any{k, fmt.Sprintf("SO%08d", k), c, contact,
						optInt(g, 0.9, func() int { return g.Int(1, nEmp(g.N)) }), pl, nbCurrencies[pl-1][0],
						g.Int(1, nAddr(g.N)), g.Timestamp(2021, 2025), status}
				},
			},
			{
				Name: "sales_order_lines",
				DDL: `order_id     integer NOT NULL,
    line_no      smallint NOT NULL,
    product_id   integer NOT NULL,
    warehouse_id integer NOT NULL,
    quantity     integer NOT NULL CHECK (quantity > 0),
    unit_price   numeric(12,2) NOT NULL,
    PRIMARY KEY (order_id, line_no)`,
				FKs: []string{
					fkx("sales_order_lines", "sales_order_lines_order_id_fk", "order_id", "sales_orders", "id", "ON DELETE CASCADE"),
					fk("sales_order_lines", "product_id", "products"),
					// composite FK: the product must be stocked in that warehouse
					fkx("sales_order_lines", "sales_order_lines_stock_fk", "warehouse_id, product_id", "stock_levels", "warehouse_id, product_id", ""),
				},
				Indexes: []string{idx("sales_order_lines", "product_id"), idx("sales_order_lines", "warehouse_id", "product_id")},
				Cols:    []string{"order_id", "line_no", "product_id", "warehouse_id", "quantity", "unit_price"},
				Rows:    nOL,
				Row: func(g *Gen, k int) []any {
					p := g.Int(1, nProd(g.N))
					return []any{(k-1)/3 + 1, (k-1)%3 + 1, p, prodWarehouse(g.N, p, g.Int(0, 1)), g.Int(1, 10), g.prices[p-1]}
				},
			},
			{
				Name: "shipments", HasID: true,
				DDL: idCol + `,
    shipment_number    text NOT NULL UNIQUE,
    order_id           integer NOT NULL,
    warehouse_id       integer NOT NULL,
    ship_to_address_id integer NOT NULL,
    carrier            text NOT NULL,
    shipped_at         timestamptz NOT NULL`,
				FKs: []string{
					fk("shipments", "order_id", "sales_orders"),
					fk("shipments", "warehouse_id", "warehouses"),
					fk("shipments", "ship_to_address_id", "addresses"),
				},
				Indexes: []string{idx("shipments", "order_id"), idx("shipments", "warehouse_id"), idx("shipments", "ship_to_address_id")},
				Cols:    []string{"id", "shipment_number", "order_id", "warehouse_id", "ship_to_address_id", "carrier", "shipped_at"},
				Rows:    nShip,
				Row: func(g *Gen, k int) []any {
					return []any{k, fmt.Sprintf("SH%08d", k), shipOrder(k), g.Int(1, nWh(g.N)), g.Int(1, nAddr(g.N)),
						g.Pick(nbCarriers), g.Timestamp(2021, 2025)}
				},
			},
			{
				Name: "shipment_lines",
				DDL: `shipment_id   integer NOT NULL,
    line_no       smallint NOT NULL,
    order_id      integer NOT NULL,
    order_line_no smallint NOT NULL,
    quantity      integer NOT NULL,
    PRIMARY KEY (shipment_id, line_no)`,
				FKs: []string{
					fkx("shipment_lines", "shipment_lines_shipment_id_fk", "shipment_id", "shipments", "id", "ON DELETE CASCADE"),
					fkx("shipment_lines", "shipment_lines_order_line_fk", "order_id, order_line_no", "sales_order_lines", "order_id, line_no", ""),
				},
				Indexes: []string{idx("shipment_lines", "order_id", "order_line_no")},
				Cols:    []string{"shipment_id", "line_no", "order_id", "order_line_no", "quantity"},
				Rows:    nSL,
				Row: func(g *Gen, k int) []any {
					s, l := (k-1)/3+1, (k-1)%3+1
					return []any{s, l, shipOrder(s), l, g.Int(1, 10)}
				},
			},
			{
				Name: "invoices", HasID: true,
				DDL: idCol + `,
    invoice_number     text NOT NULL UNIQUE,
    customer_id        integer NOT NULL,
    order_id           integer,
    billing_address_id integer NOT NULL,
    currency_code      char(3) NOT NULL,
    invoice_date       date NOT NULL,
    due_date           date NOT NULL,
    status             text NOT NULL CHECK (status IN ('open','paid','overdue','credited')),
    credit_note_for_id integer,
    CHECK (credit_note_for_id <> id)`,
				FKs: []string{
					fk("invoices", "customer_id", "customers"),
					fkx("invoices", "invoices_order_id_fk", "order_id", "sales_orders", "id", "ON DELETE SET NULL"),
					fk("invoices", "billing_address_id", "addresses"),
					fkx("invoices", "invoices_currency_fk", "currency_code", "currencies", "code", ""),
					fk("invoices", "credit_note_for_id", "invoices"),
				},
				Indexes: []string{idx("invoices", "customer_id"), idx("invoices", "order_id"),
					idx("invoices", "billing_address_id"), idx("invoices", "credit_note_for_id")},
				Cols: []string{"id", "invoice_number", "customer_id", "order_id", "billing_address_id", "currency_code",
					"invoice_date", "due_date", "status", "credit_note_for_id"},
				Rows: nInv,
				Row: func(g *Gen, k int) []any {
					o := shipOrder(k)
					d := g.Date(2021, 2025)
					var credit any
					status := []string{"paid", "paid", "paid", "open", "overdue"}[g.Int(0, 4)]
					if k > 1 && g.Chance(0.03) {
						credit, status = g.Int(1, k-1), "credited"
					}
					return []any{k, fmt.Sprintf("INV%08d", k), orderCustomer(g.N, o), o, g.Int(1, nAddr(g.N)),
						nbCurrencies[g.Int(0, len(nbCurrencies)-1)][0], d, d.AddDate(0, 0, 30), status, credit}
				},
			},
			{
				Name: "invoice_lines",
				DDL: `invoice_id    integer NOT NULL,
    line_no       smallint NOT NULL,
    order_id      integer,
    order_line_no smallint,
    description   text NOT NULL,
    amount        numeric(12,2) NOT NULL,
    PRIMARY KEY (invoice_id, line_no)`,
				FKs: []string{
					fkx("invoice_lines", "invoice_lines_invoice_id_fk", "invoice_id", "invoices", "id", "ON DELETE CASCADE"),
					// nullable composite FK (manual lines have no order line)
					fkx("invoice_lines", "invoice_lines_order_line_fk", "order_id, order_line_no", "sales_order_lines", "order_id, line_no", ""),
				},
				Indexes: []string{idx("invoice_lines", "order_id", "order_line_no")},
				Cols:    []string{"invoice_id", "line_no", "order_id", "order_line_no", "description", "amount"},
				Rows:    nIL,
				Row: func(g *Gen, k int) []any {
					inv, l := (k-1)/3+1, (k-1)%3+1
					if g.Chance(0.1) {
						return []any{inv, l, nil, nil, "Shipping and handling", g.Money(5, 50)}
					}
					return []any{inv, l, shipOrder(inv), l, fmt.Sprintf("Order line %d", l), g.Money(5, 5000)}
				},
			},
			{
				Name: "payments", HasID: true,
				DDL: idCol + `,
    invoice_id integer NOT NULL,
    amount     numeric(12,2) NOT NULL,
    paid_at    timestamptz NOT NULL,
    method     text NOT NULL`,
				FKs:     []string{fk("payments", "invoice_id", "invoices")},
				Indexes: []string{idx("payments", "invoice_id")},
				Cols:    []string{"id", "invoice_id", "amount", "paid_at", "method"},
				Rows:    nPay,
				Row: func(g *Gen, k int) []any {
					return []any{k, k + (k-1)/4, g.Money(5, 10000), g.Timestamp(2021, 2025), g.Pick(nbPayMethods)}
				},
			},

			// ======================== purchasing
			{
				Name: "purchase_orders", HasID: true,
				DDL: idCol + `,
    po_number     text NOT NULL UNIQUE,
    supplier_id   integer NOT NULL,
    buyer_id      integer,
    warehouse_id  integer NOT NULL,
    currency_code char(3) NOT NULL,
    order_date    date NOT NULL,
    status        text NOT NULL,
    UNIQUE (id, supplier_id)`,
				FKs: []string{
					fk("purchase_orders", "supplier_id", "suppliers"),
					fkx("purchase_orders", "purchase_orders_buyer_id_fk", "buyer_id", "employees", "id", "ON DELETE SET NULL"),
					fk("purchase_orders", "warehouse_id", "warehouses"),
					fkx("purchase_orders", "purchase_orders_currency_fk", "currency_code", "currencies", "code", ""),
				},
				Indexes: []string{idx("purchase_orders", "supplier_id"), idx("purchase_orders", "buyer_id"), idx("purchase_orders", "warehouse_id")},
				Cols:    []string{"id", "po_number", "supplier_id", "buyer_id", "warehouse_id", "currency_code", "order_date", "status"},
				Rows:    nPO,
				Row: func(g *Gen, k int) []any {
					return []any{k, fmt.Sprintf("PO%07d", k), poSupplier(g.N, k),
						optInt(g, 0.9, func() int { return g.Int(1, nEmp(g.N)) }), g.Int(1, nWh(g.N)),
						nbCurrencies[g.Int(0, len(nbCurrencies)-1)][0], g.Date(2021, 2025), g.Pick(nbPOStatus)}
				},
			},
			{
				Name: "purchase_order_lines",
				DDL: `po_id       integer NOT NULL,
    line_no     smallint NOT NULL,
    supplier_id integer NOT NULL,
    product_id  integer NOT NULL,
    quantity    integer NOT NULL CHECK (quantity > 0),
    unit_cost   numeric(12,2) NOT NULL,
    PRIMARY KEY (po_id, line_no)`,
				FKs: []string{
					// composite FK to a UNIQUE (id, supplier_id): the line's supplier = the PO's supplier
					fkx("purchase_order_lines", "purchase_order_lines_po_fk", "po_id, supplier_id", "purchase_orders", "id, supplier_id", "ON DELETE CASCADE"),
					// composite FK into the M:N table: the supplier must supply this product
					fkx("purchase_order_lines", "purchase_order_lines_product_supplier_fk", "product_id, supplier_id", "product_suppliers", "product_id, supplier_id", ""),
				},
				Indexes: []string{idx("purchase_order_lines", "product_id", "supplier_id")},
				Cols:    []string{"po_id", "line_no", "supplier_id", "product_id", "quantity", "unit_cost"},
				Rows:    nPOL,
				Row: func(g *Gen, k int) []any {
					ns, np := nSupp(g.N), nProd(g.N)
					po, l := (k-1)/2+1, (k-1)%2+1
					s := poSupplier(g.N, po)
					j := g.Int(0, 1)
					base := ((s-1-j)%ns+ns)%ns + 1 // prodSupplier(base, j) == s
					p := base + g.Int(0, (np-base)/ns)*ns
					return []any{po, l, s, p, g.Int(10, 500), round2(g.prices[p-1] * 0.6)}
				},
			},

			// ======================== projects
			{
				Name: "projects", HasID: true,
				DDL: idCol + `,
    code              text NOT NULL UNIQUE,
    name              text NOT NULL,
    customer_id       integer,
    department_id     integer NOT NULL,
    manager_id        integer NOT NULL,
    parent_project_id integer,
    start_date        date NOT NULL,
    end_date          date,
    CHECK (parent_project_id <> id)`,
				FKs: []string{
					fkx("projects", "projects_customer_id_fk", "customer_id", "customers", "id", "ON DELETE SET NULL"),
					fk("projects", "department_id", "departments"),
					fk("projects", "manager_id", "employees"),
					fk("projects", "parent_project_id", "projects"),
				},
				Indexes: []string{idx("projects", "customer_id"), idx("projects", "department_id"),
					idx("projects", "manager_id"), idx("projects", "parent_project_id")},
				Cols: []string{"id", "code", "name", "customer_id", "department_id", "manager_id", "parent_project_id", "start_date", "end_date"},
				Rows: nProj,
				Row: func(g *Gen, k int) []any {
					var parent any
					if k > 1 && g.Chance(0.2) {
						parent = g.Int(1, k-1)
					}
					start := g.Date(2020, 2025)
					var end any
					if g.Chance(0.6) {
						end = start.AddDate(0, 0, g.Int(30, 700))
					}
					return []any{k, fmt.Sprintf("P%05d", k), fmt.Sprintf("%s %d", g.Pick(projectNames), k),
						optInt(g, 0.6, func() int { return nbSkew(k, nCust(g.N), 3) }), g.Int(1, nDept(g.N)),
						projMember(g.N, k, 0), parent, start, end}
				},
			},
			{
				Name: "project_members",
				DDL: `project_id  integer NOT NULL,
    employee_id integer NOT NULL,
    role        text NOT NULL,
    PRIMARY KEY (project_id, employee_id)`,
				FKs: []string{
					fkx("project_members", "project_members_project_id_fk", "project_id", "projects", "id", "ON DELETE CASCADE"),
					fk("project_members", "employee_id", "employees"),
				},
				Indexes: []string{idx("project_members", "employee_id")},
				Cols:    []string{"project_id", "employee_id", "role"},
				Rows:    nPM,
				Row: func(g *Gen, k int) []any {
					p, j := (k-1)/4+1, (k-1)%4
					role := g.Pick(nbProjectRoles)
					if j == 0 {
						role = "project manager"
					}
					return []any{p, projMember(g.N, p, j), role}
				},
			},
			{
				Name: "timesheets", HasID: true,
				DDL: idCol + `,
    project_id  integer NOT NULL,
    employee_id integer NOT NULL,
    work_date   date NOT NULL,
    hours       numeric(4,1) NOT NULL CHECK (hours > 0 AND hours <= 24),
    description text`,
				FKs: []string{
					// composite FK into the junction table: only project members can book hours
					fkx("timesheets", "timesheets_member_fk", "project_id, employee_id", "project_members", "project_id, employee_id", "ON DELETE CASCADE"),
				},
				Indexes: []string{idx("timesheets", "project_id", "employee_id"), idx("timesheets", "employee_id")},
				Cols:    []string{"id", "project_id", "employee_id", "work_date", "hours", "description"},
				Rows:    nTS,
				Row: func(g *Gen, k int) []any {
					m := (k - 1) / 5
					p, j := m/4+1, m%4
					return []any{k, p, projMember(g.N, p, j), g.Date(2023, 2025), float64(g.Int(1, 16)) / 2, nil}
				},
			},

			// ======================== support
			{
				Name: "tickets", HasID: true,
				DDL: idCol + `,
    ticket_number   text NOT NULL UNIQUE,
    customer_id     integer NOT NULL,
    contact_id      integer,
    assigned_to     integer,
    order_id        integer,
    product_id      integer,
    duplicate_of_id integer,
    status          text NOT NULL,
    priority        text NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    CHECK (duplicate_of_id <> id)`,
				FKs: []string{
					fk("tickets", "customer_id", "customers"),
					fkx("tickets", "tickets_contact_fk", "contact_id, customer_id", "customer_contacts", "id, customer_id", ""),
					fkx("tickets", "tickets_assigned_to_fk", "assigned_to", "employees", "id", "ON DELETE SET NULL"),
					fkx("tickets", "tickets_order_id_fk", "order_id", "sales_orders", "id", "ON DELETE SET NULL"),
					fk("tickets", "product_id", "products"),
					fk("tickets", "duplicate_of_id", "tickets"),
				},
				Indexes: []string{idx("tickets", "customer_id"), idx("tickets", "contact_id"), idx("tickets", "assigned_to"),
					idx("tickets", "order_id"), idx("tickets", "product_id"), idx("tickets", "duplicate_of_id")},
				Cols: []string{"id", "ticket_number", "customer_id", "contact_id", "assigned_to", "order_id", "product_id",
					"duplicate_of_id", "status", "priority", "created_at"},
				Rows: nTick,
				Row: func(g *Gen, k int) []any {
					var dup any
					if k > 1 && g.Chance(0.05) {
						dup = g.Int(1, k-1)
					}
					return []any{k, fmt.Sprintf("T%07d", k), ticketCustomer(g.N, k), ticketContact(g.N, k),
						optInt(g, 0.8, func() int { return g.Int(1, nEmp(g.N)) }),
						optInt(g, 0.4, func() int { return g.Int(1, nOrd(g.N)) }),
						optInt(g, 0.3, func() int { return g.Int(1, nProd(g.N)) }),
						dup, g.Pick(nbTicketStatus), g.Pick(nbPriorities), g.Timestamp(2022, 2025)}
				},
			},
			{
				Name: "ticket_comments", HasID: true,
				DDL: idCol + `,
    ticket_id          integer NOT NULL,
    author_employee_id integer,
    author_contact_id  integer,
    body               text NOT NULL,
    created_at         timestamptz NOT NULL DEFAULT now(),
    CHECK (num_nonnulls(author_employee_id, author_contact_id) = 1)`,
				FKs: []string{
					fkx("ticket_comments", "ticket_comments_ticket_id_fk", "ticket_id", "tickets", "id", "ON DELETE CASCADE"),
					fk("ticket_comments", "author_employee_id", "employees"),
					fk("ticket_comments", "author_contact_id", "customer_contacts"),
				},
				Indexes: []string{idx("ticket_comments", "ticket_id"), idx("ticket_comments", "author_employee_id"), idx("ticket_comments", "author_contact_id")},
				Cols:    []string{"id", "ticket_id", "author_employee_id", "author_contact_id", "body", "created_at"},
				Rows:    nTC,
				Row: func(g *Gen, k int) []any {
					t := (k-1)/3 + 1
					if k%3 == 1 { // first comment comes from the customer's contact
						return []any{k, t, nil, ticketContact(g.N, t), "Customer report", g.Timestamp(2022, 2025)}
					}
					return []any{k, t, g.Int(1, nEmp(g.N)), nil, "Follow-up", g.Timestamp(2022, 2025)}
				},
			},

			// ======================== edge cases for copy tools
			{
				Name: "attachments", HasID: true,
				DDL: idCol + `,
    entity_type text NOT NULL CHECK (entity_type IN ('customer','sales_order','ticket','product')),
    entity_id   integer NOT NULL,
    file_name   text NOT NULL,
    size_bytes  bigint NOT NULL,
    uploaded_by integer`,
				// polymorphic: entity_type/entity_id has NO foreign key on purpose
				FKs:     []string{fkx("attachments", "attachments_uploaded_by_fk", "uploaded_by", "employees", "id", "ON DELETE SET NULL")},
				Indexes: []string{idx("attachments", "entity_type", "entity_id"), idx("attachments", "uploaded_by")},
				Cols:    []string{"id", "entity_type", "entity_id", "file_name", "size_bytes", "uploaded_by"},
				Rows:    nAtt,
				Row: func(g *Gen, k int) []any {
					t := g.Pick(nbEntityTypes)
					var id int
					switch t {
					case "customer":
						id = g.Int(1, nCust(g.N))
					case "sales_order":
						id = g.Int(1, nOrd(g.N))
					case "ticket":
						id = g.Int(1, nTick(g.N))
					default:
						id = g.Int(1, nProd(g.N))
					}
					return []any{k, t, id, fmt.Sprintf("%s_%d_%d.pdf", t, id, k), g.Int(10_000, 5_000_000),
						optInt(g, 0.9, func() int { return g.Int(1, nEmp(g.N)) })}
				},
			},
			{
				Name: "app_settings",
				DDL: `key   text PRIMARY KEY,
    value text NOT NULL`,
				Cols: []string{"key", "value"},
				Rows: nSet,
				Row: func(g *Gen, k int) []any {
					s := nbSettings[k-1]
					return []any{s[0], s[1]}
				},
			},
		},
		Views: []string{
			// every foreign key in this schema: handy input for a copy tool
			`CREATE VIEW {s}.fk_graph AS
SELECT c.conrelid::regclass  AS child_table,
       (SELECT string_agg(a.attname, ', ' ORDER BY k.ord)
          FROM unnest(c.conkey) WITH ORDINALITY k(attnum, ord)
          JOIN pg_attribute a ON a.attrelid = c.conrelid AND a.attnum = k.attnum) AS child_columns,
       c.confrelid::regclass AS parent_table,
       (SELECT string_agg(a.attname, ', ' ORDER BY k.ord)
          FROM unnest(c.confkey) WITH ORDINALITY k(attnum, ord)
          JOIN pg_attribute a ON a.attrelid = c.confrelid AND a.attnum = k.attnum) AS parent_columns,
       c.conname AS constraint_name,
       CASE c.confdeltype WHEN 'a' THEN 'NO ACTION' WHEN 'r' THEN 'RESTRICT' WHEN 'c' THEN 'CASCADE'
                          WHEN 'n' THEN 'SET NULL' WHEN 'd' THEN 'SET DEFAULT' END AS on_delete,
       c.condeferrable AS is_deferrable,
       c.conrelid = c.confrelid AS self_reference
FROM pg_constraint c
WHERE c.contype = 'f' AND c.connamespace = '{s}'::regnamespace`,
			`CREATE VIEW {s}.department_tree AS
WITH RECURSIVE t AS (
    SELECT id, name, parent_id, manager_id, 1 AS depth, name AS path
    FROM {s}.departments WHERE parent_id IS NULL
    UNION ALL
    SELECT d.id, d.name, d.parent_id, d.manager_id, t.depth + 1, t.path || ' > ' || d.name
    FROM {s}.departments d JOIN t ON d.parent_id = t.id
)
SELECT * FROM t`,
			`CREATE VIEW {s}.order_totals AS
SELECT o.id, o.order_number, o.customer_id, c.name AS customer, o.status, o.currency_code,
       count(l.line_no) AS lines, sum(l.quantity * l.unit_price) AS total
FROM {s}.sales_orders o
JOIN {s}.customers c ON c.id = o.customer_id
LEFT JOIN {s}.sales_order_lines l ON l.order_id = o.id
GROUP BY o.id, c.name`,
			`CREATE VIEW {s}.customer_activity AS
SELECT c.id, c.customer_number, c.name,
       (SELECT count(*) FROM {s}.sales_orders o WHERE o.customer_id = c.id) AS orders,
       (SELECT count(*) FROM {s}.tickets t WHERE t.customer_id = c.id)      AS tickets,
       (SELECT count(*) FROM {s}.customers s WHERE s.parent_customer_id = c.id) AS subsidiaries
FROM {s}.customers c`,
		},
	}
}
