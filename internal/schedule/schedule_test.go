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

func TestWindowDays(t *testing.T) {
	// 9 October 2026 is a Friday.
	friday := func(hhmm string) time.Time { return at(hhmm).AddDate(0, 0, 1) }
	weekend := Window{Hours: Hours{"08:00", "12:00"}, Days: []string{"sat", "sun"}}
	night := Window{Hours: Hours{"22:00", "01:00"}, Days: []string{"fri"}}
	tests := []struct {
		name string
		w    Window
		t    time.Time
		want bool
	}{
		{"weekend window on a Friday", weekend, friday("09:00"), false},
		{"weekend window on a Saturday", weekend, friday("09:00").AddDate(0, 0, 1), true},
		{"weekend window on a Sunday", weekend, friday("09:00").AddDate(0, 0, 2), true},
		{"weekend window outside its hours", weekend, friday("13:00").AddDate(0, 0, 1), false},
		{"every day without days", Window{Hours: Hours{"08:00", "12:00"}}, friday("09:00"), true},
		{"Friday night before midnight", night, friday("23:00"), true},
		{"Friday night after midnight, on Saturday", night, friday("00:30").AddDate(0, 0, 1), true},
		{"Thursday night after midnight, on Friday", night, friday("00:30"), false},
	}
	for _, tt := range tests {
		if got := tt.w.Contains(tt.t); got != tt.want {
			t.Errorf("%s: Contains(%s) = %v, want %v", tt.name, tt.t.Format("Mon 15:04"), got, tt.want)
		}
	}
}

func TestPlan(t *testing.T) {
	// The week of 5 to 11 October 2026: Monday to Sunday.
	day := func(weekday int, hhmm string) time.Time { return at(hhmm).AddDate(0, 0, weekday-3) }
	plan := Plan{
		{Days: []string{"mon", "tue", "wed"}, From: "06:00", Until: "10:00"},
		{Days: []string{"fri"}, From: "07:00", Until: "12:00"},
		{Days: []string{"sat", "sun"}, From: "09:00", Until: "16:00"},
		{Days: []string{"sat"}, From: "19:00", Until: "23:00"},
		{Days: []string{"fri"}, From: "22:00", Until: "01:00"},
	}
	tests := []struct {
		name string
		t    time.Time
		want bool
	}{
		{"Monday morning", day(0, "07:00"), true},
		{"Monday after its block", day(0, "10:00"), false},
		{"Thursday has no block", day(3, "08:00"), false},
		{"Friday morning", day(4, "08:00"), true},
		{"Friday afternoon", day(4, "13:00"), false},
		{"Saturday day block", day(5, "10:00"), true},
		{"Saturday between its two blocks", day(5, "17:00"), false},
		{"Saturday evening block", day(5, "20:00"), true},
		{"Sunday has only the day block", day(6, "20:00"), false},
		{"Friday night before midnight", day(4, "23:30"), true},
		{"Saturday just after Friday's night block", day(5, "00:30"), true},
	}
	for _, tt := range tests {
		if got := plan.Contains(tt.t); got != tt.want {
			t.Errorf("%s: Contains(%s) = %v, want %v", tt.name, tt.t.Format("Mon 15:04"), got, tt.want)
		}
	}
	if !(Plan{}).Contains(day(2, "03:00")) {
		t.Error("a plan without blocks is not always on")
	}
	if i, err := plan.Validate(); err != nil {
		t.Errorf("Validate = %d, %v", i, err)
	}
	if i, err := (Plan{{From: "06:00", Until: "10:00"}, {From: "08:00", Until: "08:00"}}).Validate(); err == nil || i != 1 {
		t.Errorf("equal times: %d, %v", i, err)
	}
	if i, err := make(Plan, MaxBlocks+1).Validate(); err == nil || i != -2 {
		t.Errorf("too many: %d, %v", i, err)
	}
	if p := (Hours{From: "06:30", Until: "23:00"}).Plan(); len(p) != 1 || p[0].From != "06:30" || len(p[0].Days) != 0 {
		t.Errorf("Hours.Plan = %+v", p)
	}
	if p := (Hours{}).Plan(); len(p) != 0 {
		t.Errorf("always-on Hours.Plan = %+v", p)
	}
}

func TestWindowValidate(t *testing.T) {
	for _, ok := range [][]string{nil, {"mon"}, {"sat", "sun"}, Weekdays} {
		if err := (Window{Days: ok}).Validate(); err != nil {
			t.Errorf("%v: %v", ok, err)
		}
	}
	for _, bad := range [][]string{{"Monday"}, {"mon", "mon"}, {""}} {
		if err := (Window{Days: bad}).Validate(); err == nil {
			t.Errorf("%v accepted", bad)
		}
	}
}
