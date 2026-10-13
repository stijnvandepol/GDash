package display

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/stijnvandepol/gdash/internal/datahub"
	"github.com/stijnvandepol/gdash/internal/webpage"
)

func webPageWidgetOn(id string, settings map[string]any) Widget {
	return Widget{ID: id, Type: WebPageWidget, X: 0, Y: 5, W: 6, H: 4, Settings: settings}
}

var testPages = []webpage.Page{
	{ID: 1, Name: "Grafana", URL: "http://192.168.1.10:3000/d/abc?kiosk", Zoom: 100, AllowScripts: true},
	{ID: 2, Name: "Radar", URL: "https://gadgets.buienradar.nl/gadget/zoommap", RefreshMinutes: 10, Zoom: 80},
	{ID: 3, Name: "Grafana 2", URL: "http://192.168.1.10:3000/d/def", Zoom: 100, AllowScripts: true},
	{ID: 4, Name: "GDash", URL: "http://192.168.1.2:8420/d/hall", Zoom: 100, AllowScripts: true},
}

func TestValidateWebPageWidget(t *testing.T) {
	tests := []struct {
		name   string
		widget Widget
		want   string
	}{
		{"page", webPageWidgetOn("web", map[string]any{"page": float64(2)}), ""},
		{"page and zoom", webPageWidgetOn("web", map[string]any{"page": float64(2), "zoom": float64(150)}), ""},
		{"no page", webPageWidgetOn("web", nil), "widgets[2]: choose a web page added under Integrations"},
		{"page as text", webPageWidgetOn("web", map[string]any{"page": "http://evil.example/"}), "choose a web page added under Integrations"},
		{"page zero", webPageWidgetOn("web", map[string]any{"page": float64(0)}), "choose a web page added under Integrations"},
		{"zoom too small", webPageWidgetOn("web", map[string]any{"page": float64(1), "zoom": float64(49)}), "zoom must be a whole number from 50 to 200"},
		// The editor picks a page; it can never give an address of its own.
		{"an address", webPageWidgetOn("web", map[string]any{"page": float64(1), "url": "http://evil.example/"}), `unknown setting "url"`},
		{"too small", Widget{ID: "web", Type: WebPageWidget, X: 0, Y: 5, W: 3, H: 3, Settings: map[string]any{"page": float64(1)}}, "at least 4x3 cells"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := valid()
			d.Widgets = append(d.Widgets[:2:2], tt.widget)
			got := fields(ValidateDashboard(d))
			if tt.want == "" && got != "" {
				t.Fatalf("errors = %s, want none", got)
			}
			if tt.want != "" && !strings.Contains(got, tt.want) {
				t.Fatalf("errors = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFrameOriginsAreThoseOfTheShownPagesOnly(t *testing.T) {
	hall := Dashboard{Widgets: []Widget{webPageWidgetOn("a", map[string]any{"page": float64(3)}), {Type: "clock"}}}
	night := Dashboard{Widgets: []Widget{
		webPageWidgetOn("b", map[string]any{"page": float64(1)}),
		webPageWidgetOn("c", map[string]any{"page": float64(4)}),
		// A page that was removed under Integrations adds nothing.
		webPageWidgetOn("d", map[string]any{"page": float64(99)}),
		// Only Web page widgets count.
		{Type: "waste", Settings: map[string]any{"calendar": float64(2)}},
	}}
	got := FrameOrigins([]Dashboard{hall, night}, testPages, "http://192.168.1.2:8420")
	if want := []string{"http://192.168.1.10:3000"}; !slices.Equal(got, want) {
		t.Errorf("FrameOrigins = %v, want %v: one origin for both Grafana pages, none for GDash itself", got, want)
	}
	if got := FrameOrigins([]Dashboard{{Widgets: []Widget{{Type: "clock"}}}}, testPages, ""); len(got) != 0 {
		t.Errorf("FrameOrigins without a Web page widget = %v, want none", got)
	}
}

func TestPageOriginsLeaveOutTheOwnOrigin(t *testing.T) {
	got := PageOrigins(testPages, "http://192.168.1.2:8420")
	want := []string{"http://192.168.1.10:3000", "https://gadgets.buienradar.nl"}
	if !slices.Equal(got, want) {
		t.Errorf("PageOrigins = %v, want %v", got, want)
	}
	// Reached on another address, GDash's page there is just another page.
	if got := PageOrigins(testPages, "http://gdash.local:8420"); len(got) != 3 {
		t.Errorf("PageOrigins = %v, want 3", got)
	}
}

func TestViewDashboardsListsEveryDashboardItCanShow(t *testing.T) {
	v := View{
		Dashboard: Dashboard{ID: "own"},
		Pages:     []Dashboard{{ID: "page"}},
		Schedule:  []ScheduledDashboard{{Dashboard: Dashboard{ID: "night"}}},
	}
	var ids []string
	for _, d := range v.Dashboards() {
		ids = append(ids, d.ID)
	}
	if !slices.Equal(ids, []string{"own", "page", "night"}) {
		t.Errorf("Dashboards = %v", ids)
	}
}

func TestEachDisplayGetsOnlyItsOwnWebPages(t *testing.T) {
	entry, err := WebPageEntry(testPages, now)
	if err != nil {
		t.Fatal(err)
	}
	hall := Normalize(Dashboard{ID: "hall", Widgets: []Widget{webPageWidgetOn("web", map[string]any{"page": float64(2)})}})
	kitchen := Normalize(Dashboard{ID: "kitchen", Widgets: []Widget{{ID: "clock", Type: "clock", W: 4, H: 3}}})
	b := NewBroadcaster()
	views := map[string]View{"hall": {Dashboard: hall}, "kitchen": {Dashboard: kitchen}}
	if err := b.Publish(now, views, map[string]datahub.Entry{WebPageSource: entry}, "en"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var pages map[string]shownPage
	hallData := decode(t, <-b.Subscribe(ctx, "hall")).Data
	if err := json.Unmarshal(hallData[WebPageSource].Value, &pages); err != nil {
		t.Fatal(err)
	}
	want := map[string]shownPage{"2": {Name: "Radar", URL: "https://gadgets.buienradar.nl/gadget/zoommap", RefreshMinutes: 10, Zoom: 80}}
	if len(pages) != 1 || pages["2"] != want["2"] {
		t.Errorf("hall received %+v, want only page 2", pages)
	}
	if _, ok := decode(t, <-b.Subscribe(ctx, "kitchen")).Data[WebPageSource]; ok {
		t.Error("a display without a Web page widget received the web pages")
	}
}
