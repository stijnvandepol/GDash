// Package immich reads an album of a self-hosted Immich server for the
// Photos widget. It uses Immich's REST API with an API key in the
// x-api-key header, on the paths that are stable in Immich v2 and still
// served by v3:
//
//	GET  /api/albums                        the albums to choose from
//	POST /api/search/random                 a random part of an album's images
//	GET  /api/assets/{id}/thumbnail?size=preview   one image, sized for a screen
//
// Immich v3.2 marks the albumIds and type fields of the search as
// deprecated in favour of a new filter object, which v2 does not know;
// GDash keeps the fields that both understand.
package immich

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/stijnvandepol/gdash/internal/custom"
	"github.com/stijnvandepol/gdash/internal/outbound"
	"github.com/stijnvandepol/gdash/internal/photos"
)

const (
	// MaxAssets is how many photos of the album screens get at a time;
	// the IDs travel in every snapshot, which must stay small. Every
	// refresh picks others.
	MaxAssets = 100
	// maxAlbums bounds the list the editor offers.
	maxAlbums = 500
	// maxListBytes bounds an answer with JSON: a page of albums or assets
	// with their details.
	maxListBytes = 8 << 20
	// Timeout bounds one request; Immich runs at home.
	Timeout = 15 * time.Second
)

var (
	// ErrUnauthorized means Immich refused the API key.
	ErrUnauthorized = errors.New("immich refused the API key")
	// ErrNotImmich means the address answers, but not as Immich does.
	ErrNotImmich = errors.New("the address does not answer as Immich")
	// ErrNotFound means Immich does not have the album or photo.
	ErrNotFound = errors.New("not found in immich")
	// ErrNotAPhoto means Immich sent something other than a JPEG, PNG or
	// WebP image, or one over photos.MaxBytes.
	ErrNotAPhoto = errors.New("immich did not send a usable image")
)

// validID matches Immich's IDs, UUIDs; an ID from a request or from Immich
// goes into a path only when it matches.
var validID = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// ValidID reports whether id can be an Immich album or asset ID.
func ValidID(id string) bool { return validID.MatchString(id) }

// Connection is the address of an Immich server and an API key.
type Connection struct {
	URL string
	Key string
}

// NormalizeURL turns the address an admin typed, such as
// http://192.168.1.10:2283/ or the API address ending in /api, into the
// server's base address. It returns "" and a message for the web interface
// when the address cannot be one.
func NormalizeURL(raw string) (string, string) {
	base, msg := outbound.BaseURL(raw, "paste the address you open Immich at, such as http://192.168.1.10:2283")
	return strings.TrimSuffix(base, "/api"), msg
}

// Album is an album to choose from.
type Album struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	AssetCount int    `json:"assetCount"`
}

// Client talks to Immich.
type Client struct {
	// HTTP must refuse the addresses that outbound refuses and have a
	// timeout; its redirect policy is replaced by custom.SameHostRedirects,
	// because Go would send the x-api-key header to another host.
	HTTP *http.Client
	// Shuffle orders the photos of a refresh; nil uses math/rand.
	Shuffle func(n int, swap func(i, j int))
}

// Albums lists the albums the key can read, owned and shared. It is also
// how a connection is tried.
func (c *Client) Albums(ctx context.Context, conn Connection) ([]Album, error) {
	var answer []struct {
		ID         string `json:"id"`
		AlbumName  string `json:"albumName"`
		AssetCount int    `json:"assetCount"`
	}
	if err := c.getJSON(ctx, conn, "/api/albums", &answer); err != nil {
		return nil, err
	}
	albums := make([]Album, 0, min(len(answer), maxAlbums))
	for _, a := range answer {
		if ValidID(a.ID) && len(albums) < maxAlbums {
			albums = append(albums, Album{ID: a.ID, Name: a.AlbumName, AssetCount: a.AssetCount})
		}
	}
	return albums, nil
}

// Photos returns the IDs of at most MaxAssets images of the album, picked
// at random by Immich and shuffled; videos are left out.
func (c *Client) Photos(ctx context.Context, conn Connection, albumID string) ([]string, error) {
	if !ValidID(albumID) {
		return nil, ErrNotFound
	}
	query, err := json.Marshal(map[string]any{"albumIds": []string{albumID}, "type": "IMAGE", "size": MaxAssets})
	if err != nil {
		return nil, err
	}
	req, err := c.request(ctx, conn, http.MethodPost, "/api/search/random", bytes.NewReader(query))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	var answer []struct {
		ID   string `json:"id"`
		Type string `json:"type"`
	}
	if err := c.doJSON(req, &answer); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(answer))
	seen := map[string]bool{}
	for _, a := range answer {
		// The search already asks for images; a server that ignores the
		// filter still never puts a video on the TV.
		if a.Type == "IMAGE" && ValidID(a.ID) && !seen[a.ID] && len(ids) < MaxAssets {
			seen[a.ID] = true
			ids = append(ids, a.ID)
		}
	}
	shuffle := c.Shuffle
	if shuffle == nil {
		shuffle = rand.Shuffle
	}
	shuffle(len(ids), func(i, j int) { ids[i], ids[j] = ids[j], ids[i] })
	return ids, nil
}

// Thumbnail returns the preview of an asset, the size Immich makes for
// screens, with what it says about where it was taken removed, and its
// content type.
func (c *Client) Thumbnail(ctx context.Context, conn Connection, assetID string) ([]byte, string, error) {
	if !ValidID(assetID) {
		return nil, "", ErrNotFound
	}
	req, err := c.request(ctx, conn, http.MethodGet, "/api/assets/"+assetID+"/thumbnail?size=preview", nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Accept", "image/jpeg, image/webp, image/png")
	resp, err := c.do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, photos.MaxBytes+1))
	if err != nil {
		return nil, "", fmt.Errorf("read immich photo: %w", err)
	}
	contentType, ok := photos.ImageType(data)
	if !ok || len(data) > photos.MaxBytes {
		return nil, "", ErrNotAPhoto
	}
	clean, err := photos.StripMetadata(contentType, data)
	if err != nil {
		return nil, "", ErrNotAPhoto
	}
	return clean, contentType, nil
}

func (c *Client) getJSON(ctx context.Context, conn Connection, path string, into any) error {
	req, err := c.request(ctx, conn, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	return c.doJSON(req, into)
}

func (c *Client) request(ctx context.Context, conn Connection, method, path string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, conn.URL+path, body)
	if err != nil {
		return nil, fmt.Errorf("build immich request: %w", err)
	}
	req.Header.Set("x-api-key", conn.Key)
	return req, nil
}

func (c *Client) doJSON(req *http.Request, into any) error {
	req.Header.Set("Accept", "application/json")
	resp, err := c.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxListBytes+1))
	if err != nil {
		return fmt.Errorf("read immich answer: %w", err)
	}
	if len(body) > maxListBytes || json.Unmarshal(body, into) != nil {
		return ErrNotImmich
	}
	return nil
}

// do sends req and returns a response with a success status.
func (c *Client) do(req *http.Request) (*http.Response, error) {
	client := *c.HTTP
	client.CheckRedirect = custom.SameHostRedirects
	resp, err := client.Do(req)
	if err != nil {
		// The error names the address only: the key is in a header.
		return nil, fmt.Errorf("request immich: %w", err)
	}
	if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
		return resp, nil
	}
	resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, ErrUnauthorized
	case http.StatusNotFound:
		if strings.HasPrefix(req.URL.Path, "/api/assets/") {
			return nil, ErrNotFound
		}
		return nil, ErrNotImmich
	case http.StatusBadRequest:
		// Immich answers 400 for an album or asset it does not have.
		return nil, ErrNotFound
	}
	return nil, fmt.Errorf("immich answered with status %d", resp.StatusCode)
}
