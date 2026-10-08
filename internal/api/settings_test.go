package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stijnvandepol/daycast/internal/datahub"
	"github.com/stijnvandepol/daycast/internal/weather"
)

func TestSettingsNeedLogin(t *testing.T) {
	env := newTestEnv(t, nil)
	for _, path := range []string{"/api/settings", "/api/places?name=Oss"} {
		if status, _ := env.do(t, "GET", path, ""); status != http.StatusUnauthorized {
			t.Errorf("GET %s without login = %d, want 401", path, status)
		}
	}
	if status, _ := env.do(t, "POST", "/api/calendars", `{"name":"x","url":"https://example.org/a.ics"}`); status != http.StatusUnauthorized {
		t.Errorf("POST /api/calendars without login = %d, want 401", status)
	}
}

func TestLocationSetting(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	if _, body := env.do(t, "GET", "/api/settings", ""); !strings.Contains(body, `"location":null`) {
		t.Fatalf("fresh settings = %s", body)
	}
	status, body := env.do(t, "PUT", "/api/settings/location", `{"name":"Oss","latitude":91,"longitude":5.5}`)
	if status != http.StatusUnprocessableEntity || !strings.Contains(body, `"field":"latitude"`) {
		t.Fatalf("invalid location = %d %s", status, body)
	}
	if status, body = env.do(t, "PUT", "/api/settings/location", `{"name":"Oss","latitude":51.765,"longitude":5.518}`); status != http.StatusOK {
		t.Fatalf("set location = %d %s", status, body)
	}
	if _, body = env.do(t, "GET", "/api/settings", ""); !strings.Contains(body, `"location":{"name":"Oss","latitude":51.765,"longitude":5.518}`) {
		t.Fatalf("settings = %s", body)
	}
	if status, _ = env.do(t, "DELETE", "/api/settings/location", ""); status != http.StatusNoContent {
		t.Fatalf("clear = %d", status)
	}
	if n := env.settingsChanges.Load(); n != 2 {
		t.Fatalf("SettingsChanged called %d times, want 2 (set, clear)", n)
	}
}

func TestCalendarAddressNeverLeavesTheServer(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	const address = "https://calendar.example.org/private-0123456789/basic.ics"
	status, body := env.do(t, "POST", "/api/calendars", `{"name":"Family","url":"`+address+`"}`)
	if status != http.StatusCreated || strings.Contains(body, "private-0123456789") || !strings.Contains(body, `"host":"calendar.example.org"`) {
		t.Fatalf("add calendar = %d %s", status, body)
	}
	_, body = env.do(t, "GET", "/api/settings", "")
	if strings.Contains(body, "private-0123456789") || !strings.Contains(body, `"name":"Family"`) {
		t.Fatalf("settings show the secret address or miss the calendar: %s", body)
	}
	if status, body = env.do(t, "DELETE", "/api/calendars/1", ""); status != http.StatusNoContent {
		t.Fatalf("delete = %d %s", status, body)
	}
	if status, _ = env.do(t, "DELETE", "/api/calendars/1", ""); status != http.StatusNotFound {
		t.Fatalf("delete again = %d, want 404", status)
	}
	if status, _ = env.do(t, "DELETE", "/api/calendars/abc", ""); status != http.StatusNotFound {
		t.Fatalf("delete with a bad id = %d, want 404", status)
	}
}

func TestNewsFeeds(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	status, body := env.do(t, "POST", "/api/news-feeds", `{"name":"NOS","url":"file:///etc/passwd"}`)
	if status != http.StatusUnprocessableEntity || !strings.Contains(body, `"field":"url"`) {
		t.Fatalf("file URL = %d %s", status, body)
	}
	for i := range 5 {
		feed := fmt.Sprintf(`{"name":"Feed","url":"https://example.org/%d"}`, i)
		if status, body = env.do(t, "POST", "/api/news-feeds", feed); status != http.StatusCreated {
			t.Fatalf("add feed %d = %d %s", i, status, body)
		}
	}
	if status, _ = env.do(t, "POST", "/api/news-feeds", `{"name":"Sixth","url":"https://example.org/z"}`); status != http.StatusConflict {
		t.Fatalf("sixth feed = %d, want 409", status)
	}
	if _, body = env.do(t, "GET", "/api/settings", ""); !strings.Contains(body, `"url":"https://example.org/0"`) {
		t.Fatalf("settings = %s", body)
	}
}

func TestSettingsReportSourceStatus(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	fetched := time.Date(2026, 10, 8, 16, 55, 0, 0, time.UTC)
	env.sources.set(nil, nil, map[string]datahub.Entry{
		"weather.current": {FetchedAt: fetched, Failing: true},
		"news.default":    {},
	})
	_, body := env.do(t, "GET", "/api/settings", "")
	if !strings.Contains(body, `"weather.current":{"updatedAt":"2026-10-08T16:55:00Z","failing":true}`) ||
		!strings.Contains(body, `"news.default":{"failing":false}`) {
		t.Fatalf("status = %s", body)
	}
}

func TestPlaces(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	env.sources.set([]weather.Place{{Name: "Oss", Region: "North Brabant, Netherlands", Latitude: 51.765, Longitude: 5.518}}, nil, nil)
	status, body := env.do(t, "GET", "/api/places?name=Oss", "")
	if status != http.StatusOK || !strings.Contains(body, `"region":"North Brabant, Netherlands"`) {
		t.Fatalf("places = %d %s", status, body)
	}
	if status, _ = env.do(t, "GET", "/api/places?name=O", ""); status != http.StatusUnprocessableEntity {
		t.Fatalf("one letter = %d, want 422", status)
	}
	env.sources.set(nil, errors.New("dial tcp: timeout"), nil)
	if status, body = env.do(t, "GET", "/api/places?name=Oss", ""); status != http.StatusBadGateway || strings.Contains(body, "dial tcp") {
		t.Fatalf("upstream down = %d %s", status, body)
	}
}

func TestSystemShowsVersionToAdminsOnly(t *testing.T) {
	env := newTestEnv(t, nil)
	if status, _ := env.do(t, "GET", "/api/system", ""); status != http.StatusUnauthorized {
		t.Fatalf("without login = %d, want 401", status)
	}
	env.setUp(t)
	if status, body := env.do(t, "GET", "/api/system", ""); status != http.StatusOK || body != `{"version":"v1.2.3"}`+"\n" {
		t.Fatalf("system = %d %q", status, body)
	}
}

func TestChangePassword(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	status, body := env.do(t, "PUT", "/api/me/password", `{"currentPassword":"wrong","newPassword":"a new long password"}`)
	if status != http.StatusUnprocessableEntity || !strings.Contains(body, `"field":"currentPassword"`) {
		t.Fatalf("wrong current password = %d %s; want 422 on the field, not a 401 that would log the admin out", status, body)
	}
	if status, body = env.do(t, "PUT", "/api/me/password", `{"currentPassword":"correct horse battery","newPassword":"short"}`); status != http.StatusUnprocessableEntity || !strings.Contains(body, `"field":"newPassword"`) {
		t.Fatalf("short new password = %d %s", status, body)
	}
	if status, body = env.do(t, "PUT", "/api/me/password", `{"currentPassword":"correct horse battery","newPassword":"a new long password"}`); status != http.StatusNoContent {
		t.Fatalf("change = %d %s", status, body)
	}
	if status, _ = env.do(t, "GET", "/api/me", ""); status != http.StatusOK {
		t.Fatalf("own session after the change = %d, want it kept", status)
	}
}
