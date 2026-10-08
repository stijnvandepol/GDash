package schedule

import (
	"testing"
	"time"
)

func at(hhmm string) time.Time {
	t, _ := time.Parse("15:04", hhmm)
	return time.Date(2026, 10, 8, t.Hour(), t.Minute(), 0, 0, time.Local)
}

func TestContains(t *testing.T) {
	tests := []struct {
		hours Hours
		at    string
		want  bool
	}{
		{Hours{}, "03:00", true},
		{Hours{"06:30", "23:00"}, "06:29", false},
		{Hours{"06:30", "23:00"}, "06:30", true},
		{Hours{"06:30", "23:00"}, "22:59", true},
		{Hours{"06:30", "23:00"}, "23:00", false},
		// A night window wraps past midnight.
		{Hours{"22:00", "06:00"}, "23:30", true},
		{Hours{"22:00", "06:00"}, "05:59", true},
		{Hours{"22:00", "06:00"}, "12:00", false},
		// Invalid hours never switch a screen off.
		{Hours{"7", "23:00"}, "03:00", true},
		{Hours{"08:00", "08:00"}, "03:00", true},
	}
	for _, tt := range tests {
		if got := tt.hours.Contains(at(tt.at)); got != tt.want {
			t.Errorf("%+v.Contains(%s) = %v, want %v", tt.hours, tt.at, got, tt.want)
		}
	}
}

func TestValidate(t *testing.T) {
	for _, ok := range []Hours{{}, {"06:30", "23:00"}, {"22:00", "06:00"}} {
		if err := ok.Validate(); err != nil {
			t.Errorf("%+v: %v", ok, err)
		}
	}
	for _, bad := range []Hours{{"06:30", ""}, {"6:30", "23:00"}, {"25:00", "23:00"}, {"08:00", "08:00"}, {"morning", "evening"}} {
		if err := bad.Validate(); err == nil {
			t.Errorf("%+v accepted", bad)
		}
	}
}
