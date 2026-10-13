package immich

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stijnvandepol/gdash/internal/outbound"
)

// testKey is made up.
const testKey = "made-up-immich-key"

const (
	album   = "4b1c5d6e-7f80-4912-a3b4-c5d6e7f80912"
	photoA  = "11111111-2222-4333-8444-555555555555"
	photoB  = "66666666-7777-4888-9999-aaaaaaaaaaaa"
	videoID = "bbbbbbbb-cccc-4ddd-8eee-ffffffffffff"
)

var jpegBytes = func() []byte {
	var b bytes.Buffer
	if err := jpeg.Encode(&b, image.NewRGBA(image.Rect(0, 0, 8, 8)), nil); err != nil {
		panic(err)
	}
	return b.Bytes()
}()

// standIn answers like the Immich v2 API for one album with two photos and
// a video; it records the search it was sent.
func standIn(t *testing.T, searched *map[string]any) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != testKey {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/albums":
			json.NewEncoder(w).Encode([]map[string]any{
				{"id": album, "albumName": "Holidays", "assetCount": 3},
				{"id": "not-a-uuid", "albumName": "Broken"},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/search/random":
			if searched != nil {
				json.NewDecoder(r.Body).Decode(searched)
			}
			json.NewEncoder(w).Encode([]map[string]any{
				{"id": photoA, "type": "IMAGE"}, {"id": videoID, "type": "VIDEO"}, {"id": photoB, "type": "IMAGE"}, {"id": photoA, "type": "IMAGE"},
			})
		case r.URL.Path == "/api/assets/"+photoA+"/thumbnail" && r.URL.Query().Get("size") == "preview":
			w.Header().Set("Content-Type", "image/jpeg")
			w.Write(jpegBytes)
		case r.URL.Path == "/api/assets/"+photoB+"/thumbnail":
			w.Write([]byte("<html>not an image</html>"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func client() *Client {
	return &Client{HTTP: outbound.NewClient(2 * time.Second), Shuffle: func(int, func(i, j int)) {}}
}

func TestAlbumsListsReadableAlbumsAndNamesAWrongKey(t *testing.T) {
	srv := standIn(t, nil)
	albums, err := client().Albums(context.Background(), Connection{URL: srv.URL, Key: testKey})
	if err != nil || len(albums) != 1 || albums[0] != (Album{ID: album, Name: "Holidays", AssetCount: 3}) {
		t.Fatalf("Albums = %+v, %v", albums, err)
	}
	if _, err := client().Albums(context.Background(), Connection{URL: srv.URL, Key: "wrong"}); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("wrong key = %v", err)
	}
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("<html>router</html>")) }))
	defer other.Close()
	if _, err := client().Albums(context.Background(), Connection{URL: other.URL, Key: testKey}); !errors.Is(err, ErrNotImmich) {
		t.Fatalf("another server = %v", err)
	}
}

func TestPhotosAsksForImagesOfTheAlbumAndSkipsVideos(t *testing.T) {
	var searched map[string]any
	srv := standIn(t, &searched)
	ids, err := client().Photos(context.Background(), Connection{URL: srv.URL, Key: testKey}, album)
	if err != nil || strings.Join(ids, ",") != photoA+","+photoB {
		t.Fatalf("Photos = %v, %v", ids, err)
	}
	if searched["type"] != "IMAGE" || searched["size"] != float64(MaxAssets) || searched["albumIds"].([]any)[0] != album {
		t.Fatalf("search = %v", searched)
	}
	if _, err := client().Photos(context.Background(), Connection{URL: srv.URL, Key: testKey}, "../users"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("invalid album ID = %v", err)
	}
}

func TestThumbnailIsAPreviewImage(t *testing.T) {
	srv := standIn(t, nil)
	conn := Connection{URL: srv.URL, Key: testKey}
	data, contentType, err := client().Thumbnail(context.Background(), conn, photoA)
	if err != nil || contentType != "image/jpeg" || len(data) == 0 {
		t.Fatalf("Thumbnail = %d bytes %q, %v", len(data), contentType, err)
	}
	if _, _, err := client().Thumbnail(context.Background(), conn, photoB); !errors.Is(err, ErrNotAPhoto) {
		t.Fatalf("HTML as a photo = %v", err)
	}
	if _, _, err := client().Thumbnail(context.Background(), conn, videoID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing photo = %v", err)
	}
	if _, _, err := client().Thumbnail(context.Background(), conn, "../../api/users"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("path in the ID = %v", err)
	}
}

func TestTheKeyNeverFollowsARedirectToAnotherHost(t *testing.T) {
	other := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		t.Errorf("redirect followed to another host with key %q", r.Header.Get("x-api-key"))
	}))
	defer other.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, strings.Replace(other.URL, "127.0.0.1", "localhost", 1)+r.URL.Path, http.StatusFound)
	}))
	defer srv.Close()
	if _, err := client().Albums(context.Background(), Connection{URL: srv.URL, Key: testKey}); err == nil {
		t.Fatal("a redirect to another host was followed")
	}
}

func TestNormalizeURL(t *testing.T) {
	for raw, want := range map[string]string{
		"http://192.168.1.10:2283/":    "http://192.168.1.10:2283",
		"http://192.168.1.10:2283/api": "http://192.168.1.10:2283",
		"https://photos.example/":      "https://photos.example",
	} {
		if got, msg := NormalizeURL(raw); got != want || msg != "" {
			t.Errorf("NormalizeURL(%q) = %q %q, want %q", raw, got, msg, want)
		}
	}
	if _, msg := NormalizeURL("192.168.1.10"); msg == "" {
		t.Error("an address without a scheme was accepted")
	}
}
