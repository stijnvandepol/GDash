package p2000

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestParseReadsTheReadableAlerts(t *testing.T) {
	f, err := os.Open("testdata/region-brandweer.rss")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	alerts, err := Parse(f, "brandweer")
	if err != nil {
		t.Fatal(err)
	}
	want := []Alert{
		{ID: "aaa1", At: time.Date(2026, 10, 16, 10, 15, 5, 0, time.UTC), Discipline: "brandweer", Place: "nuenen", Text: "Woningbrand op Dorpsstraat in Nuenen"},
		{ID: "aaa2", At: time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC), Discipline: "brandweer", Place: "eindhoven", Text: "Gaslucht op Kerkstraat & plein in Eindhoven"},
		// No description: the pager text; a link to the place's page.
		{ID: "aaa4", At: time.Date(2026, 10, 16, 8, 0, 0, 0, time.UTC), Discipline: "brandweer", Place: "helmond", Text: "p 1 alleen pagertekst helmond"},
	}
	if !slices.Equal(alerts, want) {
		t.Fatalf("alerts =\n%+v\nwant\n%+v", alerts, want)
	}
}

func TestParseRefusesWhatIsNoRSS(t *testing.T) {
	if _, err := Parse(strings.NewReader(`<html><body>Error 404</body></html>`), "brandweer"); err == nil {
		t.Fatal("Parse accepted an HTML page")
	}
}

func TestRegionAndPlace(t *testing.T) {
	for _, tc := range []struct{ link, region, place string }{
		{"https://alarmeringen.nl/limburg/limburg-zuid/maastricht/55969332/p2000-x.html", "limburg-zuid", "maastricht"},
		{"https://alarmeringen.nl/friesland/midlum/55969470/p2000-x.html", "friesland", "midlum"},
		{"https://alarmeringen.nl/overijssel/twente/haaksbergen/?utm_source=rss", "twente", "haaksbergen"},
		{"https://alarmeringen.nl/", "", ""},
		{"https://alarmeringen.nl/x/Bad Place/1/", "", ""},
	} {
		region, place := regionAndPlace(tc.link)
		if region != tc.region || place != tc.place {
			t.Errorf("regionAndPlace(%q) = %q, %q, want %q, %q", tc.link, region, place, tc.region, tc.place)
		}
	}
}

func TestSlugOf(t *testing.T) {
	for name, want := range map[string]string{
		"Hengelo (Ov)":       "hengelo-ov",
		"'s-Hertogenbosch":   "s-hertogenbosch",
		" Bergen op Zoom ":   "bergen-op-zoom",
		"Sint-Oedenrode":     "sint-oedenrode",
		"Ferwerderadiel/Één": "ferwerderadiel-een",
		"<script>":           "script",
		"--":                 "",
	} {
		if got := SlugOf(name); got != want {
			t.Errorf("SlugOf(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestEveryRegionHasANameAndAValidSlug(t *testing.T) {
	if len(regions) != 25 {
		t.Fatalf("%d regions, want the 25 safety regions", len(regions))
	}
	for _, r := range regions {
		if !ValidSlug(r.Slug) || r.Name == "" || RegionName(r.Slug) != r.Name {
			t.Errorf("region %+v", r)
		}
	}
}

// upstream serves the testdata feeds as alarmeringen.nl would, and counts
// the requests that were answered with a whole feed.
func upstream(t *testing.T, failing *atomic.Bool) (*Client, *atomic.Int32) {
	t.Helper()
	full := &atomic.Int32{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if failing != nil && failing.Load() {
			http.Error(w, "down", http.StatusServiceUnavailable)
			return
		}
		var file string
		switch r.URL.Path {
		case "/feeds/region/brabant-zuidoost/brandweer.rss", "/feeds/region/brabant-zuidoost/politie.rss":
			file = "testdata/region-brandweer.rss"
		case "/feeds/city/den-haag.rss":
			file = "testdata/city-den-haag.rss"
		case "/feeds/city/stilledorp.rss":
			w.Write([]byte(`<rss version="2.0"><channel><title>x</title></channel></rss>`))
			return
		default:
			http.NotFound(w, r)
			return
		}
		const modified = "Fri, 16 Oct 2026 10:15:05 GMT"
		if r.Header.Get("If-Modified-Since") == modified {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("Last-Modified", modified)
		body, err := os.ReadFile(file)
		if err != nil {
			t.Error(err)
		}
		full.Add(1)
		w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return &Client{BaseURL: srv.URL, HTTP: srv.Client(), UserAgent: "test"}, full
}

func TestAlertsByFeedWithConditionalRequests(t *testing.T) {
	client, full := upstream(t, nil)
	feeds := []Feed{{"brabant-zuidoost", "brandweer"}, {"brabant-zuidoost", "politie"}}
	for range 2 {
		got, err := client.Alerts(context.Background(), feeds)
		if err != nil {
			t.Fatal(err)
		}
		if len(got["brabant-zuidoost/brandweer"]) != 3 || got["brabant-zuidoost/politie"][0].Discipline != "politie" {
			t.Fatalf("alerts = %+v", got)
		}
	}
	if n := full.Load(); n != 2 {
		t.Fatalf("whole feeds sent = %d, want 2: the second round is answered 304", n)
	}
}

func TestAnyFailingFeedFailsTheSet(t *testing.T) {
	var failing atomic.Bool
	client, _ := upstream(t, &failing)
	failing.Store(true)
	if _, err := client.Alerts(context.Background(), []Feed{{"brabant-zuidoost", "brandweer"}}); err == nil {
		t.Fatal("Alerts succeeded while the feed failed")
	}
	if _, err := client.Alerts(context.Background(), []Feed{{"nowhere", "brandweer"}}); err == nil {
		t.Fatal("Alerts accepted an unknown region")
	}
}

func TestFindPlace(t *testing.T) {
	client, _ := upstream(t, nil)
	ctx := context.Background()
	// The links say how alerts name the place.
	got, err := client.FindPlace(ctx, " 's-Gravenhage ")
	if !errors.Is(err, ErrUnknownPlace) {
		t.Fatalf("FindPlace('s-Gravenhage) = %+v, %v; the stand-in only knows den-haag", got, err)
	}
	got, err = client.FindPlace(ctx, "Den\tHaag")
	if err != nil || got != (Place{Name: "Den Haag", Slug: "den-haag", Region: "haaglanden"}) {
		t.Fatalf("FindPlace(Den Haag) = %+v, %v", got, err)
	}
	if _, err := client.FindPlace(ctx, "Stilledorp"); !errors.Is(err, ErrPlaceWithoutAlerts) {
		t.Fatalf("FindPlace(Stilledorp) error = %v, want ErrPlaceWithoutAlerts", err)
	}
	if _, err := client.FindPlace(ctx, "!!"); !errors.Is(err, ErrUnknownPlace) {
		t.Fatalf("FindPlace(!!) error = %v, want ErrUnknownPlace", err)
	}
}
