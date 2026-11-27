package main

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// PostGIS municipality: the geography of a fictional Dutch municipality
// (Nibbelstad) in RD New (EPSG:28992, metres): districts, streets, buildings,
// addresses, cadastral parcels with owners, parks, water, sensors with
// readings, and municipal vehicles with GPS tracks. Needs PostGIS 3 in the
// database (Template.Extensions; checked in db.go before anything is made).
//
// The geometry is random but always valid (closed rings, simple polygons) and
// inside the municipality. Buildings, their parcels and their addresses are
// computed from the building number, so an address lies inside its building,
// a building inside its parcel, and a building in the district its
// district_id names: ST_Within / ST_Contains find real matches.

const (
	gisSRID   = 28992
	gisMinX   = 150000.0 // south-west corner of the municipality (RD New)
	gisMinY   = 460000.0
	gisSpan   = 9000.0 // 9 x 9 km
	gisGrid   = 6      // 6 x 6 districts
	gisCell   = gisSpan / gisGrid
	gisMargin = 60.0 // buildings and parcels stay this far from the border
	gisTown   = "Nibbelstad"
)

// ---------------------------------------------------------------- geometry text (EWKT)

type xy struct{ x, y float64 }

func ewktCoords(pts []xy) string {
	parts := make([]string, len(pts))
	for i, p := range pts {
		parts[i] = fmt.Sprintf("%.2f %.2f", p.x, p.y)
	}
	return strings.Join(parts, ",")
}

func ewktPoint(p xy) string {
	return fmt.Sprintf("SRID=%d;POINT(%.2f %.2f)", gisSRID, p.x, p.y)
}

func ewktLine(pts []xy) string {
	return fmt.Sprintf("SRID=%d;LINESTRING(%s)", gisSRID, ewktCoords(pts))
}

// ewktPolygon closes the ring itself: pts are the corners without the repeat.
func ewktPolygon(pts []xy) string {
	ring := append(append([]xy(nil), pts...), pts[0])
	return fmt.Sprintf("SRID=%d;POLYGON((%s))", gisSRID, ewktCoords(ring))
}

// gisUnit is a fixed pseudo-random number in [0,1) for (salt, i, j): the same
// every run, so geometry that has to fit together can be computed again.
func gisUnit(salt uint64, i, j int) float64 {
	return float64(nbMix(salt<<48^uint64(i)<<8^uint64(j))>>11) / (1 << 53)
}

func gisClamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }

func gisInside(p xy) xy {
	return xy{gisClamp(p.x, gisMinX+1, gisMinX+gisSpan-1), gisClamp(p.y, gisMinY+1, gisMinY+gisSpan-1)}
}

// gisDistrictOf is the district (1..36) that contains point p.
func gisDistrictOf(p xy) int {
	col := int(gisClamp(math.Floor((p.x-gisMinX)/gisCell), 0, gisGrid-1))
	row := int(gisClamp(math.Floor((p.y-gisMinY)/gisCell), 0, gisGrid-1))
	return row*gisGrid + col + 1
}

// gisDistrict is the square of district d (1..36).
func gisDistrict(d int) []xy {
	col, row := float64((d-1)%gisGrid), float64((d-1)/gisGrid)
	x0, y0 := gisMinX+col*gisCell, gisMinY+row*gisCell
	return []xy{{x0, y0}, {x0 + gisCell, y0}, {x0 + gisCell, y0 + gisCell}, {x0, y0 + gisCell}}
}

// gisBuilding is the footprint of building b: a rotated rectangle.
type gisBuilding struct {
	c        xy      // centre
	w, d     float64 // width and depth in metres
	cos, sin float64 // rotation
}

func gisBuildingOf(b int) gisBuilding {
	inner := gisSpan - 2*gisMargin
	a := gisUnit(1, b, 4) * math.Pi / 2
	return gisBuilding{
		c: xy{gisMinX + gisMargin + gisUnit(1, b, 0)*inner, gisMinY + gisMargin + gisUnit(1, b, 1)*inner},
		w: 8 + gisUnit(1, b, 2)*22, d: 8 + gisUnit(1, b, 3)*17,
		cos: math.Cos(a), sin: math.Sin(a),
	}
}

// at is the point (u, v) in the building's own frame (metres from the centre).
func (b gisBuilding) at(u, v float64) xy {
	return xy{b.c.x + u*b.cos - v*b.sin, b.c.y + u*b.sin + v*b.cos}
}

// rect is a rectangle around the centre, in the building's rotation.
func (b gisBuilding) rect(w, d float64) []xy {
	return []xy{b.at(-w/2, -d/2), b.at(w/2, -d/2), b.at(w/2, d/2), b.at(-w/2, d/2)}
}

func (b gisBuilding) footprint() []xy { return b.rect(b.w, b.d) }

// parcel: larger than the building, same centre and rotation, so it contains it.
func (b gisBuilding) parcelSize() (float64, float64) { return b.w*1.6 + 6, b.d*1.8 + 6 }

// gisStar is a simple (star-shaped) polygon: corners at increasing angles
// around a centre, so the ring never crosses itself. The centre is chosen far
// enough from the border that the polygon lies inside the municipality.
func gisStar(g *Gen, rMin, rMax, stretch float64) []xy {
	mx, my := rMax*stretch+1, rMax+1
	c := xy{gisMinX + mx + g.r.Float64()*(gisSpan-2*mx), gisMinY + my + g.r.Float64()*(gisSpan-2*my)}
	n := g.Int(6, 11)
	pts := make([]xy, n)
	for i := range pts {
		a := (float64(i) + 0.2 + 0.6*g.r.Float64()) * 2 * math.Pi / float64(n)
		r := rMin + g.r.Float64()*(rMax-rMin)
		pts[i] = xy{c.x + r*math.Cos(a)*stretch, c.y + r*math.Sin(a)}
	}
	return pts
}

// gisWalk is a route of n points from start with steps between lo and hi metres.
func gisWalk(g *Gen, start xy, n int, lo, hi float64) []xy {
	pts := []xy{start}
	dir := g.r.Float64() * 2 * math.Pi
	for len(pts) < n {
		dir += (g.r.Float64() - 0.5) * math.Pi / 2 // turn at most 45 degrees
		step := lo + g.r.Float64()*(hi-lo)
		p := pts[len(pts)-1]
		q := gisInside(xy{p.x + step*math.Cos(dir), p.y + step*math.Sin(dir)})
		if math.Hypot(q.x-p.x, q.y-p.y) < 1 { // stuck at the border: turn around
			dir += math.Pi
			continue
		}
		pts = append(pts, q)
	}
	return pts
}

func gisRandomPoint(g *Gen) xy {
	return xy{gisMinX + 1 + g.r.Float64()*(gisSpan-2), gisMinY + 1 + g.r.Float64()*(gisSpan-2)}
}

// ---------------------------------------------------------------- data

var gisDistrictBases = []string{"Centrum", "De Hoven", "Molenwijk", "Vathorst", "Kattenbroek", "Nieuwland",
	"Schothorst", "Liendert", "Rustenburg", "Hoogland", "Soesterkwartier", "Bergkwartier"}
var gisDistrictParts = []string{"", " Noord", " Zuid"}
var gisStreetTypes = []string{"residential", "residential", "residential", "main road", "cycle path", "footpath", "service road"}
var gisUsages = []string{"residential", "residential", "residential", "residential", "office", "shop", "industry", "school", "health care", "sports"}
var gisSensorTypes = []struct {
	kind, unit string
	lo, hi     float64
}{
	{"pm2_5", "µg/m3", 2, 45}, {"pm10", "µg/m3", 5, 70}, {"no2", "µg/m3", 4, 80},
	{"noise", "dB(A)", 35, 85}, {"traffic", "vehicles/h", 0, 1800},
}
var gisVehicleKinds = []string{"van", "van", "car", "sweeper", "refuse truck", "electric bike", "tractor"}
var gisParkNames = []string{"Stadspark", "Wilhelminapark", "Heiligenbergerbeek", "Bosrand", "Speelweide", "Hertenkamp",
	"Rozentuin", "Volkstuinen", "Waterspeelplaats", "Hondenlosloopgebied"}
var gisWaterKinds = []string{"pond", "pond", "canal", "lake", "retention basin"}

func gisSensorType(s int) int { return int(nbMix(uint64(s)*0x5E45) % uint64(len(gisSensorTypes))) }

func postgisMunicipality() *Template {
	nDistrict := func(int) int { return gisGrid * gisGrid }
	nStreet := func(n int) int { return max(50, n/5) }
	nBuilding := func(n int) int { return max(100, n) }
	nAddress := func(n int) int { return nBuilding(n) + nBuilding(n)/2 }
	nOwner := func(n int) int { return max(50, n/2) }
	nPark := func(n int) int { return max(10, n/50) }
	nWater := func(n int) int { return max(5, n/100) }
	nSensor := func(n int) int { return max(20, n/20) }
	const readingsPerSensor = 48 // hourly, two days
	nVehicle := func(n int) int { return max(10, n/50) }
	const tracksPerVehicle = 20
	geom := func(kind string) string { return fmt.Sprintf("geom %s(%s,%d) NOT NULL", "geometry", kind, gisSRID) }
	gist := func(table string) string { return fmt.Sprintf("CREATE INDEX ON {s}.%s USING gist (geom)", table) }
	day := func(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

	return &Template{
		Name: "PostGIS municipality",
		Description: "The geography of a Dutch municipality in RD New (EPSG:28992): districts, streets, buildings, " +
			"addresses, cadastral parcels with owners (BSN), parks, water, sensors with readings and vehicles with " +
			"GPS tracks; GiST indexes and spatial views. Needs PostGIS 3 in the database.",
		Extensions: []string{"postgis"},
		Tables: []Table{
			{
				Name: "districts", HasID: true,
				DDL: idCol + `,
    code       text NOT NULL UNIQUE,
    name       text NOT NULL,
    population integer NOT NULL,
    ` + geom("Polygon"),
				Indexes: []string{gist("districts")},
				Cols:    []string{"id", "code", "name", "population", "geom"},
				Rows:    nDistrict,
				Row: func(g *Gen, k int) []any {
					name := gisDistrictBases[(k-1)%len(gisDistrictBases)] + gisDistrictParts[(k-1)/len(gisDistrictBases)]
					return []any{k, fmt.Sprintf("WK%02d", k), name, g.Int(800, 15000), ewktPolygon(gisDistrict(k))}
				},
			},
			{
				Name: "streets", HasID: true,
				DDL: idCol + `,
    name        text NOT NULL,
    street_type text NOT NULL,
    max_speed   integer,
    ` + geom("LineString"),
				Indexes: []string{idx("streets", "name"), gist("streets")},
				Cols:    []string{"id", "name", "street_type", "max_speed", "geom"},
				Rows:    nStreet,
				Row: func(g *Gen, k int) []any {
					typ := g.Pick(gisStreetTypes)
					var speed any
					switch typ {
					case "residential":
						speed = []int{30, 30, 50}[g.Int(0, 2)]
					case "main road":
						speed = []int{50, 50, 70, 80}[g.Int(0, 3)]
					case "service road":
						speed = 30
					}
					return []any{k, g.Street(), typ, speed, ewktLine(gisWalk(g, gisRandomPoint(g), g.Int(3, 9), 60, 260))}
				},
			},
			{
				Name: "buildings", HasID: true,
				DDL: idCol + `,
    district_id integer NOT NULL,
    street_id   integer NOT NULL,
    year_built  integer NOT NULL,
    usage       text NOT NULL,
    floors      integer NOT NULL,
    ` + geom("Polygon"),
				FKs:     []string{fk("buildings", "district_id", "districts"), fk("buildings", "street_id", "streets")},
				Indexes: []string{idx("buildings", "district_id"), idx("buildings", "street_id"), gist("buildings")},
				Cols:    []string{"id", "district_id", "street_id", "year_built", "usage", "floors", "geom"},
				Rows:    nBuilding,
				Row: func(g *Gen, k int) []any {
					b := gisBuildingOf(k)
					usage := g.Pick(gisUsages)
					floors := g.Int(1, 3)
					if usage != "residential" || g.Chance(0.15) {
						floors = g.Int(1, 12)
					}
					return []any{k, gisDistrictOf(b.c), g.Int(1, nStreet(g.N)), g.Int(1850, 2025), usage, floors,
						ewktPolygon(b.footprint())}
				},
			},
			{
				Name: "addresses", HasID: true,
				DDL: idCol + `,
    building_id  integer NOT NULL,
    street_name  text NOT NULL,
    house_number integer NOT NULL,
    house_letter text,
    postcode     text NOT NULL,
    town         text NOT NULL,
    ` + geom("Point"),
				FKs:     []string{fk("addresses", "building_id", "buildings")},
				Indexes: []string{idx("addresses", "building_id"), idx("addresses", "postcode"), gist("addresses")},
				Cols:    []string{"id", "building_id", "street_name", "house_number", "house_letter", "postcode", "town", "geom"},
				Rows:    nAddress,
				Row: func(g *Gen, k int) []any {
					// buildings 1, 1, 2, 3, 3, 4, ...: every other building has two addresses
					bn := (k-1)*2/3 + 1
					b := gisBuildingOf(bn)
					p := b.at((g.r.Float64()-0.5)*0.6*b.w, (g.r.Float64()-0.5)*0.6*b.d) // inside the footprint
					var letter any
					if g.Chance(0.12) {
						letter = string(rune('A' + g.Int(0, 3)))
					}
					return []any{k, bn, g.Street(), g.Int(1, 250), letter, g.Postcode(), gisTown, ewktPoint(p)}
				},
			},
			{
				Name: "owners", HasID: true,
				DDL: idCol + `,
    bsn        char(9) NOT NULL UNIQUE CHECK (bsn ~ '^[0-9]{9}$'),
    first_name text NOT NULL,
    last_name  text NOT NULL,
    birth_date date NOT NULL,
    email      text,
    phone      text,
    iban       text`,
				Indexes: []string{idx("owners", "last_name")},
				Cols:    []string{"id", "bsn", "first_name", "last_name", "birth_date", "email", "phone", "iban"},
				Rows:    nOwner,
				Row: func(g *Gen, k int) []any {
					f, l := g.FirstName(), g.LastName()
					return []any{k, g.NatID(k), f, l, g.Date(1940, 2004), Email(f, l, k, "owner.example"), g.Phone(), g.IBAN()}
				},
			},
			{
				Name: "parcels", HasID: true,
				DDL: idCol + `,
    parcel_number text NOT NULL UNIQUE,
    building_id   integer NOT NULL UNIQUE,
    owner_id      integer NOT NULL,
    area_m2       numeric(10,2) NOT NULL,
    registered_on date NOT NULL,
    ` + geom("Polygon"),
				FKs:     []string{fk("parcels", "building_id", "buildings"), fk("parcels", "owner_id", "owners")},
				Indexes: []string{idx("parcels", "owner_id"), gist("parcels")},
				Cols:    []string{"id", "parcel_number", "building_id", "owner_id", "area_m2", "registered_on", "geom"},
				Rows:    nBuilding, // one parcel per building, containing it
				Row: func(g *Gen, k int) []any {
					b := gisBuildingOf(k)
					w, d := b.parcelSize()
					section := string(rune('A' + (gisDistrictOf(b.c)-1)%26))
					return []any{k, fmt.Sprintf("NBS00 %s %06d", section, k), k, g.Int(1, nOwner(g.N)),
						round2(w * d), g.Date(1990, 2025), ewktPolygon(b.rect(w, d))}
				},
			},
			{
				Name: "parks", HasID: true,
				DDL: idCol + `,
    name     text NOT NULL,
    has_play boolean NOT NULL,
    ` + geom("Polygon"),
				Indexes: []string{gist("parks")},
				Cols:    []string{"id", "name", "has_play", "geom"},
				Rows:    nPark,
				Row: func(g *Gen, k int) []any {
					name := gisParkNames[(k-1)%len(gisParkNames)]
					if k > len(gisParkNames) {
						name += fmt.Sprintf(" %d", (k-1)/len(gisParkNames)+1)
					}
					return []any{k, name, g.Chance(0.6), ewktPolygon(gisStar(g, 40, 220, 1))}
				},
			},
			{
				Name: "water", HasID: true,
				DDL: idCol + `,
    kind  text NOT NULL,
    depth numeric(4,1),
    ` + geom("Polygon"),
				Indexes: []string{gist("water")},
				Cols:    []string{"id", "kind", "depth", "geom"},
				Rows:    nWater,
				Row: func(g *Gen, k int) []any {
					kind := g.Pick(gisWaterKinds)
					stretch := 1.0
					if kind == "canal" {
						stretch = 4 // long and narrow
					}
					return []any{k, kind, round1(0.5 + g.r.Float64()*4), ewktPolygon(gisStar(g, 20, 120, stretch))}
				},
			},
			{
				Name: "sensors", HasID: true,
				DDL: idCol + `,
    sensor_type  text NOT NULL,
    unit         text NOT NULL,
    installed_on date NOT NULL,
    active       boolean NOT NULL,
    ` + geom("Point"),
				Indexes: []string{gist("sensors")},
				Cols:    []string{"id", "sensor_type", "unit", "installed_on", "active", "geom"},
				Rows:    nSensor,
				Row: func(g *Gen, k int) []any {
					st := gisSensorTypes[gisSensorType(k)]
					return []any{k, st.kind, st.unit, g.Date(2018, 2024), !g.Chance(0.05), ewktPoint(gisRandomPoint(g))}
				},
			},
			{
				Name: "sensor_readings", HasID: true,
				DDL: idCol + `,
    sensor_id   integer NOT NULL,
    measured_at timestamptz NOT NULL,
    value       numeric(8,2) NOT NULL`,
				FKs:     []string{fk("sensor_readings", "sensor_id", "sensors")},
				Indexes: []string{idx("sensor_readings", "sensor_id", "measured_at")},
				Cols:    []string{"id", "sensor_id", "measured_at", "value"},
				Rows:    func(n int) int { return readingsPerSensor * nSensor(n) },
				Row: func(g *Gen, k int) []any {
					s := (k-1)/readingsPerSensor + 1
					h := (k - 1) % readingsPerSensor
					st := gisSensorTypes[gisSensorType(s)]
					at := day(2025, time.March, 3).Add(time.Duration(h) * time.Hour)
					// a daily rhythm: higher in the day than at night, plus noise
					rhythm := 0.5 + 0.5*math.Sin(float64(h%24-6)*math.Pi/12)
					v := st.lo + (st.hi-st.lo)*(0.15+0.6*rhythm+0.25*g.r.Float64())
					return []any{k, s, at, round2(v)}
				},
			},
			{
				Name: "vehicles", HasID: true,
				DDL: idCol + `,
    license_plate text NOT NULL UNIQUE,
    kind          text NOT NULL,
    brand         text NOT NULL,
    driver_name   text,
    driver_phone  text`,
				Cols: []string{"id", "license_plate", "kind", "brand", "driver_name", "driver_phone"},
				Rows: nVehicle,
				Row: func(g *Gen, k int) []any {
					var name, phone any
					if g.Chance(0.85) {
						name, phone = g.FirstName()+" "+g.LastName(), g.Phone()
					}
					return []any{k, g.UniquePlate(k), g.Pick(gisVehicleKinds), g.Pick(carBrands), name, phone}
				},
			},
			{
				Name: "gps_tracks", HasID: true,
				DDL: idCol + `,
    vehicle_id integer NOT NULL,
    started_at timestamptz NOT NULL,
    ended_at   timestamptz NOT NULL,
    ` + geom("LineString") + `,
    CHECK (ended_at > started_at)`,
				FKs:     []string{fk("gps_tracks", "vehicle_id", "vehicles")},
				Indexes: []string{idx("gps_tracks", "vehicle_id", "started_at"), gist("gps_tracks")},
				Cols:    []string{"id", "vehicle_id", "started_at", "ended_at", "geom"},
				Rows:    func(n int) int { return tracksPerVehicle * nVehicle(n) },
				Row: func(g *Gen, k int) []any {
					v := (k-1)/tracksPerVehicle + 1
					start := g.Timestamp(2025, 2025)
					pts := gisWalk(g, gisRandomPoint(g), g.Int(10, 30), 30, 140)
					return []any{k, v, start, start.Add(time.Duration(g.Int(5, 120)) * time.Minute), ewktLine(pts)}
				},
			},
		},
		Views: []string{
			`CREATE VIEW {s}.district_overview AS
SELECT d.id, d.code, d.name, d.population,
       round((ST_Area(d.geom) / 10000)::numeric, 1) AS hectares,
       (SELECT count(*) FROM {s}.buildings b WHERE ST_Contains(d.geom, ST_Centroid(b.geom))) AS buildings,
       (SELECT count(*) FROM {s}.addresses a WHERE ST_Contains(d.geom, a.geom)) AS addresses,
       (SELECT count(*) FROM {s}.sensors s WHERE ST_Contains(d.geom, s.geom)) AS sensors
FROM {s}.districts d`,
			`CREATE VIEW {s}.address_nearest_sensor AS
SELECT a.id AS address_id, a.street_name, a.house_number, a.postcode,
       n.id AS sensor_id, n.sensor_type, round(ST_Distance(a.geom, n.geom)::numeric, 1) AS distance_m
FROM {s}.addresses a
CROSS JOIN LATERAL (SELECT x.id, x.sensor_type, x.geom FROM {s}.sensors x
                    WHERE x.active ORDER BY x.geom <-> a.geom LIMIT 1) n`,
			`CREATE VIEW {s}.vehicle_kilometres AS
SELECT v.id, v.license_plate, v.kind, count(t.id) AS trips,
       round((sum(ST_Length(t.geom)) / 1000)::numeric, 1) AS km
FROM {s}.vehicles v
LEFT JOIN {s}.gps_tracks t ON t.vehicle_id = v.id
GROUP BY v.id`,
		},
	}
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }
