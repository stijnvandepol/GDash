package main

import (
	"testing"

	"github.com/stijnvandepol/daycast/internal/cast"
)

func TestRefreshInterval(t *testing.T) {
	if got := refreshInterval(cast.Device{Model: "Google Nest Hub"}); got != nestHubRefreshInterval {
		t.Errorf("Nest Hub refresh = %v", got)
	}
	if got := refreshInterval(cast.Device{Model: "Chromecast HD"}); got != defaultRefreshInterval {
		t.Errorf("Chromecast HD refresh = %v", got)
	}
}

func TestDisplayURL(t *testing.T) {
	if got := displayURL("192.168.1.109", "8420", "tv-woonkamer", "abc_-1"); got != "http://192.168.1.109:8420/d/tv-woonkamer?launch=abc_-1" {
		t.Errorf("displayURL = %q", got)
	}
}
