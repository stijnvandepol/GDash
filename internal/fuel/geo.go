package fuel

import "math"

const earthRadiusKm = 6371.0

// minCosine keeps the width of the box finite next to a pole, where a
// degree of longitude is hardly any distance.
const minCosine = 0.01

// distanceKm is the great-circle distance between two points (haversine).
func distanceKm(lat1, lon1, lat2, lon2 float64) float64 {
	rad := math.Pi / 180
	dLat := (lat2 - lat1) * rad
	dLon := (lon2 - lon1) * rad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * earthRadiusKm * math.Asin(math.Min(1, math.Sqrt(a)))
}

// boundingBox is the smallest box around the circle, which is what the ANWB
// takes as a filter. The circle itself is cut out afterwards.
func (a Area) boundingBox() (minLat, minLon, maxLat, maxLon float64) {
	deg := 180 / math.Pi
	dLat := float64(a.RadiusKm) / earthRadiusKm * deg
	dLon := dLat / math.Max(minCosine, math.Cos(a.Latitude*math.Pi/180))
	return max(-90, a.Latitude-dLat), max(-180, a.Longitude-dLon), min(90, a.Latitude+dLat), min(180, a.Longitude+dLon)
}

// valid reports whether the area is a point on earth and a radius the
// widget allows. Only such areas go into a request.
func (a Area) valid() bool {
	return a.Latitude >= -90 && a.Latitude <= 90 && a.Longitude >= -180 && a.Longitude <= 180 &&
		a.RadiusKm >= MinRadiusKm && a.RadiusKm <= MaxRadiusKm
}
