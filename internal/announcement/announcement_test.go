package announcement

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stijnvandepol/gdash/internal/validation"
)

var (
	now   = time.Date(2026, 10, 9, 19, 0, 0, 0, time.UTC)
	known = []string{"kitchen", "hall", "office"}
)

func seconds(n int) *int { return &n }

func TestPostDefaultsAndTrimming(t *testing.T) {
	b := NewBoard(func() time.Time { return now })
	posted, err := b.Post(Request{Title: " Laundry ", Message: "  The washing machine is done "}, known, now)
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	if posted.Title != "Laundry" || posted.Message != "The washing machine is done" {
		t.Fatalf("texts = %q / %q, want them trimmed", posted.Title, posted.Message)
	}
	if want := now.Add(DefaultSeconds * time.Second); !posted.ExpiresAt.Equal(want) {
		t.Fatalf("ExpiresAt = %v, want %v", posted.ExpiresAt, want)
	}
	if !slices.Equal(posted.Displays, known) {
		t.Fatalf("Displays = %v, want every display", posted.Displays)
	}
}

func TestPostValidation(t *testing.T) {
	tests := []struct {
		name  string
		req   Request
		known []string
		field string
	}{
		{"message missing", Request{}, known, "message"},
		{"message blank", Request{Message: "   "}, known, "message"},
		{"message too long", Request{Message: strings.Repeat("a", maxMessageLength+1)}, known, "message"},
		{"message with a line break", Request{Message: "one\ntwo"}, known, "message"},
		{"message with an escape", Request{Message: "hi \x1b[31m"}, known, "message"},
		{"title too long", Request{Title: strings.Repeat("a", maxTitleLength+1), Message: "x"}, known, "title"},
		{"title with a tab", Request{Title: "a\tb", Message: "x"}, known, "title"},
		{"too few seconds", Request{Message: "x", Seconds: seconds(MinSeconds - 1)}, known, "seconds"},
		{"zero seconds", Request{Message: "x", Seconds: seconds(0)}, known, "seconds"},
		{"too many seconds", Request{Message: "x", Seconds: seconds(MaxSeconds + 1)}, known, "seconds"},
		{"unknown display", Request{Message: "x", Displays: []string{"kitchen", "garage"}}, known, "displays"},
		{"empty display list", Request{Message: "x", Displays: []string{}}, known, "displays"},
		{"no displays exist", Request{Message: "x"}, nil, "displays"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBoard(func() time.Time { return now })
			_, err := b.Post(tt.req, tt.known, now)
			fields := validation.Fields(err)
			if len(fields) != 1 || fields[0].Field != tt.field {
				t.Fatalf("err = %v, want one error on %s", err, tt.field)
			}
			if got := b.For("kitchen", now); got != nil {
				t.Fatalf("an invalid request is shown: %+v", got)
			}
		})
	}
}

func TestPostReportsEveryInvalidField(t *testing.T) {
	b := NewBoard(func() time.Time { return now })
	_, err := b.Post(Request{Seconds: seconds(1), Displays: []string{"garage"}}, known, now)
	if fields := validation.Fields(err); len(fields) != 3 {
		t.Fatalf("fields = %v, want message, seconds and displays", fields)
	}
}

func TestPostAcceptsTheBounds(t *testing.T) {
	b := NewBoard(func() time.Time { return now })
	for _, s := range []int{MinSeconds, MaxSeconds} {
		req := Request{Message: strings.Repeat("é", maxMessageLength), Title: strings.Repeat("é", maxTitleLength), Seconds: seconds(s)}
		if _, err := b.Post(req, known, now); err != nil {
			t.Errorf("%d seconds: %v", s, err)
		}
	}
}

func TestPostToChosenDisplays(t *testing.T) {
	b := NewBoard(func() time.Time { return now })
	posted, err := b.Post(Request{Message: "Dinner", Displays: []string{"hall", "kitchen", "hall"}}, known, now)
	if err != nil || !slices.Equal(posted.Displays, []string{"hall", "kitchen"}) {
		t.Fatalf("Post = %+v, %v; want hall and kitchen once", posted, err)
	}
	if b.For("hall", now) == nil || b.For("kitchen", now) == nil {
		t.Fatal("a chosen display shows nothing")
	}
	if got := b.For("office", now); got != nil {
		t.Fatalf("a display that was not chosen shows %+v", got)
	}
}

func TestNewerAnnouncementReplacesOlderOnItsDisplays(t *testing.T) {
	b := NewBoard(func() time.Time { return now })
	if _, err := b.Post(Request{Message: "for everyone"}, known, now); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Post(Request{Message: "for the kitchen", Displays: []string{"kitchen"}}, known, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	later := now.Add(2 * time.Second)
	if got := b.For("kitchen", later); got == nil || got.Message != "for the kitchen" {
		t.Fatalf("kitchen shows %+v, want the newer message", got)
	}
	if got := b.For("hall", later); got == nil || got.Message != "for everyone" {
		t.Fatalf("hall shows %+v, want the message for everyone", got)
	}
}

func TestAnnouncementExpires(t *testing.T) {
	b := NewBoard(func() time.Time { return now })
	if _, err := b.Post(Request{Message: "soon gone", Seconds: seconds(10)}, known, now); err != nil {
		t.Fatal(err)
	}
	if b.For("kitchen", now.Add(9*time.Second)) == nil {
		t.Fatal("gone before its time")
	}
	if got := b.For("kitchen", now.Add(10*time.Second)); got != nil {
		t.Fatalf("still shown at its expiry: %+v", got)
	}
}

func TestDeletedDisplaysAreForgotten(t *testing.T) {
	b := NewBoard(func() time.Time { return now })
	if _, err := b.Post(Request{Message: "a"}, known, now); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Post(Request{Message: "b"}, []string{"kitchen"}, now); err != nil {
		t.Fatal(err)
	}
	if got := b.For("hall", now); got != nil {
		t.Fatalf("a deleted display keeps %+v", got)
	}
}

func TestAllowLimitsASender(t *testing.T) {
	clock := now
	b := NewBoard(func() time.Time { return clock })
	for range maxPosts {
		if !b.Allow("token|1") {
			t.Fatal("refused below the limit")
		}
	}
	if b.Allow("token|1") {
		t.Fatal("allowed above the limit")
	}
	if !b.Allow("token|2") {
		t.Fatal("one sender's posts count against another")
	}
	clock = clock.Add(postWindow + time.Second)
	if !b.Allow("token|1") {
		t.Fatal("still refused after the window")
	}
}
