package display

import (
	"context"
	"strings"
	"testing"

	"github.com/stijnvandepol/gdash/internal/datahub"
)

// newsAt returns a News widget below the default layout.
func newsAt(id string, settings map[string]any) Widget {
	return Widget{ID: id, Type: "news", X: 0, Y: 5, W: 6, H: 4, Settings: settings}
}

func TestNewsFeedsSettingIsValidated(t *testing.T) {
	tooMany := make([]any, MaxNewsFeedsPerWidget+1)
	for i := range tooMany {
		tooMany[i] = float64(i + 1)
	}
	const problem = "widgets[2]: feeds must be at most 10 IDs of news feeds under Integrations, each once, or empty for every feed"
	tests := []struct {
		name     string
		settings map[string]any
		want     string
	}{
		{"every feed, as dashboards from before the setting", map[string]any{"maxItems": float64(5)}, ""},
		{"every feed, said with an empty list", map[string]any{"feeds": []any{}}, ""},
		{"two feeds", map[string]any{"feeds": []any{float64(1), float64(4)}}, ""},
		{"a feed twice", map[string]any{"feeds": []any{float64(1), float64(1)}}, problem},
		{"a name instead of an ID", map[string]any{"feeds": []any{"NOS"}}, problem},
		{"a fraction", map[string]any{"feeds": []any{1.5}}, problem},
		{"zero", map[string]any{"feeds": []any{float64(0)}}, problem},
		{"one ID, not a list", map[string]any{"feeds": float64(1)}, problem},
		{"too many", map[string]any{"feeds": tooMany}, problem},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := valid()
			d.Widgets = append(d.Widgets, newsAt("headlines", tt.settings))
			if got := fields(ValidateDashboard(d)); got != tt.want {
				t.Fatalf("errors = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewsFeedIDsAreCollectedOnce(t *testing.T) {
	a := Dashboard{Widgets: []Widget{
		newsAt("one", map[string]any{"feeds": []any{float64(4), float64(2)}}),
		{Type: "calendar", Settings: map[string]any{"feeds": []any{float64(9)}}},
	}}
	b := Dashboard{Widgets: []Widget{newsAt("two", map[string]any{"feeds": []any{float64(2)}})}}
	ids, everyFeed := NewsFeedIDs([]Dashboard{a, b})
	if len(ids) != 2 || ids[0] != 2 || ids[1] != 4 || everyFeed {
		t.Fatalf("NewsFeedIDs = %v, %v; want 2 and 4 once and not every feed", ids, everyFeed)
	}
	if _, everyFeed := NewsFeedIDs([]Dashboard{a, {Widgets: []Widget{newsAt("all", nil)}}}); !everyFeed {
		t.Fatal("a widget without feeds does not show every feed")
	}
}

func TestEachScreenReceivesOnlyTheFeedsItsWidgetsShow(t *testing.T) {
	kitchen := Normalize(Dashboard{ID: "kitchen", Widgets: []Widget{newsAt("news", map[string]any{"feeds": []any{float64(1)}})}})
	study := Normalize(Dashboard{ID: "study", Widgets: []Widget{newsAt("news", map[string]any{"feeds": []any{float64(2)}})}})
	hall := Normalize(Dashboard{ID: "hall", Widgets: []Widget{newsAt("news", map[string]any{"maxItems": float64(3)})}})
	all := `{"1":{"name":"NOS Nieuws","items":[{"title":"Kitchen news"}]},"2":{"name":"Tweakers","items":[{"title":"Study news"}]}}`
	data := map[string]datahub.Entry{NewsSource: {Value: []byte(all)}}
	b := NewBroadcaster()
	views := map[string]View{"kitchen": {Dashboard: kitchen}, "study": {Dashboard: study}, "hall": {Dashboard: hall}}
	if err := b.Publish(now, views, data, "en"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	kitchenData := string(decode(t, <-b.Subscribe(ctx, "kitchen")).Data[NewsSource].Value)
	if !strings.Contains(kitchenData, "Kitchen news") || strings.Contains(kitchenData, "Study news") {
		t.Fatalf("kitchen received %s, want only feed 1", kitchenData)
	}
	studyData := string(decode(t, <-b.Subscribe(ctx, "study")).Data[NewsSource].Value)
	if strings.Contains(studyData, "Kitchen news") || !strings.Contains(studyData, "Study news") {
		t.Fatalf("study received %s, want only feed 2", studyData)
	}
	if got := string(decode(t, <-b.Subscribe(ctx, "hall")).Data[NewsSource].Value); got != all {
		t.Fatalf("hall, which shows every feed, received %s", got)
	}
}
