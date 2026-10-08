package main

import (
	"strings"
	"testing"
)

func env(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func TestLoadConfigDefaults(t *testing.T) {
	cfg, err := loadConfig(env(map[string]string{"DAYCAST_LATITUDE": "51.6461", "DAYCAST_LONGITUDE": "5.9449"}))
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if cfg.addr != ":8420" || cfg.dataDir != "data" || cfg.castAppID != "" {
		t.Fatalf("cfg = %+v", cfg)
	}
	if len(cfg.seed.Calendars) != 0 {
		t.Fatalf("calendars = %+v, want none by default", cfg.seed.Calendars)
	}
	if len(cfg.seed.NewsFeeds) != 1 || cfg.seed.NewsFeeds[0].URL != defaultNewsFeed || cfg.seed.NewsFeeds[0].Name != "feeds.nos.nl" {
		t.Fatalf("news feeds = %+v, want the default", cfg.seed.NewsFeeds)
	}
	if loc := cfg.seed.Location; loc == nil || loc.Latitude != 51.65 || loc.Longitude != 5.94 {
		t.Fatalf("location = %+v, want rounded to 2 decimals", loc)
	}
}

func TestLoadConfigWithoutLocation(t *testing.T) {
	cfg, err := loadConfig(env(map[string]string{}))
	if err != nil || cfg.seed.Location != nil {
		t.Fatalf("loadConfig = %+v, %v; want no location and no error", cfg.seed.Location, err)
	}
}

func TestLoadConfigCalendarsAreSeeded(t *testing.T) {
	cfg, err := loadConfig(env(map[string]string{"DAYCAST_CALENDAR_URLS": "webcal://example.org/a.ics, https://example.org/b.ics"}))
	if err != nil || len(cfg.seed.Calendars) != 2 || cfg.seed.Calendars[0].URL != "https://example.org/a.ics" {
		t.Fatalf("calendars = %+v, %v", cfg.seed.Calendars, err)
	}
}

func TestLoadConfigCastAppID(t *testing.T) {
	cfg, err := loadConfig(env(map[string]string{
		"DAYCAST_LATITUDE": "51.65", "DAYCAST_LONGITUDE": "5.95", "DAYCAST_CAST_APP_ID": "ABCD1234",
	}))
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if cfg.castAppID != "ABCD1234" {
		t.Fatalf("cfg = %+v", cfg)
	}
}

func TestWithDefaultPort(t *testing.T) {
	if got := withDefaultPort("192.168.1.40"); got != "192.168.1.40:8009" {
		t.Errorf("withDefaultPort(ip) = %q", got)
	}
	if got := withDefaultPort("192.168.1.40:9000"); got != "192.168.1.40:9000" {
		t.Errorf("withDefaultPort(ip:port) = %q", got)
	}
}

func TestLoadConfigErrors(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantErr string
	}{
		{"half a location", map[string]string{"DAYCAST_LATITUDE": "51"}, "DAYCAST_LONGITUDE"},
		{"latitude not a number", map[string]string{"DAYCAST_LATITUDE": "noord", "DAYCAST_LONGITUDE": "5"}, "not a number"},
		{"latitude out of range", map[string]string{"DAYCAST_LATITUDE": "95", "DAYCAST_LONGITUDE": "5"}, "invalid location"},
		{"news feed not http", map[string]string{"DAYCAST_LATITUDE": "51", "DAYCAST_LONGITUDE": "5", "DAYCAST_NEWS_FEED": "file:///etc/passwd"}, "DAYCAST_NEWS_FEED"},
		{"calendar not a URL", map[string]string{"DAYCAST_LATITUDE": "51", "DAYCAST_LONGITUDE": "5", "DAYCAST_CALENDAR_URLS": "ftp://x/y.ics"}, "DAYCAST_CALENDAR_URLS"},
		{"malformed app id", map[string]string{"DAYCAST_LATITUDE": "51", "DAYCAST_LONGITUDE": "5", "DAYCAST_CAST_APP_ID": "dashcast"}, "8-character"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadConfig(env(tt.env))
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want it to mention %q", err, tt.wantErr)
			}
		})
	}
}
