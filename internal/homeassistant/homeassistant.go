// Package homeassistant reads entity states from a Home Assistant server
// through its REST API, so a dashboard can show sensors and switches that
// Home Assistant knows about. It only reads: Nookboard never changes
// anything in Home Assistant.
package homeassistant

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
)

const (
	// maxStateBytes bounds one entity's answer; a state with all its
	// attributes is a few kilobytes.
	maxStateBytes = 256 << 10
	// maxListBytes bounds the list of all entities, which a large home
	// with thousands of entities keeps under a few megabytes.
	maxListBytes = 16 << 20
)

// ErrUnauthorized means Home Assistant did not accept the token.
var ErrUnauthorized = errors.New("home assistant did not accept the token")

// ErrNotHomeAssistant means the address answered, but not as Home
// Assistant's API does.
var ErrNotHomeAssistant = errors.New("the address does not answer as home assistant")

var validEntityID = regexp.MustCompile(`^[a-z0-9_]{1,64}\.[a-z0-9_]{1,128}$`)

// ValidEntityID reports whether id looks like a Home Assistant entity ID,
// such as sensor.living_room_temperature. Only such IDs go into a URL.
func ValidEntityID(id string) bool { return validEntityID.MatchString(id) }

// Connection is a Home Assistant server and a long-lived access token
// made under a Home Assistant user's profile.
type Connection struct {
	// URL is the server's base address, such as http://192.168.1.10:8123,
	// without a trailing slash.
	URL   string
	Token string
}

// State is what a screen shows of one entity.
type State struct {
	State string `json:"state"`
	// Name is the entity's friendly name in Home Assistant.
	Name        string `json:"name"`
	Unit        string `json:"unit,omitempty"`
	DeviceClass string `json:"deviceClass,omitempty"`
}

// Entity is an entity as the widget editor lists it.
type Entity struct {
	ID string `json:"id"`
	State
}

// Client calls Home Assistant.
type Client struct {
	// HTTP must have a timeout; the caller owns that choice.
	HTTP *http.Client
}

// apiState is Home Assistant's JSON for one entity.
type apiState struct {
	EntityID   string `json:"entity_id"`
	State      string `json:"state"`
	Attributes struct {
		FriendlyName string `json:"friendly_name"`
		Unit         string `json:"unit_of_measurement"`
		DeviceClass  string `json:"device_class"`
	} `json:"attributes"`
}

func (s apiState) state() State {
	name := s.Attributes.FriendlyName
	if name == "" {
		name = s.EntityID
	}
	return State{State: s.State, Name: name, Unit: s.Attributes.Unit, DeviceClass: s.Attributes.DeviceClass}
}

// Check confirms that conn reaches Home Assistant's API with a valid token.
func (c *Client) Check(ctx context.Context, conn Connection) error {
	var answer struct {
		Message string `json:"message"`
	}
	if _, err := c.get(ctx, conn, "/api/", maxStateBytes, &answer); err != nil {
		return err
	}
	if answer.Message == "" {
		return ErrNotHomeAssistant
	}
	return nil
}

// States returns the current state of each entity in ids. An entity Home
// Assistant does not know is left out, so the screen can say so; any other
// failure fails the whole read and the screen keeps the last good one.
func (c *Client) States(ctx context.Context, conn Connection, ids []string) (map[string]State, error) {
	out := make(map[string]State, len(ids))
	for _, id := range ids {
		if !ValidEntityID(id) {
			continue
		}
		var s apiState
		found, err := c.get(ctx, conn, "/api/states/"+id, maxStateBytes, &s)
		if err != nil {
			return nil, fmt.Errorf("entity %s: %w", id, err)
		}
		if found {
			out[id] = s.state()
		}
	}
	return out, nil
}

// Entities lists every entity Home Assistant has, sorted by ID, for the
// widget editor to choose from.
func (c *Client) Entities(ctx context.Context, conn Connection) ([]Entity, error) {
	var all []apiState
	if _, err := c.get(ctx, conn, "/api/states", maxListBytes, &all); err != nil {
		return nil, err
	}
	list := make([]Entity, 0, len(all))
	for _, s := range all {
		if ValidEntityID(s.EntityID) {
			list = append(list, Entity{ID: s.EntityID, State: s.state()})
		}
	}
	slices.SortFunc(list, func(a, b Entity) int { return strings.Compare(a.ID, b.ID) })
	return list, nil
}

// get fetches path and decodes the JSON answer into v. found is false for a
// 404. Errors never contain the token.
func (c *Client) get(ctx context.Context, conn Connection, path string, limit int64, v any) (found bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, conn.URL+path, nil)
	if err != nil {
		return false, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+conn.Token)
	req.Header.Set("Accept", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return false, ErrUnauthorized
	case resp.StatusCode == http.StatusNotFound:
		return false, nil
	case resp.StatusCode != http.StatusOK:
		return false, fmt.Errorf("%w: status %d", ErrNotHomeAssistant, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return false, fmt.Errorf("read answer: %w", err)
	}
	if int64(len(body)) > limit {
		return false, fmt.Errorf("answer is larger than %d bytes", limit)
	}
	if err := json.Unmarshal(body, v); err != nil {
		return false, fmt.Errorf("%w: %v", ErrNotHomeAssistant, err)
	}
	return true, nil
}

// NormalizeURL turns what an admin typed into a base URL, or returns an
// error message for the web interface.
func NormalizeURL(raw string) (string, string) {
	u, err := url.Parse(strings.TrimSpace(raw))
	switch {
	case err != nil || u.Host == "" || len(raw) > 2048:
		return "", "paste the address you open Home Assistant with, such as http://192.168.1.10:8123"
	case u.Scheme != "http" && u.Scheme != "https":
		return "", "use an http or https address"
	case u.User != nil || u.RawQuery != "" || u.Fragment != "":
		return "", "use the address without a user name, ? or #"
	}
	return strings.TrimRight(u.Scheme+"://"+u.Host+u.Path, "/"), ""
}
