package main

import (
	"testing"
	"time"

	"github.com/stijnvandepol/gdash/internal/cast"
)

func TestDeviceRefreshInterval(t *testing.T) {
	tests := []struct {
		model   string
		minutes int
		want    time.Duration
	}{
		{"Chromecast HD", 15, 15 * time.Minute},
		{"Chromecast HD", 5, 5 * time.Minute},
		{"Chromecast HD", 0, 0},
		// A Nest Hub closes a receiver without media after about 10 minutes.
		{"Google Nest Hub", 15, nestHubRefreshInterval},
		{"Google Nest Hub", 0, nestHubRefreshInterval},
		{"Google Nest Hub", 5, 5 * time.Minute},
	}
	for _, tt := range tests {
		if got := deviceRefreshInterval(cast.Device{Model: tt.model}, tt.minutes); got != tt.want {
			t.Errorf("%s at %d minutes: refresh = %v, want %v", tt.model, tt.minutes, got, tt.want)
		}
	}
}

func TestDisplayURL(t *testing.T) {
	if got := displayURL("192.168.1.109", "8420", "tv-woonkamer", "abc_-1"); got != "http://192.168.1.109:8420/d/tv-woonkamer?launch=abc_-1" {
		t.Errorf("displayURL = %q", got)
	}
}
