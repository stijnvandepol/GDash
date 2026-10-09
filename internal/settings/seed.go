package settings

import (
	"context"
	"fmt"
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
		// Everything is checked before anything is written: a seed that
		// fails halfway would leave settings that are no longer empty, and
		// the next start would then skip the rest without a word.
		if err := checkSeed(seed); err != nil {
			return false, err
		}
		if err := s.apply(ctx, s.trim(seed)); err != nil {
			return false, err
		}
	}
	return empty, s.Store.SetSetting(ctx, keySeeded, "true")
}

// checkSeed validates every part of seed. Errors never include a calendar
// address: it is a secret.
func checkSeed(seed Seed) error {
	if seed.Location != nil {
		if err := checkLocation(*seed.Location); err != nil {
			return fmt.Errorf("location from the environment: %w", err)
		}
	}
	for i, f := range seed.NewsFeeds {
		if _, err := checkSource(f.Name, f.URL, false); err != nil {
			return fmt.Errorf("news feed %d from the environment: %w", i+1, err)
		}
	}
	for i, c := range seed.Calendars {
		if _, err := checkSource(c.Name, c.URL, true); err != nil {
			return fmt.Errorf("calendar %d from the environment is invalid", i+1)
		}
	}
	return nil
}

// trim drops feeds and calendars beyond the limits, and says so.
func (s *Service) trim(seed Seed) Seed {
	if n := len(seed.NewsFeeds); n > MaxNewsFeeds {
		s.Logger.Warn("more news feeds in the environment than GDash keeps; the rest are skipped", "given", n, "kept", MaxNewsFeeds)
		seed.NewsFeeds = seed.NewsFeeds[:MaxNewsFeeds]
	}
	if n := len(seed.Calendars); n > MaxCalendars {
		s.Logger.Warn("more calendars in the environment than GDash keeps; the rest are skipped", "given", n, "kept", MaxCalendars)
		seed.Calendars = seed.Calendars[:MaxCalendars]
	}
	return seed
}

func (s *Service) apply(ctx context.Context, seed Seed) error {
	if seed.Location != nil {
		if err := s.SetLocation(ctx, *seed.Location); err != nil {
			return fmt.Errorf("location from the environment: %w", err)
		}
	}
	for _, f := range seed.NewsFeeds {
		if _, err := s.AddNewsFeed(ctx, f.Name, f.URL); err != nil {
			return fmt.Errorf("news feed from the environment: %w", err)
		}
	}
	for i, c := range seed.Calendars {
		// The error never includes the address: it is a secret.
		if _, err := s.AddCalendar(ctx, c.Name, c.URL); err != nil {
			return fmt.Errorf("calendar %d from the environment could not be stored", i+1)
		}
	}
	return nil
}
