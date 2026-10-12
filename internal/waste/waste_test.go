package waste

import (
	"os"
	"slices"
	"testing"
	"time"

	"github.com/stijnvandepol/gdash/internal/calendar"
)

func TestClassify(t *testing.T) {
	tests := []struct {
		title string
		want  Kind
	}{
		{"GFT", Organic},
		{"gft container", Organic},
		{"GFT (groente-, fruit- en tuinafval)", Organic},
		{"Groene bak", Organic},
		{"Green waste", Organic},
		{"Organic waste", Organic},
		{"PMD", Plastic},
		{"Plastic, blik en drinkpakken (PMD)", Plastic},
		{"Plastic, metaal en drankkartons", Plastic},
		{"Packaging", Plastic},
		{"Verpakkingen", Plastic},
		{"Papier", Paper},
		{"Papier en karton", Paper},
		{"Paper and cardboard", Paper},
		{"Restafval", General},
		{"Rest", General},
		{"Grijze container: rest", General},
		{"General waste", General},
		{"Household waste", General},
		{"Glas", Glass},
		{"Glass bank", Glass},
		{"Groen glas", Glass},
		{"Textiel", Textile},
		{"Textile collection", Textile},
		{"Kleding", Textile},
		{"Grofvuil op afspraak", Other},
		{"Restaurant bezoek", Other},
		{"", Other},
		{"   ", Other},
	}
	for _, tc := range tests {
		if got := Classify(tc.title); got != tc.want {
			t.Errorf("Classify(%q) = %q, want %q", tc.title, got, tc.want)
		}
	}
}

func TestPickupsFromARecordedCalendar(t *testing.T) {
	file, err := os.Open("testdata/afvalkalender.ics")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	// A fixed offset instead of a named zone: the test must not depend on
	// a time zone database being installed.
	loc := time.FixedZone("CEST", 2*60*60)
	from := time.Date(2026, 10, 12, 0, 0, 0, 0, loc)
	events, err := calendar.Parse(file, calendar.Window{From: from, To: from.AddDate(0, 0, 21), Location: loc}, "Afval")
	if err != nil {
		t.Fatal(err)
	}

	type day struct {
		date string
		kind Kind
	}
	var got []day
	for _, p := range Pickups(events) {
		got = append(got, day{p.Date, p.Kind})
	}
	want := []day{
		{"2026-10-13", Organic}, {"2026-10-15", General}, {"2026-10-16", Plastic},
		// 06:00 UTC is 08:00 on the same day.
		{"2026-10-20", Paper}, {"2026-10-22", Other}, {"2026-10-27", Organic},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("pickups = %v, want %v", got, want)
	}
}

func TestPickupsKeepTheTitleOfUnknownKinds(t *testing.T) {
	pickups := Pickups([]calendar.Event{{Title: "Grofvuil op afspraak", Start: "2026-10-22", AllDay: true}})
	if len(pickups) != 1 || pickups[0].Kind != Other || pickups[0].Title != "Grofvuil op afspraak" {
		t.Fatalf("pickups = %+v", pickups)
	}
}

func TestPickupsSkipEventsWithoutADate(t *testing.T) {
	events := []calendar.Event{
		{Title: "GFT", Start: "soon", AllDay: true},
		{Title: "GFT", Start: "2026-10-13T20:00:00"},
		{Title: "Glas", Start: "2026-10-14T23:30:00+02:00"},
	}
	pickups := Pickups(events)
	if len(pickups) != 1 || pickups[0].Date != "2026-10-14" {
		t.Fatalf("pickups = %+v, want only the glass pickup on 14 October", pickups)
	}
}

func TestPickupsAreBounded(t *testing.T) {
	events := make([]calendar.Event, 3*maxPickups)
	for i := range events {
		events[i] = calendar.Event{Title: "GFT", Start: "2026-10-13", AllDay: true}
	}
	if got := len(Pickups(events)); got != maxPickups {
		t.Fatalf("%d pickups, want %d", got, maxPickups)
	}
}

func TestNoEventsGiveAnEmptyListNotNull(t *testing.T) {
	if Pickups(nil) == nil {
		t.Fatal("Pickups(nil) = nil, want an empty list so the JSON is []")
	}
}
