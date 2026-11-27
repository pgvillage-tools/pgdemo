package main

import (
	"fmt"
	"time"
)

// Nibble mini: a smaller Nibble (11 tables) built to test the table class
// "Parent only" in pgTestBuilder. Its centre is addresses: a customer's
// address book (addresses.customer_id -> customers) that orders, shipments,
// invoices, employees, suppliers and warehouses also point to. With customers
// as driving table, addresses on Follow are reached "down" (the addresses of
// the driving customers) and pull in every order, shipment and invoice to those
// addresses, also of other customers (20% of the orders go to an address of
// another customer). With addresses on Parent only they only come "up", as the
// address of selected rows, and bring their own parent (the owner) along.
// Also on addresses: created_by, an employee id without a foreign key (a
// virtual FK on a parent-only table).
//
// Customer and delivery address of an order are computed from the order number
// (not drawn), so shipments go to the delivery address of their order and
// invoices to the billing address of its customer.

var miniAddressKinds = []string{"home", "home", "office", "delivery", "billing"}

// miniMix is a fixed pseudo-random number for (salt, i).
func miniMix(salt uint64, i int) uint64 { return nbMix(salt<<40 ^ uint64(i)) }

func nibbleMini() *Template {
	nDept := func(int) int { return len(departments) }
	nCust := func(n int) int { return max(50, n) }
	nSupp := func(n int) int { return max(10, n/20) }
	const nWh = 6
	nEmp := func(n int) int { return max(30, n/10) }
	// addresses: 2 per customer, then one per supplier, warehouse and employee
	firstSupplierAddr := func(n int) int { return 2*nCust(n) + 1 }
	firstWarehouseAddr := func(n int) int { return firstSupplierAddr(n) + nSupp(n) }
	firstEmployeeAddr := func(n int) int { return firstWarehouseAddr(n) + nWh }
	nAddr := func(n int) int { return firstEmployeeAddr(n) - 1 + nEmp(n) }
	nProd := func(n int) int { return max(30, n/2) }
	nOrder := func(n int) int { return 3 * nCust(n) }
	nShip := func(n int) int { return nOrder(n) * 3 / 4 } // 3 of every 4 orders shipped
	nInv := func(n int) int { return nOrder(n) * 9 / 10 } // 9 of every 10 invoiced

	orderCustomer := func(n, o int) int { return 1 + int(miniMix(1, o)%uint64(nCust(n))) }
	// 80% to one of the two addresses of the order's own customer, 20% to an
	// address of another customer (a gift, a drop shipment)
	deliveryAddress := func(n, o int) int {
		c := orderCustomer(n, o)
		if miniMix(2, o)%5 != 0 {
			return 2*(c-1) + 1 + int(miniMix(3, o)%2)
		}
		other := 1 + int(miniMix(4, o)%uint64(nCust(n)))
		if other == c {
			other = c%nCust(n) + 1
		}
		return 2*(other-1) + 1 + int(miniMix(3, o)%2)
	}
	shipOrder := func(k int) int { return (k-1)*4/3 + 1 }
	invOrder := func(k int) int { return (k-1)*10/9 + 1 }
	day := func(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }
	orderDate := func(o int) time.Time { return day(2023, 1, 1).AddDate(0, 0, int(miniMix(5, o)%1000)) }

	return &Template{
		Name: "Nibble mini",
		Description: "A smaller Nibble (11 tables) for testing \"Parent only\" in pgTestBuilder: addresses is a customer's " +
			"address book that orders, shipments, invoices, employees, suppliers and warehouses also point to; 20% of " +
			"the orders go to another customer's address. With addresses on Follow the selection grows through them, " +
			"with Parent only it does not.",
		Tables: []Table{
			{
				Name: "departments", HasID: true,
				DDL: idCol + `,
    name text NOT NULL UNIQUE,
    city text NOT NULL`,
				Cols: []string{"id", "name", "city"},
				Rows: nDept,
				Row:  func(g *Gen, k int) []any { return []any{k, departments[k-1][0], departments[k-1][1]} },
			},
			{
				Name: "employees", HasID: true,
				DDL: idCol + `,
    first_name    text NOT NULL,
    last_name     text NOT NULL,
    email         text NOT NULL UNIQUE,
    department_id integer NOT NULL,
    manager_id    integer,
    address_id    integer NOT NULL,
    CHECK (manager_id <> id)`,
				FKs: []string{fk("employees", "department_id", "departments"), fk("employees", "manager_id", "employees"),
					fk("employees", "address_id", "addresses")},
				Indexes: []string{idx("employees", "department_id"), idx("employees", "manager_id"), idx("employees", "address_id")},
				Cols:    []string{"id", "first_name", "last_name", "email", "department_id", "manager_id", "address_id"},
				Rows:    nEmp,
				Row: func(g *Gen, k int) []any {
					f, l := g.FirstName(), g.LastName()
					var manager any
					if k > 1 {
						manager = g.Int(1, min(k-1, 10)) // the first ten lead the others
					}
					return []any{k, f, l, Email(f, l, k, "nibble.example"), g.Int(1, nDept(g.N)), manager,
						firstEmployeeAddr(g.N) + k - 1}
				},
			},
			{
				Name: "customers", HasID: true,
				DDL: idCol + `,
    customer_number    text NOT NULL UNIQUE,
    name               text NOT NULL,
    account_manager_id integer,
    created_at         timestamptz NOT NULL`,
				FKs:     []string{fk("customers", "account_manager_id", "employees")},
				Indexes: []string{idx("customers", "account_manager_id")},
				Cols:    []string{"id", "customer_number", "name", "account_manager_id", "created_at"},
				Rows:    nCust,
				Row: func(g *Gen, k int) []any {
					var am any
					if g.Chance(0.8) {
						am = g.Int(1, nEmp(g.N))
					}
					return []any{k, fmt.Sprintf("C%07d", k), g.LastName() + " " + g.CompanySuffix(), am, g.Timestamp(2015, 2024)}
				},
			},
			{
				Name: "addresses", HasID: true,
				DDL: idCol + `,
    customer_id         integer,
    kind                text NOT NULL,
    street              text NOT NULL,
    house_number        integer NOT NULL,
    postcode            text NOT NULL,
    city                text NOT NULL,
    created_by          integer NOT NULL`,
				// created_by refers to employees(id) without a foreign key on purpose
				FKs:     []string{fk("addresses", "customer_id", "customers")},
				Indexes: []string{idx("addresses", "customer_id")},
				Cols:    []string{"id", "customer_id", "kind", "street", "house_number", "postcode", "city", "created_by"},
				Rows:    nAddr,
				Row: func(g *Gen, k int) []any {
					var owner any
					kind := "office" // suppliers, warehouses
					switch {
					case k < firstSupplierAddr(g.N):
						owner = (k-1)/2 + 1
						kind = g.Pick(miniAddressKinds)
					case k >= firstEmployeeAddr(g.N):
						kind = "home"
					}
					return []any{k, owner, kind, g.Street(), g.Int(1, 250), g.Postcode(), g.City(), g.Int(1, nEmp(g.N))}
				},
			},
			{
				Name: "suppliers", HasID: true,
				DDL: idCol + `,
    name       text NOT NULL,
    address_id integer NOT NULL UNIQUE`,
				FKs:  []string{fk("suppliers", "address_id", "addresses")},
				Cols: []string{"id", "name", "address_id"},
				Rows: nSupp,
				Row: func(g *Gen, k int) []any {
					return []any{k, g.Pick(nbSupplierWords) + " " + g.Pick(nbSupplierNouns), firstSupplierAddr(g.N) + k - 1}
				},
			},
			{
				Name: "warehouses", HasID: true,
				DDL: idCol + `,
    name       text NOT NULL UNIQUE,
    address_id integer NOT NULL UNIQUE`,
				FKs:  []string{fk("warehouses", "address_id", "addresses")},
				Cols: []string{"id", "name", "address_id"},
				Rows: func(int) int { return nWh },
				Row: func(g *Gen, k int) []any {
					return []any{k, fmt.Sprintf("Warehouse %c", 'A'+k-1), firstWarehouseAddr(g.N) + k - 1}
				},
			},
			{
				Name: "products", HasID: true,
				DDL: idCol + `,
    sku         text NOT NULL UNIQUE,
    name        text NOT NULL,
    supplier_id integer NOT NULL,
    price       numeric(10,2) NOT NULL`,
				FKs:     []string{fk("products", "supplier_id", "suppliers")},
				Indexes: []string{idx("products", "supplier_id")},
				Cols:    []string{"id", "sku", "name", "supplier_id", "price"},
				Rows:    nProd,
				Row: func(g *Gen, k int) []any {
					return []any{k, fmt.Sprintf("P%06d", k), g.Pick(productAdjectives) + " " + g.Pick(productNouns),
						g.Int(1, nSupp(g.N)), g.Money(2, 500)}
				},
			},
			{
				Name: "orders", HasID: true,
				DDL: idCol + `,
    customer_id         integer NOT NULL,
    delivery_address_id integer NOT NULL,
    sales_rep_id        integer,
    order_date          date NOT NULL,
    status              text NOT NULL`,
				FKs: []string{fk("orders", "customer_id", "customers"), fk("orders", "delivery_address_id", "addresses"),
					fk("orders", "sales_rep_id", "employees")},
				Indexes: []string{idx("orders", "customer_id"), idx("orders", "delivery_address_id"), idx("orders", "sales_rep_id")},
				Cols:    []string{"id", "customer_id", "delivery_address_id", "sales_rep_id", "order_date", "status"},
				Rows:    nOrder,
				Row: func(g *Gen, k int) []any {
					var rep any
					if g.Chance(0.6) {
						rep = g.Int(1, nEmp(g.N))
					}
					return []any{k, orderCustomer(g.N, k), deliveryAddress(g.N, k), rep, orderDate(k), g.Pick(orderStatuses)}
				},
			},
			{
				Name: "order_lines", HasID: true,
				DDL: idCol + `,
    order_id   integer NOT NULL,
    product_id integer NOT NULL,
    quantity   integer NOT NULL,
    unit_price numeric(10,2) NOT NULL`,
				FKs:     []string{fk("order_lines", "order_id", "orders"), fk("order_lines", "product_id", "products")},
				Indexes: []string{idx("order_lines", "order_id"), idx("order_lines", "product_id")},
				Cols:    []string{"id", "order_id", "product_id", "quantity", "unit_price"},
				Rows:    func(n int) int { return 3 * nOrder(n) },
				Row: func(g *Gen, k int) []any {
					return []any{k, (k-1)/3 + 1, g.Int(1, nProd(g.N)), g.Int(1, 10), g.Money(2, 500)}
				},
			},
			{
				Name: "shipments", HasID: true,
				DDL: idCol + `,
    order_id     integer NOT NULL,
    warehouse_id integer NOT NULL,
    address_id   integer NOT NULL,
    shipped_at   timestamptz NOT NULL`,
				FKs: []string{fk("shipments", "order_id", "orders"), fk("shipments", "warehouse_id", "warehouses"),
					fk("shipments", "address_id", "addresses")},
				Indexes: []string{idx("shipments", "order_id"), idx("shipments", "warehouse_id"), idx("shipments", "address_id")},
				Cols:    []string{"id", "order_id", "warehouse_id", "address_id", "shipped_at"},
				Rows:    nShip,
				Row: func(g *Gen, k int) []any {
					o := shipOrder(k)
					at := orderDate(o).Add(time.Duration(g.Int(4, 96)) * time.Hour)
					return []any{k, o, g.Int(1, nWh), deliveryAddress(g.N, o), at}
				},
			},
			{
				Name: "invoices", HasID: true,
				DDL: idCol + `,
    invoice_number     text NOT NULL UNIQUE,
    order_id           integer NOT NULL UNIQUE,
    billing_address_id integer NOT NULL,
    amount             numeric(12,2) NOT NULL,
    invoice_date       date NOT NULL`,
				FKs:     []string{fk("invoices", "order_id", "orders"), fk("invoices", "billing_address_id", "addresses")},
				Indexes: []string{idx("invoices", "billing_address_id")},
				Cols:    []string{"id", "invoice_number", "order_id", "billing_address_id", "amount", "invoice_date"},
				Rows:    nInv,
				Row: func(g *Gen, k int) []any {
					o := invOrder(k)
					c := orderCustomer(g.N, o)
					return []any{k, fmt.Sprintf("INV%08d", k), o, 2*(c-1) + 1, g.Money(10, 5000), orderDate(o).AddDate(0, 0, 1)}
				},
			},
		},
	}
}
