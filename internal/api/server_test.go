package api

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stijnvandepol/gdash/internal/announcement"
	"github.com/stijnvandepol/gdash/internal/apitoken"
	"github.com/stijnvandepol/gdash/internal/auth"
	"github.com/stijnvandepol/gdash/internal/calendar"
	"github.com/stijnvandepol/gdash/internal/camera"
	"github.com/stijnvandepol/gdash/internal/cast"
	"github.com/stijnvandepol/gdash/internal/casting"
	"github.com/stijnvandepol/gdash/internal/custom"
	"github.com/stijnvandepol/gdash/internal/datahub"
	"github.com/stijnvandepol/gdash/internal/display"
	"github.com/stijnvandepol/gdash/internal/homeassistant"
	"github.com/stijnvandepol/gdash/internal/homey"
	"github.com/stijnvandepol/gdash/internal/immich"
	"github.com/stijnvandepol/gdash/internal/outbound"
	"github.com/stijnvandepol/gdash/internal/p2000"
	"github.com/stijnvandepol/gdash/internal/pairing"
	"github.com/stijnvandepol/gdash/internal/photos"
	"github.com/stijnvandepol/gdash/internal/screenreport"
	"github.com/stijnvandepol/gdash/internal/secret"
	"github.com/stijnvandepol/gdash/internal/settings"
	"github.com/stijnvandepol/gdash/internal/store"
	"github.com/stijnvandepol/gdash/internal/synology"
	"github.com/stijnvandepol/gdash/internal/update"
	"github.com/stijnvandepol/gdash/internal/weather"
	"github.com/stijnvandepol/gdash/internal/webpage"
)

const testSetupToken = "test-setup-token"

type fixedSubscriber []byte

func (f fixedSubscriber) Subscribe(ctx context.Context, _ string) <-chan []byte {
	ch := make(chan []byte, 1)
	ch <- f
	go func() { <-ctx.Done(); close(ch) }()
	return ch
}

// fakeSources stands in for the place search and the data hub. Tests change
// its answers while the server runs, hence the lock.
type fakeSources struct {
	mu      sync.Mutex
	places  []weather.Place
	err     error
	entries map[string]datahub.Entry
	cast    map[string]casting.Status
	// takeOvers are the displays whose TV an admin took back.
	takeOvers []string
	// hidden and shown are the displays whose TV reported its page hidden
	// or on screen again.
	hidden []string
	shown  []string
	now    func() time.Time
}

// ScreenHidden plays the cast manager taking a hidden report.
func (f *fakeSources) ScreenHidden(displayID string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.hidden = append(f.hidden, displayID)
	return true
}

func (f *fakeSources) hiddenDisplays() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.hidden)
}

// FindPlace plays alarmeringen.nl for the P2000 place test: it knows
// Nuenen, a quiet Stilledorp, and fails for Storing.
func (f *fakeSources) FindPlace(_ context.Context, name string) (p2000.Place, error) {
	switch name {
	case "Nuenen":
		return p2000.Place{Name: "Nuenen", Slug: "nuenen", Region: "brabant-zuidoost"}, nil
	case "Stilledorp":
		return p2000.Place{}, p2000.ErrPlaceWithoutAlerts
	case "Storing":
		return p2000.Place{}, errors.New("connection refused")
	}
	return p2000.Place{}, p2000.ErrUnknownPlace
}

// ScreenShown plays the cast manager taking a visible report.
func (f *fakeSources) ScreenShown(displayID string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.shown = append(f.shown, displayID)
	return true
}

func (f *fakeSources) shownDisplays() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.shown)
}

// receiveCustom plays the data hub taking a pushed answer: two intervals
// of a minute and 30 seconds fresh.
func (f *fakeSources) receiveCustom(sourceID int64, answer json.RawMessage) error {
	f.put(map[string]json.RawMessage{custom.SourceKey(sourceID): answer}, custom.PushFreshFor(time.Minute))
	return nil
}

// receiveHomey plays the data hub taking a Homey snapshot: fresh for three
// minutes, as cmd/gdash sets it up.
func (f *fakeSources) receiveHomey(snapshot homey.Snapshot) error {
	values, err := display.HomeySnapshotData(snapshot)
	if err != nil {
		return err
	}
	f.put(values, 3*time.Minute)
	return nil
}

func (f *fakeSources) put(values map[string]json.RawMessage, freshFor time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	// A new map, as the hub hands out copies: a handler may still read
	// the old one.
	f.entries = maps.Clone(f.entries)
	if f.entries == nil {
		f.entries = map[string]datahub.Entry{}
	}
	at := f.now()
	for key, value := range values {
		f.entries[key] = datahub.Entry{Value: value, FetchedAt: at, StaleAfter: at.Add(freshFor)}
	}
}

func (f *fakeSources) Search(context.Context, string) ([]weather.Place, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.places, f.err
}

func (f *fakeSources) Entries() map[string]datahub.Entry {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.entries
}

func (f *fakeSources) CastStatus() map[string]casting.Status {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.cast
}

// TakeOverCast plays the cast manager: a display with a status has a
// running keeper.
func (f *fakeSources) TakeOverCast(displayID string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.cast[displayID]; !ok {
		return false
	}
	f.takeOvers = append(f.takeOvers, displayID)
	return true
}

func (f *fakeSources) set(places []weather.Place, err error, entries map[string]datahub.Entry) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.places, f.err, f.entries = places, err, entries
}

// testReadTimeout is the test server's read timeout.
// It must leave room for an 8 MB photo upload under the race detector.
const testReadTimeout = 2 * time.Second

type testEnv struct {
	srv     *httptest.Server
	client  *http.Client
	changes *atomic.Int32
	// settingsChanges counts SettingsChanged calls.
	settingsChanges *atomic.Int32
	pairing         *pairing.Service
	store           *store.Store
	sources         *fakeSources
	stops           *fakeStops
	artwork         *fakeArtwork
	board           *announcement.Board
	// announced counts AnnouncementsChanged calls.
	announced *atomic.Int32
	// logs is everything the server logged.
	logs *syncBuffer
	// updates answers from a fake GitHub that names v1.3.0 the latest
	// release; nothing checks until a test calls Check.
	updates *update.Checker
	// elapsed is how far the server's clock moved on.
	elapsed *atomic.Int64
	// cameraClock is the clock of the camera keeper, which tests move.
	cameraClock *movableClock
	// traffic stands in for the ANWB request guard.
	traffic *fakeTrafficGuard
}

// advance moves the server's clock on by d.
func (e *testEnv) advance(d time.Duration) { e.elapsed.Add(int64(d)) }

func newTestEnv(t *testing.T, discover func(context.Context) ([]cast.Device, error)) *testEnv {
	t.Helper()
	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "gdash.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	box, err := secret.LoadOrCreateKey(filepath.Join(t.TempDir(), "secret.key"))
	if err != nil {
		t.Fatal(err)
	}
	authService, err := auth.NewService(st, box, testSetupToken)
	if err != nil {
		t.Fatal(err)
	}
	changes, settingsChanges, announced := &atomic.Int32{}, &atomic.Int32{}, &atomic.Int32{}
	pairingService := pairing.New(st)
	logs := &syncBuffer{}
	logger := slog.New(slog.NewTextHandler(logs, nil))
	// The clock stands still unless a test moves it on with advance.
	elapsed := &atomic.Int64{}
	now := func() time.Time {
		return time.Date(2026, 10, 8, 17, 0, 0, 0, time.UTC).Add(time.Duration(elapsed.Load()))
	}
	sources := &fakeSources{now: now}
	stops := &fakeStops{}
	artwork := &fakeArtwork{}
	trafficGuard := &fakeTrafficGuard{}
	board := announcement.NewBoard(now)
	album, err := photos.NewStore(filepath.Join(t.TempDir(), "photos"))
	if err != nil {
		t.Fatal(err)
	}
	settingsService := &settings.Service{Store: st, Box: box, Now: now, Logger: logger}
	cameraClock := &movableClock{now: now()}
	releases := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"tag_name":"v1.3.0","draft":false,"prerelease":false}`)
	}))
	t.Cleanup(releases.Close)
	updates := &update.Checker{
		URL: releases.URL, HTTP: releases.Client(), UserAgent: "GDash/v1.2.3", Current: "v1.2.3",
		Enabled: settingsService.UpdateCheck, Now: now, Logger: logger,
	}
	s := &Server{
		Snapshots: fixedSubscriber(`{"type":"snapshot","version":1}`),
		Auth:      authService,
		Pairing:   pairingService,
		APITokens: apitoken.New(st),
		Store:     st,
		Web: fstest.MapFS{
			"admin/index.html":    {Data: []byte("<html>admin</html>")},
			"display/index.html":  {Data: []byte("<html>display</html>")},
			"receiver/index.html": {Data: []byte("<html>receiver</html>")},
			"preview/index.html":  {Data: []byte("<html>preview</html>")},
			"assets/app.js":       {Data: []byte("console.log(1)")},
		},
		Discover:          discover,
		DisplaysChanged:   func() { changes.Add(1) },
		DashboardsChanged: func() { changes.Add(1) },
		Settings:          settingsService,
		Places:            sources,
		Stops:             stops,
		P2000Places:       sources,
		Traffic:           trafficGuard,
		KNMI:              fakeKNMI{},
		Calendars:         &calendar.Client{HTTP: outbound.NewClient(5 * time.Second), Location: time.UTC, Now: now},
		CustomFetcher:     &custom.Fetcher{HTTP: outbound.NewClient(5 * time.Second), Box: box, Now: now},
		WebPageChecker:    &webpage.Checker{HTTP: outbound.NewClient(5 * time.Second)},
		ReceiveCustom:     sources.receiveCustom,
		ReceiveHomey:      sources.receiveHomey,
		HomeAssistant:     &homeassistant.Client{HTTP: &http.Client{Timeout: 5 * time.Second}},
		Homey:             &homey.Client{HTTP: &http.Client{Timeout: 5 * time.Second}},
		Immich:            &immich.Client{HTTP: outbound.NewClient(5 * time.Second)},
		Synology:          &synology.Client{},
		Cameras: &camera.Keeper{
			Fetcher: &camera.Fetcher{HTTP: outbound.NewClient(5 * time.Second)}, Now: cameraClock.Now, Logger: logger,
		},
		SettingsChanged:      func() { settingsChanges.Add(1) },
		Announcements:        board,
		AnnouncementsChanged: func(time.Time) { announced.Add(1) },
		ScreenReports:        screenreport.NewBoard(now),
		ScreenHidden:         sources.ScreenHidden,
		ScreenShown:          sources.ScreenShown,
		SourceStatus:         sources.Entries,
		Data:                 sources.Entries,
		Version:              "v1.2.3",
		Updates:              updates,
		CastStatus:           sources.CastStatus,
		TakeOverCast:         sources.TakeOverCast,
		Photos:               album,
		Backup:               fakeBackup{},
		PhotosChanged:        func() { changes.Add(1) },
		Artwork:              artwork,
		SentData:             artwork.sentData,
		PingInterval:         50 * time.Millisecond,
		Now:                  now,
		Logger:               logger,
	}
	srv := httptest.NewUnstartedServer(s.Handler())
	// As in production (cmd/gdash), short enough to notice in tests.
	srv.Config.ReadTimeout = testReadTimeout
	srv.Start()
	t.Cleanup(srv.Close)
	jar, _ := cookiejar.New(nil)
	return &testEnv{
		srv: srv, client: &http.Client{Jar: jar}, changes: changes, settingsChanges: settingsChanges,
		pairing: pairingService, store: st, sources: sources, stops: stops, artwork: artwork, board: board, announced: announced, logs: logs,
		updates: updates, elapsed: elapsed, cameraClock: cameraClock, traffic: trafficGuard,
	}
}

// movableClock is a clock that tests move by hand.
type movableClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *movableClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *movableClock) add(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// do sends a request with a JSON body and returns status and body.
func (e *testEnv) do(t *testing.T, method, path, body string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(method, e.srv.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := e.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func (e *testEnv) setUp(t *testing.T) {
	t.Helper()
	status, body := e.do(t, "POST", "/api/setup", `{"setupToken":"`+testSetupToken+`","username":"stijn","password":"correct horse battery"}`)
	if status != http.StatusCreated {
		t.Fatalf("setup = %d %s", status, body)
	}
}

func TestDisplayPageHasStrictCSP(t *testing.T) {
	env := newTestEnv(t, nil)
	resp, err := http.Get(env.srv.URL + "/d/keuken")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if csp := resp.Header.Get("Content-Security-Policy"); !strings.Contains(csp, "default-src 'self'") {
		t.Fatalf("CSP = %q", csp)
	}
}

func TestAdminPageIsServedAtRoot(t *testing.T) {
	env := newTestEnv(t, nil)
	if status, body := env.do(t, "GET", "/", ""); status != http.StatusOK || body != "<html>admin</html>" {
		t.Fatalf("GET / = %d %q", status, body)
	}
	if status, _ := env.do(t, "GET", "/does-not-exist", ""); status != http.StatusNotFound {
		t.Fatalf("GET /does-not-exist = %d, want 404", status)
	}
}

func TestInvalidDisplayIDIsNotFound(t *testing.T) {
	env := newTestEnv(t, nil)
	for _, path := range []string{"/d/Keuken", "/d/..%2fetc", "/api/displays/a_b/events", "/d/" + strings.Repeat("a", 40)} {
		if status, _ := env.do(t, "GET", path, ""); status != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", path, status)
		}
	}
}

// readFirstEvent opens a display's event stream with client and returns the
// first lines up to the first data line, or the status when it is not 200.
func readFirstEvent(t *testing.T, client *http.Client, url string) (int, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, ""
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("Content-Type = %q", ct)
	}
	scanner := bufio.NewScanner(resp.Body)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if strings.HasPrefix(scanner.Text(), "data: ") {
			break
		}
	}
	return resp.StatusCode, strings.Join(lines, "\n")
}

const wantFirstEvent = "retry: 5000\n\nevent: snapshot\ndata: {\"type\":\"snapshot\",\"version\":1}"

// tv is a separate client, with its own cookies, acting as a screen.
func tv() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{Jar: jar}
}

func (e *testEnv) createKitchen(t *testing.T) {
	t.Helper()
	if status, body := e.do(t, "POST", "/api/displays", `{"id":"keuken","name":"Keuken"}`); status != http.StatusCreated {
		t.Fatalf("create display = %d %s", status, body)
	}
}

func post(t *testing.T, client *http.Client, url, body string) (int, string) {
	t.Helper()
	resp, err := client.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func TestSetupThenLoginLogout(t *testing.T) {
	env := newTestEnv(t, nil)
	if _, body := env.do(t, "GET", "/api/setup", ""); body != "{\"needed\":true,\"tokenRequired\":false}\n" {
		t.Fatalf("setup state before = %s", body)
	}
	if status, _ := env.do(t, "GET", "/api/me", ""); status != http.StatusUnauthorized {
		t.Fatalf("/api/me before setup = %d, want 401", status)
	}
	env.setUp(t)
	if status, body := env.do(t, "GET", "/api/me", ""); status != http.StatusOK || !strings.Contains(body, `"username":"stijn"`) {
		t.Fatalf("/api/me after setup = %d %s", status, body)
	}
	if status, _ := env.do(t, "POST", "/api/setup", `{"setupToken":"`+testSetupToken+`","username":"x","password":"another long password"}`); status != http.StatusConflict {
		t.Fatalf("second setup = %d, want 409", status)
	}

	if status, _ := env.do(t, "DELETE", "/api/sessions/current", ""); status != http.StatusNoContent {
		t.Fatalf("logout = %d", status)
	}
	if status, _ := env.do(t, "GET", "/api/me", ""); status != http.StatusUnauthorized {
		t.Fatalf("/api/me after logout = %d, want 401", status)
	}
	if status, _ := env.do(t, "POST", "/api/sessions", `{"username":"stijn","password":"wrong password!"}`); status != http.StatusUnauthorized {
		t.Fatalf("login with wrong password = %d, want 401", status)
	}
	if status, _ := env.do(t, "POST", "/api/sessions", `{"username":"stijn","password":"correct horse battery"}`); status != http.StatusCreated {
		t.Fatalf("login = %d", status)
	}
	if status, _ := env.do(t, "GET", "/api/me", ""); status != http.StatusOK {
		t.Fatalf("/api/me after login = %d", status)
	}
}

func TestSessionCookieFlags(t *testing.T) {
	env := newTestEnv(t, nil)
	resp, err := http.Post(env.srv.URL+"/api/setup", "application/json",
		strings.NewReader(`{"setupToken":"`+testSetupToken+`","username":"stijn","password":"correct horse battery"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	header := resp.Header.Get("Set-Cookie")
	for _, want := range []string{"gdash_session=", "HttpOnly", "SameSite=Lax", "Path=/"} {
		if !strings.Contains(header, want) {
			t.Errorf("Set-Cookie %q lacks %q", header, want)
		}
	}
}

func TestSetupWithoutTokenFromTheHomeNetwork(t *testing.T) {
	env := newTestEnv(t, nil)
	status, body := env.do(t, "POST", "/api/setup", `{"setupToken":"","username":"stijn","password":"correct horse battery"}`)
	if status != http.StatusCreated {
		t.Fatalf("setup without token from loopback = %d %s", status, body)
	}
	if _, body := env.do(t, "GET", "/api/setup", ""); body != "{\"needed\":false,\"tokenRequired\":false}\n" {
		t.Fatalf("setup state after = %s", body)
	}
}

func TestSetupThroughAnUntrustedProxyNeedsTheToken(t *testing.T) {
	env := newTestEnv(t, nil)
	// The peer is the loopback address, like a reverse proxy on the same
	// host, but the visitor could be anywhere.
	req, err := http.NewRequest("POST", env.srv.URL+"/api/setup", strings.NewReader(`{"setupToken":"","username":"stijn","password":"correct horse battery"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", "203.0.113.9")
	resp, err := env.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusForbidden || !strings.Contains(string(body), "invalid_setup_token") {
		t.Fatalf("setup without token through a proxy = %d %s", resp.StatusCode, body)
	}
}

func TestSetupRejectsWrongToken(t *testing.T) {
	env := newTestEnv(t, nil)
	status, body := env.do(t, "POST", "/api/setup", `{"setupToken":"guess","username":"stijn","password":"correct horse battery"}`)
	if status != http.StatusForbidden || !strings.Contains(body, "invalid_setup_token") {
		t.Fatalf("setup with wrong token = %d %s", status, body)
	}
}

func TestRequestBodyIsStrict(t *testing.T) {
	env := newTestEnv(t, nil)
	for _, body := range []string{`{"username":"stijn","passwrd":"x"}`, `not json`, `{"username":"a"}{"username":"b"}`} {
		if status, _ := env.do(t, "POST", "/api/sessions", body); status != http.StatusBadRequest {
			t.Errorf("POST /api/sessions %s = %d, want 400", body, status)
		}
	}
}

func TestCrossSiteRequestIsRejected(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	req, _ := http.NewRequest("POST", env.srv.URL+"/api/displays", strings.NewReader(`{"id":"keuken","name":"Keuken"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	resp, err := env.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("cross-site POST = %d, want 403", resp.StatusCode)
	}
}

func TestDisplayAPIRequiresLogin(t *testing.T) {
	env := newTestEnv(t, nil)
	for _, r := range []struct{ method, path, body string }{
		{"GET", "/api/displays", ""},
		{"POST", "/api/displays", `{"id":"keuken","name":"Keuken"}`},
		{"PATCH", "/api/displays/keuken", `{"name":"x"}`},
		{"DELETE", "/api/displays/keuken", ""},
		{"POST", "/api/displays/keuken/cast-takeovers", ""},
		{"GET", "/api/cast-devices", ""},
	} {
		if status, _ := env.do(t, r.method, r.path, r.body); status != http.StatusUnauthorized {
			t.Errorf("%s %s without login = %d, want 401", r.method, r.path, status)
		}
	}
}

func TestDisplayCRUD(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)

	status, body := env.do(t, "POST", "/api/displays", `{"id":"keuken","name":"Keuken TV","castDeviceId":"e542c41e4d94baa86e46f36dfdebf26e"}`)
	if status != http.StatusCreated || !strings.Contains(body, `"createdAt":"2026-10-08T17:00:00Z"`) {
		t.Fatalf("create = %d %s", status, body)
	}
	if status, _ := env.do(t, "POST", "/api/displays", `{"id":"keuken","name":"Dubbel"}`); status != http.StatusConflict {
		t.Fatalf("duplicate create = %d, want 409", status)
	}
	status, body = env.do(t, "POST", "/api/displays", `{"id":"Bad ID","name":""}`)
	if status != http.StatusUnprocessableEntity || !strings.Contains(body, `"field":"id"`) || !strings.Contains(body, `"field":"name"`) {
		t.Fatalf("invalid create = %d %s", status, body)
	}

	status, body = env.do(t, "PATCH", "/api/displays/keuken", `{"name":"Keuken"}`)
	if status != http.StatusOK || !strings.Contains(body, `"name":"Keuken"`) || !strings.Contains(body, "e542c41e") {
		t.Fatalf("rename = %d %s (cast device must stay)", status, body)
	}
	if status, _ := env.do(t, "PATCH", "/api/displays/bestaat-niet", `{"name":"x"}`); status != http.StatusNotFound {
		t.Fatalf("patch unknown = %d, want 404", status)
	}

	_, body = env.do(t, "GET", "/api/displays", "")
	var list struct {
		Displays []struct{ ID, Name string }
	}
	if err := json.Unmarshal([]byte(body), &list); err != nil || len(list.Displays) != 1 || list.Displays[0].Name != "Keuken" {
		t.Fatalf("list = %s (err %v)", body, err)
	}

	if status, _ := env.do(t, "DELETE", "/api/displays/keuken", ""); status != http.StatusNoContent {
		t.Fatalf("delete = %d", status)
	}
	if status, _ := env.do(t, "DELETE", "/api/displays/keuken", ""); status != http.StatusNotFound {
		t.Fatalf("delete again = %d, want 404", status)
	}
	// Create, rename and delete changed the displays; the failed requests
	// did not.
	if got := env.changes.Load(); got != 3 {
		t.Fatalf("DisplaysChanged called %d times, want 3", got)
	}
}

func TestCastDevicesListsDiscoveredDevices(t *testing.T) {
	env := newTestEnv(t, func(context.Context) ([]cast.Device, error) {
		return []cast.Device{{ID: "abc", Name: "Keuken TV", Addr: "192.168.1.40:8009"}}, nil
	})
	env.setUp(t)
	status, body := env.do(t, "GET", "/api/cast-devices", "")
	if status != http.StatusOK || !strings.Contains(body, `"name":"Keuken TV"`) {
		t.Fatalf("status %d body %s", status, body)
	}
}

func TestCastDevicesErrorHasStableShape(t *testing.T) {
	env := newTestEnv(t, func(context.Context) ([]cast.Device, error) {
		return nil, errors.New("socket: permission denied at /proc/net")
	})
	env.setUp(t)
	status, body := env.do(t, "GET", "/api/cast-devices", "")
	if status != http.StatusInternalServerError || !strings.Contains(body, `"code":"discovery_failed"`) {
		t.Fatalf("status %d body %s", status, body)
	}
	if strings.Contains(body, "/proc") {
		t.Fatalf("internal error details leaked: %s", body)
	}
}

// fakeBackup stands in for the real archive, which internal/backup tests.
type fakeBackup struct{}

func (fakeBackup) WriteZip(_ context.Context, w io.Writer) error {
	_, err := io.WriteString(w, "PK fake zip")
	return err
}
