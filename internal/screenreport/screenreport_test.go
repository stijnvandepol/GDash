package screenreport

import (
	"strings"
	"testing"
	"time"

	"github.com/stijnvandepol/gdash/internal/validation"
)

var start = time.Date(2026, 10, 14, 19, 0, 0, 0, time.UTC)

type testClock struct{ t time.Time }

func (c *testClock) now() time.Time { return c.t }

func intPtr(n int) *int { return &n }

// chromecast is a report as a Chromecast HD with an older Cast runtime sends it.
func chromecast() Report {
	return Report{
		Reason:           ReasonConnected,
		UserAgent:        "Mozilla/5.0 (Linux; Android 12.0; Build/STTL.240206.002) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/87.0.4280.141 Safari/537.36 CrKey/1.56.500000 DeviceType/AndroidTV",
		Viewport:         Viewport{Width: 1280, Height: 720},
		DevicePixelRatio: 1.5,
		StageScale:       0.6667,
		Features:         Features{AspectRatio: false, FlexGap: true, Inset: true, LineClamp: true},
		Compat:           true,
		FontsLoaded:      true,
		FirstRenderMs:    intPtr(2400),
		Errors:           []ScriptError{{Message: "TypeError: x is not a function", Source: "/assets/App-abc.js:1:200"}},
		Visibility:       []VisibilityEvent{{State: "blur", At: start}, {State: "hidden", At: start.Add(time.Second)}},
	}
}

func TestValidateAcceptsAReport(t *testing.T) {
	if err := Validate(chromecast()); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	r := chromecast()
	r.FirstRenderMs = nil
	r.Errors = nil
	r.Visibility = nil
	r.Reason = ReasonHidden
	if err := Validate(r); err != nil {
		t.Fatalf("Validate of a hidden report without a render time, errors and changes: %v", err)
	}
}

func TestValidateRejects(t *testing.T) {
	tests := []struct {
		name   string
		change func(*Report)
		field  string
	}{
		{"no reason", func(r *Report) { r.Reason = "" }, "reason"},
		{"an unknown reason", func(r *Report) { r.Reason = "bored" }, "reason"},
		{"too many visibility changes", func(r *Report) { r.Visibility = make([]VisibilityEvent, MaxVisibilityEvents+1) }, "visibility"},
		{"an unknown visibility state", func(r *Report) { r.Visibility = []VisibilityEvent{{State: "minimized", At: start}} }, "visibility[0]"},
		{"a visibility change without a time", func(r *Report) { r.Visibility = []VisibilityEvent{{State: "hidden"}} }, "visibility[0]"},
		{"no user agent", func(r *Report) { r.UserAgent = "" }, "userAgent"},
		{"long user agent", func(r *Report) { r.UserAgent = strings.Repeat("a", maxUserAgentLength+1) }, "userAgent"},
		{"user agent with a line break", func(r *Report) { r.UserAgent = "Chrome\nEvil" }, "userAgent"},
		{"zero width", func(r *Report) { r.Viewport.Width = 0 }, "viewport"},
		{"huge height", func(r *Report) { r.Viewport.Height = maxViewportPixels + 1 }, "viewport"},
		{"zero pixel ratio", func(r *Report) { r.DevicePixelRatio = 0 }, "devicePixelRatio"},
		{"negative stage scale", func(r *Report) { r.StageScale = -1 }, "stageScale"},
		{"huge stage scale", func(r *Report) { r.StageScale = maxStageScale + 1 }, "stageScale"},
		{"negative render time", func(r *Report) { r.FirstRenderMs = intPtr(-1) }, "firstRenderMs"},
		{"render time over an hour", func(r *Report) { r.FirstRenderMs = intPtr(maxFirstRenderMs + 1) }, "firstRenderMs"},
		{"too many errors", func(r *Report) { r.Errors = make([]ScriptError, MaxErrors+1) }, "errors"},
		{"empty error message", func(r *Report) { r.Errors = []ScriptError{{Message: ""}} }, "errors[0].message"},
		{"long error message", func(r *Report) { r.Errors = []ScriptError{{Message: strings.Repeat("e", maxMessageLength+1)}} }, "errors[0].message"},
		{"error with a stack", func(r *Report) { r.Errors = []ScriptError{{Message: "boom\n  at x"}} }, "errors[0].message"},
		{"long error source", func(r *Report) {
			r.Errors = []ScriptError{{Message: "x", Source: strings.Repeat("s", maxSourceLength+1)}}
		}, "errors[0].source"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := chromecast()
			tt.change(&r)
			fields := validation.Fields(Validate(r))
			if len(fields) != 1 || fields[0].Field != tt.field {
				t.Fatalf("fields = %+v, want one error for %s", fields, tt.field)
			}
		})
	}
}

func TestBoardKeepsTheLastReportPerScreen(t *testing.T) {
	clock := &testClock{t: start}
	b := NewBoard(clock.now)
	first := chromecast()
	b.Put("kitchen", "cast", "cast", first)
	clock.t = clock.t.Add(time.Minute)
	second := chromecast()
	second.Viewport = Viewport{Width: 1920, Height: 1080}
	b.Put("kitchen", "cast", "cast", second)
	clock.t = clock.t.Add(time.Minute)
	tablet := chromecast()
	tablet.UserAgent = "Mozilla/5.0 (iPad) Safari/605.1.15"
	b.Put("kitchen", "hash-1", "screen", tablet)

	got := b.For("kitchen")
	if len(got) != 2 {
		t.Fatalf("For = %d reports, want 2", len(got))
	}
	if got[0].UserAgent != tablet.UserAgent || got[0].Screen != "screen" || !got[0].ReceivedAt.Equal(start.Add(2*time.Minute)) {
		t.Fatalf("newest = %+v, want the tablet's report first", got[0])
	}
	if got[1].Viewport.Width != 1920 || got[1].Screen != "cast" {
		t.Fatalf("cast report = %+v, want the TV's second report", got[1])
	}
	if len(b.For("office")) != 0 {
		t.Fatal("another display has reports")
	}
}

func TestBoardKeepsTheNewestScreens(t *testing.T) {
	clock := &testClock{t: start}
	b := NewBoard(clock.now)
	for i := range maxScreensPerDisplay + 2 {
		clock.t = clock.t.Add(time.Second)
		b.Put("kitchen", string(rune('a'+i)), "screen", chromecast())
	}
	got := b.For("kitchen")
	if len(got) != maxScreensPerDisplay {
		t.Fatalf("For = %d reports, want %d", len(got), maxScreensPerDisplay)
	}
	if want := start.Add(3 * time.Second); !got[len(got)-1].ReceivedAt.Equal(want) {
		t.Fatalf("oldest kept report from %v, want %v: the two oldest are dropped", got[len(got)-1].ReceivedAt, want)
	}
}

func TestBoardForgets(t *testing.T) {
	b := NewBoard((&testClock{t: start}).now)
	b.Put("kitchen", "cast", "cast", chromecast())
	b.Put("kitchen", "hash-1", "screen", chromecast())
	b.ForgetScreens("kitchen", "screen")
	if got := b.For("kitchen"); len(got) != 1 || got[0].Screen != "cast" {
		t.Fatalf("after unpairing the screens: %+v, want only the TV", got)
	}
	b.Forget("kitchen")
	if got := b.For("kitchen"); len(got) != 0 {
		t.Fatalf("after deleting the display: %+v, want nothing", got)
	}
}

func TestBoardDoesNotShareErrors(t *testing.T) {
	b := NewBoard((&testClock{t: start}).now)
	r := chromecast()
	b.Put("kitchen", "cast", "cast", r)
	r.Errors[0].Message = "changed by the caller"
	got := b.For("kitchen")
	got[0].Errors[0].Message = "changed by a reader"
	if msg := b.For("kitchen")[0].Errors[0].Message; msg != "TypeError: x is not a function" {
		t.Fatalf("stored error message = %q, want the original", msg)
	}
}

func TestAllowLimitsEachScreen(t *testing.T) {
	clock := &testClock{t: start}
	b := NewBoard(clock.now)
	for i := range maxReports {
		if !b.Allow("kitchen", "cast") {
			t.Fatalf("report %d refused, want %d allowed", i+1, maxReports)
		}
	}
	if b.Allow("kitchen", "cast") {
		t.Fatal("a report over the limit was allowed")
	}
	if !b.Allow("office", "cast") || !b.Allow("kitchen", "hash-1") {
		t.Fatal("another screen was limited along with the first")
	}
	clock.t = clock.t.Add(reportWindow + time.Second)
	if !b.Allow("kitchen", "cast") {
		t.Fatal("a report after the window was refused")
	}
}
