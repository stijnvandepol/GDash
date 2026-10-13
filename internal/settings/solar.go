package settings

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/stijnvandepol/gdash/internal/solar"
	"github.com/stijnvandepol/gdash/internal/validation"
	"github.com/stijnvandepol/gdash/internal/weather"
)

const keySolar = "solar"

// Solar is the household's solar panels, for the solar forecast widget.
type Solar struct {
	// Location is where the panels are, or nil to follow the weather
	// location.
	Location         *Location     `json:"location"`
	Arrays           []solar.Array `json:"arrays"`
	PerformanceRatio float64       `json:"performanceRatio"`
	Source           string        `json:"source"`
}

// SetSolar stores the solar panels. It returns a *validation.Error for an
// invalid location, panel or choice.
func (s *Service) SetSolar(ctx context.Context, in Solar) error {
	var errs validation.Collector
	if in.Location != nil {
		for _, f := range validation.Fields(checkLocation(*in.Location)) {
			errs.Add("location."+f.Field, f.Message)
		}
	}
	for _, f := range validation.Fields(solar.CheckPanels(in.Arrays, in.PerformanceRatio, in.Source)) {
		errs.Add(f.Field, f.Message)
	}
	if err := errs.Err(); err != nil {
		return err
	}
	value, err := json.Marshal(in)
	if err != nil {
		return err
	}
	return s.Store.SetSetting(ctx, keySolar, string(value))
}

// ClearSolar removes the solar panels; solar widgets then say to set them
// up.
func (s *Service) ClearSolar(ctx context.Context) error {
	return s.Store.DeleteSetting(ctx, keySolar)
}

func (s *Service) solarPanels(ctx context.Context) (*Solar, error) {
	value, ok, err := s.Store.Setting(ctx, keySolar)
	if err != nil || !ok {
		return nil, err
	}
	var setup Solar
	if err := json.Unmarshal([]byte(value), &setup); err != nil {
		return nil, fmt.Errorf("stored solar panels are invalid: %w", err)
	}
	return &setup, nil
}

// solarSetup is what the solar forecast needs: the panels at their own
// place or the weather location. It is nil without panels, or without a
// place to put them.
func solarSetup(panels *Solar, weatherLocation *weather.Location) (*solar.Setup, error) {
	if panels == nil {
		return nil, nil
	}
	loc := weatherLocation
	if panels.Location != nil {
		own, err := weather.NewLocation(panels.Location.Latitude, panels.Location.Longitude)
		if err != nil {
			return nil, fmt.Errorf("stored solar location: %w", err)
		}
		loc = &own
	}
	if loc == nil {
		return nil, nil
	}
	return &solar.Setup{Location: *loc, Arrays: panels.Arrays, PerformanceRatio: panels.PerformanceRatio, Source: panels.Source}, nil
}
