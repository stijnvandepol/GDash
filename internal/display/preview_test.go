package display

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stijnvandepol/nookboard/internal/datahub"
)

func TestPreviewSnapshot(t *testing.T) {
	entries := map[string]datahub.Entry{
		"weather.current": {Value: json.RawMessage(`{"temperature":12}`)},
		"news.default":    {Value: json.RawMessage(`[]`)},
	}
	dashboard := Dashboard{ID: "hall", Name: "Hall", Theme: "glass", Widgets: []Widget{
		{ID: "weather", Type: "weather", X: 0, Y: 0, W: 8, H: 8, Source: "weather.current"},
	}}

	s, err := PreviewSnapshot(now, dashboard, entries, "nl", 150)
	if err != nil {
		t.Fatal(err)
	}
	if s.Type != "snapshot" || !s.Active || s.Language != "nl" || s.TextScale != 150 || s.Dashboard.ID != "hall" || !s.ServerTime.Equal(now) {
		t.Fatalf("snapshot = %+v", s)
	}
	if _, ok := s.Data["weather.current"]; !ok || len(s.Data) != 1 {
		t.Fatalf("data = %v, want only the weather the dashboard uses", s.Data)
	}
	if len(s.Pages) != 0 || s.PageSeconds != 0 {
		t.Fatalf("a preview shows one dashboard, got pages %v", s.Pages)
	}
}

func TestPreviewVersionFollowsContent(t *testing.T) {
	dashboard := Dashboard{ID: "hall", Name: "Hall", Theme: "glass", Widgets: []Widget{
		{ID: "news", Type: "news", X: 0, Y: 0, W: 8, H: 8, Source: "news.default"},
	}}
	entries := func(value string) map[string]datahub.Entry {
		return map[string]datahub.Entry{"news.default": {Value: json.RawMessage(value)}}
	}
	version := func(at time.Time, d Dashboard, data map[string]datahub.Entry, scale int) int64 {
		s, err := PreviewSnapshot(at, d, data, "en", scale)
		if err != nil {
			t.Fatal(err)
		}
		return s.Version
	}

	base := version(now, dashboard, entries(`[1]`), 100)
	if base <= 0 {
		t.Fatalf("version = %d, want a positive number a browser can hold exactly", base)
	}
	if got := version(now.Add(time.Minute), dashboard, entries(`[1]`), 100); got != base {
		t.Errorf("a later call with the same content has version %d, want %d", got, base)
	}
	otherTheme := dashboard
	otherTheme.Theme = "night"
	for name, got := range map[string]int64{
		"other data":      version(now, dashboard, entries(`[2]`), 100),
		"other theme":     version(now, otherTheme, entries(`[1]`), 100),
		"other text size": version(now, dashboard, entries(`[1]`), 125),
	} {
		if got == base {
			t.Errorf("%s keeps version %d", name, base)
		}
	}
}
