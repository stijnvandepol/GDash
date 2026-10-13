package camera

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stijnvandepol/gdash/internal/custom"
	"github.com/stijnvandepol/gdash/internal/outbound"
	"github.com/stijnvandepol/gdash/internal/validation"
)

// testSecret is made up; the tests check that it never appears in errors.
const testSecret = "camera-test-secret-9f3a"

var pngBytes = func() []byte {
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		panic(err)
	}
	return b.Bytes()
}()

func input(kind, name, secret, url string) Input {
	in := Input{Name: "Front door", URL: url}
	in.Auth.Kind, in.Auth.Name, in.Auth.Secret = kind, name, secret
	return in
}

func TestCheckAcceptsTheUsualSnapshotAddresses(t *testing.T) {
	for _, in := range []Input{
		input(custom.AuthNone, "", "", "http://192.168.1.20:5000/api/front_door/latest.jpg"),
		input(custom.AuthBasic, "admin", testSecret, "http://192.168.1.30/cgi-bin/api.cgi?cmd=Snap&channel=0"),
		input(custom.AuthBearer, "", testSecret, "http://homeassistant.local:8123/api/camera_proxy/camera.front"),
		input(custom.AuthHeader, "X-API-Key", testSecret, "https://cam.example/snap.jpg"),
		input(custom.AuthQuery, "token", testSecret, "http://192.168.1.40/snapshot.jpg"),
	} {
		cam, err := Check(in, nil)
		if err != nil {
			t.Errorf("Check(%s, %s) = %v", in.URL, in.Auth.Kind, err)
			continue
		}
		if cam.Auth.Kind != in.Auth.Kind || (in.Auth.Kind != custom.AuthNone && cam.Auth.Secret != testSecret) {
			t.Errorf("Check(%s) auth = %+v", in.URL, cam.Auth)
		}
	}
}

func TestCheckNamesInvalidFieldsWithoutTheSecret(t *testing.T) {
	cases := []struct {
		field string
		in    Input
	}{
		{"url", input(custom.AuthNone, "", "", "rtsp://192.168.1.20/stream")},
		{"url", input(custom.AuthNone, "", "", "http://admin:"+testSecret+"@192.168.1.20/snap.jpg")},
		{"url", input(custom.AuthNone, "", "", "snap.jpg")},
		{"auth.secret", input(custom.AuthBasic, "admin", "a\n"+testSecret, "http://cam/snap.jpg")},
		{"auth.name", input(custom.AuthQuery, "token", testSecret, "http://cam/snap.jpg?token=x")},
		{"auth.kind", input("digest", "", testSecret, "http://cam/snap.jpg")},
	}
	for _, tt := range cases {
		_, err := Check(tt.in, nil)
		fields := validation.Fields(err)
		if len(fields) == 0 || fields[0].Field != tt.field {
			t.Errorf("Check(%+v) = %v, want a problem with %s", tt.in, err, tt.field)
		}
		if err != nil && strings.Contains(err.Error(), testSecret) {
			t.Errorf("error %q holds the secret", err)
		}
	}
	bad := input(custom.AuthNone, "", "", "http://cam/snap.jpg")
	bad.Name = " padded"
	if fields := validation.Fields(func() error { _, err := Check(bad, nil); return err }()); len(fields) != 1 || fields[0].Field != "name" {
		t.Errorf("padded name = %v", fields)
	}
}

func TestCheckKeepsTheSecretOnlyForTheSameKind(t *testing.T) {
	old := &Camera{ID: 3, Auth: custom.Auth{Kind: custom.AuthBasic, Name: "admin", Secret: testSecret}}
	cam, err := Check(input(custom.AuthBasic, "admin", "", "http://cam/snap.jpg"), old)
	if err != nil || cam.Auth.Secret != testSecret || cam.ID != 3 {
		t.Fatalf("kept secret = %+v, %v", cam, err)
	}
	if _, err := Check(input(custom.AuthBearer, "", "", "http://cam/snap.jpg"), old); err == nil {
		t.Fatal("a password became a bearer token")
	}
}

func testFetcher() *Fetcher {
	return &Fetcher{HTTP: outbound.NewClient(2 * time.Second), UserAgent: "GDash/test"}
}

func TestFetchSendsTheAuthenticationAndChecksTheImage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if user, pass, ok := r.BasicAuth(); !ok || user != "admin" || pass != testSecret {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		// Cameras often send a wrong content type; GDash looks at the bytes.
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write(pngBytes)
	}))
	defer srv.Close()
	cam := Camera{URL: srv.URL + "/snap", Auth: custom.Auth{Kind: custom.AuthBasic, Name: "admin", Secret: testSecret}}
	data, contentType, err := testFetcher().Fetch(context.Background(), cam)
	if err != nil || contentType != "image/png" || !bytes.Equal(data, pngBytes) {
		t.Fatalf("Fetch = %d bytes, %q, %v", len(data), contentType, err)
	}
	cam.Auth.Secret = "wrong"
	if _, _, err := testFetcher().Fetch(context.Background(), cam); err == nil || !strings.Contains(err.Error(), "status 401; check the authentication") {
		t.Fatalf("wrong password = %v", err)
	}
}

func TestFetchRefusesWhatIsNoSnapshot(t *testing.T) {
	cases := map[string]http.HandlerFunc{
		"JPEG, PNG or WebP": func(w http.ResponseWriter, _ *http.Request) { io.WriteString(w, "<html>login</html>") },
		"more than 5 MB": func(w http.ResponseWriter, _ *http.Request) {
			w.Write(pngBytes)
			w.Write(make([]byte, MaxBytes))
		},
	}
	for want, handler := range cases {
		srv := httptest.NewServer(handler)
		_, _, err := testFetcher().Fetch(context.Background(), Camera{URL: srv.URL, Auth: custom.Auth{Kind: custom.AuthNone}})
		srv.Close()
		var fetchErr *custom.FetchError
		if !errors.As(err, &fetchErr) || !strings.Contains(fetchErr.Message, want) {
			t.Errorf("Fetch = %v, want a message with %q", err, want)
		}
	}
}

func TestFetchKeepsTheKeyFromOtherHostsAndMessages(t *testing.T) {
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("the redirect was followed to another host with %q", r.URL.RawQuery)
	}))
	defer other.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, strings.Replace(other.URL, "127.0.0.1", "localhost", 1)+"/snap", http.StatusFound)
	}))
	defer srv.Close()
	cam := Camera{URL: srv.URL + "/snap", Auth: custom.Auth{Kind: custom.AuthQuery, Name: "token", Secret: testSecret}}
	_, _, err := testFetcher().Fetch(context.Background(), cam)
	if err == nil || strings.Contains(err.Error(), testSecret) || !strings.Contains(err.Error(), "redirected to another host") {
		t.Fatalf("redirect = %v", err)
	}
	cam.URL = "http://169.254.169.254/latest/meta-data/"
	if _, _, err := testFetcher().Fetch(context.Background(), cam); err == nil || !strings.Contains(err.Error(), "link-local") || strings.Contains(err.Error(), testSecret) {
		t.Fatalf("metadata address = %v", err)
	}
}

// clock is a test clock that tests move by hand.
type clock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *clock) add(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// standIn is a camera that counts its requests and can be switched off.
type standIn struct {
	srv      *httptest.Server
	requests atomic.Int32
	broken   atomic.Bool
}

func newStandIn(t *testing.T) *standIn {
	s := &standIn{}
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		s.requests.Add(1)
		if s.broken.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Write(pngBytes)
	}))
	t.Cleanup(s.srv.Close)
	return s
}

func TestKeeperAsksTheCameraOncePerRefreshForAllScreens(t *testing.T) {
	cam := newStandIn(t)
	c := &clock{now: time.Date(2026, 10, 13, 14, 2, 0, 0, time.UTC)}
	k := &Keeper{Fetcher: testFetcher(), Now: c.Now, Logger: slog.New(slog.DiscardHandler)}
	camera := Camera{ID: 1, URL: cam.srv.URL, Auth: custom.Auth{Kind: custom.AuthNone}}

	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if img, err := k.Latest(context.Background(), camera, 10*time.Second); err != nil || img.Stale {
				t.Errorf("Latest = %+v, %v", img.Stale, err)
			}
		})
	}
	wg.Wait()
	c.add(9 * time.Second)
	k.Latest(context.Background(), camera, 10*time.Second)
	if n := cam.requests.Load(); n != 1 {
		t.Fatalf("camera asked %d times by 9 screens within its refresh, want 1", n)
	}
	c.add(time.Second)
	k.Latest(context.Background(), camera, 10*time.Second)
	if n := cam.requests.Load(); n != 2 {
		t.Fatalf("camera asked %d times after its refresh, want 2", n)
	}
}

func TestKeeperKeepsTheLastImageMarkedStaleWhenTheCameraFails(t *testing.T) {
	cam := newStandIn(t)
	start := time.Date(2026, 10, 13, 14, 2, 0, 0, time.UTC)
	c := &clock{now: start}
	k := &Keeper{Fetcher: testFetcher(), Now: c.Now, Logger: slog.New(slog.DiscardHandler)}
	camera := Camera{ID: 1, URL: cam.srv.URL, Auth: custom.Auth{Kind: custom.AuthNone}}
	if _, err := k.Latest(context.Background(), camera, 5*time.Second); err != nil {
		t.Fatal(err)
	}

	cam.broken.Store(true)
	c.add(5 * time.Second)
	img, err := k.Latest(context.Background(), camera, 5*time.Second)
	if err != nil || !img.Stale || !img.TakenAt.Equal(start) || !bytes.Equal(img.Data, pngBytes) {
		t.Fatalf("after a failure = stale %v, taken %v, %v", img.Stale, img.TakenAt, err)
	}

	cam.broken.Store(false)
	c.add(5 * time.Second)
	if img, err := k.Latest(context.Background(), camera, 5*time.Second); err != nil || img.Stale || !img.TakenAt.Equal(start.Add(10*time.Second)) {
		t.Fatalf("after recovery = stale %v, taken %v, %v", img.Stale, img.TakenAt, err)
	}
}

func TestKeeperWithoutAnyImageReturnsTheCameraError(t *testing.T) {
	cam := newStandIn(t)
	cam.broken.Store(true)
	c := &clock{now: time.Date(2026, 10, 13, 14, 2, 0, 0, time.UTC)}
	k := &Keeper{Fetcher: testFetcher(), Now: c.Now, Logger: slog.New(slog.DiscardHandler)}
	var fetchErr *custom.FetchError
	if _, err := k.Latest(context.Background(), Camera{ID: 1, URL: cam.srv.URL, Auth: custom.Auth{Kind: custom.AuthNone}}, 5*time.Second); !errors.As(err, &fetchErr) {
		t.Fatalf("Latest of a broken camera = %v", err)
	}
}

func TestKeeperDropsImagesOfChangedForgottenAndIdleCameras(t *testing.T) {
	cam := newStandIn(t)
	c := &clock{now: time.Date(2026, 10, 13, 14, 2, 0, 0, time.UTC)}
	k := &Keeper{Fetcher: testFetcher(), Now: c.Now, Logger: slog.New(slog.DiscardHandler)}
	camera := Camera{ID: 1, URL: cam.srv.URL, Auth: custom.Auth{Kind: custom.AuthNone}}
	k.Latest(context.Background(), camera, time.Minute)

	moved := camera
	moved.URL += "/other"
	k.Latest(context.Background(), moved, time.Minute)
	if n := cam.requests.Load(); n != 2 {
		t.Fatalf("a changed address was asked %d times in all, want a new request", n)
	}
	k.Forget(1)
	k.Latest(context.Background(), moved, time.Minute)
	if n := cam.requests.Load(); n != 3 {
		t.Fatalf("a forgotten camera was asked %d times in all, want a new request", n)
	}

	c.add(idleAfter + time.Second)
	k.Latest(context.Background(), Camera{ID: 2, URL: cam.srv.URL, Auth: custom.Auth{Kind: custom.AuthNone}}, time.Minute)
	k.mu.Lock()
	_, kept := k.cameras[1]
	k.mu.Unlock()
	if kept {
		t.Fatal("the image of a camera that no screen asked for is still in memory")
	}
}
