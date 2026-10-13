// Package synology reads a shared album of Synology Photos for the Photos
// widget, through the album's share link, so GDash needs no account on the
// NAS. It speaks the DSM Web API the way the Photos web page does for a
// shared link, at {link base}/{xx}/sharing/webapi/entry.cgi:
//
//	SYNO.Core.Sharing.Login  login  v1  sharing_id, password   opens a sharing session (cookie)
//	SYNO.Foto.Browse.Item    list   v1  offset, limit, additional=["thumbnail"],
//	                                    passphrase, header X-SYNO-SHARING
//	SYNO.Foto.Thumbnail      get    v2  id, cache_key, type="unit", size="xl"|"m",
//	                                    passphrase, _sharing_id
//
// Synology does not document these for shared links; the calls follow the
// open-source clients that use them: syno-photo-frame (Rust, the login and
// the header), MMM-SynPhotoSlideshow (passphrase on list and thumbnail) and
// py-synologydsm-api, which Home Assistant uses (thumbnail sizes and
// cache_key).
package synology

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/stijnvandepol/gdash/internal/custom"
	"github.com/stijnvandepol/gdash/internal/outbound"
	"github.com/stijnvandepol/gdash/internal/photos"
)

const (
	// MaxPhotos is how many photos of the album screens get at a time;
	// the tokens travel in every snapshot. Every refresh picks others.
	MaxPhotos = 100
	// listLimit is the most items the list call returns at once.
	listLimit = 5000
	// maxListBytes bounds the answer of the list call.
	maxListBytes = 16 << 20
	// Timeout bounds one request; the NAS is at home.
	Timeout = 20 * time.Second
)

var (
	// ErrLogin means the NAS refused the link or its password.
	ErrLogin = errors.New("synology refused the share link or its password")
	// ErrNotSynology means the address answers, but not as Synology Photos.
	ErrNotSynology = errors.New("the address does not answer as Synology Photos")
	// ErrNotFound means the photo is not one GDash can ask for.
	ErrNotFound = errors.New("no such photo in the shared album")
	// ErrNotAPhoto means the NAS sent something other than a JPEG, PNG or
	// WebP image, or one over photos.MaxBytes.
	ErrNotAPhoto = errors.New("synology did not send a usable image")
)

// shareLink matches a Synology Photos share link: the NAS's address, an
// optional /photo when Photos runs under DSM's own port, the two-letter
// part before /sharing and the passphrase, such as
// https://nas:5001/mo/sharing/AbCd12 or http://nas:5000/photo/mo/sharing/AbCd12.
var shareLink = regexp.MustCompile(`^(https?://[^/?#]+(?:/photo)?)/([a-z]{2}/sharing)/([A-Za-z0-9_-]{1,64})/?$`)

// Album is a shared album with what opens it, for the server only.
type Album struct {
	// Link is the share link as the admin pasted it, normalized.
	Link string
	// Password is empty for an album shared without one.
	Password string
	// Pin is the fingerprint of the NAS's certificate that the admin
	// accepted, or zero to check it as browsers do.
	Pin outbound.Fingerprint
}

// parts are the addresses of a share link.
type parts struct {
	api        string // the entry.cgi of the shared album
	passphrase string
	host       string
}

// ParseLink checks a share link and returns it normalized, or "" and a
// message for the web interface.
func ParseLink(raw string) (string, string) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" || u.User != nil {
		return "", "paste the share link of the album, such as https://nas:5001/mo/sharing/AbCd12"
	}
	u.RawQuery, u.Fragment = "", ""
	link := strings.TrimRight(u.String(), "/")
	if !shareLink.MatchString(link) {
		return "", "this is not a Synology Photos share link; in Synology Photos, share the album and copy its link"
	}
	return link, ""
}

func split(link string) (parts, error) {
	m := shareLink.FindStringSubmatch(link)
	if m == nil {
		return parts{}, errors.New("invalid share link")
	}
	u, err := url.Parse(m[1])
	if err != nil {
		return parts{}, err
	}
	return parts{api: m[1] + "/" + m[2] + "/webapi/entry.cgi", passphrase: m[3], host: u.Host}, nil
}

// Host returns the host and port of a share link, for messages and to read
// the certificate of the NAS.
func Host(link string) string {
	p, err := split(link)
	if err != nil {
		return ""
	}
	return p.host
}

// Client talks to Synology Photos. It keeps one sharing session per album,
// with its own cookies.
type Client struct {
	// NewHTTP makes the client for an album: one that refuses the
	// addresses that outbound refuses, with a timeout, pinned to the
	// album's certificate when it has a pin. Nil uses outbound.
	NewHTTP func(pin outbound.Fingerprint) *http.Client
	// Shuffle orders the photos of a refresh; nil uses math/rand.
	Shuffle func(n int, swap func(i, j int))

	mu       sync.Mutex
	sessions map[Album]*session
}

type session struct {
	http *http.Client
	// mu makes one login at a time.
	mu       sync.Mutex
	loggedIn bool
}

func (c *Client) session(album Album) (*session, parts, error) {
	p, err := split(album.Link)
	if err != nil {
		return nil, parts{}, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if s, ok := c.sessions[album]; ok {
		return s, p, nil
	}
	if c.sessions == nil {
		c.sessions = map[Album]*session{}
	}
	// One album is configured at a time; an old session goes with its
	// cookies.
	clear(c.sessions)
	client := c.newHTTP(album.Pin)
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, parts{}, err
	}
	client.Jar = jar
	// The password goes in a form, and the session cookie must not follow
	// a redirect to another host.
	client.CheckRedirect = custom.SameHostRedirects
	s := &session{http: client}
	c.sessions[album] = s
	return s, p, nil
}

func (c *Client) newHTTP(pin outbound.Fingerprint) *http.Client {
	if c.NewHTTP != nil {
		return c.NewHTTP(pin)
	}
	if pin != (outbound.Fingerprint{}) {
		return outbound.NewPinnedClient(Timeout, pin)
	}
	return outbound.NewClient(Timeout)
}

// apiAnswer is the envelope of every DSM Web API answer.
type apiAnswer struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   *struct {
		Code int `json:"code"`
	} `json:"error"`
}

// login opens the sharing session, once, or again when force is set.
func (s *session) login(ctx context.Context, p parts, album Album, force bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loggedIn && !force {
		return nil
	}
	password := ""
	if album.Password != "" {
		// DSM takes the string parameters of this call JSON-encoded.
		quoted, err := json.Marshal(album.Password)
		if err != nil {
			return err
		}
		password = string(quoted)
	}
	form := url.Values{
		"api": {"SYNO.Core.Sharing.Login"}, "method": {"login"}, "version": {"1"},
		"sharing_id": {p.passphrase}, "password": {password},
	}
	answer, err := s.post(ctx, p, form)
	if err != nil {
		return err
	}
	if !answer.Success {
		return ErrLogin
	}
	s.loggedIn = true
	return nil
}

func (s *session) post(ctx context.Context, p parts, form url.Values) (apiAnswer, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.api, strings.NewReader(form.Encode()))
	if err != nil {
		return apiAnswer{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-SYNO-SHARING", p.passphrase)
	resp, err := s.http.Do(req)
	if err != nil {
		return apiAnswer{}, requestError(p.host, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return apiAnswer{}, ErrNotSynology
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxListBytes+1))
	if err != nil {
		return apiAnswer{}, requestError(p.host, err)
	}
	var answer apiAnswer
	if len(body) > maxListBytes || json.Unmarshal(body, &answer) != nil {
		return apiAnswer{}, ErrNotSynology
	}
	return answer, nil
}

// requestError keeps what an admin can act on, such as a certificate that
// is not trusted, without the address, which holds the passphrase.
func requestError(host string, err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		err = urlErr.Err
	}
	return fmt.Errorf("request synology photos at %s: %w", host, err)
}

// Check opens the album and returns how many photos GDash can show of it.
// It is how a link is tried before it is stored.
func (c *Client) Check(ctx context.Context, album Album) (int, error) {
	items, err := c.list(ctx, album)
	return len(items), err
}

// Photos returns tokens for at most MaxPhotos of the album's photos,
// picked at random and shuffled; videos are left out. A token names a
// photo and the size to ask for; the server hands it to screens and takes
// it back in Thumbnail.
func (c *Client) Photos(ctx context.Context, album Album) ([]string, error) {
	items, err := c.list(ctx, album)
	if err != nil {
		return nil, err
	}
	shuffle := c.Shuffle
	if shuffle == nil {
		shuffle = rand.Shuffle
	}
	shuffle(len(items), func(i, j int) { items[i], items[j] = items[j], items[i] })
	return items[:min(len(items), MaxPhotos)], nil
}

func (c *Client) list(ctx context.Context, album Album) ([]string, error) {
	s, p, err := c.session(album)
	if err != nil {
		return nil, err
	}
	// A fresh session for every refresh: a sharing session ends after a
	// while, and the list is read only every 30 minutes.
	if err := s.login(ctx, p, album, true); err != nil {
		return nil, err
	}
	form := url.Values{
		"api": {"SYNO.Foto.Browse.Item"}, "method": {"list"}, "version": {"1"},
		"offset": {"0"}, "limit": {strconv.Itoa(listLimit)}, "additional": {`["thumbnail"]`},
		"passphrase": {p.passphrase},
	}
	answer, err := s.post(ctx, p, form)
	if err != nil {
		return nil, err
	}
	if !answer.Success {
		return nil, ErrLogin
	}
	var data struct {
		List []item `json:"list"`
	}
	if err := json.Unmarshal(answer.Data, &data); err != nil {
		return nil, ErrNotSynology
	}
	tokens := make([]string, 0, len(data.List))
	for _, it := range data.List {
		if token, ok := it.token(); ok {
			tokens = append(tokens, token)
		}
	}
	return tokens, nil
}

// item is one entry of an album, with only what GDash uses.
type item struct {
	ID         int64  `json:"id"`
	Type       string `json:"type"`
	Additional struct {
		Thumbnail struct {
			CacheKey string `json:"cache_key"`
			UnitID   int64  `json:"unit_id"`
			XL       string `json:"xl"`
			M        string `json:"m"`
		} `json:"thumbnail"`
	} `json:"additional"`
}

// validToken is what token returns, so a token from a request can go into
// a URL only when it has this shape.
var validToken = regexp.MustCompile(`^([0-9]{1,15})-([0-9A-Za-z_]{1,64})-(xl|m)$`)

// ValidToken reports whether token can name a photo.
func ValidToken(token string) bool { return validToken.MatchString(token) }

// token names a photo as "<unit id>-<cache key>-<size>", choosing the
// largest size Synology has made. A video, or a photo without a thumbnail,
// has none.
func (it item) token() (string, bool) {
	if it.Type != "photo" && it.Type != "live" {
		return "", false
	}
	th := it.Additional.Thumbnail
	unit := th.UnitID
	if unit == 0 {
		unit = it.ID
	}
	size := ""
	switch {
	case th.XL == "ready":
		size = "xl"
	case th.M == "ready":
		size = "m"
	}
	token := fmt.Sprintf("%d-%s-%s", unit, th.CacheKey, size)
	return token, size != "" && validToken.MatchString(token)
}

// Thumbnail returns the photo that token names, with what it says about
// where it was taken removed, and its content type.
func (c *Client) Thumbnail(ctx context.Context, album Album, token string) ([]byte, string, error) {
	m := validToken.FindStringSubmatch(token)
	if m == nil {
		return nil, "", ErrNotFound
	}
	s, p, err := c.session(album)
	if err != nil {
		return nil, "", err
	}
	if err := s.login(ctx, p, album, false); err != nil {
		return nil, "", err
	}
	data, contentType, err := s.thumbnail(ctx, p, m[1], m[2], m[3])
	if errors.Is(err, ErrNotAPhoto) {
		// The session may have ended: log in again and ask once more.
		if err := s.login(ctx, p, album, true); err != nil {
			return nil, "", err
		}
		data, contentType, err = s.thumbnail(ctx, p, m[1], m[2], m[3])
	}
	if err != nil {
		return nil, "", err
	}
	clean, err := photos.StripMetadata(contentType, data)
	if err != nil {
		return nil, "", ErrNotAPhoto
	}
	return clean, contentType, nil
}

func (s *session) thumbnail(ctx context.Context, p parts, unit, cacheKey, size string) ([]byte, string, error) {
	query := url.Values{
		"api": {"SYNO.Foto.Thumbnail"}, "method": {"get"}, "version": {"2"},
		"id": {unit}, "cache_key": {`"` + cacheKey + `"`}, "type": {`"unit"`}, "size": {`"` + size + `"`},
		"passphrase": {p.passphrase}, "_sharing_id": {p.passphrase},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.api+"?"+query.Encode(), nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("X-SYNO-SHARING", p.passphrase)
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, "", requestError(p.host, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, "", ErrNotFound
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, photos.MaxBytes+1))
	if err != nil {
		return nil, "", requestError(p.host, err)
	}
	contentType, ok := photos.ImageType(data)
	if resp.StatusCode != http.StatusOK || !ok || len(data) > photos.MaxBytes {
		// DSM answers an error as JSON with status 200.
		return nil, "", ErrNotAPhoto
	}
	return data, contentType, nil
}
