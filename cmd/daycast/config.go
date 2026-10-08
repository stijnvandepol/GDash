package main

import (
	"errors"
	"fmt"
	"net"
	"strconv"

	"github.com/stijnvandepol/daycast/internal/weather"
)

const defaultCastPort = "8009"

type config struct {
	addr     string
	dataDir  string
	location weather.Location
	// castDevice and castAppID are optional. When both are set, Daycast
	// keeps its receiver on that device.
	castDevice string
	castAppID  string
}

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

	device, appID := getenv("DAYCAST_CAST_DEVICE"), getenv("DAYCAST_CAST_APP_ID")
	if (device == "") != (appID == "") {
		return config{}, errors.New("set both DAYCAST_CAST_DEVICE and DAYCAST_CAST_APP_ID, or neither")
	}
	if device != "" {
		if cfg.castDevice, err = withDefaultPort(device); err != nil {
			return config{}, fmt.Errorf("DAYCAST_CAST_DEVICE: %w", err)
		}
		cfg.castAppID = appID
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
func withDefaultPort(addr string) (string, error) {
	if _, _, err := net.SplitHostPort(addr); err == nil {
		return addr, nil
	}
	return net.JoinHostPort(addr, defaultCastPort), nil
}
