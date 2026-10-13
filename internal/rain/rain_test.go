package rain

import (
	"os"
	"strings"
	"testing"
	"time"
	_ "time/tzdata" // the tests do not rely on the machine having a time zone database
)

func amsterdam(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Amsterdam")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func fixture(t *testing.T, name string) string {
	t.Helper()
	body, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func TestBuienradarLinesBecomeIntensities(t *testing.T) {
	loc := amsterdam(t)
	now := time.Date(2026, 10, 13, 14, 3, 0, 0, loc)

	points, err := parseBuienradar(fixture(t, "buienradar-shower.txt"), now, loc)
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 24 {
		t.Fatalf("%d points, want the 24 five-minute steps of two hours", len(points))
	}
	if first := points[0].Time; !first.Equal(time.Date(2026, 10, 13, 14, 5, 0, 0, loc)) {
		t.Errorf("first point at %v, want 14:05 today", first)
	}
	// 0 is dry, 77 is 0.1 mm/h, 109 is 1 mm/h and 141 is 10 mm/h.
	want := map[int]float64{0: 0, 4: 0.1, 8: 10, 10: 0.52}
	for i, mm := range want {
		if points[i].MMPerHour != mm {
			t.Errorf("point %d (%s) = %v mm/h, want %v", i, points[i].Time.Format("15:04"), points[i].MMPerHour, mm)
		}
	}
}

func TestBuienradarForecastRunsPastMidnight(t *testing.T) {
	loc := amsterdam(t)
	now := time.Date(2026, 10, 13, 23, 18, 0, 0, loc)

	points, err := parseBuienradar(fixture(t, "buienradar-midnight.txt"), now, loc)
	if err != nil {
		t.Fatal(err)
	}
	last := points[len(points)-1].Time
	if !last.Equal(time.Date(2026, 10, 14, 1, 15, 0, 0, loc)) {
		t.Fatalf("last point at %v, want 01:15 the next day", last)
	}
	for i := 1; i < len(points); i++ {
		if points[i].Time.Sub(points[i-1].Time) != 5*time.Minute {
			t.Fatalf("points %d and %d are %v apart, want 5 minutes", i-1, i, points[i].Time.Sub(points[i-1].Time))
		}
	}
}

func TestBuienradarAnswerJustAfterMidnightStaysOnToday(t *testing.T) {
	loc := amsterdam(t)
	// The server's clock says 00:02, the first line is from 23:55: that is
	// yesterday evening, not tonight.
	now := time.Date(2026, 10, 14, 0, 2, 0, 0, loc)
	body := "000|23:55\n000|00:00\n000|00:05\n000|00:10\n000|00:15\n000|00:20\n"

	points, err := parseBuienradar(body, now, loc)
	if err != nil {
		t.Fatal(err)
	}
	if first := points[0].Time; !first.Equal(time.Date(2026, 10, 13, 23, 55, 0, 0, loc)) {
		t.Fatalf("first point at %v, want 23:55 on the 13th", first)
	}
}

func TestBrokenBuienradarLinesAreSkippedAndTooFewFail(t *testing.T) {
	loc := amsterdam(t)
	now := time.Date(2026, 10, 13, 14, 3, 0, 0, loc)
	broken := "000|14:05\r\nabc|14:10\r\n300|14:15\r\n010\r\n020|25:99\r\n\r\n"
	if _, err := parseBuienradar(broken, now, loc); err == nil {
		t.Fatal("an answer with one readable line was accepted as a forecast")
	}
	body := fixture(t, "buienradar-shower.txt")
	withNoise := strings.Replace(body, "000|14:10", "garbage", 1)
	points, err := parseBuienradar(withNoise, now, loc)
	if err != nil || len(points) != 23 {
		t.Fatalf("points = %d, err = %v; want the 23 readable lines", len(points), err)
	}
	if _, err := parseBuienradar("Not found: location must be inside the Netherlands or Belgium.", now, loc); err == nil {
		t.Fatal("Buienradar's refusal was read as a forecast")
	}
}

func TestOpenMeteoTotalsBecomeQuarterHours(t *testing.T) {
	now := time.Date(2026, 10, 13, 12, 7, 0, 0, time.UTC) // 14:07 in Paris

	points, err := parseOpenMeteo([]byte(fixture(t, "openmeteo-paris.json")), now)
	if err != nil {
		t.Fatal(err)
	}
	// The quarter that ended at 14:00 is over; the one with no value is left out.
	if len(points) != 8 {
		t.Fatalf("%d points, want 8", len(points))
	}
	first := points[0]
	if first.Time.Format(time.RFC3339) != "2026-10-13T14:00:00+02:00" || first.MMPerHour != 0 {
		t.Errorf("first point = %+v, want the quarter from 14:00 local time, dry", first)
	}
	// 0.6 mm in the quarter until 14:45 is 2.4 mm/h from 14:30.
	if p := points[2]; p.Time.Format("15:04") != "14:30" || p.MMPerHour != 2.4 {
		t.Errorf("third point = %+v, want 2.4 mm/h from 14:30", p)
	}
}

func TestOpenMeteoAnswersThatAreNoForecastFail(t *testing.T) {
	now := time.Date(2026, 10, 13, 12, 7, 0, 0, time.UTC)
	for name, body := range map[string]string{
		"not json":     `<html>`,
		"no offset":    `{"minutely_15":{"time":["2026-10-13T15:00"],"precipitation":[0]}}`,
		"no quarters":  `{"utc_offset_seconds":0,"minutely_15":{"time":[],"precipitation":[]}}`,
		"all negative": `{"utc_offset_seconds":0,"minutely_15":{"time":["2026-10-13T13:00","2026-10-13T13:15","2026-10-13T13:30","2026-10-13T13:45"],"precipitation":[-1,-1,-1,-1]}}`,
	} {
		if _, err := parseOpenMeteo([]byte(body), now); err == nil {
			t.Errorf("%s: accepted as a forecast", name)
		}
	}
}

func TestKeysMatchTheWidget(t *testing.T) {
	for _, tc := range []struct {
		q    Query
		want string
	}{
		{Query{Here: true, Latitude: 51.77, Longitude: 5.53}, "here"},
		{Query{Latitude: 51.765, Longitude: 5.514}, "51.77,5.51"},
		{Query{Latitude: 52.1, Longitude: -0.001}, "52.1,0"},
	} {
		if got := tc.q.Key(); got != tc.want {
			t.Errorf("Key(%+v) = %q, want %q", tc.q, got, tc.want)
		}
	}
}
