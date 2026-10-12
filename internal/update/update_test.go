package update

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

type fakeGitHub struct {
	srv      *httptest.Server
	requests atomic.Int32
	// status and body answer every request.
	status int
	body   string
	// header is the last request's headers.
	header atomic.Pointer[http.Header]
	query  atomic.Pointer[string]
}

func newFakeGitHub(t *testing.T, status int, body string) *fakeGitHub {
	t.Helper()
	f := &fakeGitHub{status: status, body: body}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.requests.Add(1)
		h := r.Header.Clone()
		f.header.Store(&h)
		q := r.URL.RawQuery
		f.query.Store(&q)
		w.WriteHeader(f.status)
		_, _ = io.WriteString(w, f.body)
	}))
	t.Cleanup(f.srv.Close)
	return f
}

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func newChecker(f *fakeGitHub, current string, enabled *bool) (*Checker, *clock) {
	c := &clock{t: time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)}
	return &Checker{
		URL:       f.srv.URL,
		HTTP:      f.srv.Client(),
		UserAgent: "GDash/" + current,
		Current:   current,
		Enabled:   func(context.Context) (bool, error) { return *enabled, nil },
		Now:       c.now,
		Logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
	}, c
}

func TestNewerReleaseIsOffered(t *testing.T) {
	f := newFakeGitHub(t, http.StatusOK, `{"tag_name":"v0.8.0","name":"0.8.0","draft":false,"prerelease":false,"body":"Notes"}`)
	on := true
	c, _ := newChecker(f, "v0.7.0", &on)
	if got := c.Available(); got != nil {
		t.Fatalf("Available before a check = %+v, want nil", got)
	}
	if err := c.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	got := c.Available()
	if got == nil || got.Version != "0.8.0" || got.URL != "https://github.com/stijnvandepol/gdash/releases/tag/v0.8.0" {
		t.Fatalf("Available = %+v", got)
	}
	// Only the version leaves the server, in the User-Agent; nothing in the address.
	if h := *f.header.Load(); h.Get("User-Agent") != "GDash/v0.7.0" || h.Get("Cookie") != "" || h.Get("Authorization") != "" {
		t.Fatalf("request headers = %v", h)
	}
	if q := *f.query.Load(); q != "" {
		t.Fatalf("query = %q, want none", q)
	}
}

func TestNoNoticeForTheSameOrAnOlderRelease(t *testing.T) {
	for _, tt := range []struct{ current, latest string }{
		{"v0.8.0", "v0.8.0"},
		{"0.8.0", "v0.8.0"},
		{"v0.9.1", "v0.8.0"},
		{"v1.0.0", "v0.10.0"},
	} {
		f := newFakeGitHub(t, http.StatusOK, `{"tag_name":"`+tt.latest+`"}`)
		on := true
		c, _ := newChecker(f, tt.current, &on)
		if err := c.Check(context.Background()); err != nil {
			t.Fatal(err)
		}
		if got := c.Available(); got != nil {
			t.Errorf("running %s, latest %s: Available = %+v, want nil", tt.current, tt.latest, got)
		}
	}
}

func TestVersionsCompareByNumberNotText(t *testing.T) {
	f := newFakeGitHub(t, http.StatusOK, `{"tag_name":"v0.10.0"}`)
	on := true
	c, _ := newChecker(f, "v0.9.0", &on)
	if err := c.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := c.Available(); got == nil || got.Version != "0.10.0" {
		t.Fatalf("Available = %+v, want 0.10.0", got)
	}
}

func TestReleaseIsNewerThanItsPreRelease(t *testing.T) {
	f := newFakeGitHub(t, http.StatusOK, `{"tag_name":"v0.8.0"}`)
	on := true
	c, _ := newChecker(f, "v0.8.0-rc.1", &on)
	if err := c.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := c.Available(); got == nil || got.Version != "0.8.0" {
		t.Fatalf("Available = %+v, want 0.8.0", got)
	}
}

func TestDraftsAndPreReleasesAreIgnored(t *testing.T) {
	for _, body := range []string{
		`{"tag_name":"v0.9.0","draft":true}`,
		`{"tag_name":"v0.9.0","prerelease":true}`,
		`{"tag_name":"v0.9.0-beta.1"}`,
	} {
		f := newFakeGitHub(t, http.StatusOK, body)
		on := true
		c, _ := newChecker(f, "v0.8.0", &on)
		if err := c.Check(context.Background()); err != nil {
			t.Fatalf("%s: %v", body, err)
		}
		if got := c.Available(); got != nil {
			t.Errorf("%s: Available = %+v, want nil", body, got)
		}
	}
}

func TestFailuresAreErrorsWithoutANotice(t *testing.T) {
	for _, tt := range []struct {
		status int
		body   string
	}{
		{http.StatusForbidden, `{"message":"API rate limit exceeded"}`},
		{http.StatusTooManyRequests, ``},
		{http.StatusNotFound, `{"message":"Not Found"}`},
		{http.StatusOK, `not json`},
		{http.StatusOK, `{"tag_name":"latest"}`},
	} {
		f := newFakeGitHub(t, tt.status, tt.body)
		on := true
		c, _ := newChecker(f, "v0.8.0", &on)
		if err := c.Check(context.Background()); err == nil {
			t.Errorf("%d %s: no error", tt.status, tt.body)
		}
		if got := c.Available(); got != nil {
			t.Errorf("%d %s: Available = %+v, want nil", tt.status, tt.body, got)
		}
	}
}

func TestTurnedOffMeansNoRequestAndNoNotice(t *testing.T) {
	f := newFakeGitHub(t, http.StatusOK, `{"tag_name":"v0.9.0"}`)
	on := true
	c, clk := newChecker(f, "v0.8.0", &on)
	if err := c.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	on = false
	clk.t = clk.t.Add(2 * time.Hour)
	if err := c.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := c.Available(); got != nil {
		t.Fatalf("Available with the check off = %+v", got)
	}
	if n := f.requests.Load(); n != 1 {
		t.Fatalf("requests = %d, want 1: none while the check is off", n)
	}

	c.Off = true
	on = true
	if err := c.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got, n := c.Available(), f.requests.Load(); got != nil || n != 1 {
		t.Fatalf("with GDASH_UPDATE_CHECK=off: Available = %+v, requests = %d", got, n)
	}
}

func TestTurningOnAgainSoonAsksNoAgain(t *testing.T) {
	f := newFakeGitHub(t, http.StatusOK, `{"tag_name":"v0.9.0"}`)
	on := true
	c, clk := newChecker(f, "v0.8.0", &on)
	for range 3 {
		if err := c.Check(context.Background()); err != nil {
			t.Fatal(err)
		}
		clk.t = clk.t.Add(10 * time.Minute)
	}
	if n := f.requests.Load(); n != 1 {
		t.Fatalf("requests = %d within the hour, want 1", n)
	}
	if got := c.Available(); got == nil {
		t.Fatal("the release found earlier is no longer offered")
	}
	clk.t = clk.t.Add(time.Hour)
	if err := c.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	if n := f.requests.Load(); n != 2 {
		t.Fatalf("requests = %d after the hour, want 2", n)
	}
}

func TestDevelopmentBuildNeverAsks(t *testing.T) {
	f := newFakeGitHub(t, http.StatusOK, `{"tag_name":"v0.9.0"}`)
	on := true
	c, _ := newChecker(f, "dev-0123456789ab", &on)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	c.Run(ctx) // returns at once for a development build
	if n := f.requests.Load(); n != 0 {
		t.Fatalf("requests = %d, want none", n)
	}
	if got := c.Available(); got != nil {
		t.Fatalf("Available = %+v", got)
	}
}

func TestRunChecksAtOnceAndAfterChanged(t *testing.T) {
	f := newFakeGitHub(t, http.StatusOK, `{"tag_name":"v0.9.0"}`)
	off := false
	c, _ := newChecker(f, "v0.8.0", &off)
	var enabled atomic.Bool
	c.Enabled = func(context.Context) (bool, error) { return enabled.Load(), nil }
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		c.Run(ctx)
		close(done)
	}()
	defer func() {
		cancel()
		<-done
	}()
	enabled.Store(true)
	c.Changed()
	deadline := time.Now().Add(2 * time.Second)
	for c.Available() == nil {
		if time.Now().After(deadline) {
			t.Fatal("no release offered after turning the check on")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if n := f.requests.Load(); n != 1 {
		t.Fatalf("requests = %d, want 1", n)
	}
}

func TestParseVersion(t *testing.T) {
	for _, s := range []string{"dev", "dev-abc", "v1.2", "1.2.3.4", "v01.2.3", "", "vv1.2.3"} {
		if _, ok := parseVersion(s); ok {
			t.Errorf("parseVersion(%q) accepted", s)
		}
	}
	v, ok := parseVersion("v1.12.3-rc.1+build.5")
	if !ok || v.String() != "1.12.3-rc.1" {
		t.Fatalf("parseVersion = %v, %v", v, ok)
	}
}
