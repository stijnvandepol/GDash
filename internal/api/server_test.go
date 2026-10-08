package api

import (
	"bufio"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stijnvandepol/daycast/internal/cast"
)

type fixedSubscriber []byte

func (f fixedSubscriber) Subscribe(ctx context.Context) <-chan []byte {
	ch := make(chan []byte, 1)
	ch <- f
	go func() { <-ctx.Done(); close(ch) }()
	return ch
}

func newTestServer(t *testing.T, discover func(context.Context) ([]cast.Device, error)) *httptest.Server {
	t.Helper()
	s := &Server{
		Displays: fixedSubscriber(`{"type":"snapshot","version":1}`),
		Web: fstest.MapFS{
			"display/index.html":  {Data: []byte("<html>display</html>")},
			"receiver/index.html": {Data: []byte("<html>receiver</html>")},
			"assets/app.js":       {Data: []byte("console.log(1)")},
		},
		Discover: discover,
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)
	return srv
}

func TestDisplayPageHasStrictCSP(t *testing.T) {
	srv := newTestServer(t, nil)
	resp, err := http.Get(srv.URL + "/d/keuken")
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

func TestInvalidDisplayIDIsNotFound(t *testing.T) {
	srv := newTestServer(t, nil)
	for _, path := range []string{"/d/Keuken", "/d/..%2fetc", "/api/displays/a_b/events", "/d/" + strings.Repeat("a", 40)} {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", path, resp.StatusCode)
		}
	}
}

func TestEventStreamSendsSnapshot(t *testing.T) {
	srv := newTestServer(t, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/displays/keuken/events", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
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
	got := strings.Join(lines, "\n")
	want := "retry: 5000\n\nevent: snapshot\ndata: {\"type\":\"snapshot\",\"version\":1}"
	if got != want {
		t.Fatalf("stream = %q, want %q", got, want)
	}
}

func TestCastDevicesListsDiscoveredDevices(t *testing.T) {
	srv := newTestServer(t, func(context.Context) ([]cast.Device, error) {
		return []cast.Device{{ID: "abc", Name: "Keuken TV", Addr: "192.168.1.40:8009"}}, nil
	})
	resp, err := http.Get(srv.URL + "/api/cast-devices")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), `"name":"Keuken TV"`) {
		t.Fatalf("status %d body %s", resp.StatusCode, body)
	}
}

func TestCastDevicesErrorHasStableShape(t *testing.T) {
	srv := newTestServer(t, func(context.Context) ([]cast.Device, error) {
		return nil, errors.New("socket: permission denied at /proc/net")
	})
	resp, err := http.Get(srv.URL + "/api/cast-devices")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusInternalServerError || !strings.Contains(string(body), `"code":"discovery_failed"`) {
		t.Fatalf("status %d body %s", resp.StatusCode, body)
	}
	if strings.Contains(string(body), "/proc") {
		t.Fatalf("internal error details leaked: %s", body)
	}
}
