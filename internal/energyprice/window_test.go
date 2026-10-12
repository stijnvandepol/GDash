package energyprice

import (
	"math"
	"testing"
	"time"
)

// hoursFrom builds consecutive hours starting at 00:00 on 12 October 2026
// in loc.
func hoursFrom(loc *time.Location, prices ...float64) []Hour {
	start := time.Date(2026, 10, 12, 0, 0, 0, 0, loc)
	hours := make([]Hour, len(prices))
	for i, p := range prices {
		hours[i] = Hour{Start: start.Add(time.Duration(i) * time.Hour), Price: p}
	}
	return hours
}

func TestCheapestWindow(t *testing.T) {
	loc := time.UTC
	tests := []struct {
		name      string
		hours     []Hour
		wantStart int
		wantAvg   float64
		wantOK    bool
	}{
		{"cheapest in the middle", hoursFrom(loc, 0.30, 0.25, 0.10, 0.12, 0.11, 0.30, 0.35), 2, 0.11, true},
		{"cheapest at the start", hoursFrom(loc, 0.10, 0.10, 0.10, 0.30, 0.30), 0, 0.10, true},
		{"cheapest at the end", hoursFrom(loc, 0.30, 0.30, 0.30, 0.10, 0.10, 0.10), 3, 0.10, true},
		{"ties go to the earliest", hoursFrom(loc, 0.20, 0.20, 0.20, 0.20, 0.20), 0, 0.20, true},
		{"negative prices win", hoursFrom(loc, 0.10, -0.02, -0.05, -0.03, 0.10), 1, -0.1 / 3, true},
		{"exactly three hours", hoursFrom(loc, 0.30, 0.20, 0.10), 0, 0.20, true},
		{"too few hours", hoursFrom(loc, 0.30, 0.20), 0, 0, false},
		{"no hours", nil, 0, 0, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := CheapestWindow(tc.hours, WindowHours)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if !ok {
				return
			}
			if want := tc.hours[tc.wantStart].Start; !got.Start.Equal(want) {
				t.Errorf("start = %v, want %v", got.Start, want)
			}
			if want := tc.hours[tc.wantStart].Start.Add(WindowHours * time.Hour); !got.End.Equal(want) {
				t.Errorf("end = %v, want %v", got.End, want)
			}
			if math.Abs(got.Average-tc.wantAvg) > 1e-9 {
				t.Errorf("average = %v, want %v", got.Average, tc.wantAvg)
			}
		})
	}
}

func TestCheapestWindowSkipsAGap(t *testing.T) {
	hours := hoursFrom(time.UTC, 0.30, 0.30, 0.01, 0.30, 0.01, 0.01, 0.20, 0.20, 0.20)
	// The hour at 03:00 is missing. The cheap hours at 02:00, 04:00 and
	// 05:00 are not consecutive, so they are no stretch together.
	hours = append(hours[:3], hours[4:]...)
	got, ok := CheapestWindow(hours, WindowHours)
	if !ok || got.Start.Hour() != 4 {
		t.Fatalf("window = %+v (%v), want the stretch from 04:00", got, ok)
	}
}

func TestCheapestWindowOnTheDayTheClocksGoBack(t *testing.T) {
	loc := amsterdam(t)
	// 25 October 2026 has 25 hours; the stretch is counted in real hours,
	// and 02:00 appears twice on the wall clock.
	start := time.Date(2026, 10, 25, 0, 0, 0, 0, loc)
	prices := make([]float64, 25)
	for i := range prices {
		prices[i] = 0.30
	}
	prices[3], prices[4], prices[5] = 0.05, 0.05, 0.05
	hours := make([]Hour, len(prices))
	for i, p := range prices {
		hours[i] = Hour{Start: start.Add(time.Duration(i) * time.Hour), Price: p}
	}
	got, ok := CheapestWindow(hours, WindowHours)
	if !ok {
		t.Fatal("no window")
	}
	if want := start.Add(3 * time.Hour); !got.Start.Equal(want) {
		t.Errorf("start = %v, want %v", got.Start, want)
	}
	if got.End.Sub(got.Start) != 3*time.Hour {
		t.Errorf("window is %v long, want 3h", got.End.Sub(got.Start))
	}
}
