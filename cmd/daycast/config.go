package main

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strconv"

	"github.com/stijnvandepol/daycast/internal/calendar"
	"github.com/stijnvandepol/daycast/internal/settings"
	"github.com/stijnvandepol/daycast/internal/weather"
)

const defaultCastPort = "8009"

// validAppID matches the 8 hex characters of a Cast application ID.
var validAppID = regexp.MustCompile(`^[0-9A-F]{8}$`)

type config struct {
	addr    string
	dataDir string
	// castAppID is the app ID of an own registered receiver. Empty means
	// Daycast uses the public DashCast receiver.
	castAppID string
	// seed fills the settings on the first start only; after that the web
	// interface manages location, feeds and calendars.
	seed settings.Seed
}

// defaultNewsFeed is the general news feed of the NOS, the Dutch public
// broadcaster. Daycast starts in the Netherlands; any RSS or Atom URL works,
// and the feed can be removed in Settings.
const defaultNewsFeed = "https://feeds.nos.nl/nosnieuwsalgemeen"

// loadConfig reads the configuration from environment variables. Only the
// address, data directory and Cast app ID are read on every start.
func loadConfig(getenv func(string) string) (config, error) {
	cfg := config{
		addr:    valueOr(getenv("DAYCAST_ADDR"), ":8420"),
		dataDir: valueOr(getenv("DAYCAST_DATA_DIR"), "data"),
	}
	var err error
	if cfg.seed, err = loadSeed(getenv); err != nil {
		return config{}, err
	}

	cfg.castAppID = getenv("DAYCAST_CAST_APP_ID")
	if cfg.castAppID != "" && !validAppID.MatchString(cfg.castAppID) {
		return config{}, fmt.Errorf("DAYCAST_CAST_APP_ID %q is not an 8-character Cast application ID", cfg.castAppID)
	}
	return cfg, nil
}

// loadSeed reads the environment variables that configured the sources
// before the web interface did.
func loadSeed(getenv func(string) string) (settings.Seed, error) {
	var seed settings.Seed
	latText, lonText := getenv("DAYCAST_LATITUDE"), getenv("DAYCAST_LONGITUDE")
	if (latText == "") != (lonText == "") {
		return settings.Seed{}, errors.New("set both DAYCAST_LATITUDE and DAYCAST_LONGITUDE, or neither and choose the location in Settings")
	}
	if latText != "" {
		lat, err := strconv.ParseFloat(latText, 64)
		if err != nil {
			return settings.Seed{}, fmt.Errorf("DAYCAST_LATITUDE is not a number: %q", latText)
		}
		lon, err := strconv.ParseFloat(lonText, 64)
		if err != nil {
			return settings.Seed{}, fmt.Errorf("DAYCAST_LONGITUDE is not a number: %q", lonText)
		}
		loc, err := weather.NewLocation(lat, lon)
		if err != nil {
			return settings.Seed{}, fmt.Errorf("invalid location: %w", err)
		}
		seed.Location = &settings.Location{Name: "Home", Latitude: loc.Latitude, Longitude: loc.Longitude}
	}

	feed := valueOr(getenv("DAYCAST_NEWS_FEED"), defaultNewsFeed)
	u, err := url.Parse(feed)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return settings.Seed{}, fmt.Errorf("DAYCAST_NEWS_FEED %q is not an http(s) URL", feed)
	}
	seed.NewsFeeds = []settings.NewsFeed{{Name: u.Hostname(), URL: feed}}

	calendars, err := calendar.ParseSources(getenv("DAYCAST_CALENDAR_URLS"))
	if err != nil {
		return settings.Seed{}, fmt.Errorf("DAYCAST_CALENDAR_URLS: %w", err)
	}
	for _, c := range calendars {
		seed.Calendars = append(seed.Calendars, settings.SeedCalendar{Name: c.Name, URL: c.URL})
	}
	return seed, nil
}

func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

// withDefaultPort accepts "192.168.1.40" or "192.168.1.40:8009".
func withDefaultPort(addr string) string {
	if _, _, err := net.SplitHostPort(addr); err == nil {
		return addr
	}
	return net.JoinHostPort(addr, defaultCastPort)
}
