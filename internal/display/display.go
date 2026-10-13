// Package display builds the state that displays render and streams it to
// them. Browser displays receive it over Server-Sent Events and Cast
// displays over the Cast message channel, in the same message format.
package display

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/stijnvandepol/gdash/internal/announcement"
	"github.com/stijnvandepol/gdash/internal/datahub"
	"github.com/stijnvandepol/gdash/internal/schedule"
)

// CastNamespace is the Cast message namespace between the GDash server
// and its receiver. web/src/protocol.ts uses the same value.
const CastNamespace = "urn:x-cast:app.gdash.display"

// Widget is one tile on the dashboard grid. Positions are in grid units.
type Widget struct {
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	X        int            `json:"x"`
	Y        int            `json:"y"`
	W        int            `json:"w"`
	H        int            `json:"h"`
	Source   string         `json:"source,omitempty"`
	Settings map[string]any `json:"settings,omitempty"`
}

// Dashboard is a layout on a fixed 16:9 grid.
type Dashboard struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Theme string `json:"theme"`
	// Font is one of Fonts, or empty for the theme's own typeface.
	Font string `json:"font"`
	// NightTheme is the theme to show between sunset and sunrise at the
	// weather location, or empty to keep Theme. A snapshot already carries
	// the theme that applies, in Theme.
	NightTheme string   `json:"nightTheme"`
	Columns    int      `json:"columns"`
	Rows       int      `json:"rows"`
	Widgets    []Widget `json:"widgets"`
}

// DefaultDashboard is the only dashboard in the prototype. The editor in
// v0.5 replaces it with dashboards stored in SQLite. withCalendar adds the
// calendar when calendars are configured.
func DefaultDashboard(withCalendar bool) Dashboard {
	d := Dashboard{
		ID:      DefaultID,
		Name:    "Default",
		Theme:   defaultTheme,
		Columns: GridColumns,
		Rows:    GridRows,
		Widgets: []Widget{
			{ID: "clock", Type: "clock", X: 0, Y: 0, W: 14, H: 6},
			{ID: "weather", Type: "weather", X: 14, Y: 0, W: 10, H: 14, Source: WeatherSource},
			{ID: "news", Type: "news", X: 0, Y: 6, W: 14, H: 8, Source: "news.default", Settings: map[string]any{"maxItems": 5}},
		},
	}
	if withCalendar {
		d.Widgets = []Widget{
			{ID: "clock", Type: "clock", X: 0, Y: 0, W: 10, H: 5},
			{ID: "weather", Type: "weather", X: 0, Y: 5, W: 10, H: 9, Source: WeatherSource},
			{ID: "calendar", Type: "calendar", X: 10, Y: 0, W: 14, H: 9, Source: "calendar.default", Settings: map[string]any{"maxItems": 5}},
			{ID: "news", Type: "news", X: 10, Y: 9, W: 14, H: 5, Source: "news.default", Settings: map[string]any{"maxItems": 3}},
		}
	}
	return d
}

// Snapshot is the complete state of a display. Version increases with every
// change, so a display can ignore a snapshot it has already rendered.
type Snapshot struct {
	Type       string    `json:"type"`
	Version    int64     `json:"version"`
	ServerTime time.Time `json:"serverTime"`
	// Active is false outside the display's active hours; the page then
	// stays dark and Data is empty.
	Active    bool                     `json:"active"`
	Dashboard Dashboard                `json:"dashboard"`
	Data      map[string]datahub.Entry `json:"data"`
	// Language is what the screen shows its texts and dates in: "en",
	// "nl", "de" or "fr".
	Language string `json:"language"`
	// Pages, when set, are shown in turn, each for PageSeconds; Dashboard
	// is then the first of them. The screen turns the pages by its clock.
	Pages       []Dashboard `json:"pages,omitempty"`
	PageSeconds int         `json:"pageSeconds,omitempty"`
	// TextScale is the text size in percent; the screen draws the stage
	// smaller by that factor, so everything on it looks larger.
	TextScale int `json:"textScale"`
	// Announcement is shown over the dashboard until its ExpiresAt, which
	// the screen compares with ServerTime. Absent when there is none and
	// outside active hours.
	Announcement *announcement.Announcement `json:"announcement,omitempty"`
}

// View is what one display shows and when.
type View struct {
	Dashboard Dashboard
	Hours     schedule.Hours
	// Schedule replaces Dashboard during parts of the day.
	Schedule []ScheduledDashboard
	// Pages are shown in turn with Dashboard, each for PageSeconds, while
	// no slot of the schedule applies.
	Pages       []Dashboard
	PageSeconds int
	// TextScale is the display's text size in percent.
	TextScale int
	// Announcement is the message to show over the dashboard now, if any.
	Announcement *announcement.Announcement
}

// ScheduledDashboard is a dashboard shown during a window of the week.
type ScheduledDashboard struct {
	Window    schedule.Window
	Dashboard Dashboard
}

// DashboardAt returns the dashboard the display shows at t: the first slot
// of its schedule that contains t, or else its own dashboard.
func (v View) DashboardAt(t time.Time) Dashboard {
	if slot, ok := v.slotAt(t); ok {
		return slot.Dashboard
	}
	return v.Dashboard
}

// PagesAt returns the pages to show in turn at t, starting with the
// display's own dashboard, or nil when a slot applies or nothing rotates.
func (v View) PagesAt(t time.Time) []Dashboard {
	if _, ok := v.slotAt(t); ok || len(v.Pages) == 0 {
		return nil
	}
	return append([]Dashboard{v.Dashboard}, v.Pages...)
}

func (v View) slotAt(t time.Time) (ScheduledDashboard, bool) {
	for _, slot := range v.Schedule {
		if slot.Window.Contains(t) {
			return slot, true
		}
	}
	return ScheduledDashboard{}, false
}

// Broadcaster holds the latest encoded snapshot of every display and hands
// it to that display's subscribers. A slow subscriber only ever gets the
// newest snapshot, never a backlog.
type Broadcaster struct {
	mu          sync.Mutex
	version     int64
	latest      map[string][]byte
	subscribers map[string]map[chan []byte]struct{}

	// sent is the data in each display's latest snapshot.
	sent map[string]map[string]datahub.Entry
}

// NewBroadcaster creates an empty broadcaster.
func NewBroadcaster() *Broadcaster {
	return &Broadcaster{latest: map[string][]byte{}, subscribers: map[string]map[chan []byte]struct{}{}}
}

// Publish builds a snapshot for every display in views (display ID to what
// it shows) and sends it to that display's subscribers. Each snapshot only
// carries the data its widgets use, so a hall screen without a calendar
// never receives calendar entries, and a display outside its active hours
// receives none at all.
func (b *Broadcaster) Publish(now time.Time, views map[string]View, data map[string]datahub.Entry, language string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.version++
	latest := make(map[string][]byte, len(views))
	sent := make(map[string]map[string]datahub.Entry, len(views))
	night := IsNight(now, data)
	for displayID, view := range views {
		active := view.Hours.Contains(now)
		dashboard := withNightTheme(view.DashboardAt(now), night)
		pages := withNightThemes(view.PagesAt(now), night)
		seconds := 0
		if pages != nil {
			seconds = view.PageSeconds
		}
		used := map[string]datahub.Entry{}
		var shown *announcement.Announcement
		if active {
			used = dataFor(append([]Dashboard{dashboard}, pages...), data, now)
			shown = view.Announcement
		}
		msg, err := json.Marshal(Snapshot{
			Type: "snapshot", Version: b.version, ServerTime: now, Active: active,
			Dashboard: screenDashboard(dashboard), Data: used, Language: language, Pages: screenDashboards(pages), PageSeconds: seconds,
			TextScale: view.TextScale, Announcement: shown,
		})
		if err != nil {
			return fmt.Errorf("encode snapshot for display %s: %w", displayID, err)
		}
		latest[displayID] = msg
		sent[displayID] = used
		for ch := range b.subscribers[displayID] {
			replaceLatest(ch, msg)
		}
	}
	b.latest, b.sent = latest, sent
	return nil
}

// Sent returns the data entry with key that the display was last sent,
// for checking what its screens may load, such as the cover art of what
// it shows playing.
func (b *Broadcaster) Sent(displayID, key string) (datahub.Entry, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	entry, ok := b.sent[displayID][key]
	return entry, ok
}

// Subscribe returns a channel that first receives the display's current
// snapshot (if one exists) and then every new one. It is closed when ctx
// ends.
func (b *Broadcaster) Subscribe(ctx context.Context, displayID string) <-chan []byte {
	ch := make(chan []byte, 1)
	b.mu.Lock()
	if msg, ok := b.latest[displayID]; ok {
		ch <- msg
	}
	if b.subscribers[displayID] == nil {
		b.subscribers[displayID] = map[chan []byte]struct{}{}
	}
	b.subscribers[displayID][ch] = struct{}{}
	b.mu.Unlock()

	go func() {
		<-ctx.Done()
		b.mu.Lock()
		delete(b.subscribers[displayID], ch)
		if len(b.subscribers[displayID]) == 0 {
			delete(b.subscribers, displayID)
		}
		b.mu.Unlock()
		close(ch)
	}()
	return ch
}

func replaceLatest(ch chan []byte, msg []byte) {
	select {
	case <-ch:
	default:
	}
	ch <- msg
}

// Publisher rebuilds every display's snapshot when data or configuration
// changes. Calls are serialized, so snapshots never go out of order.
type Publisher struct {
	Broadcaster *Broadcaster
	// Views returns, per display ID, what it shows and when.
	Views func(ctx context.Context) (map[string]View, error)
	Data  func() map[string]datahub.Entry
	// Language returns the display language from the settings.
	Language func(ctx context.Context) (string, error)
	// Announcements holds the messages to show over the dashboards.
	Announcements *announcement.Board
	Now           func() time.Time
	Logger        *slog.Logger

	mu sync.Mutex
}

// Refresh publishes new snapshots. A failure is logged and the displays keep
// their last snapshot; the next change publishes again.
func (p *Publisher) Refresh(ctx context.Context) {
	p.mu.Lock()
	defer p.mu.Unlock()
	views, err := p.Views(ctx)
	if err != nil {
		p.Logger.Error("could not read dashboards; displays keep their last state", "error", err)
		return
	}
	language, err := p.Language(ctx)
	if err != nil {
		p.Logger.Error("could not read the display language; displays keep their last state", "error", err)
		return
	}
	now := p.Now()
	for id, view := range views {
		view.Announcement = p.Announcements.For(id, now)
		views[id] = view
	}
	if err := p.Broadcaster.Publish(now, views, p.Data(), language); err != nil {
		p.Logger.Error("could not publish display state", "error", err)
	}
}
