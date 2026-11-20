package main

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// ---------------------------------------------------------------- Nibble enterprise
//
// Three schemas that belong to one application, with foreign keys between them:
//
//   <name>_core   core: Nibble secure, unchanged (same data for the same seed and size)
//   <name>_hr     HR: contracts, positions, leave, absence (medical notes), daily
//                 attendance for 2025, overtime, monthly payslips for 2025,
//                 emergency contacts, ID documents, reviews, trainings
//   <name>_fleet  lease cars: lease companies, models, cars, lease contracts, fuel
//                 cards and transactions, trip log, damages, traffic fines,
//                 maintenance, private-use declarations
//
// Links between the schemas:
//   hr    -> core   employees, departments, persons(bsn) (natural key)
//   fleet -> core   employees, vehicles, persons(bsn)
//   fleet -> hr     salary_scales, contracts
//   hr    -> fleet  payslip_lines.lease_contract_id (so hr and fleet refer to each other)
//   no FK on purpose (virtual FK tests): hr.hr_notes.employee_id -> core employees(id),
//   fleet.traffic_fines.license_plate -> fleet.cars(license_plate), with some
//   plates that are not in the fleet (orphans).
//
// Cross-table consistency comes from arithmetic on the employee number, not from
// stored state: employee k has salary scale entScale(k), every employee whose
// number ends in 1, 4 or 7 drives a lease car (lease contract entLeaseIdx(k),
// car with the same number), and HR contract k is employee k's current contract.
//
// All values are generated; any resemblance to real people, companies or
// vehicles is chance.

const entYear = 2025

// entWorkdays are the weekdays of entYear (public holidays are ignored).
var entWorkdays = func() []time.Time {
	var days []time.Time
	for d := time.Date(entYear, 1, 1, 0, 0, 0, 0, time.UTC); d.Year() == entYear; d = d.AddDate(0, 0, 1) {
		if wd := d.Weekday(); wd != time.Saturday && wd != time.Sunday {
			days = append(days, d)
		}
	}
	return days
}()

var entJobTitles = []string{
	"Software Developer", "Senior Software Developer", "Data Analyst", "Business Analyst", "Consultant",
	"Senior Consultant", "Project Manager", "Team Lead", "Account Manager", "Sales Representative",
	"Customer Support Agent", "Support Team Lead", "Warehouse Operator", "Logistics Planner",
	"Purchaser", "Financial Controller", "Accountant", "Payroll Administrator", "HR Advisor",
	"Recruiter", "Office Manager", "Management Assistant", "System Administrator", "Security Officer",
	"UX Designer", "Product Owner", "Scrum Master", "Quality Engineer", "Facility Coordinator", "Director",
}

var entLeaveTypes = []struct {
	code, name string
	paid       bool
}{
	{"VAC", "Statutory vacation", true}, {"VACX", "Extra-statutory vacation", true}, {"ATV", "Reduced working hours (ATV)", true},
	{"SPEC", "Special leave", true}, {"PAR", "Parental leave", false}, {"CARE", "Short-term care leave", true},
	{"UNP", "Unpaid leave", false}, {"MOVE", "Moving day", true},
}

var entTrainingWords1 = []string{"Advanced", "Practical", "Introduction to", "Masterclass", "Certified", "Hands-on"}
var entTrainingWords2 = []string{"PostgreSQL", "Leadership", "Negotiation", "GDPR", "Project Management", "Time Management",
	"First Aid (BHV)", "Excel", "Customer Service", "Agile", "Forklift Safety", "Presentation Skills"}
var entTrainingProviders = []string{"Academie Noord", "Kennispunt", "SkillHouse", "Opleidingscentrum Midden", "LearnLab"}

var entLeaveReasons = []string{
	"Vacation", "Vacation", "Vacation", "Family visit", "Doctor's appointment", "School holiday",
	"Wedding of my sister %s", "Funeral of my grandmother", "Moving to %s", "Care for my partner %s after surgery",
	"Birth of our child", "Study leave",
}

var entMedicalNotes = []string{
	"Flu, expects to be back within a week.",
	"Back complaints; treated by physiotherapist %s (phone %s).",
	"Burn-out symptoms; guided by the company doctor, reintegration plan agreed.",
	"Knee surgery at %s hospital, recovery about six weeks.",
	"Migraine, recurring. Employee asked not to share this with the team.",
	"Pregnancy-related absence; due date reported to HR only.",
	"Broken wrist after a bicycle accident; can do partial office work.",
	"Long COVID complaints; works 50%% on therapeutic basis.",
}

var entRelationships = []string{"partner", "partner", "mother", "father", "sister", "brother", "friend", "neighbour"}
var entReviewPhrases = []string{
	"Strong results this year, clients are positive.", "Needs to improve planning and communication.",
	"Took over the role of %s during long-term absence.", "Excellent technical skills, ready for the next scale.",
	"Conflict with colleague %s discussed and resolved.", "Meets expectations.",
}

var entLeaseCompanies = []string{
	"Autolease Noord B.V.", "Fleetwise Mobiliteit B.V.", "Rijnmond Lease B.V.", "Kompas Autolease B.V.",
	"Polder Fleet Services B.V.", "Veluwe Wagenparkbeheer B.V.", "Zuiderlicht Lease B.V.", "Kustlijn Mobility B.V.",
}

// car models: make, model, fuel, CO2 g/km, list price
var entCarModels = []struct {
	make, model, fuel string
	co2               int
	price             float64
}{
	{"Volkswagen", "ID.3", "electric", 0, 38990}, {"Volkswagen", "ID.4", "electric", 0, 46990},
	{"Volkswagen", "Golf", "petrol", 128, 34490}, {"Volkswagen", "Passat Variant", "hybrid", 32, 49990},
	{"Kia", "Niro EV", "electric", 0, 44990}, {"Kia", "EV6", "electric", 0, 52990}, {"Kia", "Ceed SW", "petrol", 135, 31990},
	{"Hyundai", "Kona Electric", "electric", 0, 41990}, {"Hyundai", "Ioniq 5", "electric", 0, 49990},
	{"Skoda", "Enyaq", "electric", 0, 47990}, {"Skoda", "Octavia Combi", "petrol", 125, 36990},
	{"Skoda", "Superb Combi", "hybrid", 28, 51990}, {"Peugeot", "e-208", "electric", 0, 36990},
	{"Peugeot", "308 SW", "hybrid", 30, 42990}, {"Renault", "Megane E-Tech", "electric", 0, 42990},
	{"Toyota", "Corolla Touring Sports", "hybrid", 102, 38990}, {"Toyota", "bZ4X", "electric", 0, 47990},
	{"Tesla", "Model 3", "electric", 0, 44990}, {"Tesla", "Model Y", "electric", 0, 49990},
	{"Volvo", "EX30", "electric", 0, 39990}, {"Volvo", "V60", "hybrid", 22, 62990}, {"BMW", "i4", "electric", 0, 64990},
	{"Audi", "Q4 e-tron", "electric", 0, 56990}, {"Mercedes-Benz", "EQA", "electric", 0, 58990},
}

var entColours = []string{"black", "white", "grey", "silver", "dark blue", "red", "green"}
var entFuelStations = []string{"Tankpunt", "Snellaad Plein", "Pomp & Co", "LaadHub", "Brandstof Direct", "Oase Tankstation"}
var entGarages = []string{"Garage De Brug", "Autobedrijf Van Dam", "Bandencentrum Oost", "Autoservice Centrum", "Garage Noorderpoort"}
var entOffences = []struct {
	text   string
	amount float64
}{
	{"Speeding 6 km/h over the limit (built-up area)", 69}, {"Speeding 12 km/h over the limit (motorway)", 112},
	{"Speeding 19 km/h over the limit", 230}, {"Parking without a valid ticket", 110}, {"Red light", 290},
	{"Using a phone while driving", 430}, {"Driving in a bus lane", 110},
}
var entTripPurposes = []string{"business", "business", "business", "commute", "commute", "private"}
var entDamages = []string{
	"Scratch on the rear bumper, parking garage.", "Windscreen chip from a stone on the A2.",
	"Collision at a roundabout; other driver %s gave right of way too late.", "Side mirror broken by a passing van (plate %s).",
	"Dent in the door, cause unknown.", "Rear-end collision in a traffic jam, other party %s, phone %s.",
}
var entMaintenance = []string{"Annual service", "Tyre change (winter tyres)", "Tyre change (summer tyres)", "Brake pads replaced",
	"Software update", "Air conditioning service", "Windscreen repair"}

// entScale gives employee k's salary scale (1..12). Lease drivers have scale 8..12.
func entScale(k int) int {
	h := nbMix(uint64(k)*0x9E37 + 17)
	if entIsLease(k) {
		return 8 + int(h%5)
	}
	return 1 + int(h%11)
}

func entScaleMin(s int) float64 { return 2200 + float64(s-1)*450 }
func entScaleMax(s int) float64 { return entScaleMin(s) * 1.35 }

// entSalary is employee k's gross monthly salary (fixed for the whole year).
func entSalary(k int) float64 {
	s := entScale(k)
	f := float64(nbMix(uint64(k)*0xC2B2+5)%1000) / 1000
	return round2(entScaleMin(s) + f*(entScaleMax(s)-entScaleMin(s)))
}

// lease drivers: employees whose number ends in 1, 4 or 7
func entIsLease(k int) bool { r := k % 10; return r == 1 || r == 4 || r == 7 }

// entLeaseIdx is the number of employee k's lease contract (and car), 1-based.
func entLeaseIdx(k int) int {
	m := map[int]int{1: 0, 4: 1, 7: 2}[k%10]
	return (k/10)*3 + m + 1
}

// entLeaseEmp is the inverse of entLeaseIdx.
func entLeaseEmp(i int) int { return ((i-1)/3)*10 + []int{1, 4, 7}[(i-1)%3] }

// entNLease is the number of lease drivers among e employees.
func entNLease(e int) int {
	n := (e / 10) * 3
	for _, r := range []int{1, 4, 7} {
		if r <= e%10 {
			n++
		}
	}
	return n
}

// entCarModel is the model (index in entCarModels) of car i.
func entCarModel(i int) int { return int(nbMix(uint64(i)*0x51ED+3) % uint64(len(entCarModels))) }

// entBenefit is the monthly taxable benefit (private use) of car i.
func entBenefit(i int) float64 {
	m := entCarModels[entCarModel(i)]
	pct := 22.0
	if m.fuel == "electric" {
		pct = 17
	}
	return round2(m.price * pct / 100 / 12)
}

// entPayslip is one month's pay of an employee. net = salary - pension - tax +
// travel + other (the benefit of a lease car is taxed but not paid out, the
// holiday allowance is reserved).
type entPayslip struct {
	salary, benefit, travel, other, gross, pension, tax, net float64
}

func entPay(emp, month int) entPayslip {
	p := entPayslip{salary: entSalary(emp)}
	if entIsLease(emp) {
		p.benefit = entBenefit(entLeaseIdx(emp))
	} else {
		p.other = 25
	}
	p.travel = float64(nbMix(uint64(emp)*13+uint64(month))%18000) / 100
	p.gross = round2(p.salary + p.benefit)
	p.pension = round2(p.salary * 0.06)
	p.tax = round2(p.gross * 0.33)
	p.net = round2(p.salary - p.pension - p.tax + p.travel + p.other)
	return p
}

// entPlate gives a unique NL licence plate (sidecode 8, XX-999-X) for number i.
func entPlate(i int) string {
	const space = 16 * 16 * 1000 * 16
	x := (uint64(i)*2654435761 + 12345) % space // bijection on 0..space-1 (odd multiplier, space is 2^16*1000)
	l := func(v uint64) byte { return plateLetters[v%16] }
	return fmt.Sprintf("%c%c-%03d-%c", l(x/16000/16), l(x/16000), (x/16)%1000, l(x))
}

// entVIN gives a unique 17-character vehicle identification number for car i.
func entVIN(i int) string {
	wmi := []string{"WVW", "KNA", "TMB", "VF3", "VF1", "JTD", "5YJ", "YV1", "WBA", "WAU"}
	// i -> i*999999937 mod 10^11 is a bijection (the multiplier is coprime to 10), so VINs are unique
	return fmt.Sprintf("%s%s%011d", wmi[nbMix(uint64(i))%uint64(len(wmi))], "ZZZ", uint64(i)*999999937%100000000000)
}

func entAddress(g *Gen) string { return g.ShortAddress() }

func entDay(k int) time.Time { return entWorkdays[k%len(entWorkdays)] }

// entOpt returns f() with probability p, otherwise nil (NULL).
func entOpt(g *Gen, p float64, f func() int) any {
	if g.Chance(p) {
		return f()
	}
	return nil
}

func nibbleEnterprise() *Template {
	t := nibbleSecure()
	t.Name = "Nibble enterprise"
	t.Description = "Three schemas with foreign keys between them: <name>_core (Nibble secure), <name>_hr (contracts, leave, " +
		"absence with medical notes, daily attendance and payslips for 2025) and <name>_fleet (lease cars, fuel cards, " +
		"trip log, traffic fines). Large: about 140 rows per unit of size."
	t.Schemas = []string{"hr", "fleet"}
	t.MainSuffix = "core"

	nEmp := findTable(t, "employees").Rows
	nContact := findTable(t, "customer_contacts").Rows
	nVehicle := findTable(t, "vehicles").Rows
	nDept := findTable(t, "departments").Rows
	bsnOfEmp := func(g *Gen, k int) string { return g.NatID(nContact(g.N) + k) }

	const hr, fl = "hr", "fleet"
	wd := len(entWorkdays)
	nLease := func(n int) int { return entNLease(nEmp(n)) }
	nCars := func(n int) int { return nLease(n) + max(1, nLease(n)/10) } // plus pool cars
	fixed := func(c int) func(int) int { return func(int) int { return c } }
	coreEmp := func(table, col string) string {
		return fks(hr, table, table+"_"+col+"_fk", col, "", "employees", "id", "")
	}
	flEmp := func(table, col string) string {
		return fks(fl, table, table+"_"+col+"_fk", col, "", "employees", "id", "")
	}
	idCol64 := "id bigint GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY"

	// ======================================================== HR
	hrTables := []Table{
		{
			Name: "job_titles", Schema: hr, HasID: true,
			DDL:  idCol + ",\n    title text NOT NULL UNIQUE",
			Cols: []string{"id", "title"},
			Rows: fixed(len(entJobTitles)),
			Row:  func(g *Gen, k int) []any { return []any{k, entJobTitles[k-1]} },
		},
		{
			Name: "salary_scales", Schema: hr, HasID: true,
			DDL: idCol + `,
    scale_code  text NOT NULL UNIQUE,
    min_monthly numeric(10,2) NOT NULL,
    max_monthly numeric(10,2) NOT NULL,
    CHECK (max_monthly >= min_monthly)`,
			Cols: []string{"id", "scale_code", "min_monthly", "max_monthly"},
			Rows: fixed(12),
			Row: func(g *Gen, k int) []any {
				return []any{k, fmt.Sprintf("S%02d", k), entScaleMin(k), round2(entScaleMax(k))}
			},
		},
		{
			Name: "leave_types", Schema: hr, HasID: true,
			DDL:  idCol + ",\n    code text NOT NULL UNIQUE,\n    name text NOT NULL,\n    paid boolean NOT NULL",
			Cols: []string{"id", "code", "name", "paid"},
			Rows: fixed(len(entLeaveTypes)),
			Row: func(g *Gen, k int) []any {
				l := entLeaveTypes[k-1]
				return []any{k, l.code, l.name, l.paid}
			},
		},
		{
			Name: "trainings", Schema: hr, HasID: true,
			DDL:  idCol + ",\n    name     text NOT NULL,\n    provider text NOT NULL,\n    hours    integer NOT NULL,\n    cost     numeric(10,2) NOT NULL",
			Cols: []string{"id", "name", "provider", "hours", "cost"},
			Rows: fixed(40),
			Row: func(g *Gen, k int) []any {
				name := entTrainingWords1[(k-1)%len(entTrainingWords1)] + " " + entTrainingWords2[(k-1)%len(entTrainingWords2)]
				return []any{k, name, g.Pick(entTrainingProviders), g.Int(4, 40), g.Money(250, 3500)}
			},
		},
		// contract k (k <= employees) is employee k's current contract; the rest are earlier ones
		{
			Name: "contracts", Schema: hr, HasID: true,
			DDL: idCol + `,
    employee_id         integer NOT NULL,
    contract_type       text NOT NULL,
    start_date          date NOT NULL,
    end_date            date,
    fte                 numeric(3,2) NOT NULL,
    salary_scale_id     integer NOT NULL,
    monthly_salary      numeric(10,2) NOT NULL,
    notice_period_weeks integer NOT NULL,
    CHECK (end_date IS NULL OR end_date > start_date)`,
			FKs: []string{coreEmp("contracts", "employee_id"),
				fks(hr, "contracts", "contracts_salary_scale_id_fk", "salary_scale_id", hr, "salary_scales", "id", "")},
			Indexes: []string{idxs(hr, "contracts", "employee_id"), idxs(hr, "contracts", "salary_scale_id")},
			Cols:    []string{"id", "employee_id", "contract_type", "start_date", "end_date", "fte", "salary_scale_id", "monthly_salary", "notice_period_weeks"},
			Rows:    func(n int) int { return nEmp(n) + 3*nEmp(n)/10 },
			Row: func(g *Gen, k int) []any {
				e := nEmp(g.N)
				fte := []float64{1, 1, 1, 0.9, 0.8, 0.6, 0.5}[g.Int(0, 6)]
				if k <= e {
					start := g.Date(2012, 2025)
					var end any
					typ := "permanent"
					if g.Chance(0.2) {
						typ = "temporary"
						end = start.AddDate(1, 0, 0)
					}
					return []any{k, k, typ, start, end, fte, entScale(k), round2(entSalary(k) * fte), g.Int(4, 8)}
				}
				emp := nbSkew(k, e, 11)
				start := g.Date(2005, 2010)
				s := max(1, entScale(emp)-g.Int(1, 3))
				return []any{k, emp, g.Pick([]string{"temporary", "temporary", "on-call", "internship"}), start,
					start.AddDate(g.Int(1, 4), 0, 0), fte, s, round2(entScaleMin(s) * fte), 4}
			},
		},
		{
			Name: "employee_positions", Schema: hr, HasID: true,
			DDL: idCol + `,
    employee_id   integer NOT NULL,
    department_id integer NOT NULL,
    job_title_id  integer NOT NULL,
    valid_from    date NOT NULL,
    valid_to      date,
    CHECK (valid_to IS NULL OR valid_to > valid_from)`,
			FKs: []string{coreEmp("employee_positions", "employee_id"),
				fks(hr, "employee_positions", "employee_positions_department_id_fk", "department_id", "", "departments", "id", ""),
				fks(hr, "employee_positions", "employee_positions_job_title_id_fk", "job_title_id", hr, "job_titles", "id", "")},
			Indexes: []string{idxs(hr, "employee_positions", "employee_id"), idxs(hr, "employee_positions", "department_id"),
				idxs(hr, "employee_positions", "job_title_id")},
			Cols: []string{"id", "employee_id", "department_id", "job_title_id", "valid_from", "valid_to"},
			Rows: func(n int) int { return nEmp(n) + nEmp(n)/2 },
			Row: func(g *Gen, k int) []any {
				e := nEmp(g.N)
				if k <= e { // current position
					return []any{k, k, g.Int(1, nDept(g.N)), g.Int(1, len(entJobTitles)), g.Date(2018, 2025), nil}
				}
				from := g.Date(2008, 2016)
				return []any{k, nbSkew(k, e, 12), g.Int(1, nDept(g.N)), g.Int(1, len(entJobTitles)), from, from.AddDate(g.Int(1, 5), 0, 0)}
			},
		},
		// per employee: 2025 and 2026, three balance types
		{
			Name: "leave_balances", Schema: hr, HasID: true,
			DDL: idCol + `,
    employee_id        integer NOT NULL,
    year               integer NOT NULL,
    leave_type_id      integer NOT NULL,
    entitled_hours     numeric(6,2) NOT NULL,
    used_hours         numeric(6,2) NOT NULL,
    carried_over_hours numeric(6,2) NOT NULL`,
			FKs: []string{coreEmp("leave_balances", "employee_id"),
				fks(hr, "leave_balances", "leave_balances_leave_type_id_fk", "leave_type_id", hr, "leave_types", "id", "")},
			Indexes: []string{uidxs(hr, "leave_balances", "employee_id", "year", "leave_type_id"), idxs(hr, "leave_balances", "leave_type_id")},
			Cols:    []string{"id", "employee_id", "year", "leave_type_id", "entitled_hours", "used_hours", "carried_over_hours"},
			Rows:    func(n int) int { return 6 * nEmp(n) },
			Row: func(g *Gen, k int) []any {
				emp, j := (k-1)/6+1, (k-1)%6
				ent := []float64{160, 40, 96}[j%3]
				used := math.Round(g.r.Float64()*ent*4) / 4
				if j/3 == 1 { // 2026 has only just started
					used = math.Round(used/8*4) / 4
				}
				return []any{k, emp, entYear + j/3, j%3 + 1, ent, used, float64(g.Int(0, 80))}
			},
		},
		{
			Name: "leave_requests", Schema: hr, HasID: true,
			DDL: idCol + `,
    employee_id   integer NOT NULL,
    leave_type_id integer NOT NULL,
    start_date    date NOT NULL,
    end_date      date NOT NULL,
    hours         numeric(6,2) NOT NULL,
    status        text NOT NULL,
    approved_by   integer,
    reason        text,
    CHECK (end_date >= start_date)`,
			FKs: []string{coreEmp("leave_requests", "employee_id"),
				fks(hr, "leave_requests", "leave_requests_leave_type_id_fk", "leave_type_id", hr, "leave_types", "id", ""),
				fks(hr, "leave_requests", "leave_requests_approved_by_fk", "approved_by", "", "employees", "id", "")},
			Indexes: []string{idxs(hr, "leave_requests", "employee_id"), idxs(hr, "leave_requests", "leave_type_id"),
				idxs(hr, "leave_requests", "approved_by")},
			Cols: []string{"id", "employee_id", "leave_type_id", "start_date", "end_date", "hours", "status", "approved_by", "reason"},
			Rows: func(n int) int { return 6 * nEmp(n) },
			Row: func(g *Gen, k int) []any {
				e := nEmp(g.N)
				days := g.Int(1, 15)
				start := g.Date(entYear, entYear)
				status := g.Pick([]string{"approved", "approved", "approved", "approved", "requested", "rejected"})
				var by any
				if status != "requested" {
					by = g.Int(1, min(e, nDept(g.N))) // a department head
				}
				reason := g.Pick(entLeaveReasons)
				if strings.Contains(reason, "%s") {
					arg := g.FirstName() + " " + g.LastName()
					if strings.HasPrefix(reason, "Moving") {
						arg = entAddress(g)
					}
					reason = fmt.Sprintf(reason, arg)
				}
				return []any{k, g.Int(1, e), g.Int(1, len(entLeaveTypes)), start, start.AddDate(0, 0, days-1),
					float64(min(days, 10) * 8), status, by, reason}
			},
		},
		// health data (GDPR special category)
		{
			Name: "absences", Schema: hr, HasID: true,
			DDL: idCol + `,
    employee_id   integer NOT NULL,
    start_date    date NOT NULL,
    end_date      date,
    percentage    integer NOT NULL CHECK (percentage BETWEEN 1 AND 100),
    reported_to   integer,
    medical_notes text,
    CHECK (end_date IS NULL OR end_date >= start_date)`,
			FKs: []string{coreEmp("absences", "employee_id"),
				fks(hr, "absences", "absences_reported_to_fk", "reported_to", "", "employees", "id", "")},
			Indexes: []string{idxs(hr, "absences", "employee_id"), idxs(hr, "absences", "reported_to")},
			Cols:    []string{"id", "employee_id", "start_date", "end_date", "percentage", "reported_to", "medical_notes"},
			Rows:    func(n int) int { return 4 * nEmp(n) / 5 },
			Row: func(g *Gen, k int) []any {
				e := nEmp(g.N)
				start := g.Date(entYear, entYear)
				var end any
				if g.Chance(0.9) {
					end = start.AddDate(0, 0, g.Int(0, 40))
				}
				note := g.Pick(entMedicalNotes)
				switch strings.Count(note, "%s") {
				case 2:
					note = fmt.Sprintf(note, g.LastName(), g.Phone())
				case 1:
					note = fmt.Sprintf(note, g.City())
				default:
					note = strings.ReplaceAll(note, "%%", "%")
				}
				return []any{k, g.Int(1, e), start, end, []int{100, 100, 100, 50, 50}[g.Int(0, 4)],
					g.Int(1, min(e, nDept(g.N))), note}
			},
		},
		// one row per employee per working day of 2025
		{
			Name: "attendance", Schema: hr, HasID: true,
			DDL: idCol64 + `,
    employee_id   integer NOT NULL,
    work_date     date NOT NULL,
    status        text NOT NULL,
    clock_in      timestamptz,
    clock_out     timestamptz,
    break_minutes integer`,
			FKs:     []string{coreEmp("attendance", "employee_id")},
			Indexes: []string{uidxs(hr, "attendance", "employee_id", "work_date"), idxs(hr, "attendance", "work_date")},
			Cols:    []string{"id", "employee_id", "work_date", "status", "clock_in", "clock_out", "break_minutes"},
			Rows:    func(n int) int { return nEmp(n) * wd },
			Row: func(g *Gen, k int) []any {
				emp, d := (k-1)/wd+1, entWorkdays[(k-1)%wd]
				r := g.Int(1, 100)
				switch {
				case r <= 8:
					return []any{k, emp, d, "leave", nil, nil, nil}
				case r <= 12:
					return []any{k, emp, d, "sick", nil, nil, nil}
				}
				status := "office"
				if r > 67 {
					status = "remote"
				}
				in := d.Add(time.Duration(7*60+g.Int(0, 150)) * time.Minute)
				out := in.Add(time.Duration(8*60+g.Int(-60, 90)) * time.Minute)
				return []any{k, emp, d, status, in, out, []int{30, 30, 45, 60}[g.Int(0, 3)]}
			},
		},
		{
			Name: "overtime", Schema: hr, HasID: true,
			DDL: idCol + `,
    employee_id integer NOT NULL,
    work_date   date NOT NULL,
    hours       numeric(4,2) NOT NULL,
    kind        text NOT NULL,
    approved_by integer`,
			FKs: []string{coreEmp("overtime", "employee_id"),
				fks(hr, "overtime", "overtime_approved_by_fk", "approved_by", "", "employees", "id", "")},
			Indexes: []string{idxs(hr, "overtime", "employee_id"), idxs(hr, "overtime", "approved_by")},
			Cols:    []string{"id", "employee_id", "work_date", "hours", "kind", "approved_by"},
			Rows:    func(n int) int { return 2 * nEmp(n) },
			Row: func(g *Gen, k int) []any {
				e := nEmp(g.N)
				return []any{k, g.Int(1, e), entDay(g.Int(0, wd-1)), float64(g.Int(2, 16)) / 4,
					g.Pick([]string{"paid", "paid", "time off"}), entOpt(g, 0.9, func() int { return g.Int(1, min(e, nDept(g.N))) })}
			},
		},
		// payslip k: employee (k-1)/12+1, month (k-1)%12+1 of 2025
		{
			Name: "payslips", Schema: hr, HasID: true,
			DDL: idCol + `,
    employee_id integer NOT NULL,
    bsn         char(9) NOT NULL,
    period      date NOT NULL,
    gross       numeric(10,2) NOT NULL,
    wage_tax    numeric(10,2) NOT NULL,
    pension     numeric(10,2) NOT NULL,
    net         numeric(10,2) NOT NULL,
    iban        text NOT NULL,
    paid_on     date NOT NULL`,
			FKs: []string{coreEmp("payslips", "employee_id"),
				// natural key in another schema
				fks(hr, "payslips", "payslips_bsn_fk", "bsn", "", "persons", "bsn", "")},
			Indexes: []string{uidxs(hr, "payslips", "employee_id", "period"), idxs(hr, "payslips", "bsn")},
			Cols:    []string{"id", "employee_id", "bsn", "period", "gross", "wage_tax", "pension", "net", "iban", "paid_on"},
			Rows:    func(n int) int { return 12 * nEmp(n) },
			Row: func(g *Gen, k int) []any {
				emp, m := (k-1)/12+1, (k-1)%12+1
				p := entPay(emp, m)
				period := time.Date(entYear, time.Month(m), 1, 0, 0, 0, 0, time.UTC)
				return []any{k, emp, bsnOfEmp(g, emp), period, p.gross, p.tax, p.pension, p.net, g.IBAN(),
					period.AddDate(0, 0, 24)}
			},
		},
		// six lines per payslip; line 6 is the company car benefit for lease drivers
		{
			Name: "payslip_lines", Schema: hr, HasID: true,
			DDL: idCol64 + `,
    payslip_id        integer NOT NULL,
    line_no           integer NOT NULL,
    code              text NOT NULL,
    description       text NOT NULL,
    amount            numeric(10,2) NOT NULL,
    lease_contract_id integer`,
			FKs: []string{fks(hr, "payslip_lines", "payslip_lines_payslip_id_fk", "payslip_id", hr, "payslips", "id", "ON DELETE CASCADE"),
				// hr -> fleet (fleet also refers to hr)
				fks(hr, "payslip_lines", "payslip_lines_lease_contract_id_fk", "lease_contract_id", fl, "lease_contracts", "id", "")},
			Indexes: []string{uidxs(hr, "payslip_lines", "payslip_id", "line_no"), idxs(hr, "payslip_lines", "lease_contract_id")},
			Cols:    []string{"id", "payslip_id", "line_no", "code", "description", "amount", "lease_contract_id"},
			Rows:    func(n int) int { return 6 * 12 * nEmp(n) },
			Row: func(g *Gen, k int) []any {
				ps, ln := (k-1)/6+1, (k-1)%6+1
				emp, m := (ps-1)/12+1, (ps-1)%12+1
				p := entPay(emp, m)
				switch ln {
				case 1:
					return []any{k, ps, ln, "BASE", "Monthly salary", p.salary, nil}
				case 2:
					return []any{k, ps, ln, "PENS", "Pension contribution", -p.pension, nil}
				case 3:
					return []any{k, ps, ln, "TAX", "Wage tax", -p.tax, nil}
				case 4:
					return []any{k, ps, ln, "HOL", "Holiday allowance, reserved (paid in May)", round2(p.salary * 0.08), nil}
				case 5:
					return []any{k, ps, ln, "TRAV", "Travel allowance", p.travel, nil}
				}
				if entIsLease(emp) {
					i := entLeaseIdx(emp)
					return []any{k, ps, ln, "CAR", "Private use company car " + g.UniquePlate(i) + " (taxed, not paid out)", p.benefit, i}
				}
				return []any{k, ps, ln, "NET", "Internet allowance", p.other, nil}
			},
		},
		{
			Name: "emergency_contacts", Schema: hr, HasID: true,
			DDL: idCol + `,
    employee_id  integer NOT NULL,
    name         text NOT NULL,
    relationship text NOT NULL,
    phone        text NOT NULL,
    email        text`,
			FKs:     []string{coreEmp("emergency_contacts", "employee_id")},
			Indexes: []string{idxs(hr, "emergency_contacts", "employee_id")},
			Cols:    []string{"id", "employee_id", "name", "relationship", "phone", "email"},
			Rows:    func(n int) int { return nEmp(n) + nEmp(n)/2 },
			Row: func(g *Gen, k int) []any {
				f, l := g.FirstName(), g.LastName()
				var email any
				if g.Chance(0.6) {
					email = Email(f, l, k, "home.example")
				}
				return []any{k, (k-1)%nEmp(g.N) + 1, f + " " + l, g.Pick(entRelationships), g.Phone(), email}
			},
		},
		{
			Name: "id_documents", Schema: hr, HasID: true,
			DDL: idCol + `,
    employee_id     integer NOT NULL,
    document_type   text NOT NULL,
    document_number text NOT NULL UNIQUE,
    issuing_country text NOT NULL,
    expires_on      date NOT NULL`,
			FKs:     []string{coreEmp("id_documents", "employee_id")},
			Indexes: []string{idxs(hr, "id_documents", "employee_id")},
			Cols:    []string{"id", "employee_id", "document_type", "document_number", "issuing_country", "expires_on"},
			Rows:    nEmp,
			Row: func(g *Gen, k int) []any {
				l := func(v uint64) byte { return "ABCDEFGHJKLMNPRSTUVWXYZ"[v%23] }
				h := nbMix(uint64(k) + 99)
				num := fmt.Sprintf("%c%c%07d", l(h>>8), l(h>>16), k) // unique by k
				return []any{k, k, g.Pick([]string{"passport", "ID card", "ID card", "residence permit"}), num,
					g.IssuingCountry(), g.Date(2026, 2035)}
			},
		},
		{
			Name: "performance_reviews", Schema: hr, HasID: true,
			DDL: idCol + `,
    employee_id integer NOT NULL,
    reviewer_id integer NOT NULL,
    review_date date NOT NULL,
    score       integer NOT NULL CHECK (score BETWEEN 1 AND 5),
    summary     text NOT NULL`,
			FKs: []string{coreEmp("performance_reviews", "employee_id"),
				fks(hr, "performance_reviews", "performance_reviews_reviewer_id_fk", "reviewer_id", "", "employees", "id", "")},
			Indexes: []string{idxs(hr, "performance_reviews", "employee_id"), idxs(hr, "performance_reviews", "reviewer_id")},
			Cols:    []string{"id", "employee_id", "reviewer_id", "review_date", "score", "summary"},
			Rows:    func(n int) int { return 2 * nEmp(n) },
			Row: func(g *Gen, k int) []any {
				e := nEmp(g.N)
				s := g.Pick(entReviewPhrases)
				if strings.Contains(s, "%s") {
					s = fmt.Sprintf(s, g.FirstName()+" "+g.LastName())
				}
				return []any{k, (k-1)%e + 1, g.Int(1, min(e, nDept(g.N))), g.Date(entYear-1, entYear), g.Int(1, 5), s}
			},
		},
		{
			Name: "employee_trainings", Schema: hr, HasID: true,
			DDL: idCol + `,
    employee_id        integer NOT NULL,
    training_id        integer NOT NULL,
    completed_on       date,
    certificate_number text`,
			FKs: []string{coreEmp("employee_trainings", "employee_id"),
				fks(hr, "employee_trainings", "employee_trainings_training_id_fk", "training_id", hr, "trainings", "id", "")},
			Indexes: []string{idxs(hr, "employee_trainings", "employee_id"), idxs(hr, "employee_trainings", "training_id")},
			Cols:    []string{"id", "employee_id", "training_id", "completed_on", "certificate_number"},
			Rows:    func(n int) int { return 2 * nEmp(n) },
			Row: func(g *Gen, k int) []any {
				var done, cert any
				if g.Chance(0.8) {
					done = g.Date(2020, entYear)
					cert = fmt.Sprintf("CERT-%08d", k)
				}
				return []any{k, g.Int(1, nEmp(g.N)), g.Int(1, 40), done, cert}
			},
		},
		// No foreign key on purpose: employee_id refers to <name>_core.employees(id) by
		// convention only (a virtual FK into the core schema).
		{
			Name: "hr_notes", Schema: hr,
			DDL: `note_id     integer PRIMARY KEY,
    employee_id integer NOT NULL,
    note        text NOT NULL`,
			Indexes: []string{idxs(hr, "hr_notes", "employee_id")},
			Cols:    []string{"note_id", "employee_id", "note"},
			Rows:    nEmp,
			Row: func(g *Gen, k int) []any {
				return []any{k, g.Int(1, nEmp(g.N)), employeeNote(g, k)}
			},
		},
	}

	// ======================================================== fleet
	fleetTables := []Table{
		{
			Name: "lease_companies", Schema: fl, HasID: true,
			DDL: idCol + `,
    name         text NOT NULL UNIQUE,
    kvk_number   char(8) NOT NULL UNIQUE,
    contact_name text NOT NULL,
    phone        text NOT NULL,
    email        text NOT NULL,
    iban         text NOT NULL`,
			Cols: []string{"id", "name", "kvk_number", "contact_name", "phone", "email", "iban"},
			Rows: fixed(len(entLeaseCompanies)),
			Row: func(g *Gen, k int) []any {
				f, l := g.FirstName(), g.LastName()
				return []any{k, entLeaseCompanies[k-1], fmt.Sprintf("%08d", 60000000+int(nbMix(uint64(k))%9999999)),
					f + " " + l, g.Phone(), Email(f, l, k, "lease.example"), g.IBAN()}
			},
		},
		{
			Name: "car_models", Schema: fl, HasID: true,
			DDL: idCol + `,
    make        text NOT NULL,
    model       text NOT NULL,
    fuel_type   text NOT NULL,
    co2_g_km    integer NOT NULL,
    list_price  numeric(10,2) NOT NULL,
    benefit_pct numeric(4,1) NOT NULL,
    UNIQUE (make, model)`,
			Cols: []string{"id", "make", "model", "fuel_type", "co2_g_km", "list_price", "benefit_pct"},
			Rows: fixed(len(entCarModels)),
			Row: func(g *Gen, k int) []any {
				m := entCarModels[k-1]
				pct := 22.0
				if m.fuel == "electric" {
					pct = 17
				}
				return []any{k, m.make, m.model, m.fuel, m.co2, m.price, pct}
			},
		},
		// one category per salary scale 8..12
		{
			Name: "lease_categories", Schema: fl, HasID: true,
			DDL: idCol + `,
    name            text NOT NULL UNIQUE,
    salary_scale_id integer NOT NULL UNIQUE,
    monthly_budget  numeric(10,2) NOT NULL`,
			FKs:  []string{fks(fl, "lease_categories", "lease_categories_salary_scale_id_fk", "salary_scale_id", hr, "salary_scales", "id", "")},
			Cols: []string{"id", "name", "salary_scale_id", "monthly_budget"},
			Rows: fixed(5),
			Row: func(g *Gen, k int) []any {
				return []any{k, "Category " + string(rune('A'+k-1)), 7 + k, float64(550 + 125*k)}
			},
		},
		// car i (i <= lease contracts) belongs to lease contract i; the rest are pool cars
		{
			Name: "cars", Schema: fl, HasID: true,
			DDL: idCol + `,
    license_plate      text NOT NULL UNIQUE,
    vin                char(17) NOT NULL UNIQUE,
    car_model_id       integer NOT NULL,
    colour             text NOT NULL,
    first_registration date NOT NULL,
    core_vehicle_id    integer`,
			FKs: []string{fks(fl, "cars", "cars_car_model_id_fk", "car_model_id", fl, "car_models", "id", ""),
				fks(fl, "cars", "cars_core_vehicle_id_fk", "core_vehicle_id", "", "vehicles", "id", "")},
			Indexes: []string{idxs(fl, "cars", "car_model_id"), idxs(fl, "cars", "core_vehicle_id")},
			Cols:    []string{"id", "license_plate", "vin", "car_model_id", "colour", "first_registration", "core_vehicle_id"},
			Rows:    nCars,
			Row: func(g *Gen, k int) []any {
				return []any{k, g.UniquePlate(k), entVIN(k), entCarModel(k) + 1, g.Pick(entColours), g.Date(2021, entYear),
					entOpt(g, 0.2, func() int { return g.Int(1, nVehicle(g.N)) })}
			},
		},
		{
			Name: "lease_contracts", Schema: fl, HasID: true,
			DDL: idCol + `,
    car_id            integer NOT NULL UNIQUE,
    employee_id       integer NOT NULL,
    hr_contract_id    integer NOT NULL,
    lease_company_id  integer NOT NULL,
    lease_category_id integer NOT NULL,
    start_date        date NOT NULL,
    end_date          date NOT NULL,
    monthly_fee       numeric(10,2) NOT NULL,
    km_per_year       integer NOT NULL,
    CHECK (end_date > start_date)`,
			FKs: []string{
				fks(fl, "lease_contracts", "lease_contracts_car_id_fk", "car_id", fl, "cars", "id", ""),
				flEmp("lease_contracts", "employee_id"),
				// fleet -> hr
				fks(fl, "lease_contracts", "lease_contracts_hr_contract_id_fk", "hr_contract_id", hr, "contracts", "id", ""),
				fks(fl, "lease_contracts", "lease_contracts_lease_company_id_fk", "lease_company_id", fl, "lease_companies", "id", ""),
				fks(fl, "lease_contracts", "lease_contracts_lease_category_id_fk", "lease_category_id", fl, "lease_categories", "id", ""),
			},
			Indexes: []string{idxs(fl, "lease_contracts", "employee_id"), idxs(fl, "lease_contracts", "hr_contract_id"),
				idxs(fl, "lease_contracts", "lease_company_id"), idxs(fl, "lease_contracts", "lease_category_id")},
			Cols: []string{"id", "car_id", "employee_id", "hr_contract_id", "lease_company_id", "lease_category_id",
				"start_date", "end_date", "monthly_fee", "km_per_year"},
			Rows: nLease,
			Row: func(g *Gen, k int) []any {
				emp := entLeaseEmp(k)
				start := g.Date(2021, entYear)
				m := entCarModels[entCarModel(k)]
				return []any{k, k, emp, emp, g.Int(1, len(entLeaseCompanies)), entScale(emp) - 7, start,
					start.AddDate(g.Int(3, 5), 0, 0), round2(m.price * 0.0145), []int{20000, 25000, 30000, 35000, 40000}[g.Int(0, 4)]}
			},
		},
		// assignment k <= lease contracts: the current driver; the rest: earlier drivers
		{
			Name: "car_assignments", Schema: fl, HasID: true,
			DDL: idCol + `,
    car_id        integer NOT NULL,
    employee_id   integer NOT NULL,
    assigned_from date NOT NULL,
    assigned_to   date,
    CHECK (assigned_to IS NULL OR assigned_to > assigned_from)`,
			FKs: []string{fks(fl, "car_assignments", "car_assignments_car_id_fk", "car_id", fl, "cars", "id", ""),
				flEmp("car_assignments", "employee_id")},
			Indexes: []string{idxs(fl, "car_assignments", "car_id"), idxs(fl, "car_assignments", "employee_id")},
			Cols:    []string{"id", "car_id", "employee_id", "assigned_from", "assigned_to"},
			Rows:    func(n int) int { return 2 * nLease(n) },
			Row: func(g *Gen, k int) []any {
				l := nLease(g.N)
				if k <= l {
					return []any{k, k, entLeaseEmp(k), g.Date(2021, entYear), nil}
				}
				from := g.Date(2018, 2020)
				return []any{k, g.Int(1, nCars(g.N)), g.Int(1, nEmp(g.N)), from, from.AddDate(0, g.Int(3, 30), 0)}
			},
		},
		{
			Name: "fuel_cards", Schema: fl, HasID: true,
			DDL: idCol + `,
    card_number   text NOT NULL UNIQUE,
    pin_hash      text NOT NULL,
    employee_id   integer NOT NULL,
    car_id        integer NOT NULL,
    monthly_limit numeric(10,2) NOT NULL,
    valid_until   date NOT NULL`,
			FKs: []string{flEmp("fuel_cards", "employee_id"),
				fks(fl, "fuel_cards", "fuel_cards_car_id_fk", "car_id", fl, "cars", "id", "")},
			Indexes: []string{idxs(fl, "fuel_cards", "employee_id"), idxs(fl, "fuel_cards", "car_id")},
			Cols:    []string{"id", "card_number", "pin_hash", "employee_id", "car_id", "monthly_limit", "valid_until"},
			Rows:    nLease,
			Row: func(g *Gen, k int) []any {
				return []any{k, fmt.Sprintf("7077 %04d %04d %04d", 1000+k/100000000%9000, k/10000%10000, k%10000),
					fmt.Sprintf("$2b$10$%022x", g.r.Uint64()), entLeaseEmp(k), k, float64(g.Int(3, 8) * 100), g.Date(2026, 2028)}
			},
		},
		// about one per week per card
		{
			Name: "fuel_transactions", Schema: fl, HasID: true,
			DDL: idCol64 + `,
    fuel_card_id   integer NOT NULL,
    transaction_at timestamptz NOT NULL,
    station        text NOT NULL,
    city           text NOT NULL,
    quantity       numeric(7,2) NOT NULL,
    unit           text NOT NULL,
    amount         numeric(10,2) NOT NULL,
    odometer_km    integer NOT NULL`,
			FKs:     []string{fks(fl, "fuel_transactions", "fuel_transactions_fuel_card_id_fk", "fuel_card_id", fl, "fuel_cards", "id", "")},
			Indexes: []string{idxs(fl, "fuel_transactions", "fuel_card_id")},
			Cols:    []string{"id", "fuel_card_id", "transaction_at", "station", "city", "quantity", "unit", "amount", "odometer_km"},
			Rows:    func(n int) int { return 52 * nLease(n) },
			Row: func(g *Gen, k int) []any {
				card, w := (k-1)/52+1, (k-1)%52
				at := time.Date(entYear, 1, 1, 6, 0, 0, 0, time.UTC).AddDate(0, 0, 7*w+g.Int(0, 6)).
					Add(time.Duration(g.Int(0, 15*60)) * time.Minute)
				q, unit, price := g.Money(25, 60), "l", 1.95
				if entCarModels[entCarModel(card)].fuel == "electric" {
					q, unit, price = g.Money(15, 70), "kWh", 0.59
				}
				base := 10000 + int(nbMix(uint64(card))%60000)
				return []any{k, card, at, g.Pick(entFuelStations), g.City(), q, unit, round2(q * price), base + w*g.Int(380, 520)}
			},
		},
		// the trip log: about 200 trips per car per year
		{
			Name: "trips", Schema: fl, HasID: true,
			DDL: idCol64 + `,
    car_id       integer NOT NULL,
    employee_id  integer NOT NULL,
    trip_date    date NOT NULL,
    from_address text NOT NULL,
    to_address   text NOT NULL,
    km           numeric(6,1) NOT NULL,
    purpose      text NOT NULL,
    notes        text`,
			FKs: []string{fks(fl, "trips", "trips_car_id_fk", "car_id", fl, "cars", "id", ""),
				flEmp("trips", "employee_id")},
			Indexes: []string{idxs(fl, "trips", "car_id"), idxs(fl, "trips", "employee_id")},
			Cols:    []string{"id", "car_id", "employee_id", "trip_date", "from_address", "to_address", "km", "purpose", "notes"},
			Rows:    func(n int) int { return 200 * nLease(n) },
			Row: func(g *Gen, k int) []any {
				car, j := (k-1)/200+1, (k-1)%200
				var notes any
				purpose := g.Pick(entTripPurposes)
				if purpose == "business" && g.Chance(0.3) {
					notes = fmt.Sprintf("Visit %s %s at the customer site", g.FirstName(), g.LastName())
				}
				return []any{k, car, entLeaseEmp(car), entWorkdays[j*len(entWorkdays)/200], entAddress(g), entAddress(g),
					float64(g.Int(30, 2500)) / 10, purpose, notes}
			},
		},
		{
			Name: "damages", Schema: fl, HasID: true,
			DDL: idCol + `,
    car_id            integer NOT NULL,
    damage_date       date NOT NULL,
    driver_id         integer,
    description       text NOT NULL,
    third_party_name  text,
    third_party_plate text,
    repair_cost       numeric(10,2) NOT NULL`,
			FKs: []string{fks(fl, "damages", "damages_car_id_fk", "car_id", fl, "cars", "id", ""),
				flEmp("damages", "driver_id")},
			Indexes: []string{idxs(fl, "damages", "car_id"), idxs(fl, "damages", "driver_id")},
			Cols:    []string{"id", "car_id", "damage_date", "driver_id", "description", "third_party_name", "third_party_plate", "repair_cost"},
			Rows:    func(n int) int { return max(1, nLease(n)/4) },
			Row: func(g *Gen, k int) []any {
				car := g.Int(1, nLease(g.N))
				d := g.Pick(entDamages)
				var name, plate any
				switch strings.Count(d, "%s") {
				case 2:
					n, p := g.FirstName()+" "+g.LastName(), g.Phone()
					d, name = fmt.Sprintf(d, n, p), n
				case 1:
					if strings.Contains(d, "plate") {
						p := g.Plate()
						d, plate = fmt.Sprintf(d, p), p
					} else {
						n := g.FirstName() + " " + g.LastName()
						d, name = fmt.Sprintf(d, n), n
					}
				}
				return []any{k, car, g.Date(entYear, entYear), entLeaseEmp(car), d, name, plate, g.Money(150, 6500)}
			},
		},
		// No foreign key on purpose: license_plate refers to cars(license_plate) by
		// convention; about 5% are plates no longer (or never) in the fleet.
		{
			Name: "traffic_fines", Schema: fl, HasID: true,
			DDL: idCol + `,
    license_plate  text NOT NULL,
    offence_date   timestamptz NOT NULL,
    location       text NOT NULL,
    offence        text NOT NULL,
    amount         numeric(10,2) NOT NULL,
    cjib_reference text NOT NULL UNIQUE,
    driver_id      integer`,
			FKs:     []string{flEmp("traffic_fines", "driver_id")},
			Indexes: []string{idxs(fl, "traffic_fines", "license_plate"), idxs(fl, "traffic_fines", "driver_id")},
			Cols:    []string{"id", "license_plate", "offence_date", "location", "offence", "amount", "cjib_reference", "driver_id"},
			Rows:    func(n int) int { return max(1, nLease(n)/2) },
			Row: func(g *Gen, k int) []any {
				c := nCars(g.N)
				car := g.Int(1, c)
				var driver any
				if g.Chance(0.05) {
					car = c + g.Int(1, c) // not in the fleet
				} else if car <= nLease(g.N) && g.Chance(0.8) {
					driver = entLeaseEmp(car)
				}
				o := entOffences[g.Int(0, len(entOffences)-1)]
				return []any{k, g.UniquePlate(car), g.Timestamp(entYear, entYear), g.Street() + ", " + g.City(), o.text, o.amount,
					fmt.Sprintf("CJIB-%010d", 3000000000+k), driver}
			},
		},
		{
			Name: "maintenance", Schema: fl, HasID: true,
			DDL: idCol + `,
    car_id       integer NOT NULL,
    garage       text NOT NULL,
    service_date date NOT NULL,
    odometer_km  integer NOT NULL,
    description  text NOT NULL,
    cost         numeric(10,2) NOT NULL`,
			FKs:     []string{fks(fl, "maintenance", "maintenance_car_id_fk", "car_id", fl, "cars", "id", "")},
			Indexes: []string{idxs(fl, "maintenance", "car_id")},
			Cols:    []string{"id", "car_id", "garage", "service_date", "odometer_km", "description", "cost"},
			Rows:    func(n int) int { return 2 * nCars(n) },
			Row: func(g *Gen, k int) []any {
				return []any{k, (k-1)%nCars(g.N) + 1, g.Pick(entGarages), g.Date(entYear, entYear), g.Int(5000, 120000),
					g.Pick(entMaintenance), g.Money(60, 1400)}
			},
		},
		// a third of the lease drivers declared no private use for 2025
		{
			Name: "private_use_declarations", Schema: fl, HasID: true,
			DDL: idCol + `,
    employee_id integer NOT NULL,
    bsn         char(9) NOT NULL,
    year        integer NOT NULL,
    declared_on date NOT NULL,
    reference   text NOT NULL UNIQUE,
    UNIQUE (employee_id, year)`,
			FKs: []string{flEmp("private_use_declarations", "employee_id"),
				// natural key in another schema
				fks(fl, "private_use_declarations", "private_use_declarations_bsn_fk", "bsn", "", "persons", "bsn", "")},
			Indexes: []string{idxs(fl, "private_use_declarations", "bsn")},
			Cols:    []string{"id", "employee_id", "bsn", "year", "declared_on", "reference"},
			Rows:    func(n int) int { return max(1, nLease(n)/3) },
			Row: func(g *Gen, k int) []any {
				emp := entLeaseEmp(3*k - 2)
				return []any{k, emp, bsnOfEmp(g, emp), entYear, g.Date(entYear-1, entYear-1),
					fmt.Sprintf("VGP-%d-%07d", entYear, k)}
			},
		},
	}

	t.Tables = append(append(t.Tables, hrTables...), fleetTables...)
	return t
}
