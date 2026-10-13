package api

import (
	"context"
	"net/http"
	"sync"
	"testing"

	"github.com/stijnvandepol/gdash/internal/datahub"
	"github.com/stijnvandepol/gdash/internal/display"
	"github.com/stijnvandepol/gdash/internal/nowplaying"
)

// fakeArtwork holds the covers that play and what each display was sent.
type fakeArtwork struct {
	mu     sync.Mutex
	images map[string]nowplaying.Image
	sent   map[string]datahub.Entry
}

func (f *fakeArtwork) Get(_ context.Context, id string) (nowplaying.Image, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	img, ok := f.images[id]
	if !ok {
		return nowplaying.Image{}, nowplaying.ErrNoArtwork
	}
	return img, nil
}

func (f *fakeArtwork) sentData(displayID, key string) (datahub.Entry, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if key != display.NowPlayingSource {
		return datahub.Entry{}, false
	}
	entry, ok := f.sent[displayID]
	return entry, ok
}

func (f *fakeArtwork) set(images map[string]nowplaying.Image, sent map[string]datahub.Entry) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.images, f.sent = images, sent
}

func TestCoversAreOnlyForAdminsAndScreensThatShowThem(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	env.createKitchen(t)
	const cover, other = "0123456789abcdef0123456789abcdef", "fedcba9876543210fedcba9876543210"
	png := nowplaying.Image{Data: []byte("\x89PNG\r\n\x1a\n"), ContentType: "image/png"}
	env.artwork.set(map[string]nowplaying.Image{cover: png, other: png}, map[string]datahub.Entry{
		"keuken": {Value: []byte(`{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa":{"title":"Song","image":"` + nowplaying.ArtworkPath + cover + `"}}`)},
	})
	url := env.srv.URL + nowplaying.ArtworkPath

	if status, _ := get(t, tv(), url+cover); status != http.StatusNotFound {
		t.Fatalf("cover for a stranger on the network = %d, want 404", status)
	}
	if status, header := get(t, env.client, url+other); status != http.StatusOK || header.Get("Content-Type") != "image/png" {
		t.Fatalf("cover for the admin = %d %s", status, header.Get("Content-Type"))
	}
	if status, _ := get(t, env.client, url+"00000000000000000000000000000000"); status != http.StatusNotFound {
		t.Fatalf("unknown cover for the admin = %d, want 404", status)
	}
	screen := pairedScreen(t, env)
	if status, header := get(t, screen, url+cover); status != http.StatusOK || header.Get("Cache-Control") != artworkMaxAge {
		t.Fatalf("the cover the kitchen shows = %d %q", status, header.Get("Cache-Control"))
	}
	if status, _ := get(t, screen, url+other); status != http.StatusNotFound {
		t.Fatalf("a cover the kitchen does not show = %d, want 404", status)
	}
}
