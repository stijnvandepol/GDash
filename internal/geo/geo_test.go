package geo

import (
	"math"
	"testing"
)

func TestDistanceIsGreatCircle(t *testing.T) {
	// Amsterdam Centraal to Rotterdam Centraal is about 57 km as the crow flies.
	if got := DistanceKm(52.3791, 4.9003, 51.9244, 4.4693); math.Abs(got-57) > 1.5 {
		t.Errorf("Amsterdam to Rotterdam = %.1f km, want about 57", got)
	}
	if got := DistanceKm(51.76, 5.52, 51.76, 5.52); got != 0 {
		t.Errorf("distance to itself = %v, want 0", got)
	}
	if got := DistanceKm(0, 0, 0, 180); math.Abs(got-math.Pi*EarthRadiusKm) > 0.001 {
		t.Errorf("half the earth = %.1f km", got)
	}
}
