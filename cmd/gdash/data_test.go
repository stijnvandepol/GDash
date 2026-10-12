package main

import (
	"fmt"
	"testing"

	"github.com/stijnvandepol/gdash/internal/display"
	"github.com/stijnvandepol/gdash/internal/homeassistant"
	"github.com/stijnvandepol/gdash/internal/secret"
	"github.com/stijnvandepol/gdash/internal/settings"
	"github.com/stijnvandepol/gdash/internal/weather"
)

func keys(t *testing.T, cfg settings.Sources, entities []string, stops ...string) map[string]string {
	t.Helper()
	out := map[string]string{}
	box, err := secret.NewBox(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	for _, src := range (&dataSources{settings: &settings.Service{Box: box}}).build(cfg, entities, stops) {
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

func TestDeparturesAreFetchedOnlyForShownStops(t *testing.T) {
	// No settings are needed: OVapi has no key.
	if got := keys(t, settings.Sources{}, nil); got[display.DeparturesSource] != "" {
		t.Fatalf("sources without Departures widgets = %v, want no departures", got)
	}
	one := keys(t, settings.Sources{}, nil, "utrcsw")
	two := keys(t, settings.Sources{}, nil, "utrcsw", "05003")
	if one[display.DeparturesSource] == "" || one[display.DeparturesSource] == two[display.DeparturesSource] {
		t.Fatalf("versions = %v and %v; want a source whose version follows the stops", one, two)
	}
}

func TestDepartureStopsAreCapped(t *testing.T) {
	var many []string
	for i := range maxDepartureStops + 5 {
		many = append(many, fmt.Sprintf("s%02d", i))
	}
	capped := keys(t, settings.Sources{}, nil, many[:maxDepartureStops]...)
	over := keys(t, settings.Sources{}, nil, many...)
	if capped[display.DeparturesSource] != over[display.DeparturesSource] {
		t.Fatal("stops beyond the cap changed the source; the request would grow without bound")
	}
}
