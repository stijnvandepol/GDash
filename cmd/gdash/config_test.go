package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stijnvandepol/gdash/internal/holidays"
)

func env(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func TestLoadConfigDefaults(t *testing.T) {
	cfg, err := loadConfig(env(map[string]string{"GDASH_LATITUDE": "51.6461", "GDASH_LONGITUDE": "5.9449"}))
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
	cfg, err := loadConfig(env(map[string]string{"GDASH_CALENDAR_URLS": "webcal://example.org/a.ics, https://example.org/b.ics"}))
	if err != nil || len(cfg.seed.Calendars) != 2 || cfg.seed.Calendars[0].URL != "https://example.org/a.ics" {
		t.Fatalf("calendars = %+v, %v", cfg.seed.Calendars, err)
	}
}

func TestLoadConfigCastAppID(t *testing.T) {
	cfg, err := loadConfig(env(map[string]string{
		"GDASH_LATITUDE": "51.65", "GDASH_LONGITUDE": "5.95", "GDASH_CAST_APP_ID": "ABCD1234",
	}))
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if cfg.castAppID != "ABCD1234" {
		t.Fatalf("cfg = %+v", cfg)
	}
}

func TestLoadConfigUpdateCheck(t *testing.T) {
	for value, off := range map[string]bool{"": false, "on": false, "off": true} {
		cfg, err := loadConfig(env(map[string]string{"GDASH_UPDATE_CHECK": value}))
		if err != nil || cfg.updateCheckOff != off {
			t.Errorf("GDASH_UPDATE_CHECK=%q: updateCheckOff = %v, %v; want %v", value, cfg.updateCheckOff, err, off)
		}
	}
}

func TestLoadConfigSchoolHolidaysURL(t *testing.T) {
	cfg, err := loadConfig(env(map[string]string{}))
	if err != nil || cfg.schoolHolidaysURL != holidays.DefaultSchoolURL {
		t.Fatalf("school holidays URL = %q, %v; want the government's open data", cfg.schoolHolidaysURL, err)
	}
	cfg, err = loadConfig(env(map[string]string{"GDASH_SCHOOL_HOLIDAYS_URL": "http://127.0.0.1:9999/school.json"}))
	if err != nil || cfg.schoolHolidaysURL != "http://127.0.0.1:9999/school.json" {
		t.Fatalf("school holidays URL = %q, %v; want the stand-in", cfg.schoolHolidaysURL, err)
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

func TestLoadConfigTrustedProxies(t *testing.T) {
	cfg, err := loadConfig(env(map[string]string{"GDASH_TRUSTED_PROXIES": " 172.17.0.1, 10.1.2.3/8 ,fd00::/8"}))
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	got := fmt.Sprint(cfg.trustedProxies)
	if got != "[172.17.0.1/32 10.0.0.0/8 fd00::/8]" {
		t.Fatalf("trusted proxies = %s", got)
	}
}

func TestLoadConfigErrors(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantErr string
	}{
		{"half a location", map[string]string{"GDASH_LATITUDE": "51"}, "GDASH_LONGITUDE"},
		{"latitude not a number", map[string]string{"GDASH_LATITUDE": "noord", "GDASH_LONGITUDE": "5"}, "not a number"},
		{"latitude out of range", map[string]string{"GDASH_LATITUDE": "95", "GDASH_LONGITUDE": "5"}, "invalid location"},
		{"news feed not http", map[string]string{"GDASH_LATITUDE": "51", "GDASH_LONGITUDE": "5", "GDASH_NEWS_FEED": "file:///etc/passwd"}, "GDASH_NEWS_FEED"},
		{"calendar not a URL", map[string]string{"GDASH_LATITUDE": "51", "GDASH_LONGITUDE": "5", "GDASH_CALENDAR_URLS": "ftp://x/y.ics"}, "GDASH_CALENDAR_URLS"},
		{"bad trusted proxy", map[string]string{"GDASH_TRUSTED_PROXIES": "10.0.0.1, proxy.local"}, "GDASH_TRUSTED_PROXIES"},
		{"malformed app id", map[string]string{"GDASH_LATITUDE": "51", "GDASH_LONGITUDE": "5", "GDASH_CAST_APP_ID": "dashcast"}, "8-character"},
		{"school holidays not http", map[string]string{"GDASH_SCHOOL_HOLIDAYS_URL": "file:///etc/passwd"}, "GDASH_SCHOOL_HOLIDAYS_URL"},
		{"update check neither on nor off", map[string]string{"GDASH_UPDATE_CHECK": "false"}, "GDASH_UPDATE_CHECK"},
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

func TestCheckDataDirCreatesTheDirectoryAndLeavesNothingBehind(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "data")
	if err := checkDataDir(dir); err != nil {
		t.Fatalf("checkDataDir: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("directory = %v, %v; want it created and empty", entries, err)
	}
}

func TestCheckDataDirExplainsAnUnusableDirectory(t *testing.T) {
	file := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	err := checkDataDir(file)
	if err == nil || !strings.Contains(err.Error(), "is not writable") || !strings.Contains(err.Error(), "docs/install.md") {
		t.Fatalf("checkDataDir = %v, want a message that says what to do", err)
	}
}

func TestCheckDataDirNeedsWriteAccess(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("permission bits do not stop this user")
	}
	dir := filepath.Join(t.TempDir(), "data")
	if err := os.Mkdir(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	if err := checkDataDir(dir); err == nil || !strings.Contains(err.Error(), "is not writable") {
		t.Fatalf("checkDataDir = %v, want a not-writable error", err)
	}
}

func TestCheckDataDirTakesAwayOthersAccess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no Unix file modes")
	}
	dir := filepath.Join(t.TempDir(), "data")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o755); err != nil { // past the umask
		t.Fatal(err)
	}
	if err := checkDataDir(dir); err != nil {
		t.Fatalf("checkDataDir: %v", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode != 0o700 {
		t.Fatalf("data directory mode = %o, want 700", mode)
	}
}
