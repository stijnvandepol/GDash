// Package homey reads devices and users from a Homey Pro through its local
// Web API, so a dashboard can show sensors, switches and who is home, or
// from the snapshot that an older Homey without API keys sends itself
// (snapshot.go). It only reads: GDash never changes anything in Homey.
package homey

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"slices"
	"strings"

	"github.com/stijnvandepol/gdash/internal/outbound"
)

const (
	// maxListBytes bounds one answer: every device with all its
	// capabilities stays under a few megabytes in a large home.
	maxListBytes = 16 << 20

	devicesPath = "/api/manager/devices/device"
	zonesPath   = "/api/manager/zones/zone"
	usersPath   = "/api/manager/users/user"

	// keySeparator joins a device ID and a capability ID into a key.
	keySeparator = "/"
)

// ErrUnauthorized means Homey did not accept the API key, or the key lacks
// a permission the request needs.
var ErrUnauthorized = errors.New("homey did not accept the api key")

// ErrNotHomey means the address answered, but not as Homey's API does.
var ErrNotHomey = errors.New("the address does not answer as homey")

var (
	validKey    = regexp.MustCompile(`^[A-Za-z0-9-]{1,64}/[A-Za-z0-9_]{1,64}(\.[A-Za-z0-9_-]{1,64})?$`)
	validUserID = regexp.MustCompile(`^[A-Za-z0-9-]{1,64}$`)
)

// ValidKey reports whether key names a capability of a device as
// "<device id>/<capability id>", such as "5f2c…/measure_temperature".
func ValidKey(key string) bool { return validKey.MatchString(key) }

// ValidUserID reports whether id looks like the ID of a Homey user.
func ValidUserID(id string) bool { return validUserID.MatchString(id) }

// Connection is a Homey and an API key made in its web app.
type Connection struct {
	// URL is the Homey's base address, such as http://192.168.1.20,
	// without a trailing slash.
	URL string
	Key string
}

// State is what a screen shows of one capability.
type State struct {
	Device string `json:"device"`
	Title  string `json:"title"`
	// Capability is the capability's ID, such as alarm_motion, so the
	// screen can put a yes or no into words.
	Capability string `json:"capability"`
	// Value is a number, a boolean, a text or null when the device is
	// unavailable or has no value yet.
	Value    any    `json:"value"`
	Units    string `json:"units,omitempty"`
	Decimals *int   `json:"decimals,omitempty"`
}

// Capability is a capability as the widget editor lists it.
type Capability struct {
	Key  string `json:"key"`
	Zone string `json:"zone,omitempty"`
	State
}

// User is a Homey user. Present is false when Homey does not know.
type User struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Present bool   `json:"present"`
}

// Client calls Homey.
type Client struct {
	// HTTP must have a timeout; the caller owns that choice. Redirects are
	// never followed, whatever HTTP's own policy says.
	HTTP *http.Client
}

// text is a JSON field that is normally a string. Anything else, such as a
// title translated per language, reads as empty instead of failing the
// whole answer, so one odd device cannot hide all the others.
type text string

func (t *text) UnmarshalJSON(b []byte) error {
	var s string
	if json.Unmarshal(b, &s) == nil {
		*t = text(s)
	}
	return nil
}

// number is a JSON field that is normally a number; anything else reads as
// absent, for the same reason as text.
type number struct {
	value float64
	set   bool
}

func (n *number) UnmarshalJSON(b []byte) error {
	n.set = json.Unmarshal(b, &n.value) == nil
	return nil
}

// maxDecimals bounds the decimals a screen is asked to show.
const maxDecimals = 3

// decimals returns n as a number of decimals, or nil when it is not one.
func (n number) decimals() *int {
	if !n.set || n.value < 0 || n.value > maxDecimals || n.value != float64(int(n.value)) {
		return nil
	}
	d := int(n.value)
	return &d
}

type apiCapability struct {
	Title    text   `json:"title"`
	Value    any    `json:"value"`
	Units    text   `json:"units"`
	Decimals number `json:"decimals"`
	Getable  *bool  `json:"getable"`
}

// apiDevice is a device in Homey's answer, which maps device IDs to these.
type apiDevice struct {
	Name text `json:"name"`
	// Zone is the ID of the zone the device is in.
	Zone      text                     `json:"zone"`
	Available *bool                    `json:"available"`
	Caps      map[string]apiCapability `json:"capabilitiesObj"`
}

type apiZone struct {
	Name text `json:"name"`
}

type apiUser struct {
	Name    text  `json:"name"`
	Present *bool `json:"present"`
}

// state is what a screen shows of capability id of device d.
func (d apiDevice) state(id string, c apiCapability) State {
	title := string(c.Title)
	if title == "" {
		title = id
	}
	s := State{Device: string(d.Name), Title: title, Capability: id, Units: string(c.Units), Decimals: c.Decimals.decimals()}
	// An unavailable device keeps its last value in Homey; the screen
	// shows that there is none instead of an old one.
	if d.Available == nil || *d.Available {
		s.Value = plainValue(c.Value)
	}
	return s
}

// plainValue keeps the values a screen can show: a number, a boolean or a
// text. Anything else becomes null.
func plainValue(v any) any {
	switch v.(type) {
	case float64, bool, string:
		return v
	}
	return nil
}

// Check confirms that conn reaches a Homey whose API accepts the key for
// zones and devices.
func (c *Client) Check(ctx context.Context, conn Connection) error {
	zones, err := c.zones(ctx, conn)
	if err != nil {
		return err
	}
	// Every Homey has at least its home as a zone.
	if len(zones) == 0 {
		return ErrNotHomey
	}
	_, err = c.devices(ctx, conn)
	return err
}

// States returns the current state of each capability in keys. A key Homey
// does not know is left out, so the screen can say so; any other failure
// fails the whole read and the screen keeps the last good one.
func (c *Client) States(ctx context.Context, conn Connection, keys []string) (map[string]State, error) {
	devices, err := c.devices(ctx, conn)
	if err != nil {
		return nil, err
	}
	out := make(map[string]State, len(keys))
	for _, key := range keys {
		deviceID, capID, ok := strings.Cut(key, keySeparator)
		if !ok || !ValidKey(key) {
			continue
		}
		device, found := devices[deviceID]
		if !found {
			continue
		}
		if capability, found := device.Caps[capID]; found {
			out[key] = device.state(capID, capability)
		}
	}
	return out, nil
}

// Capabilities lists every readable capability of every device, sorted by
// zone, device and title, for the widget editor to choose from.
func (c *Client) Capabilities(ctx context.Context, conn Connection) ([]Capability, error) {
	zones, err := c.zones(ctx, conn)
	if err != nil {
		return nil, err
	}
	devices, err := c.devices(ctx, conn)
	if err != nil {
		return nil, err
	}
	return capabilityList(devices, func(d apiDevice) string { return zones[string(d.Zone)] }), nil
}

// capabilityList lists every readable capability of devices, sorted by
// zone, device and title; zoneOf names the zone of a device.
func capabilityList(devices map[string]apiDevice, zoneOf func(apiDevice) string) []Capability {
	var list []Capability
	for deviceID, device := range devices {
		for capID, capability := range device.Caps {
			key := deviceID + keySeparator + capID
			if !ValidKey(key) || (capability.Getable != nil && !*capability.Getable) {
				continue
			}
			list = append(list, Capability{Key: key, Zone: zoneOf(device), State: device.state(capID, capability)})
		}
	}
	slices.SortFunc(list, func(a, b Capability) int {
		return cmp.Or(cmp.Compare(a.Zone, b.Zone), cmp.Compare(a.Device, b.Device), cmp.Compare(a.Title, b.Title), cmp.Compare(a.Key, b.Key))
	})
	return list
}

// Users lists Homey's users, sorted by name, with whether each is home.
func (c *Client) Users(ctx context.Context, conn Connection) ([]User, error) {
	var all map[string]apiUser
	if err := c.get(ctx, conn, usersPath, &all); err != nil {
		return nil, err
	}
	return userList(all), nil
}

// userList lists users by ID, sorted by name, leaving out invalid IDs.
func userList(all map[string]apiUser) []User {
	list := make([]User, 0, len(all))
	for id, u := range all {
		if !ValidUserID(id) {
			continue
		}
		name := string(u.Name)
		if name == "" {
			name = id
		}
		list = append(list, User{ID: id, Name: name, Present: u.Present != nil && *u.Present})
	}
	slices.SortFunc(list, func(a, b User) int { return cmp.Or(cmp.Compare(a.Name, b.Name), cmp.Compare(a.ID, b.ID)) })
	return list
}

// Presence returns, for each user in ids that Homey knows, whether that
// user is home. Users that are not asked for are left out.
func (c *Client) Presence(ctx context.Context, conn Connection, ids []string) (map[string]bool, error) {
	users, err := c.Users(ctx, conn)
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(ids))
	for _, u := range users {
		if slices.Contains(ids, u.ID) {
			out[u.ID] = u.Present
		}
	}
	return out, nil
}

func (c *Client) devices(ctx context.Context, conn Connection) (map[string]apiDevice, error) {
	var all map[string]apiDevice
	if err := c.get(ctx, conn, devicesPath, &all); err != nil {
		return nil, err
	}
	return all, nil
}

// zones returns the name of every zone by its ID.
func (c *Client) zones(ctx context.Context, conn Connection) (map[string]string, error) {
	var all map[string]apiZone
	if err := c.get(ctx, conn, zonesPath, &all); err != nil {
		return nil, err
	}
	names := make(map[string]string, len(all))
	for id, z := range all {
		names[id] = string(z.Name)
	}
	return names, nil
}

// noRedirects returns c's HTTP client with redirects turned off. Homey's
// API does not redirect, and Go would resend the key to another port or
// from https to http on the same host.
func (c *Client) noRedirects() *http.Client {
	hc := *c.HTTP
	hc.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &hc
}

// get fetches path and decodes the JSON answer into v. Errors never
// contain the key.
func (c *Client) get(ctx context.Context, conn Connection, path string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, conn.URL+path, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+conn.Key)
	req.Header.Set("Accept", "application/json")
	resp, err := c.noRedirects().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return ErrUnauthorized
	case resp.StatusCode != http.StatusOK:
		return fmt.Errorf("%w: status %d", ErrNotHomey, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxListBytes+1))
	if err != nil {
		return fmt.Errorf("read answer: %w", err)
	}
	if len(body) > maxListBytes {
		return fmt.Errorf("answer is larger than %d bytes", maxListBytes)
	}
	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("%w: %v", ErrNotHomey, err)
	}
	return nil
}

// NormalizeURL turns what an admin typed into a base URL, or returns an
// error message for the web interface.
func NormalizeURL(raw string) (string, string) {
	return outbound.BaseURL(raw, "paste the address of your Homey, such as http://192.168.1.20")
}
