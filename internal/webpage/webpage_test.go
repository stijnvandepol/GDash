package webpage

import (
	"strings"
	"testing"

	"github.com/stijnvandepol/gdash/internal/validation"
)

const own = "http://192.168.1.2:8420"

func fieldsOf(t *testing.T, err error) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, f := range validation.Fields(err) {
		out[f.Field] = f.Message
	}
	return out
}

func TestCheckAcceptsAPageAndFillsTheDefaults(t *testing.T) {
	page, err := Check(Input{Name: " Grafana ", URL: " http://192.168.1.10:3000/d/abc?orgId=1&kiosk#top "}, own)
	if err != nil {
		t.Fatal(err)
	}
	want := Page{Name: "Grafana", URL: "http://192.168.1.10:3000/d/abc?orgId=1&kiosk#top", Zoom: DefaultZoom, AllowScripts: true}
	if page != want {
		t.Errorf("page = %+v, want %+v", page, want)
	}
	off := false
	page, err = Check(Input{Name: "Radar", URL: "https://gadgets.buienradar.nl/", RefreshMinutes: 1440, Zoom: 50, AllowScripts: &off}, own)
	if err != nil {
		t.Fatal(err)
	}
	if page.AllowScripts || page.Zoom != 50 || page.RefreshMinutes != 1440 {
		t.Errorf("page = %+v", page)
	}
}

func TestCheckRefusesAddressesThatAFrameCannotOrMustNotShow(t *testing.T) {
	for _, address := range []string{
		"",
		"javascript:alert(1)",
		"data:text/html,<script>alert(1)</script>",
		"file:///etc/passwd",
		"ftp://nas.local/",
		"//grafana.local/",
		"grafana.local:3000",
		"http://user:secret@grafana.local/",
		"http://[::1]:3000/",
		"http://grafa_na.local/",
		"http://grafana.local:99999/",
		"http://grafana.local/a b",
		"http://grafana.local/\x00",
		"https://" + strings.Repeat("a", 2050) + ".local/",
		// GDash itself, in any spelling of its origin.
		"http://192.168.1.2:8420/",
		"HTTP://192.168.1.2:8420/d/kitchen",
	} {
		_, err := Check(Input{Name: "Page", URL: address}, own)
		if fieldsOf(t, err)["url"] == "" {
			t.Errorf("%q: accepted, want a url error", address)
		}
	}
}

func TestCheckAllowsGDashsHostOnAnotherPortOrScheme(t *testing.T) {
	for _, address := range []string{"http://192.168.1.2:3000/", "https://192.168.1.2:8420/", "http://192.168.1.2/"} {
		if _, err := Check(Input{Name: "Page", URL: address}, own); err != nil {
			t.Errorf("%q: %v", address, err)
		}
	}
}

func TestCheckRefusesTheDefaultPortSpellingOfTheOwnOrigin(t *testing.T) {
	if _, err := Check(Input{Name: "Page", URL: "http://gdash.home:80/x"}, "http://gdash.home"); fieldsOf(t, err)["url"] == "" {
		t.Error("accepted GDash's own origin with its default port")
	}
}

func TestCheckRefusesOtherFields(t *testing.T) {
	_, err := Check(Input{Name: strings.Repeat("n", 61), URL: "http://a.local/", RefreshMinutes: 1441, Zoom: 49}, own)
	got := fieldsOf(t, err)
	for _, field := range []string{"name", "refreshMinutes", "zoom"} {
		if got[field] == "" {
			t.Errorf("no error for %s: %v", field, got)
		}
	}
	_, err = Check(Input{Name: "a\tb", URL: "http://a.local/", RefreshMinutes: -1, Zoom: 201}, own)
	got = fieldsOf(t, err)
	for _, field := range []string{"name", "refreshMinutes", "zoom"} {
		if got[field] == "" {
			t.Errorf("no error for %s: %v", field, got)
		}
	}
}

func TestOrigin(t *testing.T) {
	for address, want := range map[string]string{
		"http://Grafana.Local:3000/d/x?kiosk": "http://grafana.local:3000",
		"https://example.com:443/":            "https://example.com",
		"http://example.com:80":               "http://example.com",
		"https://example.com:8443/a":          "https://example.com:8443",
		"http://10.0.0.5:0080/":               "http://10.0.0.5",
	} {
		got, ok := Origin(address)
		if !ok || got != want {
			t.Errorf("Origin(%q) = %q, %v; want %q", address, got, ok, want)
		}
	}
	for _, address := range []string{"javascript:x", "http://[::1]/", "http:///x", "http://a.local:70000/"} {
		if got, ok := Origin(address); ok {
			t.Errorf("Origin(%q) = %q, want none", address, got)
		}
	}
}

func TestIsOwnOrigin(t *testing.T) {
	if !IsOwnOrigin("http://gdash.local:8420", "http://GDash.local:8420") {
		t.Error("the same origin in other case is not own")
	}
	if IsOwnOrigin("http://gdash.local:8420", "") || IsOwnOrigin("http://gdash.local:8421", "http://gdash.local:8420") {
		t.Error("another origin counts as own")
	}
}
