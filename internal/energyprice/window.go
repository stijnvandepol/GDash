package energyprice

import "time"

// WindowHours is how long the cheapest stretch is: three hours is about
// what a dishwasher cycle or a laundry load takes.
const WindowHours = 3

// Window is a stretch of consecutive hours and its average price.
type Window struct {
	Start time.Time `json:"start"`
	// End is when the last hour ends, so 13:00 to 16:00 covers three hours.
	End     time.Time `json:"end"`
	Average float64   `json:"average"`
}

// CheapestWindow finds the stretch of length consecutive hours with the
// lowest average price. hours must be sorted; a gap in them (a missing
// hour) breaks a stretch, because the cheap hours around it are not
// consecutive. When several stretches cost the same, the earliest wins.
// ok is false when no stretch of that length exists.
func CheapestWindow(hours []Hour, length int) (Window, bool) {
	best, found := Window{}, false
	for i := 0; i+length <= len(hours); i++ {
		run := hours[i : i+length]
		if !consecutive(run) {
			continue
		}
		var total float64
		for _, h := range run {
			total += h.Price
		}
		average := total / float64(length)
		// A strict comparison keeps the earliest of equal stretches. The
		// prices are cents, but sums of floats are not exact, so a tie is
		// anything within a hundredth of a cent.
		if !found || average < best.Average-tieMargin {
			best = Window{Start: run[0].Start, End: run[length-1].Start.Add(time.Hour), Average: average}
			found = true
		}
	}
	return best, found
}

// tieMargin is far below the cent that prices are given in.
const tieMargin = 0.0001

func consecutive(run []Hour) bool {
	for i := 1; i < len(run); i++ {
		if !run[i].Start.Equal(run[i-1].Start.Add(time.Hour)) {
			return false
		}
	}
	return true
}
