package main

import (
	"testing"

	"github.com/stijnvandepol/nookboard/internal/display"
	"github.com/stijnvandepol/nookboard/internal/homeassistant"
	"github.com/stijnvandepol/nookboard/internal/settings"
	"github.com/stijnvandepol/nookboard/internal/weather"
)

func keys(t *testing.T, cfg settings.Sources, entities []string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, src := range (&dataSources{}).build(cfg, entities) {
		out[src.Key] = src.Version
	}
	return out
}

func TestWeatherAndAirQualityFollowTheLocation(t *testing.T) {
	if got := keys(t, settings.Sources{}, nil); len(got) != 0 {
		t.Fatalf("sources without a location = %v, want none", got)
	}
	oss := keys(t, settings.Sources{Location: &weather.Location{Latitude: 51.77, Longitude: 5.52}}, nil)
	hall := keys(t, settings.Sources{Location: &weather.Location{Latitude: 52.37, Longitude: 4.9}}, nil)
	for _, key := range []string{"weather.current", display.AirQualitySource} {
		if oss[key] == "" || oss[key] == hall[key] {
			t.Fatalf("%s versions = %q and %q; want a source that restarts when the location changes", key, oss[key], hall[key])
		}
	}
}

func TestHomeAssistantIsFetchedOnlyWhenShown(t *testing.T) {
	conn := &homeassistant.Connection{URL: "http://ha:8123", Token: "secret"}
	if got := keys(t, settings.Sources{HomeAssistant: conn}, nil); len(got) != 0 {
		t.Fatalf("sources without Home Assistant widgets = %v, want none", got)
	}
	if got := keys(t, settings.Sources{}, []string{"sensor.temp"}); len(got) != 0 {
		t.Fatalf("sources without a connection = %v, want none", got)
	}
	one := keys(t, settings.Sources{HomeAssistant: conn}, []string{"sensor.temp"})
	two := keys(t, settings.Sources{HomeAssistant: conn}, []string{"sensor.temp", "sensor.door"})
	if one[display.HomeAssistantSource] == "" || one[display.HomeAssistantSource] == two[display.HomeAssistantSource] {
		t.Fatalf("versions = %v and %v; want a source whose version follows the entities", one, two)
	}
}
