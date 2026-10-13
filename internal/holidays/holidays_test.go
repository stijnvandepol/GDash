package holidays

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
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

func TestEasterSunday(t *testing.T) {
	// The dates of Easter as the calendars list them, with the earliest and
	// latest possible ones (22 March 1818, 25 April 1943).
	want := map[int]string{
		1818: "1818-03-22", 1943: "1943-04-25", 2000: "2000-04-23", 2019: "2019-04-21", 2024: "2024-03-31",
		2025: "2025-04-20", 2026: "2026-04-05", 2027: "2027-03-28", 2028: "2028-04-16", 2029: "2029-04-01", 2030: "2030-04-21",
	}
	for year, date := range want {
		if got := EasterSunday(year).Format(dateLayout); got != date {
			t.Errorf("EasterSunday(%d) = %s, want %s", year, got, date)
		}
	}
}

func TestPublicHolidaysOf2026(t *testing.T) {
	want := []PublicHoliday{
		{NewYear, "2026-01-01"}, {GoodFriday, "2026-04-03"}, {Easter, "2026-04-05"}, {EasterMonday, "2026-04-06"},
		{KingsDay, "2026-04-27"}, {LiberationDay, "2026-05-05"}, {Ascension, "2026-05-14"}, {WhitSunday, "2026-05-24"},
		{WhitMonday, "2026-05-25"}, {Christmas, "2026-12-25"}, {BoxingDay, "2026-12-26"},
	}
	if got := PublicHolidays(2026); !slices.Equal(got, want) {
		t.Fatalf("PublicHolidays(2026) =\n%v\nwant\n%v", got, want)
	}
}

func TestPublicHolidaysFollowEasterAndStayInOrder(t *testing.T) {
	// 2027: Easter in March, so Easter Monday comes long before King's Day.
	// 2030: Ascension falls on 30 May, after Liberation Day.
	cases := map[int]map[string]string{
		2027: {GoodFriday: "2027-03-26", EasterMonday: "2027-03-29", Ascension: "2027-05-06", WhitMonday: "2027-05-17"},
		2030: {GoodFriday: "2030-04-19", EasterMonday: "2030-04-22", Ascension: "2030-05-30", WhitSunday: "2030-06-09"},
	}
	for year, want := range cases {
		list := PublicHolidays(year)
		if len(list) != 11 || !slices.IsSortedFunc(list, func(a, b PublicHoliday) int { return strings.Compare(a.Date, b.Date) }) {
			t.Errorf("PublicHolidays(%d) = %v, want 11 in date order", year, list)
		}
		for _, h := range list {
			if date, ok := want[h.Kind]; ok && h.Date != date {
				t.Errorf("%d: %s on %s, want %s", year, h.Kind, h.Date, date)
			}
		}
	}
}

func TestKingsDayMovesToSaturdayWhenTheTwentySeventhIsASunday(t *testing.T) {
	// 27 April was a Sunday in 2014 and 2025 and is again in 2031.
	want := map[int]string{2014: "2014-04-26", 2025: "2025-04-26", 2026: "2026-04-27", 2028: "2028-04-27", 2031: "2031-04-26"}
	for year, date := range want {
		i := slices.IndexFunc(PublicHolidays(year), func(h PublicHoliday) bool { return h.Kind == KingsDay })
		if got := PublicHolidays(year)[i].Date; got != date {
			t.Errorf("King's Day %d = %s, want %s", year, got, date)
		}
	}
}

func TestUpcomingPublicHolidaysStartToday(t *testing.T) {
	loc := amsterdam(t)
	got := UpcomingPublicHolidays(time.Date(2026, 10, 13, 9, 0, 0, 0, loc))
	if len(got) != 13 || got[0] != (PublicHoliday{Christmas, "2026-12-25"}) || got[12] != (PublicHoliday{BoxingDay, "2027-12-26"}) {
		t.Fatalf("upcoming on 13 October 2026 = %v, want Christmas 2026 to Boxing Day 2027", got)
	}
	// On a holiday itself, the holiday is still to come.
	if got := UpcomingPublicHolidays(time.Date(2026, 12, 25, 23, 0, 0, 0, loc)); got[0].Kind != Christmas {
		t.Fatalf("upcoming on Christmas Day starts with %v", got[0])
	}
}

func readFixture(t *testing.T) []byte {
	t.Helper()
	body, err := os.ReadFile("testdata/schoolholidays.json")
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func TestParseSchoolHolidays(t *testing.T) {
	// The fixture is the government's answer for the school years 2025-2026
	// and 2026-2027, as served in October 2026.
	got, err := parseSchool(strings.NewReader(string(readFixture(t))), amsterdam(t), "2026-10-13")
	if err != nil {
		t.Fatal(err)
	}
	christmas := SchoolHoliday{ChristmasBreak, "Kerstvakantie", "2026-12-19", "2027-01-03"}
	want := map[string][]SchoolHoliday{
		// The north is in its autumn holiday on the 13th; it ends on Sunday.
		North: {
			{Autumn, "Herfstvakantie", "2026-10-10", "2026-10-18"}, christmas,
			{Spring, "Voorjaarsvakantie", "2027-02-20", "2027-02-28"}, {May, "Meivakantie", "2027-04-24", "2027-05-02"},
			{Summer, "Zomervakantie", "2027-07-10", "2027-08-22"},
		},
		Middle: {
			{Autumn, "Herfstvakantie", "2026-10-17", "2026-10-25"}, christmas,
			{Spring, "Voorjaarsvakantie", "2027-02-20", "2027-02-28"}, {May, "Meivakantie", "2027-04-24", "2027-05-02"},
			{Summer, "Zomervakantie", "2027-07-17", "2027-08-29"},
		},
		South: {
			{Autumn, "Herfstvakantie", "2026-10-17", "2026-10-25"}, christmas,
			{Spring, "Voorjaarsvakantie", "2027-02-13", "2027-02-21"}, {May, "Meivakantie", "2027-04-24", "2027-05-02"},
			{Summer, "Zomervakantie", "2027-07-24", "2027-09-05"},
		},
	}
	for _, region := range Regions {
		if !slices.Equal(got[region], want[region]) {
			t.Errorf("%s =\n%v\nwant\n%v", region, got[region], want[region])
		}
	}
}

func TestParseSchoolSkipsWhatItCannotRead(t *testing.T) {
	feed := `[{"content": [{"vacations": [
		{"type": " Pinkstervakantie ", "regions": [{"region": " heel Nederland ", "startdate": "2027-05-15T00:00:00.000Z", "enddate": "2027-05-17T21:59:00.000Z"}]},
		{"type": "Herfstvakantie", "regions": [
			{"region": "oost", "startdate": "2027-10-16T00:00:00.000Z", "enddate": "2027-10-24T21:59:00.000Z"},
			{"region": "noord", "startdate": "16 oktober", "enddate": "2027-10-24T21:59:00.000Z"},
			{"region": "midden", "startdate": "2027-10-24T00:00:00.000Z", "enddate": "2027-10-16T21:59:00.000Z"},
			{"region": "zuid", "startdate": "2027-10-23T00:00:00.000Z", "enddate": "2027-10-31T22:59:00.000Z"}
		]}
	]}]}]`
	got, err := parseSchool(strings.NewReader(feed), amsterdam(t), "2027-01-01")
	if err != nil {
		t.Fatal(err)
	}
	// An unknown holiday keeps its name; an unknown region, an unreadable
	// date and an end before the start are left out.
	other := SchoolHoliday{OtherSchoolKind, "Pinkstervakantie", "2027-05-15", "2027-05-17"}
	if !slices.Equal(got[North], []SchoolHoliday{other}) || !slices.Equal(got[Middle], []SchoolHoliday{other}) {
		t.Errorf("north = %v, middle = %v, want only the unknown holiday", got[North], got[Middle])
	}
	if want := []SchoolHoliday{other, {Autumn, "Herfstvakantie", "2027-10-23", "2027-10-31"}}; !slices.Equal(got[South], want) {
		t.Errorf("south = %v, want %v", got[South], want)
	}
}

func TestParseSchoolFailsOnAFeedWithoutHolidays(t *testing.T) {
	for name, body := range map[string]string{"not json": "<html>", "empty": "[]", "no vacations": `[{"content": [{"vacations": []}]}]`} {
		if _, err := parseSchool(strings.NewReader(body), amsterdam(t), "2026-10-13"); err == nil {
			t.Errorf("%s: parseSchool succeeded", name)
		}
	}
	// Holidays that have all ended are a feed that is not updated yet, not
	// a broken one: the regions are empty.
	got, err := parseSchool(strings.NewReader(string(readFixture(t))), amsterdam(t), "2030-01-01")
	if err != nil || len(got[Middle]) != 0 {
		t.Errorf("after the last holiday = %v, %v; want no holidays and no error", got, err)
	}
}

func client(t *testing.T, url string, now time.Time) Client {
	t.Helper()
	return Client{SchoolURL: url, HTTP: http.DefaultClient, UserAgent: "GDash/test", Location: amsterdam(t), Now: func() time.Time { return now }}
}

func TestReport(t *testing.T) {
	body := readFixture(t)
	requests := 0
	agent := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		agent = r.UserAgent()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	// 23:30 UTC on the 12th is already the 13th in Amsterdam.
	now := time.Date(2026, 10, 12, 23, 30, 0, 0, time.UTC)

	public, err := client(t, srv.URL, now).Report(context.Background(), false)
	if err != nil || requests != 0 || public.Middle != nil || len(public.Public) != 13 {
		t.Fatalf("public only = %+v, %v after %d requests; want 13 holidays without a request", public, err, requests)
	}
	full, err := client(t, srv.URL, now).Report(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if requests != 1 || agent != "GDash/test" {
		t.Errorf("%d requests with User-Agent %q, want 1 with GDash/test", requests, agent)
	}
	if len(full.Public) != 13 || len(full.North) != 5 || full.North[0].Start != "2026-10-10" || full.Middle[0].Start != "2026-10-17" {
		t.Errorf("report = %+v", full)
	}
}

func TestReportTellsAnEmptyRegionFromOneNotAskedFor(t *testing.T) {
	body := readFixture(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(body) }))
	t.Cleanup(srv.Close)
	// After the fixture's last holiday every region is empty, and still there.
	later := client(t, srv.URL, time.Date(2030, 1, 1, 12, 0, 0, 0, time.UTC))
	for withSchool, want := range map[bool]string{false: `{"public":[`, true: `"north":[],"middle":[],"south":[]}`} {
		report, err := later.Report(context.Background(), withSchool)
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(report)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(encoded), want) || (!withSchool && strings.Contains(string(encoded), "north")) {
			t.Errorf("school %v: %s, want it to hold %s", withSchool, encoded, want)
		}
	}
}

func TestReportFailsWhenTheFeedFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "down", http.StatusBadGateway) }))
	t.Cleanup(srv.Close)
	if _, err := client(t, srv.URL, time.Now()).Report(context.Background(), true); err == nil {
		t.Fatal("Report succeeded while the feed answered 502")
	}
}
