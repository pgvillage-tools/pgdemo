package main

import (
	"fmt"
	"math"
	"math/rand/v2"
	"strings"
	"time"
)

// Gen produces random but realistic-looking demo values.
// One Gen is used for a whole run, so tables can share state (e.g. product prices).
type Gen struct {
	N      int    // base size chosen by the user
	loc    string // country of the personal values: "" (NL), "BE", "DE", "UK"; see locale.go
	r      *rand.Rand
	prices []float64
}

func NewGen(n int, seed uint64) *Gen {
	return &Gen{N: n, r: rand.New(rand.NewPCG(seed, seed^0x9E3779B97F4A7C15))}
}

func (g *Gen) Int(lo, hi int) int        { return lo + g.r.IntN(hi-lo+1) }
func (g *Gen) Pick(list []string) string { return list[g.r.IntN(len(list))] }
func (g *Gen) Chance(p float64) bool     { return g.r.Float64() < p }

func (g *Gen) Money(lo, hi float64) float64 {
	return math.Round((lo+g.r.Float64()*(hi-lo))*100) / 100
}

// Grade returns a school grade between 1.0 and 10.0 with one decimal.
func (g *Gen) Grade() float64 {
	return math.Round((1+g.r.Float64()*9)*10) / 10
}

func (g *Gen) Date(fromYear, toYear int) time.Time {
	start := time.Date(fromYear, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(toYear, 12, 31, 0, 0, 0, 0, time.UTC)
	days := int(end.Sub(start).Hours() / 24)
	return start.AddDate(0, 0, g.r.IntN(days+1))
}

func (g *Gen) Timestamp(fromYear, toYear int) time.Time {
	return g.Date(fromYear, toYear).Add(time.Duration(g.r.IntN(86400)) * time.Second)
}

func (g *Gen) FirstName() string {
	return g.byLoc(firstNames, beFirstNames, deFirstNames, ukFirstNames)
}
func (g *Gen) LastName() string { return g.byLoc(lastNames, beLastNames, deLastNames, ukLastNames) }
func (g *Gen) City() string     { return g.byLoc(cities, beCities, deCities, ukCities) }

func (g *Gen) Phone() string {
	switch g.loc {
	case "BE": // mobile 04xx xx xx xx
		return fmt.Sprintf("04%02d %02d %02d %02d", g.r.IntN(40)+60, g.r.IntN(100), g.r.IntN(100), g.r.IntN(100))
	case "DE": // mobile 01xx xxxxxxx
		return fmt.Sprintf("0%s %07d", g.Pick(deMobilePrefixes), g.r.IntN(10000000))
	case "UK": // mobile 07xxx xxxxxx
		return fmt.Sprintf("07%03d %06d", 400+g.r.IntN(600), g.r.IntN(1000000))
	}
	return fmt.Sprintf("06-%08d", g.r.IntN(100000000))
}

// Email builds a unique address; id keeps it unique within a table.
func Email(first, last string, id int, domain string) string {
	clean := func(s string) string {
		return strings.Map(func(r rune) rune {
			if r >= 'a' && r <= 'z' {
				return r
			}
			return -1
		}, emailTranslit.Replace(strings.ToLower(s)))
	}
	return fmt.Sprintf("%s.%s%d@%s", clean(first), clean(last), id, domain)
}

// emailTranslit writes accented letters of the BE, DE and UK names in ASCII.
var emailTranslit = strings.NewReplacer("ä", "ae", "ö", "oe", "ü", "ue", "ß", "ss", "é", "e", "è", "e", "ê", "e",
	"ë", "e", "ç", "c", "ş", "s", "ı", "i", "ğ", "g", "ć", "c", "à", "a", "â", "a", "î", "i", "ï", "i", "ô", "o", "û", "u")

var firstNames = []string{
	"Jan", "Piet", "Klaas", "Sanne", "Emma", "Lucas", "Sophie", "Daan", "Julia", "Sem",
	"Lotte", "Milan", "Anna", "Thijs", "Eva", "Bram", "Fleur", "Ruben", "Lisa", "Jesse",
	"Noah", "Mila", "Finn", "Tess", "Luuk", "Sara", "Max", "Nora", "Tim", "Iris",
	"Mohammed", "Fatima", "Yusuf", "Aisha", "Liam", "Olivia", "James", "Mia", "David", "Elena",
}

var lastNames = []string{
	"de Jong", "Jansen", "de Vries", "van den Berg", "van Dijk", "Bakker", "Janssen", "Visser",
	"Smit", "Meijer", "de Boer", "Mulder", "de Groot", "Bos", "Vos", "Peters", "Hendriks",
	"van Leeuwen", "Dekker", "Brouwer", "de Wit", "Dijkstra", "Smits", "de Graaf", "van der Meer",
	"Kok", "Jacobs", "Vermeulen", "Willems", "Schouten", "Yilmaz", "El Amrani", "Smith", "Brown",
}

var cities = []string{
	"Amsterdam", "Rotterdam", "Den Haag", "Utrecht", "Eindhoven", "Groningen", "Tilburg",
	"Almere", "Breda", "Nijmegen", "Apeldoorn", "Haarlem", "Arnhem", "Enschede", "Amersfoort",
	"Zwolle", "Leiden", "Maastricht", "Dordrecht", "Delft",
}

var categoryNames = []string{
	"Electronics", "Books", "Garden", "Kitchen", "Toys", "Sports", "Clothing", "Office", "Beauty", "Tools",
}

var productAdjectives = []string{
	"Smart", "Compact", "Deluxe", "Classic", "Eco", "Wireless", "Premium", "Mini", "Pro", "Ultra",
}

var productNouns = []string{
	"Lamp", "Kettle", "Speaker", "Backpack", "Chair", "Drill", "Notebook", "Watch", "Blender",
	"Headphones", "Jacket", "Ball", "Mug", "Desk", "Camera", "Puzzle",
}

var orderStatuses = []string{
	"new", "paid", "paid", "shipped", "shipped", "delivered", "delivered", "delivered", "delivered", "cancelled",
}

var departments = [][2]string{
	{"Sales", "Amsterdam"}, {"Marketing", "Amsterdam"}, {"Finance", "Utrecht"}, {"IT", "Eindhoven"},
	{"HR", "Utrecht"}, {"Operations", "Rotterdam"}, {"Support", "Groningen"}, {"R&D", "Delft"},
}

var jobTitles = []string{
	"Developer", "Analyst", "Consultant", "Manager", "Engineer", "Advisor", "Specialist",
	"Coordinator", "Designer", "Administrator",
}

var projectNames = []string{
	"Falcon", "Orion", "Phoenix", "Atlas", "Nova", "Zephyr", "Titan", "Aurora", "Delta", "Mercury",
}

var projectRoles = []string{"member", "member", "member", "lead", "reviewer", "advisor"}

var countries = []string{"Netherlands", "Belgium", "Germany", "United Kingdom", "France", "USA", "Sweden", "Spain"}

var genres = []string{"Fiction", "Thriller", "Science", "History", "Children", "Fantasy", "Biography", "Poetry", "Travel", "Cooking"}

var titleWords1 = []string{"The Silent", "The Last", "A Distant", "The Hidden", "The Golden", "The Broken", "A Quiet", "The Lost", "The Secret", "The Endless"}

var titleWords2 = []string{"River", "Garden", "Kingdom", "Letter", "Harbour", "Mountain", "Promise", "Island", "City", "Winter"}

var subjects = []string{
	"Mathematics", "Dutch", "English", "Physics", "Chemistry", "Biology", "History",
	"Geography", "Economics", "Computer Science", "Art", "Music",
}
