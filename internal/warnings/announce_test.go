package warnings

import (
	"testing"
	"time"
)

func severe(id, level string, onset time.Time) Warning {
	return Warning{
		ID: id, Level: level, Kind: "wind", Onset: onset, Expires: onset.Add(6 * time.Hour),
		Event:    map[string]string{"nl": "Zware windstoten", "en": "Severe wind"},
		Headline: map[string]string{"nl": "Code oranje: zware windstoten", "en": "Code orange: severe gusts"},
	}
}

func TestSevereWarningsAreAnnouncedOnceWhenTheyStart(t *testing.T) {
	var a Announcer
	noon := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	wind := severe("w1", LevelOrange, noon)
	brabant := func(ws ...Warning) []Area { return []Area{{Region: "Noord-Brabant", Warnings: ws}} }

	// The first check after a start only learns the state.
	if got := a.Check(brabant(), noon.Add(-time.Hour)); len(got) != 0 {
		t.Fatalf("first check = %+v, want nothing", got)
	}
	// Issued but not started yet: nothing.
	if got := a.Check(brabant(wind), noon.Add(-time.Minute)); len(got) != 0 {
		t.Fatalf("before the onset = %+v, want nothing", got)
	}
	got := a.Check(brabant(wind), noon)
	if len(got) != 1 || got[0].Warning.ID != "w1" || got[0].Region != "Noord-Brabant" {
		t.Fatalf("at the onset = %+v, want the wind warning", got)
	}
	// Still in effect, with a new end time: not again.
	longer := wind
	longer.Expires = wind.Expires.Add(time.Hour)
	if got := a.Check(brabant(longer), noon.Add(time.Hour)); len(got) != 0 {
		t.Fatalf("an update = %+v, want nothing", got)
	}
	// Raised to red: announced again.
	red := longer
	red.Level = LevelRed
	if got := a.Check(brabant(red), noon.Add(2*time.Hour)); len(got) != 1 || got[0].Warning.Level != LevelRed {
		t.Fatalf("raised to red = %+v, want it announced", got)
	}
}

func TestYellowWarningsAndKnownOnesAreNotAnnounced(t *testing.T) {
	var a Announcer
	noon := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	areas := []Area{{Region: "Utrecht", Warnings: []Warning{severe("w1", LevelOrange, noon)}}}
	// In effect when the server starts: not news.
	if got := a.Check(areas, noon.Add(time.Minute)); len(got) != 0 {
		t.Fatalf("after a start = %+v, want nothing", got)
	}
	areas[0].Warnings = append(areas[0].Warnings, severe("w2", LevelYellow, noon))
	if got := a.Check(areas, noon.Add(2*time.Minute)); len(got) != 0 {
		t.Fatalf("a yellow warning = %+v, want nothing", got)
	}
}

func TestTwoWidgetsOnOneRegionAnnounceOnce(t *testing.T) {
	var a Announcer
	noon := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	a.Check([]Area{{Region: "Zeeland"}, {Region: "Zeeland"}}, noon.Add(-time.Hour))
	w := severe("w1", LevelRed, noon)
	got := a.Check([]Area{{Region: "Zeeland", Warnings: []Warning{w}}, {Region: "Zeeland", Warnings: []Warning{w}}}, noon)
	if len(got) != 1 {
		t.Fatalf("got %d notices, want one", len(got))
	}
}

func TestNoticeTextFollowsTheDisplayLanguage(t *testing.T) {
	n := Notice{Region: "Noord-Brabant", Warning: severe("w1", LevelOrange, time.Now())}
	for language, want := range map[string][2]string{
		"nl": {"Code oranje · Noord-Brabant", "Code oranje: zware windstoten"},
		"en": {"Code orange · Noord-Brabant", "Code orange: severe gusts"},
		"de": {"Warnstufe Orange · Noord-Brabant", "Code orange: severe gusts"},
		"xx": {"Code orange · Noord-Brabant", "Code orange: severe gusts"},
	} {
		title, message := n.Text(language)
		if title != want[0] || message != want[1] {
			t.Errorf("%s: %q / %q, want %q / %q", language, title, message, want[0], want[1])
		}
	}
	bare := Notice{Region: "Zeeland", Warning: Warning{Level: LevelRed}}
	if _, message := bare.Text("fr"); message != "Vigilance rouge · Zeeland" {
		t.Errorf("message without texts = %q, want the title", message)
	}
}
