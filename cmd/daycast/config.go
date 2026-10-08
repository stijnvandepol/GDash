package main

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strconv"

	"github.com/stijnvandepol/daycast/internal/weather"
)

const defaultCastPort = "8009"

// validAppID matches the 8 hex characters of a Cast application ID.
var validAppID = regexp.MustCompile(`^[0-9A-F]{8}$`)

type config struct {
	addr     string
	dataDir  string
	location weather.Location
	// castAppID is the app ID of an own registered receiver. Empty means
	// Daycast uses the public DashCast receiver.
	castAppID string
	newsFeed  string
}

// defaultNewsFeed is the general news feed of the NOS, the Dutch public
// broadcaster. Daycast starts in the Netherlands; any RSS or Atom URL works.
const defaultNewsFeed = "https://feeds.nos.nl/nosnieuwsalgemeen"

// loadConfig reads the configuration from environment variables. The
// prototype has no setup wizard yet, so the location is required here.
func loadConfig(getenv func(string) string) (config, error) {
	cfg := config{
		addr:    valueOr(getenv("DAYCAST_ADDR"), ":8420"),
		dataDir: valueOr(getenv("DAYCAST_DATA_DIR"), "data"),
	}

	latText, lonText := getenv("DAYCAST_LATITUDE"), getenv("DAYCAST_LONGITUDE")
	if latText == "" || lonText == "" {
		return config{}, errors.New("set DAYCAST_LATITUDE and DAYCAST_LONGITUDE to the location for the weather, for example 51.65 and 5.95")
	}
	lat, err := strconv.ParseFloat(latText, 64)
	if err != nil {
		return config{}, fmt.Errorf("DAYCAST_LATITUDE is not a number: %q", latText)
	}
	lon, err := strconv.ParseFloat(lonText, 64)
	if err != nil {
		return config{}, fmt.Errorf("DAYCAST_LONGITUDE is not a number: %q", lonText)
	}
	if cfg.location, err = weather.NewLocation(lat, lon); err != nil {
		return config{}, fmt.Errorf("invalid location: %w", err)
	}

	cfg.newsFeed = valueOr(getenv("DAYCAST_NEWS_FEED"), defaultNewsFeed)
	if u, err := url.Parse(cfg.newsFeed); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return config{}, fmt.Errorf("DAYCAST_NEWS_FEED %q is not an http(s) URL", cfg.newsFeed)
	}

	cfg.castAppID = getenv("DAYCAST_CAST_APP_ID")
	if cfg.castAppID != "" && !validAppID.MatchString(cfg.castAppID) {
		return config{}, fmt.Errorf("DAYCAST_CAST_APP_ID %q is not an 8-character Cast application ID", cfg.castAppID)
	}
	return cfg, nil
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
