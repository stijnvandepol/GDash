// Package screenreport keeps what each screen reports about its own browser:
// its user agent, its size, which newer CSS features it has and the script
// errors it ran into. A TV's built-in browser is hard to inspect, and this
// is how an admin sees why a dashboard looks different there. Reports live
// in memory only, the last one per screen: after a restart every screen
// reports again when it reconnects.
package screenreport

import (
	"fmt"
	"maps"
	"slices"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/stijnvandepol/gdash/internal/ratelimit"
	"github.com/stijnvandepol/gdash/internal/validation"
)

const (
	// MaxBodyBytes bounds a report as it arrives; a full one is about 1 KB.
	MaxBodyBytes = 8 << 10
	// MaxErrors is how many script errors one report may carry.
	MaxErrors = 5
	// MaxVisibilityEvents is how many changes of the page's visibility and
	// focus one report may carry.
	MaxVisibilityEvents = 10

	maxUserAgentLength = 512
	maxMessageLength   = 300
	maxSourceLength    = 200
	maxViewportPixels  = 16384
	maxPixelRatio      = 16
	maxStageScale      = 32
	// A dashboard that takes longer than an hour to draw is not drawing.
	maxFirstRenderMs = 3_600_000
	// A screen reports when it loads and when it reconnects; a TV that a
	// cast keeper restarts every 15 minutes reports four times an hour.
	// This leaves room for a flaky network and stops a loop.
	maxReports   = 10
	reportWindow = 10 * time.Minute
	// maxScreensPerDisplay keeps the newest screens of a display; an older
	// one is usually a token a screen no longer uses.
	maxScreensPerDisplay = 5
)

// Viewport is the screen's size in CSS pixels.
type Viewport struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// Features says which CSS features the screen's browser supports. The TV
// page relies on some of them and has a fallback for others.
type Features struct {
	ContainerUnits   bool `json:"containerUnits"`
	ContainerQueries bool `json:"containerQueries"`
	Where            bool `json:"where"`
	Has              bool `json:"has"`
	ColorMix         bool `json:"colorMix"`
	AspectRatio      bool `json:"aspectRatio"`
	FlexGap          bool `json:"flexGap"`
	Inset            bool `json:"inset"`
	LineClamp        bool `json:"lineClamp"`
	TextWrap         bool `json:"textWrap"`
	Nesting          bool `json:"nesting"`
	CascadeLayers    bool `json:"cascadeLayers"`
}

// ScriptError is an uncaught error on the screen: its message and where in
// GDash's scripts it happened, without the page's address.
type ScriptError struct {
	Message string `json:"message"`
	Source  string `json:"source"`
}

// Why a screen reports.
const (
	// ReasonConnected: the page connected to the server, at load or after
	// a reconnect.
	ReasonConnected = "connected"
	// ReasonHidden: the page just became hidden, such as by the TV's
	// screensaver.
	ReasonHidden = "hidden"
)

// visibilityStates are the changes a page records: its visibility and
// whether its window has the focus.
var visibilityStates = []string{"hidden", "visible", "blur", "focus"}

// VisibilityEvent is one change of the page's visibility or focus.
type VisibilityEvent struct {
	State string    `json:"state"`
	At    time.Time `json:"at"`
}

// Report is what a screen sends about itself.
type Report struct {
	// Reason is ReasonConnected or ReasonHidden.
	Reason           string   `json:"reason"`
	UserAgent        string   `json:"userAgent"`
	Viewport         Viewport `json:"viewport"`
	DevicePixelRatio float64  `json:"devicePixelRatio"`
	// StageScale is how far the 1920-pixel stage is scaled to fit the screen.
	StageScale float64  `json:"stageScale"`
	Features   Features `json:"features"`
	// Compat is true when the page uses its fallback for container queries.
	Compat      bool `json:"compat"`
	FontsLoaded bool `json:"fontsLoaded"`
	// FirstRenderMs is how long after loading the dashboard was first on
	// screen; nil when it has not been drawn, for example at night.
	FirstRenderMs *int          `json:"firstRenderMs"`
	Errors        []ScriptError `json:"errors"`
	// Visibility holds the last changes of the page's visibility and focus,
	// oldest first.
	Visibility []VisibilityEvent `json:"visibility"`
}

// Received is a report as the server keeps it.
type Received struct {
	Report
	// Screen is "cast" for the TV a cast keeper opened, "screen" for a
	// screen paired with a code.
	Screen     string    `json:"screen"`
	ReceivedAt time.Time `json:"receivedAt"`
}

// Validate checks a report from a screen. It returns a *validation.Error
// listing every invalid field.
func Validate(r Report) error {
	var errs validation.Collector
	if r.Reason != ReasonConnected && r.Reason != ReasonHidden {
		errs.Add("reason", "use connected or hidden")
	}
	checkText(&errs, "userAgent", r.UserAgent, 1, maxUserAgentLength)
	if r.Viewport.Width < 1 || r.Viewport.Width > maxViewportPixels || r.Viewport.Height < 1 || r.Viewport.Height > maxViewportPixels {
		errs.Add("viewport", fmt.Sprintf("use a width and height from 1 to %d", maxViewportPixels))
	}
	if r.DevicePixelRatio <= 0 || r.DevicePixelRatio > maxPixelRatio {
		errs.Add("devicePixelRatio", fmt.Sprintf("use a number above 0, at most %d", maxPixelRatio))
	}
	if r.StageScale <= 0 || r.StageScale > maxStageScale {
		errs.Add("stageScale", fmt.Sprintf("use a number above 0, at most %d", maxStageScale))
	}
	if r.FirstRenderMs != nil && (*r.FirstRenderMs < 0 || *r.FirstRenderMs > maxFirstRenderMs) {
		errs.Add("firstRenderMs", fmt.Sprintf("use a whole number from 0 to %d, or null", maxFirstRenderMs))
	}
	if len(r.Errors) > MaxErrors {
		errs.Add("errors", fmt.Sprintf("send at most %d errors", MaxErrors))
	} else {
		for i, e := range r.Errors {
			checkText(&errs, fmt.Sprintf("errors[%d].message", i), e.Message, 1, maxMessageLength)
			checkText(&errs, fmt.Sprintf("errors[%d].source", i), e.Source, 0, maxSourceLength)
		}
	}
	if len(r.Visibility) > MaxVisibilityEvents {
		errs.Add("visibility", fmt.Sprintf("send at most %d changes", MaxVisibilityEvents))
	} else {
		for i, e := range r.Visibility {
			if !slices.Contains(visibilityStates, e.State) || e.At.IsZero() {
				errs.Add(fmt.Sprintf("visibility[%d]", i), "use a state of hidden, visible, blur or focus and the time it happened")
			}
		}
	}
	return errs.Err()
}

func checkText(errs *validation.Collector, field, text string, minLength, maxLength int) {
	if n := utf8.RuneCountInString(text); n < minLength || n > maxLength || !utf8.ValidString(text) {
		errs.Add(field, fmt.Sprintf("use %d to %d characters", minLength, maxLength))
		return
	}
	if validation.HasControlCharacter(text) {
		errs.Add(field, "use plain text without line breaks or control characters")
	}
}

// Board holds the last report of each screen, per display.
type Board struct {
	now     func() time.Time
	limiter *ratelimit.Limiter

	mu sync.Mutex
	// byDisplay maps a display ID to its screens' reports by screen key.
	byDisplay map[string]map[string]Received
}

// NewBoard creates an empty board.
func NewBoard(now func() time.Time) *Board {
	return &Board{now: now, limiter: ratelimit.New(now, maxReports, reportWindow), byDisplay: map[string]map[string]Received{}}
}

// Allow reports whether a screen may send another report now, and counts
// this one. The limiter counts "failures"; here every report is one.
func (b *Board) Allow(displayID, screenKey string) bool {
	return b.limiter.Attempt(displayID + "/" + screenKey)
}

// Put keeps r as the last report of a screen; screen is the kind of screen
// as Received.Screen names it. A display keeps its newest
// maxScreensPerDisplay screens.
func (b *Board) Put(displayID, screenKey, screen string, r Report) {
	r.Errors, r.Visibility = slices.Clone(r.Errors), slices.Clone(r.Visibility)
	b.mu.Lock()
	defer b.mu.Unlock()
	screens := b.byDisplay[displayID]
	if screens == nil {
		screens = map[string]Received{}
		b.byDisplay[displayID] = screens
	}
	screens[screenKey] = Received{Report: r, Screen: screen, ReceivedAt: b.now().UTC()}
	for len(screens) > maxScreensPerDisplay {
		oldest := ""
		for key, rec := range screens {
			if oldest == "" || rec.ReceivedAt.Before(screens[oldest].ReceivedAt) {
				oldest = key
			}
		}
		delete(screens, oldest)
	}
}

// For returns the reports of a display's screens, newest first; an empty
// list, not nil, when there are none.
func (b *Board) For(displayID string) []Received {
	b.mu.Lock()
	defer b.mu.Unlock()
	screens := b.byDisplay[displayID]
	out := slices.AppendSeq(make([]Received, 0, len(screens)), maps.Values(screens))
	slices.SortFunc(out, func(a, c Received) int { return c.ReceivedAt.Compare(a.ReceivedAt) })
	for i := range out {
		out[i].Errors, out[i].Visibility = slices.Clone(out[i].Errors), slices.Clone(out[i].Visibility)
	}
	return out
}

// Forget drops the reports of a display, for one that was deleted.
func (b *Board) Forget(displayID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.byDisplay, displayID)
}

// ForgetScreens drops the reports of a display's screens of kind screen,
// for screens that were unpaired; the TV a cast keeper opened stays.
func (b *Board) ForgetScreens(displayID, screen string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	maps.DeleteFunc(b.byDisplay[displayID], func(_ string, rec Received) bool { return rec.Screen == screen })
}
