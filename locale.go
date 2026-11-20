package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Country versions of the Nibble secure and Nibble enterprise templates
// (BE, DE, UK). The tables and rows are the same as the Dutch original; the
// personal values follow the country (names, addresses, phone numbers, IBAN,
// licence plates, national identification number) and the personal data
// columns get local names. Belgium is mixed: the customer side and the fleet
// schema in French, the employees, persons and HR in Dutch.
//
// The Gen methods below choose by Gen.loc; "" is the Dutch original and uses
// exactly the same random draws as before, so a Dutch run gives the same data
// for the same seed.
//
// All values are generated; any resemblance to real people, addresses,
// accounts or vehicles is chance.

// Rename is a column rename done at the end of a run (after the foreign keys,
// indexes and views, which PostgreSQL adjusts by itself).
type Rename struct {
	Schema string // suffix as in Table.Schema
	Table  string
	Column string
	To     string
}

// Locales of the country templates; "" (NL) is the original.
var templateLocales = []string{"BE", "DE", "UK"}

// ---------------------------------------------------------------- Gen helpers

func (g *Gen) byLoc(nl, be, de, uk []string) string {
	switch g.loc {
	case "BE":
		return g.Pick(be)
	case "DE":
		return g.Pick(de)
	case "UK":
		return g.Pick(uk)
	}
	return g.Pick(nl)
}

// Street is a street name of the country.
func (g *Gen) Street() string { return g.byLoc(nbStreets, beStreets, deStreets, ukStreets) }

// CompanySuffix is a legal form or word for a company name.
func (g *Gen) CompanySuffix() string {
	return g.byLoc(nbCompanySuffix, beCompanySuffix, deCompanySuffix, ukCompanySuffix)
}

// Postcode is a postal code of the country.
func (g *Gen) Postcode() string {
	switch g.loc {
	case "BE":
		return fmt.Sprint(g.Int(1000, 9999))
	case "DE":
		return fmt.Sprintf("%05d", g.Int(1067, 99998))
	case "UK":
		const letters = "ABDEFGHJLNPQRSTUWXYZ" // inward code letters (no C, I, K, M, O, V)
		l := func() byte { return letters[g.Int(0, len(letters)-1)] }
		return fmt.Sprintf("%s%d %d%c%c", g.Pick(ukPostcodeAreas), g.Int(1, 20), g.Int(0, 9), l(), l())
	}
	return fmt.Sprintf("%04d %c%c", g.Int(1000, 9999), 'A'+rune(g.Int(0, 25)), 'A'+rune(g.Int(0, 25)))
}

// PostalAddress is a full address in one line: street, number, postcode and town
// in the order of the country.
func (g *Gen) PostalAddress() string {
	switch g.loc {
	case "BE", "DE":
		s, n := g.Street(), g.Int(1, 250)
		return fmt.Sprintf("%s %d, %s %s", s, n, g.Postcode(), g.City())
	case "UK":
		n, s := g.Int(1, 250), g.Street()
		c, p := g.City(), g.Postcode()
		return fmt.Sprintf("%d %s, %s %s", n, s, c, p)
	}
	return fmt.Sprintf("%s %d, %04d %c%c %s", g.Pick(nbStreets), g.Int(1, 250), g.Int(1000, 9999),
		'A'+rune(g.Int(0, 25)), 'A'+rune(g.Int(0, 25)), g.City())
}

// ShortAddress is street, number and town (no postcode).
func (g *Gen) ShortAddress() string {
	if g.loc == "UK" {
		n, s := g.Int(1, 250), g.Street()
		return fmt.Sprintf("%d %s, %s", n, s, g.City())
	}
	return fmt.Sprintf("%s %d, %s", g.Street(), g.Int(1, 250), g.City())
}

// IBAN is a valid IBAN (mod-97) of the country.
func (g *Gen) IBAN() string {
	switch g.loc {
	case "BE":
		// bank code (3) + account (7) + national check (2): the first 10 digits mod 97, 0 -> 97
		acct := g.Pick(beBankCodes) + fmt.Sprintf("%07d", g.r.Int64N(10_000_000))
		n, _ := strconv.ParseInt(acct, 10, 64) // not fmt.Sscan: that reads a leading 0 as octal
		c := n % 97
		if c == 0 {
			c = 97
		}
		return ibanFor("BE", fmt.Sprintf("%s%02d", acct, c))
	case "DE":
		return ibanFor("DE", g.Pick(deBankCodes)+fmt.Sprintf("%010d", g.r.Int64N(10_000_000_000)))
	case "UK":
		return ibanFor("GB", g.Pick(ukBankCodes)+fmt.Sprintf("%06d%08d", g.r.Int64N(1_000_000), g.r.Int64N(100_000_000)))
	}
	return ibanNL(g)
}

// ibanFor puts the country code and the check digits in front of a BBAN.
func ibanFor(country, bban string) string {
	mod := 0
	for _, c := range bban + country + "00" {
		if c >= 'A' && c <= 'Z' {
			mod = (mod*100 + int(c-'A') + 10) % 97
		} else {
			mod = (mod*10 + int(c-'0')) % 97
		}
	}
	return fmt.Sprintf("%s%02d%s", country, 98-mod, bban)
}

const plateLetters23 = "ABCDEFGHJKLMNPRSTUVWXYZ" // no I, O, Q

// Plate is a random licence plate of the country (not unique).
func (g *Gen) Plate() string {
	l := func() byte { return plateLetters23[g.Int(0, len(plateLetters23)-1)] }
	switch g.loc {
	case "BE":
		return fmt.Sprintf("%d-%c%c%c-%03d", g.Int(1, 2), l(), l(), l(), g.Int(0, 999))
	case "DE":
		city := g.Pick(dePlateCities)
		if g.Chance(0.5) {
			return fmt.Sprintf("%s-%c %d", city, l(), g.Int(1, 9999))
		}
		return fmt.Sprintf("%s-%c%c %d", city, l(), l(), g.Int(1, 9999))
	case "UK":
		return fmt.Sprintf("%c%c%s %c%c%c", l(), l(), g.Pick(ukAgeIDs), l(), l(), l())
	}
	return plateNL(g)
}

// UniquePlate gives a unique licence plate of the country for number i.
// An affine map with a multiplier coprime to the number of plates is a
// bijection, so different numbers give different plates.
func (g *Gen) UniquePlate(i int) string {
	const L = uint64(len(plateLetters23))
	l := func(v uint64) byte { return plateLetters23[v%L] }
	mix := func(space uint64) uint64 { return (uint64(i)*2654435761 + 12345) % space }
	switch g.loc {
	case "BE": // 1-ABC-123 / 2-ABC-123
		x := mix(2 * L * L * L * 1000)
		return fmt.Sprintf("%d-%c%c%c-%03d", 1+x/(L*L*L*1000), l(x/(L*L*1000)), l(x/(L*1000)), l(x/1000), x%1000)
	case "DE": // B-AB 1234
		c := uint64(len(dePlateCities))
		x := mix(c * L * L * 9999)
		return fmt.Sprintf("%s-%c%c %d", dePlateCities[x/(L*L*9999)], l(x/(L*9999)), l(x/9999), 1+x%9999)
	case "UK": // AB12 CDE
		a := uint64(len(ukAgeIDs))
		x := mix(L * L * a * L * L * L)
		return fmt.Sprintf("%c%c%s %c%c%c", l(x/(L*a*L*L*L)), l(x/(a*L*L*L)), ukAgeIDs[(x/(L*L*L))%a],
			l(x/(L*L)), l(x/L), l(x))
	}
	return entPlate(i)
}

// NatIDLabel is the name of the national identification number in free text.
func (g *Gen) NatIDLabel() string {
	switch g.loc {
	case "BE":
		return "national number"
	case "DE":
		return "tax ID"
	case "UK":
		return "NI number"
	}
	return "BSN"
}

// NatID gives a unique valid national identification number for index i (1-based):
// NL BSN (11-check), BE rijksregisternummer, DE Steuer-ID, UK National Insurance number.
// Capacity: BE 24 million, DE 29 million, UK 1.4 billion (the largest demo,
// size 10,000,000, has 22 million persons).
func (g *Gen) NatID(i int) string {
	// The index is first shuffled with a bijection on 0..capacity-1, so the
	// numbers look random and stay unique.
	shuffle := func(capacity int) int {
		return int((uint64(i-1)*2654435761+40503)%uint64(capacity)) + 1
	}
	switch g.loc {
	case "BE":
		return rrnFor(shuffle(rrnCapacity))
	case "DE":
		return steuerIDFor(shuffle(steuerCapacity))
	case "UK":
		return ninoFor(shuffle(ninoCapacity))
	}
	return bsnFor(i)
}

const (
	rrnCapacity    = 24090 * 998             // days x sequence numbers
	steuerCapacity = 9 * (362880 + 9*322560) // repeated position x orders
	ninoCapacity   = 373 * 1_000_000 * 4     // prefixes x numbers x suffixes
)

var entBase1940 = time.Date(1940, 1, 1, 0, 0, 0, 0, time.UTC)

// rrnFor: YYMMDD + sequence (001-998) + check (2). Index i picks the day
// (1940-01-01 .. 2005-12-15) and the sequence number, so all are unique.
func rrnFor(i int) string {
	const days = 24090
	k := i - 1
	day, seq := k%days, k/days+1
	d := entBase1940.AddDate(0, 0, day)
	s := fmt.Sprintf("%02d%02d%02d%03d", d.Year()%100, int(d.Month()), d.Day(), seq)
	p, _ := strconv.ParseInt(s, 10, 64)
	if d.Year() >= 2000 {
		p += 2_000_000_000
	}
	return fmt.Sprintf("%s%02d", s, 97-p%97)
}

// steuerIDFor: 11 digits, the first not 0; in the first 10 exactly one digit
// appears twice and one digit is missing; the last is the ISO 7064 check digit.
// Index i picks the missing digit, the order of the other nine and which one is
// repeated at position 10, so all are unique.
func steuerIDFor(i int) string {
	const p9, p8first = 362880, 322560 // 9!, and 9! minus the orders with 0 first (8 * 8!)
	k := i - 1
	j, r := k%9, k/9
	var digits []byte
	if r < p9 { // missing 0: the digits 1..9 in any order
		digits = nthPerm([]byte("123456789"), r)
	} else {
		r -= p9
		m := byte('1' + r/p8first)
		q := r % p8first
		var rest []byte
		for c := byte('0'); c <= '9'; c++ {
			if c != m {
				rest = append(rest, c)
			}
		}
		// first digit: one of the 8 non-zero digits, then the other 8 in any order
		first := rest[1+q/40320]
		var others []byte
		for _, c := range rest {
			if c != first {
				others = append(others, c)
			}
		}
		digits = append([]byte{first}, nthPerm(others, q%40320)...)
	}
	p := string(append(digits, digits[j]))
	return p + fmt.Sprint(steuerCheck(p))
}

// nthPerm returns permutation number n (0-based, lexicographic) of set.
func nthPerm(set []byte, n int) []byte {
	s := append([]byte(nil), set...)
	f := 1
	for x := 2; x < len(s); x++ {
		f *= x
	}
	out := make([]byte, 0, len(s))
	for len(s) > 0 {
		idx := 0
		if len(s) > 1 {
			idx = n / f
			n %= f
			f /= max(1, len(s)-1)
		}
		out = append(out, s[idx])
		s = append(s[:idx], s[idx+1:]...)
	}
	return out
}

// steuerCheck is the ISO 7064 MOD 11,10 check digit of the Steuer-ID.
func steuerCheck(p string) int {
	prod := 10
	for i := 0; i < len(p); i++ {
		sum := (int(p[i]-'0') + prod) % 10
		if sum == 0 {
			sum = 10
		}
		prod = (sum * 2) % 11
	}
	if c := 11 - prod; c != 10 {
		return c
	}
	return 0
}

// ninoFor: two prefix letters, six digits and a suffix A-D, unique by index.
func ninoFor(i int) string {
	k := i - 1
	n := len(ninoPrefixList)
	return fmt.Sprintf("%s%06d%c", ninoPrefixList[k%n], (k/n)%1_000_000, "ABCD"[(k/n/1_000_000)%4])
}

var ninoPrefixList = func() []string {
	var out []string
	for _, a := range "ABCEGHJKLMNOPRSTWXYZ" {
		for _, b := range "ABCEGHJKLMNPRSTWXYZ" {
			switch string(a) + string(b) {
			case "BG", "GB", "NK", "KN", "TN", "NT", "ZZ":
				continue
			}
			out = append(out, string(a)+string(b))
		}
	}
	return out
}()

// IssuingCountry is the country of an identity document: mostly the own country.
func (g *Gen) IssuingCountry() string {
	switch g.loc {
	case "BE":
		return g.Pick([]string{"BEL", "BEL", "BEL", "BEL", "NLD", "FRA", "MAR", "TUR"})
	case "DE":
		return g.Pick([]string{"DEU", "DEU", "DEU", "DEU", "TUR", "POL", "ITA", "SYR"})
	case "UK":
		return g.Pick([]string{"GBR", "GBR", "GBR", "GBR", "IRL", "IND", "POL", "PAK"})
	}
	return g.Pick([]string{"NLD", "NLD", "NLD", "NLD", "BEL", "DEU", "MAR", "TUR"})
}

// ---------------------------------------------------------------- templates

// localized turns a fresh Nibble secure or Nibble enterprise template into the
// version for a country: name, description, the national ID columns and the
// column renames.
func localized(t *Template, loc string) *Template {
	t.Name += " " + loc
	t.Locale = loc
	t.Description = localeIntro[loc] + " " + t.Description
	idType, idCheck := "char(11)", `'^[0-9]{11}$'`
	if loc == "UK" {
		idType, idCheck = "char(9)", `'^[A-Z]{2}[0-9]{6}[A-D]$'`
	}
	for i := range t.Tables {
		tb := &t.Tables[i]
		lines := strings.Split(tb.DDL, "\n")
		for j, ln := range lines {
			if strings.Contains(ln, "char(9)") && strings.Contains(ln, "bsn") {
				ln = strings.Replace(ln, "char(9)", idType, 1)
				lines[j] = strings.Replace(ln, `'^[0-9]{9}$'`, idCheck, 1)
			}
		}
		tb.DDL = strings.Join(lines, "\n")
	}
	t.Renames = localRenames(t, loc)
	return t
}

var localeIntro = map[string]string{
	"BE": "Belgian version (customers and fleet in French, employees and HR in Dutch): Belgian names, addresses, phone numbers, IBAN, plates and rijksregisternummer; personal data columns with local names.",
	"DE": "German version: German names, addresses, phone numbers, IBAN, plates and Steuer-ID; personal data columns with German names.",
	"UK": "UK version: British names, addresses, phone numbers, IBAN, plates and National Insurance number; personal data columns with British names.",
}

// beFrench: Belgian tables with French column names (customer side and fleet).
var beFrench = map[string]bool{
	"addresses": true, "customers": true, "customer_contacts": true, "customer_notes": true,
	"payment_cards": true, "ticket_comments": true, "suppliers": true,
	"his_customers": true, "his_customer_contacts": true, "his_suppliers": true,
}

// localRenames lists the renames for the personal data columns of t.
// A "table.column" entry wins over a "column" entry.
func localRenames(t *Template, loc string) []Rename {
	var out []Rename
	for _, tb := range t.Tables {
		m := renameMaps[loc]
		if loc == "BE" {
			m = renamesBENL
			if beFrench[tb.Name] || tb.Schema == "fleet" {
				m = renamesBEFR
			}
		}
		for _, c := range tb.Cols {
			to, ok := m[tb.Name+"."+c]
			if !ok {
				to = m[c]
			}
			if to != "" && to != c {
				out = append(out, Rename{tb.Schema, tb.Name, c, to})
			}
		}
	}
	return out
}

var renameMaps = map[string]map[string]string{"DE": renamesDE, "UK": renamesUK}

var renamesDE = map[string]string{
	"bsn": "steuer_id", "person_bsn": "person_steuer_id", "driver_bsn": "fahrer_steuer_id",
	"first_name": "vorname", "last_name": "nachname", "birth_date": "geburtsdatum", "gender": "geschlecht",
	"email": "email_adresse", "phone": "telefon",
	"street": "strasse", "house_number": "hausnummer", "postal_code": "plz", "city": "ort",
	"employees.salary": "gehalt", "his_employees.salary": "gehalt", "contracts.monthly_salary": "monatsgehalt", "password_hash": "passwort_hash",
	"card_number": "kartennummer", "holder_name": "karteninhaber",
	"license_plate": "kennzeichen", "third_party_plate": "gegner_kennzeichen", "third_party_name": "gegner_name",
	"notes": "notizen", "hr_notes.note": "notiz", "ticket_comments.body": "nachricht",
	"medical_notes": "aerztliche_bemerkung", "reason": "begruendung", "summary": "zusammenfassung",
	"damages.description": "schadensbeschreibung",
	"document_number":     "ausweisnummer", "contact_name": "ansprechpartner",
	"from_address": "start_adresse", "to_address": "ziel_adresse", "traffic_fines.location": "tatort",
	"payslips.gross": "brutto", "payslips.net": "netto", "payslips.wage_tax": "lohnsteuer", "payslips.pension": "rentenbeitrag",
}

var renamesUK = map[string]string{
	"bsn": "ni_number", "person_bsn": "person_ni_number", "driver_bsn": "driver_ni_number",
	"first_name": "forename", "last_name": "surname", "birth_date": "date_of_birth", "gender": "sex",
	"email": "email_address", "phone": "telephone",
	"street": "address_line_1", "house_number": "building_number", "postal_code": "postcode", "city": "town",
	"employees.salary": "annual_salary", "his_employees.salary": "annual_salary", "holder_name": "cardholder_name",
	"license_plate": "registration_plate", "third_party_plate": "third_party_registration_plate",
	"medical_notes": "medical_comments", "document_number": "passport_number",
	"payslips.wage_tax": "paye_tax", "traffic_fines.location": "offence_location",
}

// Belgium, Dutch side: employees, persons, HR.
var renamesBENL = map[string]string{
	"bsn": "rijksregisternummer", "driver_bsn": "bestuurder_rijksregisternummer",
	"first_name": "voornaam", "last_name": "achternaam", "birth_date": "geboortedatum", "gender": "geslacht",
	"email": "e_mailadres", "phone": "telefoon",
	"employees.salary": "brutoloon", "his_employees.salary": "brutoloon", "contracts.monthly_salary": "bruto_maandloon", "password_hash": "wachtwoord_hash",
	"license_plate": "nummerplaat", "notes": "opmerkingen", "hr_notes.note": "opmerking",
	"medical_notes": "medische_opmerkingen", "reason": "reden", "summary": "samenvatting",
	"document_number": "identiteitsbewijs_nummer",
	"payslips.gross":  "bruto", "payslips.net": "netto", "payslips.wage_tax": "bedrijfsvoorheffing", "payslips.pension": "pensioen",
	"emergency_contacts.name": "naam",
}

// Belgium, French side: customers and fleet.
var renamesBEFR = map[string]string{
	"bsn": "numero_national", "person_bsn": "numero_national",
	"first_name": "prenom", "last_name": "nom_de_famille", "email": "courriel", "phone": "telephone",
	"street": "rue", "house_number": "numero_de_maison", "postal_code": "code_postal", "city": "localite",
	"card_number": "numero_de_carte", "holder_name": "titulaire",
	"license_plate": "plaque", "third_party_plate": "plaque_tiers", "third_party_name": "nom_tiers",
	"notes": "remarques", "ticket_comments.body": "message", "contact_name": "personne_de_contact",
	"from_address": "adresse_depart", "to_address": "adresse_arrivee", "traffic_fines.location": "lieu",
}

// ---------------------------------------------------------------- dictionaries

var beCompanySuffix = []string{"BV", "NV", "SRL", "SA", "& Zonen", "& Fils", "Trading", "Services", "Logistics", "Solutions"}
var deCompanySuffix = []string{"GmbH", "GmbH & Co. KG", "AG", "KG", "& Söhne", "Handel", "Service", "Logistik", "Solutions"}
var ukCompanySuffix = []string{"Ltd", "Ltd", "plc", "& Sons", "Holdings", "Trading", "Services", "Logistics", "Solutions"}

// German plate district codes (Unterscheidungszeichen) of large cities.
var dePlateCities = []string{"B", "HH", "M", "K", "F", "S", "D", "L", "DO", "E", "HB", "DD", "H", "N", "DU",
	"BO", "W", "BI", "BN", "MS", "MA", "KA", "A", "WI", "AC", "BS", "KI", "C", "HAL", "MD", "FR", "KR", "MZ", "HL", "EF", "KS"}

// UK age identifiers (registration periods 2019-2025).
var ukAgeIDs = []string{"19", "69", "20", "70", "21", "71", "22", "72", "23", "73", "24", "74", "25", "75"}

var ukPostcodeAreas = []string{"B", "BS", "CF", "CB", "E", "EH", "G", "L", "LS", "M", "N", "NE", "NG", "NW", "OX", "S", "SE", "SW", "W", "YO"}

var (
	beBankCodes = []string{"001", "068", "063", "310", "363", "539", "735", "750"}
	deBankCodes = []string{"10010010", "20040000", "37040044", "50010517", "70020270", "12030000", "43060967", "60050101"}
	ukBankCodes = []string{"BARC", "LOYD", "HBUK", "NWBK", "MIDL", "NAIA", "SRLG", "MONZ"}
)

var deMobilePrefixes = []string{"151", "152", "157", "160", "170", "171", "172", "175", "176", "177", "178", "179"}

// Names, streets and towns (the same lists as the masking dictionaries of pgTestBuilder).

var beFirstNames = []string{
	"Lucas", "Louis", "Noah", "Arthur", "Jules", "Adam", "Liam", "Victor", "Mathis", "Lars",
	"Wout", "Jef", "Pieter", "Koen", "Bart", "Tom", "Dirk", "Geert", "Luc", "Marc",
	"Jean", "Pierre", "Michel", "Philippe", "Alain", "Didier", "Olivier", "Nicolas", "Thomas", "Maxime",
	"Emma", "Olivia", "Louise", "Mila", "Lotte", "Elena", "Marie", "Julie", "Sarah", "Laura",
	"Hanne", "Lien", "Ine", "Els", "An", "Katrien", "Sofie", "Lies", "Nele", "Fien",
	"Camille", "Chloé", "Léa", "Manon", "Inès", "Nathalie", "Isabelle", "Sophie", "Céline", "Valérie",
	"Mohamed", "Youssef", "Rayan", "Amina", "Yasmine", "Fatima", "Bilal", "Nora", "Zeynep", "Mehmet",
}

var beLastNames = []string{
	"Peeters", "Janssens", "Maes", "Jacobs", "Mertens", "Willems", "Claes", "Goossens", "Wouters", "De Smet",
	"Dubois", "Lambert", "Dupont", "Martin", "Simon", "Hermans", "Van den Broeck", "Michiels", "Leroy", "Desmet",
	"De Clercq", "Vermeulen", "Pauwels", "Van Damme", "Lemmens", "Aerts", "Verstraete", "Coppens", "Cools", "Hendrickx",
	"Laurent", "Lejeune", "Renard", "Denis", "Lambrechts", "Stevens", "Verhoeven", "Van de Velde", "Thys", "De Backer",
	"Wuyts", "Smets", "Segers", "Martens", "Declercq", "Bogaert", "De Vos", "Mathieu", "Dumont", "Gilles",
	"Charlier", "François", "Collard", "Gerard", "Moreau", "Fontaine", "Henry", "Leclercq", "Bertrand", "Lefebvre",
	"El Idrissi", "Benali", "Yilmaz", "Demir", "Kaya", "Nguyen", "Rossi", "Esposito", "Kowalski", "Popescu",
}

var beStreets = []string{
	"Kerkstraat", "Stationsstraat", "Molenstraat", "Nieuwstraat", "Schoolstraat", "Dorpsstraat", "Kapelstraat", "Beekstraat", "Kouter", "Grote Markt",
	"Leopoldlaan", "Koningin Astridlaan", "Albertlaan", "Veldstraat", "Brugstraat", "Hoogstraat", "Lindenlaan", "Bosstraat", "Heirweg", "Steenweg",
	"Rue de la Station", "Rue de l'Église", "Rue du Moulin", "Rue Haute", "Rue Neuve", "Rue des Écoles", "Rue de la Gare", "Rue du Centre", "Rue Saint-Michel", "Rue de Namur",
	"Avenue Louise", "Avenue de la Liberté", "Avenue des Tilleuls", "Avenue du Parc", "Chaussée de Bruxelles", "Chaussée de Louvain", "Boulevard d'Avroy", "Place du Marché", "Place Communale", "Chemin du Bois",
}

var beCities = []string{
	"Brussel", "Antwerpen", "Gent", "Brugge", "Leuven", "Mechelen", "Aalst", "Kortrijk", "Hasselt", "Sint-Niklaas",
	"Oostende", "Genk", "Roeselare", "Turnhout", "Dendermonde", "Beveren", "Lier", "Geel", "Vilvoorde", "Waregem",
	"Liège", "Charleroi", "Namur", "Mons", "Louvain-la-Neuve", "Tournai", "Wavre", "Seraing", "Verviers", "Mouscron",
	"Arlon", "Nivelles", "Waterloo", "Ottignies", "Huy", "Spa", "Eupen", "Bastogne", "Dinant", "Braine-l'Alleud",
}

var deFirstNames = []string{
	"Lukas", "Leon", "Finn", "Paul", "Jonas", "Elias", "Felix", "Max", "Noah", "Ben",
	"Thomas", "Michael", "Andreas", "Stefan", "Christian", "Markus", "Sebastian", "Daniel", "Tobias", "Florian",
	"Klaus", "Jürgen", "Wolfgang", "Uwe", "Frank", "Dieter", "Hans", "Peter", "Günter", "Bernd",
	"Mia", "Emma", "Hannah", "Sofia", "Lea", "Lina", "Marie", "Anna", "Lena", "Laura",
	"Julia", "Katharina", "Sarah", "Lisa", "Johanna", "Sabine", "Petra", "Monika", "Claudia", "Andrea",
	"Ursula", "Renate", "Birgit", "Susanne", "Heike", "Nicole", "Stefanie", "Anja", "Kerstin", "Melanie",
	"Mehmet", "Ali", "Ayşe", "Fatma", "Murat", "Elif", "Dimitri", "Natalia", "Piotr", "Agnieszka",
}

var deLastNames = []string{
	"Müller", "Schmidt", "Schneider", "Fischer", "Weber", "Meyer", "Wagner", "Becker", "Schulz", "Hoffmann",
	"Schäfer", "Koch", "Bauer", "Richter", "Klein", "Wolf", "Schröder", "Neumann", "Schwarz", "Zimmermann",
	"Braun", "Krüger", "Hofmann", "Hartmann", "Lange", "Schmitt", "Werner", "Schmitz", "Krause", "Meier",
	"Lehmann", "Schmid", "Schulze", "Maier", "Köhler", "Herrmann", "König", "Walter", "Mayer", "Huber",
	"Kaiser", "Fuchs", "Peters", "Lang", "Scholz", "Möller", "Weiß", "Jung", "Hahn", "Schubert",
	"Vogel", "Friedrich", "Keller", "Günther", "Frank", "Berger", "Winkler", "Roth", "Beck", "Lorenz",
	"Yılmaz", "Kaya", "Demir", "Şahin", "Nowak", "Kowalski", "Ivanov", "Popović", "Rossi", "Nguyen",
}

var deStreets = []string{
	"Hauptstraße", "Schulstraße", "Gartenstraße", "Bahnhofstraße", "Dorfstraße", "Bergstraße", "Kirchstraße", "Waldstraße", "Ringstraße", "Lindenstraße",
	"Schillerstraße", "Goethestraße", "Mozartstraße", "Wiesenweg", "Feldweg", "Birkenweg", "Ahornweg", "Am Markt", "Marktplatz", "Kirchplatz",
	"Friedrichstraße", "Wilhelmstraße", "Rosenstraße", "Talstraße", "Mühlenweg", "Lessingstraße", "Parkstraße", "Poststraße", "Blumenstraße", "Uferstraße",
	"Kastanienallee", "Lindenallee", "Schlossallee", "Bismarckstraße", "Beethovenstraße", "Industriestraße", "Am Bach", "Im Winkel", "Hafenstraße", "Sonnenweg",
}

var deCities = []string{
	"Berlin", "Hamburg", "München", "Köln", "Frankfurt am Main", "Stuttgart", "Düsseldorf", "Leipzig", "Dortmund", "Essen",
	"Bremen", "Dresden", "Hannover", "Nürnberg", "Duisburg", "Bochum", "Wuppertal", "Bielefeld", "Bonn", "Münster",
	"Mannheim", "Karlsruhe", "Augsburg", "Wiesbaden", "Mönchengladbach", "Gelsenkirchen", "Aachen", "Braunschweig", "Kiel", "Chemnitz",
	"Halle (Saale)", "Magdeburg", "Freiburg im Breisgau", "Krefeld", "Mainz", "Lübeck", "Erfurt", "Oberhausen", "Rostock", "Kassel",
}

var ukFirstNames = []string{
	"Oliver", "George", "Harry", "Noah", "Jack", "Leo", "Arthur", "Oscar", "Charlie", "Jacob",
	"James", "William", "Thomas", "Daniel", "Matthew", "Joshua", "Samuel", "Joseph", "David", "Andrew",
	"Paul", "Mark", "Richard", "Simon", "Stephen", "Robert", "John", "Michael", "Christopher", "Gareth",
	"Olivia", "Amelia", "Isla", "Ava", "Ivy", "Freya", "Lily", "Florence", "Mia", "Willow",
	"Emily", "Sophie", "Grace", "Chloe", "Jessica", "Hannah", "Charlotte", "Lucy", "Ellie", "Megan",
	"Sarah", "Emma", "Rebecca", "Laura", "Rachel", "Claire", "Helen", "Karen", "Susan", "Fiona",
	"Mohammed", "Muhammad", "Aisha", "Priya", "Arjun", "Ravi", "Amir", "Zara", "Kwame", "Chinedu",
}

var ukLastNames = []string{
	"Smith", "Jones", "Williams", "Taylor", "Brown", "Davies", "Evans", "Wilson", "Thomas", "Johnson",
	"Roberts", "Robinson", "Thompson", "Wright", "Walker", "White", "Edwards", "Hughes", "Green", "Hall",
	"Lewis", "Harris", "Clarke", "Patel", "Jackson", "Wood", "Turner", "Martin", "Cooper", "Hill",
	"Ward", "Morris", "Moore", "Clark", "Lee", "King", "Baker", "Harrison", "Morgan", "Allen",
	"James", "Scott", "Phillips", "Watson", "Davis", "Parker", "Price", "Bennett", "Young", "Griffiths",
	"Mitchell", "Kelly", "Cook", "Carter", "Richardson", "Bailey", "Collins", "Bell", "Shaw", "Murphy",
	"Khan", "Ali", "Hussain", "Ahmed", "Singh", "Kaur", "MacDonald", "Campbell", "Stewart", "O'Brien",
}

var ukStreets = []string{
	"High Street", "Station Road", "Main Street", "Park Road", "Church Road", "Church Street", "London Road", "Victoria Road", "Green Lane", "Manor Road",
	"Church Lane", "Park Avenue", "The Avenue", "The Crescent", "Queens Road", "New Road", "Grange Road", "Kings Road", "Kingsway", "Windsor Road",
	"Highfield Road", "Mill Lane", "Alexandra Road", "York Road", "Main Road", "Broadway", "King Street", "The Green", "Springfield Road", "George Street",
	"Victoria Street", "Albert Road", "Queensway", "New Street", "Stanley Road", "Mill Road", "Richmond Road", "Chester Road", "Elm Grove", "Oak Close",
}

var ukCities = []string{
	"London", "Birmingham", "Manchester", "Leeds", "Glasgow", "Sheffield", "Bradford", "Liverpool", "Edinburgh", "Bristol",
	"Cardiff", "Leicester", "Coventry", "Nottingham", "Newcastle upon Tyne", "Belfast", "Brighton", "Hull", "Plymouth", "Stoke-on-Trent",
	"Wolverhampton", "Derby", "Swansea", "Southampton", "Aberdeen", "Portsmouth", "York", "Peterborough", "Dundee", "Oxford",
	"Cambridge", "Norwich", "Exeter", "Bath", "Reading", "Milton Keynes", "Luton", "Ipswich", "Inverness", "Canterbury",
}
