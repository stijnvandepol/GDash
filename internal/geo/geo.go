// Package geo holds the one great-circle distance that the widgets which
// look "near a place" (fuel, traffic, weather stations) share.
package geo

import "math"

// EarthRadiusKm is the mean radius of the earth.
const EarthRadiusKm = 6371.0

// DistanceKm is the great-circle distance between two points (haversine).
func DistanceKm(lat1, lon1, lat2, lon2 float64) float64 {
	rad := math.Pi / 180
	dLat := (lat2 - lat1) * rad
	dLon := (lon2 - lon1) * rad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * EarthRadiusKm * math.Asin(math.Min(1, math.Sqrt(a)))
}
