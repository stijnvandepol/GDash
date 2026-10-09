// Package announcement holds the short messages that an automation, such as
// "The washing machine is done", puts on top of the displays for a while.
// They live in memory only: after a restart a message that was meant for a
// minute is no longer news.
package announcement

import (
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/stijnvandepol/gdash/internal/ratelimit"
	"github.com/stijnvandepol/gdash/internal/validation"
)

// How long an announcement stays on screen, in seconds.
const (
	DefaultSeconds = 60
	MinSeconds     = 5
	MaxSeconds     = 3600
)

const (
	maxTitleLength = 60
	// Short enough to read from the sofa and far below the 64 KB limit of
	// a Cast message.
	maxMessageLength = 280
	// A sender may post this many announcements per window.
	maxPosts   = 30
	postWindow = time.Minute
)

// Announcement is what a display shows. A snapshot carries ExpiresAt in the
// same time base as its serverTime, so a screen can drop the message on time
// by itself.
type Announcement struct {
	Title     string    `json:"title,omitempty"`
	Message   string    `json:"message"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// Request is an announcement as a sender asks for it.
type Request struct {
	Title   string
	Message string
	// Seconds is how long to show it; nil means DefaultSeconds.
	Seconds *int
	// Displays are the IDs to show it on; nil means every display.
	Displays []string
}

// Posted is an announcement together with the displays it was sent to.
type Posted struct {
	Announcement
	Displays []string `json:"displays"`
}

// Board keeps the current announcement of each display. A newer one replaces
// the older one on the displays it targets and leaves the other displays
// alone, so a message to the kitchen does not wipe the one for the hall.
// Replacing instead of queueing means a screen never shows a backlog of
// messages that went stale while they waited.
type Board struct {
	limiter *ratelimit.Limiter

	mu      sync.Mutex
	current map[string]Announcement
}

// NewBoard creates an empty board.
func NewBoard(now func() time.Time) *Board {
	return &Board{limiter: ratelimit.New(now, maxPosts, postWindow), current: map[string]Announcement{}}
}

// Allow reports whether sender may post another announcement, and counts
// this one. The limiter counts "failures"; here every post is one.
func (b *Board) Allow(sender string) bool {
	if !b.limiter.Allowed(sender) {
		return false
	}
	b.limiter.Fail(sender)
	return true
}

// Post validates req and shows it from now on the displays it targets.
// known lists the IDs of the displays that exist. It returns a
// *validation.Error listing every invalid field.
func (b *Board) Post(req Request, known []string, now time.Time) (Posted, error) {
	posted, err := validate(req, known, now)
	if err != nil {
		return Posted{}, err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	// Displays that were deleted since are dropped, so the map stays as
	// small as the list of displays.
	for id := range b.current {
		if !slices.Contains(known, id) {
			delete(b.current, id)
		}
	}
	for _, id := range posted.Displays {
		b.current[id] = posted.Announcement
	}
	return posted, nil
}

// For returns the announcement displayID shows at now, or nil.
func (b *Board) For(displayID string, now time.Time) *Announcement {
	b.mu.Lock()
	defer b.mu.Unlock()
	a, ok := b.current[displayID]
	if !ok {
		return nil
	}
	if !now.Before(a.ExpiresAt) {
		delete(b.current, displayID)
		return nil
	}
	return &a
}

func validate(req Request, known []string, now time.Time) (Posted, error) {
	var errs validation.Collector
	title := strings.TrimSpace(req.Title)
	message := strings.TrimSpace(req.Message)
	checkText(&errs, "title", title, 0, maxTitleLength)
	checkText(&errs, "message", message, 1, maxMessageLength)

	seconds := DefaultSeconds
	if req.Seconds != nil {
		seconds = *req.Seconds
	}
	if seconds < MinSeconds || seconds > MaxSeconds {
		errs.Add("seconds", "use a whole number from 5 to 3600")
	}
	targets := targetDisplays(&errs, req.Displays, known)
	if err := errs.Err(); err != nil {
		return Posted{}, err
	}
	return Posted{
		Announcement: Announcement{Title: title, Message: message, ExpiresAt: now.Add(time.Duration(seconds) * time.Second)},
		Displays:     targets,
	}, nil
}

func checkText(errs *validation.Collector, field, text string, minLength, maxLength int) {
	if n := utf8.RuneCountInString(text); n < minLength || n > maxLength {
		if minLength == 0 {
			errs.Add(field, fmt.Sprintf("use at most %d characters", maxLength))
		} else {
			errs.Add(field, fmt.Sprintf("use %d to %d characters", minLength, maxLength))
		}
		return
	}
	if validation.HasControlCharacter(text) {
		errs.Add(field, "use plain text without line breaks or control characters")
	}
}

// targetDisplays resolves the displays to post to: every known display when
// none are named, else the named ones without duplicates.
func targetDisplays(errs *validation.Collector, named, known []string) []string {
	if named == nil {
		if len(known) == 0 {
			errs.Add("displays", "there are no displays yet; add one in the web interface")
		}
		return slices.Clone(known)
	}
	if len(named) == 0 {
		errs.Add("displays", "name at least one display, or leave this out to use all of them")
		return nil
	}
	var targets []string
	for _, id := range named {
		if !slices.Contains(known, id) {
			errs.Add("displays", fmt.Sprintf("unknown display %q", shorten(id)))
			return nil
		}
		if !slices.Contains(targets, id) {
			targets = append(targets, id)
		}
	}
	return targets
}

// shorten keeps an ID from a sender short in an error message.
func shorten(s string) string {
	const maxRunes = 32
	if runes := []rune(s); len(runes) > maxRunes {
		return string(runes[:maxRunes]) + "…"
	}
	return s
}
