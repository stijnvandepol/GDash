package warnings

import "math"

// town is a place whose region is known.
type town struct {
	latitude, longitude float64
	region              string
}

// towns spread over every region, its border areas included. RegionAt
// takes the region of the nearest one: a coarse map, without shipping
// province borders, that is right for nearly every home and can be wrong
// within a few kilometres of a border. A widget's own region setting
// overrides it; docs/guide.md says so.
var towns = []town{
	{53.22, 6.57, "Groningen"}, {53.33, 6.92, "Groningen"}, {53.14, 7.03, "Groningen"}, {52.99, 6.95, "Groningen"},
	{53.11, 6.88, "Groningen"}, {53.25, 6.40, "Groningen"}, {53.40, 6.68, "Groningen"},
	{53.20, 5.80, "Friesland"}, {53.03, 5.66, "Friesland"}, {52.96, 5.92, "Friesland"}, {53.11, 6.10, "Friesland"},
	{53.17, 5.42, "Friesland"}, {53.33, 6.00, "Friesland"}, {52.84, 5.71, "Friesland"}, {52.99, 6.29, "Friesland"},
	{52.88, 6.00, "Friesland"}, {52.94, 5.40, "Friesland"},
	{52.99, 6.56, "Drenthe"}, {52.79, 6.90, "Drenthe"}, {52.72, 6.48, "Drenthe"}, {52.70, 6.19, "Drenthe"},
	{52.66, 6.74, "Drenthe"}, {52.86, 6.51, "Drenthe"}, {53.14, 6.43, "Drenthe"}, {52.90, 6.84, "Drenthe"},
	{52.51, 6.09, "Overijssel"}, {52.22, 6.89, "Overijssel"}, {52.25, 6.16, "Overijssel"}, {52.36, 6.66, "Overijssel"},
	{52.27, 6.79, "Overijssel"}, {52.56, 5.91, "Overijssel"}, {52.58, 6.62, "Overijssel"}, {52.79, 6.12, "Overijssel"},
	{52.39, 6.28, "Overijssel"}, {52.31, 6.93, "Overijssel"}, {52.31, 6.52, "Overijssel"}, {52.71, 5.95, "Overijssel"},
	{52.52, 5.47, "Flevoland"}, {52.37, 5.21, "Flevoland"}, {52.71, 5.75, "Flevoland"}, {52.53, 5.72, "Flevoland"},
	{52.33, 5.54, "Flevoland"}, {52.66, 5.60, "Flevoland"}, {52.40, 5.33, "Flevoland"},
	{51.98, 5.91, "Gelderland"}, {51.84, 5.86, "Gelderland"}, {52.21, 5.97, "Gelderland"}, {52.04, 5.67, "Gelderland"},
	{51.97, 6.29, "Gelderland"}, {52.34, 5.62, "Gelderland"}, {52.14, 6.20, "Gelderland"}, {51.89, 5.43, "Gelderland"},
	{51.97, 6.72, "Gelderland"}, {51.96, 5.23, "Gelderland"}, {51.81, 5.25, "Gelderland"}, {52.14, 5.58, "Gelderland"},
	{52.45, 5.84, "Gelderland"}, {51.97, 5.67, "Gelderland"}, {52.04, 6.61, "Gelderland"}, {52.22, 5.49, "Gelderland"},
	{51.89, 5.61, "Gelderland"}, {52.30, 6.05, "Gelderland"},
	{52.09, 5.12, "Utrecht"}, {52.16, 5.39, "Utrecht"}, {52.03, 5.56, "Utrecht"}, {52.09, 5.23, "Utrecht"},
	{52.03, 5.08, "Utrecht"}, {52.09, 4.88, "Utrecht"}, {51.97, 5.34, "Utrecht"}, {52.21, 5.29, "Utrecht"},
	{52.17, 5.00, "Utrecht"}, {51.99, 5.09, "Utrecht"}, {51.93, 5.07, "Utrecht"},
	{52.37, 4.90, "Noord-Holland"}, {52.38, 4.64, "Noord-Holland"}, {52.63, 4.75, "Noord-Holland"}, {52.96, 4.76, "Noord-Holland"},
	{52.22, 5.18, "Noord-Holland"}, {52.64, 5.06, "Noord-Holland"}, {52.44, 4.83, "Noord-Holland"}, {52.70, 5.29, "Noord-Holland"},
	{52.50, 4.95, "Noord-Holland"}, {52.30, 4.69, "Noord-Holland"}, {52.30, 5.24, "Noord-Holland"}, {52.31, 5.04, "Noord-Holland"},
	{52.79, 4.80, "Noord-Holland"}, {52.26, 4.76, "Noord-Holland"},
	{52.08, 4.31, "Zuid-Holland"}, {51.92, 4.48, "Zuid-Holland"}, {52.16, 4.49, "Zuid-Holland"}, {51.81, 4.67, "Zuid-Holland"},
	{52.01, 4.36, "Zuid-Holland"}, {52.01, 4.71, "Zuid-Holland"}, {51.83, 4.97, "Zuid-Holland"}, {52.13, 4.66, "Zuid-Holland"},
	{51.85, 4.33, "Zuid-Holland"}, {51.83, 4.13, "Zuid-Holland"}, {51.82, 4.41, "Zuid-Holland"}, {52.24, 4.44, "Zuid-Holland"},
	{51.95, 4.85, "Zuid-Holland"}, {51.76, 4.17, "Zuid-Holland"},
	{51.50, 3.61, "Zeeland"}, {51.44, 3.57, "Zeeland"}, {51.50, 3.89, "Zeeland"}, {51.34, 3.83, "Zeeland"},
	{51.65, 3.92, "Zeeland"}, {51.28, 4.05, "Zeeland"}, {51.33, 3.49, "Zeeland"}, {51.53, 4.22, "Zeeland"},
	{51.56, 3.50, "Zeeland"}, {51.49, 4.05, "Zeeland"},
	{51.69, 5.30, "Noord-Brabant"}, {51.44, 5.48, "Noord-Brabant"}, {51.56, 5.09, "Noord-Brabant"}, {51.59, 4.78, "Noord-Brabant"},
	{51.48, 5.66, "Noord-Brabant"}, {51.77, 5.52, "Noord-Brabant"}, {51.53, 4.46, "Noord-Brabant"}, {51.49, 4.29, "Noord-Brabant"},
	{51.66, 5.62, "Noord-Brabant"}, {51.65, 5.95, "Noord-Brabant"}, {51.69, 5.07, "Noord-Brabant"}, {51.62, 5.55, "Noord-Brabant"},
	{51.35, 5.46, "Noord-Brabant"}, {51.46, 5.79, "Noord-Brabant"}, {51.64, 4.86, "Noord-Brabant"}, {51.73, 5.88, "Noord-Brabant"},
	{51.81, 4.89, "Noord-Brabant"}, {51.57, 4.64, "Noord-Brabant"}, {51.58, 4.32, "Noord-Brabant"}, {51.27, 5.57, "Noord-Brabant"},
	{51.76, 5.74, "Noord-Brabant"},
	{50.85, 5.69, "Limburg"}, {51.37, 6.17, "Limburg"}, {51.19, 5.99, "Limburg"}, {50.89, 5.98, "Limburg"},
	{51.00, 5.87, "Limburg"}, {51.25, 5.71, "Limburg"}, {51.53, 5.97, "Limburg"}, {51.70, 5.97, "Limburg"},
	{50.87, 6.06, "Limburg"}, {50.86, 5.83, "Limburg"}, {51.10, 5.87, "Limburg"}, {51.45, 6.05, "Limburg"},
	{51.33, 5.98, "Limburg"},
	{53.05, 4.80, "Waddeneilanden"}, {53.30, 5.07, "Waddeneilanden"}, {53.36, 5.22, "Waddeneilanden"},
	{53.44, 5.77, "Waddeneilanden"}, {53.48, 6.16, "Waddeneilanden"},
}

const (
	kmPerDegree = 111.2
	// maxTownDistanceKm is how far a place may be from the nearest town and
	// still be in the Netherlands; Antwerp, at about 27 km from the
	// nearest one, is not.
	maxTownDistanceKm = 25
)

// RegionAt returns the warning region of a place in the Netherlands, by
// the nearest town in towns. ok is false for a place outside the country.
func RegionAt(latitude, longitude float64) (string, bool) {
	best, bestKm := "", math.Inf(1)
	for _, t := range towns {
		if km := approximateKm(latitude, longitude, t.latitude, t.longitude); km < bestKm {
			best, bestKm = t.region, km
		}
	}
	return best, bestKm <= maxTownDistanceKm
}

// approximateKm is the distance between two nearby points on a flat map,
// which within a country is close enough to pick the nearest town.
func approximateKm(lat1, lon1, lat2, lon2 float64) float64 {
	dLat := lat2 - lat1
	dLon := (lon2 - lon1) * math.Cos((lat1+lat2)/2*math.Pi/180)
	return kmPerDegree * math.Hypot(dLat, dLon)
}
