package nowplaying

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func pngBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	img.Set(1, 1, color.RGBA{R: 200, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// coverServer serves body with contentType on /cover and counts requests.
func coverServer(t *testing.T, contentType string, body []byte) (string, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path != "/cover" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", contentType)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv.URL + "/cover", &hits
}

func newArtwork(now *time.Time) *Artwork {
	return &Artwork{HTTP: &http.Client{Timeout: 5 * time.Second}, Now: func() time.Time { return *now }}
}

func TestArtworkServesTheCoverOfWhatPlays(t *testing.T) {
	ctx := testContext(t)
	cover := pngBytes(t)
	address, hits := coverServer(t, "image/png", cover)
	now := time.Now()
	a := newArtwork(&now)
	a.keep([]string{address})
	id := artworkID(address)
	if got := a.path(address); got != ArtworkPath+id {
		t.Fatalf("path = %q", got)
	}

	for range 2 {
		img, err := a.Get(ctx, id)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if img.ContentType != "image/png" || !bytes.Equal(img.Data, cover) {
			t.Fatalf("got %s of %d bytes", img.ContentType, len(img.Data))
		}
	}
	if n := hits.Load(); n != 1 {
		t.Fatalf("fetched %d times, want once for both screens", n)
	}
	now = now.Add(artworkCacheFor + time.Second)
	if _, err := a.Get(ctx, id); err != nil || hits.Load() != 2 {
		t.Fatalf("after the cache expired: err %v, %d fetches", err, hits.Load())
	}
}

func TestArtworkRefusesWhatIsNotACover(t *testing.T) {
	ctx := testContext(t)
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`)
	tests := []struct {
		name        string
		contentType string
		body        []byte
	}{
		{"svg", "image/svg+xml", svg},
		{"svg that says png", "image/png", svg},
		{"html", "text/html", []byte("<html></html>")},
		{"too large", "image/png", append(pngBytes(t), make([]byte, MaxArtworkBytes)...)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			address, hits := coverServer(t, tt.contentType, tt.body)
			now := time.Now()
			a := newArtwork(&now)
			a.keep([]string{address})
			if _, err := a.Get(ctx, artworkID(address)); !errors.Is(err, ErrNoArtwork) {
				t.Fatalf("err = %v, want ErrNoArtwork", err)
			}
			// A screen that asks again does not make GDash fetch again.
			if _, err := a.Get(ctx, artworkID(address)); !errors.Is(err, ErrNoArtwork) || hits.Load() != 1 {
				t.Fatalf("second try: err %v, %d fetches", err, hits.Load())
			}
		})
	}
}

func TestArtworkOnlyServesCurrentCovers(t *testing.T) {
	ctx := testContext(t)
	address, hits := coverServer(t, "image/png", pngBytes(t))
	now := time.Now()
	a := newArtwork(&now)
	if _, err := a.Get(ctx, artworkID(address)); !errors.Is(err, ErrNoArtwork) {
		t.Fatalf("a cover that was never kept: err = %v", err)
	}
	a.keep([]string{address})
	for _, id := range []string{"../../etc/passwd", strings.ToUpper(artworkID(address))} {
		if _, err := a.Get(ctx, id); !errors.Is(err, ErrNoArtwork) {
			t.Fatalf("%q: err = %v", id, err)
		}
	}
	if _, err := a.Get(ctx, artworkID(address)); err != nil {
		t.Fatal(err)
	}
	a.keep(nil)
	if _, err := a.Get(ctx, artworkID(address)); !errors.Is(err, ErrNoArtwork) {
		t.Fatalf("a cover that stopped playing: err = %v", err)
	}
	if n := hits.Load(); n != 1 {
		t.Fatalf("fetched %d times", n)
	}
}
