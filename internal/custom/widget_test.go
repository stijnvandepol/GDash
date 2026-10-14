package custom

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

// settingsOf decodes settings as a dashboard stores them.
func settingsOf(t *testing.T, s string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func mustWidget(t *testing.T, s string) Widget {
	t.Helper()
	w, err := ParseWidget(settingsOf(t, s))
	if err != nil {
		t.Fatalf("ParseWidget(%s): %v", s, err)
	}
	return w
}

func TestParseWidgetAcceptsEachMode(t *testing.T) {
	for _, s := range []string{
		`{"source": 1, "mode": "value", "path": "data.items[0].price", "label": "Milk", "unit": "€", "decimals": 2, "title": "Shop"}`,
		`{"source": 1, "mode": "list", "path": "data.items", "fields": ["name", "price"]}`,
		`{"source": 1, "mode": "keyValues", "pairs": [{"label": "First", "path": "data.items[0].name"}]}`,
		`{"source": 1, "mode": "badges", "pairs": [{"label": "ST", "path": "home.stijn"}]}`,
		`{"source": 1, "mode": "value", "path": "ok", "rules": [
			{"when": "value == true", "tone": "ok"},
			{"when": "value == false", "message": "Down", "announce": true, "hide": true}]}`,
	} {
		mustWidget(t, s)
	}
}

func TestParseWidgetNamesTheSetting(t *testing.T) {
	cases := map[string]string{
		`{"mode": "value", "path": "a"}`:                                                                 "source",
		`{"source": 0, "mode": "value", "path": "a"}`:                                                    "source",
		`{"source": 1.5, "mode": "value", "path": "a"}`:                                                  "source",
		`{"source": 1, "mode": "chart", "path": "a"}`:                                                    "mode",
		`{"source": 1, "mode": "value"}`:                                                                 "path",
		`{"source": 1, "mode": "value", "path": "a..b"}`:                                                 "path",
		`{"source": 1, "mode": "value", "path": "a", "extra": 1}`:                                        "extra",
		`{"source": 1, "mode": "value", "path": "a", "decimals": 7}`:                                     "decimals",
		`{"source": 1, "mode": "value", "path": "a", "unit": "far too long a unit"}`:                     "unit",
		`{"source": 1, "mode": "value", "path": "a", "title": "two\nlines"}`:                             "title",
		`{"source": 1, "mode": "value", "path": "a", "hideTitle": "yes"}`:                                "hideTitle",
		`{"source": 1, "mode": "list", "path": "a"}`:                                                     "fields",
		`{"source": 1, "mode": "list", "path": "a", "fields": ["a", "b", "c", "d"]}`:                     "fields",
		`{"source": 1, "mode": "list", "path": "a", "fields": ["a", "("]}`:                               "fields[1]",
		`{"source": 1, "mode": "keyValues"}`:                                                             "pairs",
		`{"source": 1, "mode": "badges", "pairs": []}`:                                                   "pairs",
		`{"source": 1, "mode": "keyValues", "pairs": [{"label": "", "path": "a"}]}`:                      "pairs[0].label",
		`{"source": 1, "mode": "keyValues", "pairs": [{"label": "A", "path": 3}]}`:                       "pairs[0].path",
		`{"source": 1, "mode": "value", "path": "a", "rules": [{"when": "value >", "tone": "ok"}]}`:      "rules[0].when",
		`{"source": 1, "mode": "value", "path": "a", "rules": [{"when": "true", "tone": "red"}]}`:        "rules[0].tone",
		`{"source": 1, "mode": "value", "path": "a", "rules": [{"when": "true"}]}`:                       "rules[0]",
		`{"source": 1, "mode": "value", "path": "a", "rules": [{"when": "true", "announce": true}]}`:     "rules[0].message",
		`{"source": 1, "mode": "value", "path": "a", "rules": [{"when": "true", "hide": "yes"}]}`:        "rules[0].hide",
		`{"source": 1, "mode": "value", "path": "a", "rules": [{"when": "true", "hide": true, "x": 1}]}`: "rules[0].x",
		`{"source": 1, "mode": "value", "path": "a", "rules": {}}`:                                       "rules",
	}
	for s, field := range cases {
		_, err := ParseWidget(settingsOf(t, s))
		if got := fieldsOf(err); !slices.Contains(got, field) {
			t.Errorf("%s: fields %v, want %s", s, got, field)
		}
	}
	many := `{"source": 1, "mode": "value", "path": "a", "rules": [` + strings.Repeat(`{"when": "true", "tone": "ok"},`, MaxRules) + `{"when": "true", "tone": "ok"}]}`
	if _, err := ParseWidget(settingsOf(t, many)); !slices.Contains(fieldsOf(err), "rules") {
		t.Errorf("%d rules were accepted", MaxRules+1)
	}
}

func TestShowValueWithRules(t *testing.T) {
	w := mustWidget(t, `{"source": 1, "mode": "value", "path": "data.items[1].price", "decimals": 1, "rules": [
		{"when": "value > 2", "tone": "warn", "message": "Bread is dear"},
		{"when": "value > 2.4", "tone": "bad"},
		{"when": "value > 100", "hide": true, "message": "Never"}]}`)
	got := w.Show(Data{Doc: decode(t, sampleAnswer), HasValue: true, Stale: true}, exprNow)
	if got.State != StateOK || !got.Stale || got.Value != 2.5 || got.Tone != ToneBad || got.Message != "Bread is dear" ||
		got.Hidden || got.Decimals == nil || *got.Decimals != 1 {
		t.Fatalf("Show = %+v", got)
	}
}

func TestShowWithoutData(t *testing.T) {
	w := mustWidget(t, `{"source": 1, "mode": "value", "path": "a", "title": "T"}`)
	for d, want := range map[Data]string{
		{Missing: true}:                  StateMissing,
		{}:                               StateLoading,
		{Failing: true}:                  StateFailing,
		{HasValue: true, Failing: true}:  StateOK,
		{HasValue: true, Doc: "nothing"}: StateOK,
	} {
		if got := w.Show(d, exprNow); got.State != want || got.Title != "T" {
			t.Errorf("Show(%+v) = %+v, want state %s", d, got, want)
		}
	}
}

func TestShowList(t *testing.T) {
	w := mustWidget(t, `{"source": 1, "mode": "list", "path": "data.items", "fields": ["name", "price", "tags"], "rules": [
		{"when": "row.price == null", "hide": true},
		{"when": "row.price > 2", "tone": "warn", "message": "Something is dear"},
		{"when": "len(data.items) > 2", "tone": "ok"}]}`)
	got := w.Show(Data{Doc: decode(t, sampleAnswer), HasValue: true}, exprNow)
	want := []ShownRow{
		{Cells: []any{"Milk", 1.25, "dairy"}},
		{Cells: []any{"Bread", 2.5, ""}, Tone: ToneWarn},
	}
	if !reflect.DeepEqual(got.Rows, want) || got.Message != "Something is dear" || got.Tone != ToneOK {
		t.Fatalf("Show = %+v", got)
	}
	matched := w.Matching(decode(t, sampleAnswer), exprNow)
	if !reflect.DeepEqual(matched, []bool{true, true, true}) {
		t.Errorf("Matching = %v", matched)
	}
}

func TestShowListIsBounded(t *testing.T) {
	items := make([]any, maxRows+10)
	for i := range items {
		items[i] = map[string]any{"name": strings.Repeat("x", 500)}
	}
	w := mustWidget(t, `{"source": 1, "mode": "list", "path": "items", "fields": ["name"]}`)
	got := w.Show(Data{Doc: map[string]any{"items": items}, HasValue: true}, exprNow)
	if len(got.Rows) != maxRows || len([]rune(got.Rows[0].Cells[0].(string))) != maxCellRunes {
		t.Fatalf("rows %d, first cell %d characters", len(got.Rows), len([]rune(got.Rows[0].Cells[0].(string))))
	}
}

func TestShowKeyValues(t *testing.T) {
	w := mustWidget(t, `{"source": 1, "mode": "keyValues", "pairs": [
		{"label": "Names", "path": "data.items[*].name"}, {"label": "Odd", "path": "[\"weird key\"]"},
		{"label": "None", "path": "missing"}], "rules": [{"when": "ok", "hide": true}]}`)
	got := w.Show(Data{Doc: decode(t, sampleAnswer), HasValue: true}, exprNow)
	want := []ShownPair{{"Names", "Milk, Bread, Eggs"}, {"Odd", `{"a.b":7}`}, {"None", nil}}
	if !reflect.DeepEqual(got.Pairs, want) || !got.Hidden {
		t.Fatalf("Show = %+v", got)
	}
}

func TestShowBadges(t *testing.T) {
	w := mustWidget(t, `{"source": 1, "mode": "badges", "pairs": [
		{"label": "ST", "path": "home.stijn"}, {"label": "BR", "path": "home.britt"},
		{"label": "RE", "path": "home.rens"}, {"label": "SA", "path": "home.sandra"},
		{"label": "AL", "path": "home.alfredo"}, {"label": "X", "path": "missing"}]}`)
	doc := map[string]any{"home": map[string]any{
		"stijn": true, "britt": false, "rens": " Home ", "sandra": 0.0, "alfredo": 2.0}}
	got := w.Show(Data{Doc: doc, HasValue: true}, exprNow)
	if want := []string{"ST", "RE", "AL"}; !reflect.DeepEqual(got.Badges, want) || got.Pairs != nil {
		t.Fatalf("Show = %+v, want badges %v", got, want)
	}
	for _, v := range []any{"off", "not_home", "", nil, map[string]any{}, []any{true}} {
		if isOn(v) {
			t.Errorf("isOn(%#v) = true", v)
		}
	}
}

func TestCellIsPlainOneLineText(t *testing.T) {
	if got := cell("a\nb\x1b[31m"); got != "a b [31m" {
		t.Errorf("cell = %q", got)
	}
	if got := cell([]any{1.0, true, nil, "x"}); got != "1, true, null, x" {
		t.Errorf("cell of a list = %q", got)
	}
}

func TestAnnouncerFiresOnEdgesOnly(t *testing.T) {
	w := mustWidget(t, `{"source": 1, "mode": "value", "path": "level", "title": "Water", "rules": [
		{"when": "value > 10", "message": "Too high", "announce": true}]}`)
	a := &Announcer{Gap: AnnounceGap}
	at := func(level float64, minutes int) []Announcement {
		doc := map[string]any{"level": level}
		return a.Check([]Target{{Key: "d/w", Widget: w, Doc: doc}}, exprNow.Add(time.Duration(minutes)*time.Minute))
	}
	if got := at(20, 0); got != nil {
		t.Fatalf("the first look announced %v; it only records the state", got)
	}
	if got := at(5, 1); got != nil {
		t.Fatalf("falling announced %v", got)
	}
	if got := at(20, 2); len(got) != 1 || got[0].Message != "Too high" || got[0].Title != "Water" {
		t.Fatalf("rising = %v, want one announcement", got)
	}
	if got := at(25, 3); got != nil {
		t.Fatalf("staying high announced again: %v", got)
	}
	at(5, 4)
	if got := at(20, 5); got != nil {
		t.Fatalf("rising again within the gap announced: %v", got)
	}
	at(5, 13)
	if got := at(20, 14); len(got) != 1 {
		t.Fatalf("rising after the gap = %v, want one announcement", got)
	}
	// A widget that is gone is forgotten.
	a.Check(nil, exprNow.Add(15*time.Minute))
	if len(a.rules) != 0 {
		t.Fatalf("rules kept for widgets that are gone: %v", a.rules)
	}
}
