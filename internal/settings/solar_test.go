package settings

import (
	"context"
	"testing"

	"github.com/stijnvandepol/gdash/internal/solar"
)

func TestSolarPanels(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	panels := Solar{Arrays: []solar.Array{{Tilt: 35, Azimuth: -10, PeakKW: 4.2}}, PerformanceRatio: 0.85, Source: solar.SourceOpenMeteo}

	if got, err := s.Get(ctx); err != nil || got.Solar != nil {
		t.Fatalf("fresh settings have solar panels %+v, %v", got.Solar, err)
	}
	invalid := Solar{Location: &Location{Name: "", Latitude: 99}, Arrays: []solar.Array{{Tilt: 95, PeakKW: 4}}, PerformanceRatio: 0.85, Source: "pvgis"}
	if err := s.SetSolar(ctx, invalid); fieldNames(err) != "location.name,location.latitude,arrays[0].tilt,source" {
		t.Fatalf("invalid panels: fields = %q", fieldNames(err))
	}
	if err := s.SetSolar(ctx, panels); err != nil {
		t.Fatal(err)
	}
	if got, err := s.Get(ctx); err != nil || got.Solar == nil || got.Solar.Arrays[0].PeakKW != 4.2 || got.Solar.Location != nil {
		t.Fatalf("settings solar = %+v, %v", got.Solar, err)
	}

	// Without a place of their own or a weather location, there is
	// nothing to fetch.
	if src, err := s.Sources(ctx); err != nil || src.Solar != nil {
		t.Fatalf("sources solar without any location = %+v, %v; want none", src.Solar, err)
	}
	if err := s.SetLocation(ctx, Location{Name: "Oss", Latitude: 51.765, Longitude: 5.518}); err != nil {
		t.Fatal(err)
	}
	src, err := s.Sources(ctx)
	if err != nil || src.Solar == nil || src.Solar.Location.Latitude != 51.77 || src.Solar.Source != solar.SourceOpenMeteo {
		t.Fatalf("sources solar = %+v, %v; want the panels at the rounded weather location", src.Solar, err)
	}

	panels.Location = &Location{Name: "Cabin", Latitude: 52.3456, Longitude: 6.0}
	if err := s.SetSolar(ctx, panels); err != nil {
		t.Fatal(err)
	}
	if src, _ := s.Sources(ctx); src.Solar == nil || src.Solar.Location.Latitude != 52.35 {
		t.Fatalf("sources solar = %+v; want the panels' own place, rounded", src.Solar)
	}

	if err := s.ClearSolar(ctx); err != nil {
		t.Fatal(err)
	}
	if src, _ := s.Sources(ctx); src.Solar != nil {
		t.Fatal("solar panels still set after removing them")
	}
}
