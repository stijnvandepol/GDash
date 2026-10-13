package synology

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stijnvandepol/gdash/internal/outbound"
)

// Made up: the passphrase of the share link and its password.
const (
	passphrase = "AbCd12xyz"
	password   = "made-up-share-password"
)

var pngBytes = func() []byte {
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		panic(err)
	}
	return b.Bytes()
}()

// nas stands in for Synology Photos with one album shared by link with a
// password: two photos, a video and a photo without a thumbnail yet.
type nas struct {
	logins atomic.Int32
	// expired makes the next thumbnail fail as an ended session does.
	expired atomic.Bool
}

func (n *nas) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/mo/sharing/webapi/entry.cgi" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	r.ParseForm()
	answer := func(v any) { json.NewEncoder(w).Encode(v) }
	failure := map[string]any{"success": false, "error": map[string]int{"code": 119}}
	session, _ := r.Cookie("sharing_sid")
	loggedIn := session != nil && session.Value == "session-1"
	switch r.Form.Get("api") {
	case "SYNO.Core.Sharing.Login":
		if r.Form.Get("sharing_id") != passphrase || r.Form.Get("password") != `"`+password+`"` {
			answer(failure)
			return
		}
		n.logins.Add(1)
		http.SetCookie(w, &http.Cookie{Name: "sharing_sid", Value: "session-1", Path: "/"})
		answer(map[string]any{"success": true, "data": map[string]any{}})
	case "SYNO.Foto.Browse.Item":
		if !loggedIn || r.Header.Get("X-SYNO-SHARING") != passphrase || r.Form.Get("additional") != `["thumbnail"]` {
			answer(failure)
			return
		}
		answer(map[string]any{"success": true, "data": map[string]any{"list": []any{
			map[string]any{"id": 11, "type": "photo", "additional": map[string]any{"thumbnail": map[string]any{"cache_key": "11_1633859830", "unit_id": 11, "xl": "ready", "m": "ready"}}},
			map[string]any{"id": 12, "type": "live", "additional": map[string]any{"thumbnail": map[string]any{"cache_key": "12_1633859831", "unit_id": 12, "xl": "broken", "m": "ready"}}},
			map[string]any{"id": 13, "type": "video", "additional": map[string]any{"thumbnail": map[string]any{"cache_key": "13_1633859832", "unit_id": 13, "xl": "ready"}}},
			map[string]any{"id": 14, "type": "photo", "additional": map[string]any{"thumbnail": map[string]any{"cache_key": "14_1633859833", "unit_id": 14, "xl": "processing"}}},
		}}})
	case "SYNO.Foto.Thumbnail":
		if !loggedIn || n.expired.Swap(false) || r.Form.Get("passphrase") != passphrase || r.Form.Get("type") != `"unit"` {
			answer(failure)
			return
		}
		if r.Form.Get("id") == "11" && r.Form.Get("cache_key") == `"11_1633859830"` && r.Form.Get("size") == `"xl"` {
			w.Write(pngBytes)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	default:
		answer(failure)
	}
}

func client() *Client {
	return &Client{NewHTTP: func(pin outbound.Fingerprint) *http.Client {
		if pin != (outbound.Fingerprint{}) {
			return outbound.NewPinnedClient(2*time.Second, pin)
		}
		return outbound.NewClient(2 * time.Second)
	}, Shuffle: func(int, func(i, j int)) {}}
}

func TestParseLinkTakesTheLinksOfSynologyPhotos(t *testing.T) {
	for raw, want := range map[string]string{
		"https://nas.local:5001/mo/sharing/AbCd12":          "https://nas.local:5001/mo/sharing/AbCd12",
		"http://192.168.1.2:5000/photo/mo/sharing/AbCd12/":  "http://192.168.1.2:5000/photo/mo/sharing/AbCd12",
		" https://photos.example.com/mo/sharing/Xy_9-z?x=1": "https://photos.example.com/mo/sharing/Xy_9-z",
	} {
		if got, msg := ParseLink(raw); got != want || msg != "" {
			t.Errorf("ParseLink(%q) = %q %q, want %q", raw, got, msg, want)
		}
	}
	for _, raw := range []string{"nas.local/mo/sharing/AbCd12", "https://nas.local:5001/photo/", "https://u:p@nas/mo/sharing/AbCd12", "https://nas/mo/sharing/../../x"} {
		if _, msg := ParseLink(raw); msg == "" {
			t.Errorf("ParseLink(%q) accepted it", raw)
		}
	}
	if host := Host("https://nas.local:5001/mo/sharing/AbCd12"); host != "nas.local:5001" {
		t.Errorf("Host = %q", host)
	}
}

func TestPhotosListsTheStillsOfTheAlbum(t *testing.T) {
	srv := httptest.NewServer(&nas{})
	defer srv.Close()
	album := Album{Link: srv.URL + "/mo/sharing/" + passphrase, Password: password}
	tokens, err := client().Photos(context.Background(), album)
	if err != nil || strings.Join(tokens, ",") != "11-11_1633859830-xl,12-12_1633859831-m" {
		t.Fatalf("Photos = %v, %v", tokens, err)
	}
	album.Password = "wrong"
	if _, err := client().Check(context.Background(), album); !errors.Is(err, ErrLogin) {
		t.Fatalf("wrong password = %v, want ErrLogin", err)
	}
}

func TestThumbnailLogsInAgainWhenTheSessionEnded(t *testing.T) {
	n := &nas{}
	srv := httptest.NewServer(n)
	defer srv.Close()
	c := client()
	album := Album{Link: srv.URL + "/mo/sharing/" + passphrase, Password: password}
	data, contentType, err := c.Thumbnail(context.Background(), album, "11-11_1633859830-xl")
	if err != nil || contentType != "image/png" || len(data) == 0 {
		t.Fatalf("Thumbnail = %d bytes %q, %v", len(data), contentType, err)
	}
	n.expired.Store(true)
	if _, _, err := c.Thumbnail(context.Background(), album, "11-11_1633859830-xl"); err != nil {
		t.Fatalf("Thumbnail after the session ended = %v", err)
	}
	if got := n.logins.Load(); got != 2 {
		t.Fatalf("logins = %d, want 2", got)
	}
	if _, _, err := c.Thumbnail(context.Background(), album, "11-x_1&size=original-xl"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("token with a query in it = %v, want ErrNotFound", err)
	}
	if _, _, err := c.Thumbnail(context.Background(), album, "99-99_1-xl"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a photo the NAS does not have = %v, want ErrNotFound", err)
	}
}

func TestASelfSignedNASNeedsItsCertificateAccepted(t *testing.T) {
	srv := httptest.NewTLSServer(&nas{})
	defer srv.Close()
	album := Album{Link: srv.URL + "/mo/sharing/" + passphrase, Password: password}
	if _, err := client().Check(context.Background(), album); err == nil {
		t.Fatal("a self-signed NAS was trusted without accepting its certificate")
	}
	album.Pin = sha256.Sum256(srv.Certificate().Raw)
	if n, err := client().Check(context.Background(), album); err != nil || n != 2 {
		t.Fatalf("Check with the accepted certificate = %d, %v", n, err)
	}
	album.Pin[0] ^= 0xFF
	if _, err := client().Check(context.Background(), album); !errors.Is(err, outbound.ErrCertificateChanged) {
		t.Fatalf("Check after the certificate changed = %v", err)
	}
}

func TestThePasswordNeverFollowsARedirectToAnotherHost(t *testing.T) {
	other := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		t.Errorf("the login followed a redirect to another host")
	}))
	defer other.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, strings.Replace(other.URL, "127.0.0.1", "localhost", 1)+r.URL.Path, http.StatusTemporaryRedirect)
	}))
	defer srv.Close()
	if _, err := client().Check(context.Background(), Album{Link: srv.URL + "/mo/sharing/" + passphrase, Password: password}); err == nil {
		t.Fatal("a redirect to another host was followed")
	}
}
