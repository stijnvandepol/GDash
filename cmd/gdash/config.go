package main

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/stijnvandepol/gdash/internal/calendar"
	"github.com/stijnvandepol/gdash/internal/settings"
	"github.com/stijnvandepol/gdash/internal/weather"
)

const (
	defaultAddr     = ":8420"
	defaultCastPort = "8009"
)

// validAppID matches the 8 hex characters of a Cast application ID.
var validAppID = regexp.MustCompile(`^[0-9A-F]{8}$`)

type config struct {
	addr    string
	dataDir string
	// castAppID is the app ID of an own registered receiver. Empty means
	// GDash uses the public DashCast receiver.
	castAppID string
	// seed fills the settings on the first start only; after that the web
	// interface manages location, feeds and calendars.
	seed settings.Seed
	// trustedProxies are reverse proxies whose forwarding headers count.
	trustedProxies []netip.Prefix
	// updateCheckOff is set by GDASH_UPDATE_CHECK=off: GDash never asks
	// for the latest release, whatever the setting in the web interface.
	updateCheckOff bool
}

// defaultNewsFeed is the general news feed of the NOS, the Dutch public
// broadcaster. GDash starts in the Netherlands; any RSS or Atom URL works,
// and the feed can be removed in Settings.
const defaultNewsFeed = "https://feeds.nos.nl/nosnieuwsalgemeen"

// loadConfig reads the configuration from environment variables. Only the
// address, data directory, trusted proxies, update check and Cast app ID are
// read on every start.
func loadConfig(getenv func(string) string) (config, error) {
	cfg := config{
		addr:    valueOr(getenv("GDASH_ADDR"), defaultAddr),
		dataDir: valueOr(getenv("GDASH_DATA_DIR"), "data"),
	}
	var err error
	if cfg.seed, err = loadSeed(getenv); err != nil {
		return config{}, err
	}

	if cfg.trustedProxies, err = parsePrefixes(getenv("GDASH_TRUSTED_PROXIES")); err != nil {
		return config{}, fmt.Errorf("GDASH_TRUSTED_PROXIES: %w", err)
	}

	switch getenv("GDASH_UPDATE_CHECK") {
	case "", "on":
	case "off":
		cfg.updateCheckOff = true
	default:
		return config{}, fmt.Errorf("GDASH_UPDATE_CHECK %q is neither on nor off", getenv("GDASH_UPDATE_CHECK"))
	}

	cfg.castAppID = getenv("GDASH_CAST_APP_ID")
	if cfg.castAppID != "" && !validAppID.MatchString(cfg.castAppID) {
		return config{}, fmt.Errorf("GDASH_CAST_APP_ID %q is not an 8-character Cast application ID", cfg.castAppID)
	}
	return cfg, nil
}

// checkDataDir makes sure GDash can write in its data directory, creating
// it when it is missing, so a wrong owner on a Docker volume or a mounted
// folder is reported at the start instead of as a database error later.
func checkDataDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return dataDirError(dir, err)
	}
	if err := ownerOnlyDir(dir); err != nil {
		return err
	}
	probe, err := os.CreateTemp(dir, ".write-check-*")
	if err != nil {
		return dataDirError(dir, err)
	}
	probe.Close()
	return os.Remove(probe.Name())
}

// ownerOnlyDir takes group and others' access away from an existing data
// directory, which MkdirAll leaves as it was: it holds the database, the
// secret key and cached calendar data. Windows has no such modes.
func ownerOnlyDir(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		return dataDirError(dir, err)
	}
	if runtime.GOOS == "windows" || info.Mode().Perm()&0o077 == 0 {
		return nil
	}
	if err := os.Chmod(dir, info.Mode().Perm()&0o700); err != nil {
		return fmt.Errorf("data directory %q can be read by other users and cannot be restricted: %w; "+
			"run chmod 700 on it", dir, err)
	}
	return nil
}

func dataDirError(dir string, err error) error {
	owner := ""
	if uid := os.Getuid(); uid >= 0 {
		owner = fmt.Sprintf(" for user id %d", uid)
	}
	return fmt.Errorf("data directory %q is not writable%s: %w; give that user write access to it, "+
		`see "Data directory permissions" in docs/install.md`, dir, owner, err)
}

// parsePrefixes reads a comma-separated list of IP addresses and CIDR
// ranges, such as "172.17.0.1, 10.0.0.0/8". An address is a range of one.
func parsePrefixes(list string) ([]netip.Prefix, error) {
	var prefixes []netip.Prefix
	for item := range strings.SplitSeq(list, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if addr, err := netip.ParseAddr(item); err == nil {
			prefixes = append(prefixes, netip.PrefixFrom(addr.Unmap(), addr.Unmap().BitLen()))
			continue
		}
		prefix, err := netip.ParsePrefix(item)
		if err != nil {
			return nil, fmt.Errorf("%q is not an IP address or CIDR range", item)
		}
		prefixes = append(prefixes, prefix.Masked())
	}
	return prefixes, nil
}

// loadSeed reads the environment variables that configured the sources
// before the web interface did.
func loadSeed(getenv func(string) string) (settings.Seed, error) {
	var seed settings.Seed
	latText, lonText := getenv("GDASH_LATITUDE"), getenv("GDASH_LONGITUDE")
	if (latText == "") != (lonText == "") {
		return settings.Seed{}, errors.New("set both GDASH_LATITUDE and GDASH_LONGITUDE, or neither and choose the location in Settings")
	}
	if latText != "" {
		lat, err := strconv.ParseFloat(latText, 64)
		if err != nil {
			return settings.Seed{}, fmt.Errorf("GDASH_LATITUDE is not a number: %q", latText)
		}
		lon, err := strconv.ParseFloat(lonText, 64)
		if err != nil {
			return settings.Seed{}, fmt.Errorf("GDASH_LONGITUDE is not a number: %q", lonText)
		}
		loc, err := weather.NewLocation(lat, lon)
		if err != nil {
			return settings.Seed{}, fmt.Errorf("invalid location: %w", err)
		}
		seed.Location = &settings.Location{Name: "Home", Latitude: loc.Latitude, Longitude: loc.Longitude}
	}

	feed := valueOr(getenv("GDASH_NEWS_FEED"), defaultNewsFeed)
	u, err := url.Parse(feed)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return settings.Seed{}, fmt.Errorf("GDASH_NEWS_FEED %q is not an http(s) URL", feed)
	}
	seed.NewsFeeds = []settings.NewsFeed{{Name: feedName(u.Hostname()), URL: feed}}

	calendars, err := calendar.ParseSources(getenv("GDASH_CALENDAR_URLS"))
	if err != nil {
		return settings.Seed{}, fmt.Errorf("GDASH_CALENDAR_URLS: %w", err)
	}
	for _, c := range calendars {
		seed.Calendars = append(seed.Calendars, settings.SeedCalendar{Name: c.Name, URL: c.URL})
	}
	return seed, nil
}

// feedName names a feed from the environment after its host, cut to the
// longest name settings accept.
func feedName(host string) string {
	const maxRunes = 60
	if r := []rune(host); len(r) > maxRunes {
		return string(r[:maxRunes])
	}
	return host
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
