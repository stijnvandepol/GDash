package settings

import (
	"context"
	"errors"
	"fmt"

	"github.com/stijnvandepol/daycast/internal/store"
)

// Seed is what environment variables configured before the web interface
// managed the sources. Empty fields are skipped.
type Seed struct {
	Location  *Location
	NewsFeeds []NewsFeed
	// Calendars hold their secret address in URL.
	Calendars []SeedCalendar
}

// SeedCalendar is a calendar from the environment.
type SeedCalendar struct {
	Name string
	URL  string
}

// SeedOnce fills the settings from seed on the first start, so an existing
// installation keeps its sources when it upgrades. It does nothing when the
// settings were seeded before or already hold sources, and reports whether
// it seeded.
func (s *Service) SeedOnce(ctx context.Context, seed Seed) (bool, error) {
	if _, done, err := s.Store.Setting(ctx, keySeeded); err != nil || done {
		return false, err
	}
	current, err := s.Get(ctx)
	if err != nil {
		return false, err
	}
	empty := current.Location == nil && len(current.NewsFeeds) == 0 && len(current.Calendars) == 0
	if empty {
		if err := s.apply(ctx, seed); err != nil {
			return false, err
		}
	}
	return empty, s.Store.SetSetting(ctx, keySeeded, "true")
}

func (s *Service) apply(ctx context.Context, seed Seed) error {
	if seed.Location != nil {
		if err := s.SetLocation(ctx, *seed.Location); err != nil {
			return fmt.Errorf("location from the environment: %w", err)
		}
	}
	for _, f := range seed.NewsFeeds {
		if _, err := s.AddNewsFeed(ctx, f.Name, f.URL); err != nil && !errors.Is(err, store.ErrConflict) {
			return fmt.Errorf("news feed from the environment: %w", err)
		}
	}
	for i, c := range seed.Calendars {
		// The error never includes the address: it is a secret.
		if _, err := s.AddCalendar(ctx, c.Name, c.URL); err != nil && !errors.Is(err, store.ErrConflict) {
			return fmt.Errorf("calendar %d from the environment is invalid", i+1)
		}
	}
	return nil
}
