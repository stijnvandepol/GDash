package nowplaying

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"regexp"
	"sync"
	"time"
)

const (
	// ArtworkPath is where screens load cover art: ArtworkPath + an ID.
	ArtworkPath = "/api/now-playing/artwork/"
	// MaxArtworkBytes bounds one cover. Apps announce covers of 640 pixels
	// or less, far below this.
	MaxArtworkBytes = 2 << 20
	// artworkCacheFor is how long a fetched cover is served from memory;
	// a track lasts minutes, and every screen of the house asks for it.
	artworkCacheFor = 10 * time.Minute
	// artworkRetryAfter is how long a cover that could not be fetched is
	// not asked for again.
	artworkRetryAfter = time.Minute
	artworkTimeout    = 10 * time.Second
	// artworkIDLength is the hex characters of an ID: half a SHA-256.
	artworkIDLength = 32
)

// ErrNoArtwork is returned for an ID that is not the cover of anything
// playing now, and for a cover that could not be fetched.
var ErrNoArtwork = errors.New("no such cover art")

// validArtworkID matches what artworkID returns.
var validArtworkID = regexp.MustCompile(`^[0-9a-f]{32}$`)

// artworkTypes are the image types a cover may have; anything else, such as
// SVG with its scripts, is refused.
var artworkTypes = map[string]bool{"image/jpeg": true, "image/png": true, "image/webp": true, "image/gif": true}

// Image is a fetched cover.
type Image struct {
	Data        []byte
	ContentType string
}

// Artwork fetches the cover art that Cast apps announce, so screens load it
// from GDash: the address an app gives can hold a token (Plex does) and
// points anywhere, so it never reaches a screen. Only the covers of what
// plays now have an ID.
type Artwork struct {
	// HTTP fetches the covers; it should refuse link-local addresses (see
	// internal/outbound), as an app on the network chooses the address.
	HTTP      *http.Client
	UserAgent string
	Now       func() time.Time

	mu sync.Mutex
	// addresses maps the ID of each current cover to its address.
	addresses map[string]string
	cache     map[string]cachedImage
}

type cachedImage struct {
	image   Image
	err     error
	expires time.Time
}

// Shows reports whether value, the Now playing data that a screen was
// sent, holds the cover with the given ID.
func Shows(value json.RawMessage, id string) bool {
	var items map[string]Item
	if json.Unmarshal(value, &items) != nil {
		return false
	}
	for _, item := range items {
		if item.Image == ArtworkPath+id {
			return true
		}
	}
	return false
}

// artworkID is the ID of a cover address.
func artworkID(address string) string {
	sum := sha256.Sum256([]byte(address))
	return hex.EncodeToString(sum[:])[:artworkIDLength]
}

// path returns the GDash address of a cover, which serves it while the
// cover is kept (see keep).
func (a *Artwork) path(address string) string {
	return ArtworkPath + artworkID(address)
}

// keep makes addresses the covers that have an ID; the others, and what
// was fetched for them, are forgotten.
func (a *Artwork) keep(addresses []string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	next := make(map[string]string, len(addresses))
	for _, address := range addresses {
		next[artworkID(address)] = address
	}
	for id := range a.cache {
		if _, ok := next[id]; !ok {
			delete(a.cache, id)
		}
	}
	a.addresses = next
}

// Get returns the cover with the given ID, fetching it when it is not in
// memory. It returns ErrNoArtwork for an unknown ID or a failed fetch,
// wrapped with the reason. A failure is remembered for a minute, so a
// screen that asks again does not fetch again.
func (a *Artwork) Get(ctx context.Context, id string) (Image, error) {
	if !validArtworkID.MatchString(id) {
		return Image{}, ErrNoArtwork
	}
	a.mu.Lock()
	address, known := a.addresses[id]
	cached, hit := a.cache[id]
	a.mu.Unlock()
	if !known {
		return Image{}, ErrNoArtwork
	}
	if hit && a.Now().Before(cached.expires) {
		return cached.image, cached.err
	}
	image, err := a.fetch(ctx, address)
	expires := a.Now().Add(artworkCacheFor)
	if err != nil {
		err = fmt.Errorf("%w: %w", ErrNoArtwork, err)
		expires = a.Now().Add(artworkRetryAfter)
	}
	a.mu.Lock()
	// A cover that stopped playing meanwhile is not cached.
	if _, ok := a.addresses[id]; ok {
		if a.cache == nil {
			a.cache = map[string]cachedImage{}
		}
		a.cache[id] = cachedImage{image: image, err: err, expires: expires}
	}
	a.mu.Unlock()
	return image, err
}

// fetch downloads one cover and checks that it is an image of an allowed
// type and size, by its header and by its content.
func (a *Artwork) fetch(ctx context.Context, address string) (Image, error) {
	ctx, cancel := context.WithTimeout(ctx, artworkTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return Image{}, err
	}
	req.Header.Set("User-Agent", a.UserAgent)
	req.Header.Set("Accept", "image/webp,image/jpeg,image/png,image/gif")
	resp, err := a.HTTP.Do(req)
	if err != nil {
		return Image{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Image{}, fmt.Errorf("cover answered %s", resp.Status)
	}
	declared, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil || !artworkTypes[declared] {
		return Image{}, fmt.Errorf("cover has type %q", resp.Header.Get("Content-Type"))
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxArtworkBytes+1))
	if err != nil {
		return Image{}, err
	}
	if len(data) > MaxArtworkBytes {
		return Image{}, fmt.Errorf("cover is larger than %d bytes", MaxArtworkBytes)
	}
	sniffed := http.DetectContentType(data)
	if !artworkTypes[sniffed] {
		return Image{}, fmt.Errorf("cover content is %q", sniffed)
	}
	return Image{Data: data, ContentType: sniffed}, nil
}
