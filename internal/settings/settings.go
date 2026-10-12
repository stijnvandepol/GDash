// Package settings manages what GDash fetches: the weather location, news
// feeds, calendars, the Home Assistant connection and custom API sources.
// They are edited in the web interface and stored in
// SQLite; environment variables only fill them on the very first start.
package settings

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/stijnvandepol/gdash/internal/calendar"
	"github.com/stijnvandepol/gdash/internal/homeassistant"
	"github.com/stijnvandepol/gdash/internal/news"
	"github.com/stijnvandepol/gdash/internal/secret"
	"github.com/stijnvandepol/gdash/internal/store"
	"github.com/stijnvandepol/gdash/internal/validation"
	"github.com/stijnvandepol/gdash/internal/weather"
)

// Limits per kind. Every feed costs a request per refresh, and every
// calendar is downloaded whole, so a household needs a handful.
const (
	MaxNewsFeeds = 5
	MaxCalendars = 10
)

const (
	keyLocation = "location"
	keyLanguage = "language"
	// DefaultLanguage is the display language until one is chosen.
	DefaultLanguage = "en"
	// keySeeded marks that the environment was read once; after that the
	// web interface owns the settings.
	keySeeded     = "seeded"
	maxNameLength = 60
	maxURLLength  = 2048
)

// Location is where the weather is for, with the name the admin chose.
type Location struct {
	Name      string  `json:"name"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// NewsFeed is a news feed. Its address is public and shown.
type NewsFeed struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

// Settings is everything the Settings page shows.
type Settings struct {
	// Location is nil until one is chosen.
	Location  *Location  `json:"location"`
	NewsFeeds []NewsFeed `json:"newsFeeds"`
	Calendars []Calendar `json:"calendars"`
	// Language is what TVs show their texts and dates in.
	Language string `json:"language"`
	// HomeAssistant is nil until a connection is set up.
	HomeAssistant *HomeAssistant `json:"homeAssistant"`
}

// Languages are the display languages; web/src/i18n.svelte.ts has their
// texts.
var Languages = []string{"en", "nl", "de", "fr"}

// Sources is what the data hub needs to fetch, with calendar addresses
// decrypted. It never leaves the server.
type Sources struct {
	Location  *weather.Location
	NewsFeeds []news.Source
	Calendars []calendar.Source
	// HomeAssistant is nil when no readable connection is set up.
	HomeAssistant *homeassistant.Connection
}

// Service reads and changes the settings.
type Service struct {
	Store  *store.Store
	Box    *secret.Box
	Now    func() time.Time
	Logger *slog.Logger
}

// Get returns the settings for the web interface.
func (s *Service) Get(ctx context.Context) (Settings, error) {
	loc, err := s.location(ctx)
	if err != nil {
		return Settings{}, err
	}
	language, err := s.Language(ctx)
	if err != nil {
		return Settings{}, err
	}
	out := Settings{Location: loc, NewsFeeds: []NewsFeed{}, Calendars: []Calendar{}, Language: language}
	feeds, err := s.Store.Sources(ctx, store.SourceNews)
	if err != nil {
		return Settings{}, err
	}
	for _, f := range feeds {
		out.NewsFeeds = append(out.NewsFeeds, NewsFeed{ID: f.ID, Name: f.Name, URL: f.Address})
	}
	calendars, err := s.Store.Sources(ctx, store.SourceCalendar)
	if err != nil {
		return Settings{}, err
	}
	for i, c := range calendars {
		_, openErr := s.Box.Open(c.Address)
		out.Calendars = append(out.Calendars, Calendar{
			ID: c.ID, Name: c.Name, Host: c.Host, Color: calendarColor(c.Color, i), Unreadable: openErr != nil,
		})
	}
	if out.HomeAssistant, err = s.homeAssistantView(ctx); err != nil {
		return Settings{}, err
	}
	return out, nil
}

// Sources returns what to fetch. A calendar that cannot be decrypted is
// left out and logged, so the other calendars keep working.
func (s *Service) Sources(ctx context.Context) (Sources, error) {
	var out Sources
	loc, err := s.location(ctx)
	if err != nil {
		return Sources{}, err
	}
	if loc != nil {
		wl, err := weather.NewLocation(loc.Latitude, loc.Longitude)
		if err != nil {
			return Sources{}, fmt.Errorf("stored location: %w", err)
		}
		out.Location = &wl
	}
	feeds, err := s.Store.Sources(ctx, store.SourceNews)
	if err != nil {
		return Sources{}, err
	}
	for _, f := range feeds {
		out.NewsFeeds = append(out.NewsFeeds, news.Source{Name: f.Name, URL: f.Address})
	}
	calendars, err := s.Store.Sources(ctx, store.SourceCalendar)
	if err != nil {
		return Sources{}, err
	}
	for i, c := range calendars {
		address, err := s.Box.Open(c.Address)
		if err != nil {
			s.Logger.Error("calendar address cannot be decrypted; remove the calendar and add it again", "calendar", c.Name, "error", err)
			continue
		}
		out.Calendars = append(out.Calendars, calendar.Source{ID: c.ID, Name: c.Name, URL: address, Color: calendarColor(c.Color, i)})
	}
	conn, ok, err := s.HomeAssistantConnection(ctx)
	if err != nil {
		s.Logger.Error("home assistant connection cannot be decrypted; connect again", "error", err)
	} else if ok {
		out.HomeAssistant = &conn
	}
	return out, nil
}

// SetLocation stores the weather location. It returns a *validation.Error
// for an invalid name or coordinate.
func (s *Service) SetLocation(ctx context.Context, loc Location) error {
	if err := checkLocation(loc); err != nil {
		return err
	}
	value, err := json.Marshal(loc)
	if err != nil {
		return err
	}
	return s.Store.SetSetting(ctx, keyLocation, string(value))
}

// Language returns the display language, or DefaultLanguage.
func (s *Service) Language(ctx context.Context) (string, error) {
	value, ok, err := s.Store.Setting(ctx, keyLanguage)
	if err != nil || !ok {
		return DefaultLanguage, err
	}
	return value, nil
}

// SetLanguage chooses the display language. It returns a
// *validation.Error for a language GDash has no texts for.
func (s *Service) SetLanguage(ctx context.Context, language string) error {
	if !slices.Contains(Languages, language) {
		var errs validation.Collector
		errs.Add("language", "choose one of: "+strings.Join(Languages, ", "))
		return errs.Err()
	}
	return s.Store.SetSetting(ctx, keyLanguage, language)
}

// ClearLocation removes the weather location; weather widgets then show
// that none is set.
func (s *Service) ClearLocation(ctx context.Context) error {
	return s.Store.DeleteSetting(ctx, keyLocation)
}

// AddNewsFeed adds an RSS or Atom feed. It returns a *validation.Error for
// invalid input and store.ErrConflict when MaxNewsFeeds is reached.
func (s *Service) AddNewsFeed(ctx context.Context, name, address string) (NewsFeed, error) {
	u, err := checkSource(name, address, false)
	if err != nil {
		return NewsFeed{}, err
	}
	id, err := s.Store.AddSource(ctx, store.Source{
		Kind: store.SourceNews, Name: name, Address: u.String(), Host: u.Hostname(), CreatedAt: s.Now().UTC(),
	}, MaxNewsFeeds)
	if err != nil {
		return NewsFeed{}, err
	}
	return NewsFeed{ID: id, Name: name, URL: u.String()}, nil
}

// DeleteNewsFeed removes a feed, or returns store.ErrNotFound.
func (s *Service) DeleteNewsFeed(ctx context.Context, id int64) error {
	return s.Store.DeleteSource(ctx, store.SourceNews, id)
}

func (s *Service) location(ctx context.Context) (*Location, error) {
	value, ok, err := s.Store.Setting(ctx, keyLocation)
	if err != nil || !ok {
		return nil, err
	}
	var loc Location
	if err := json.Unmarshal([]byte(value), &loc); err != nil {
		return nil, fmt.Errorf("stored location is invalid: %w", err)
	}
	return &loc, nil
}

func checkLocation(loc Location) error {
	var errs validation.Collector
	if msg := nameProblem(loc.Name); msg != "" {
		errs.Add("name", msg)
	}
	if _, err := weather.NewLocation(loc.Latitude, 0); err != nil {
		errs.Add("latitude", "use a number from -90 to 90")
	}
	if _, err := weather.NewLocation(0, loc.Longitude); err != nil {
		errs.Add("longitude", "use a number from -180 to 180")
	}
	return errs.Err()
}

// checkSource validates a name and an http(s) address. Calendars also take
// webcal://, which calendar apps fetch over https.
func checkSource(name, address string, isCalendar bool) (*url.URL, error) {
	var errs validation.Collector
	if msg := nameProblem(name); msg != "" {
		errs.Add("name", msg)
	}
	u, err := url.Parse(strings.TrimSpace(address))
	switch {
	case err != nil || u.Host == "" || len(address) > maxURLLength:
		errs.Add("url", "paste the full address, starting with https://")
	case isCalendar && u.Scheme == "webcal":
		u.Scheme = "https"
	case u.Scheme != "http" && u.Scheme != "https":
		errs.Add("url", "use an http or https address")
	case !isCalendar && u.User != nil:
		// Feed addresses are stored and shown as they are; a password in
		// one would be visible in the web interface and the logs.
		errs.Add("url", "use the feed's address without a user name or password")
	}
	if err := errs.Err(); err != nil {
		return nil, err
	}
	return u, nil
}

func nameProblem(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" || trimmed != name || utf8.RuneCountInString(name) > maxNameLength {
		return "use 1 to 60 characters without leading or trailing spaces"
	}
	return ""
}
