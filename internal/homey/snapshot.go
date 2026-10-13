package homey

import (
	"encoding/json"
	"errors"
	"slices"
	"unicode/utf8"
)

// Limits of a snapshot. A large home has a few hundred devices; the limits
// keep what is stored and sent to the editor bounded whatever a script
// sends.
const (
	maxSnapshotDevices = 500
	maxSnapshotUsers   = 50
	maxSnapshotCaps    = 64
	maxSnapshotText    = 200
)

// ErrNotSnapshot means a body is JSON, but not a snapshot: it has no list
// of devices.
var ErrNotSnapshot = errors.New("not a homey snapshot")

// Snapshot is what a Homey without API keys, such as a Homey Pro (Early
// 2019), sends with HomeyScript: every device with its capabilities, and its
// users. ParseSnapshot makes one; its JSON is the same shape.
type Snapshot struct {
	Devices []SnapshotDevice `json:"devices"`
	Users   []SnapshotUser   `json:"users"`
}

// SnapshotDevice is a device as the script sends it, with the name of its
// zone instead of the zone's ID.
type SnapshotDevice struct {
	ID        string                        `json:"id"`
	Name      string                        `json:"name"`
	Zone      string                        `json:"zone,omitempty"`
	Available bool                          `json:"available"`
	Caps      map[string]SnapshotCapability `json:"caps"`
}

// SnapshotCapability is a capability's current value.
type SnapshotCapability struct {
	Title string `json:"title,omitempty"`
	// Value is a number, a boolean, a text or null.
	Value any    `json:"value"`
	Units string `json:"units,omitempty"`
}

// SnapshotUser is a Homey user and whether that user is home.
type SnapshotUser struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Present bool   `json:"present"`
	Asleep  bool   `json:"asleep"`
}

// The shapes a snapshot is read with. Each device and user is read on its
// own, so one odd entry is left out instead of refusing the whole snapshot.
type (
	snapshotIn struct {
		Devices *[]json.RawMessage `json:"devices"`
		Users   []json.RawMessage  `json:"users"`
	}
	deviceIn struct {
		ID        text                       `json:"id"`
		Name      text                       `json:"name"`
		Zone      text                       `json:"zone"`
		Available flag                       `json:"available"`
		Caps      map[string]json.RawMessage `json:"caps"`
	}
	capabilityIn struct {
		Title text `json:"title"`
		Value any  `json:"value"`
		Units text `json:"units"`
	}
	userIn struct {
		ID      text `json:"id"`
		Name    text `json:"name"`
		Present flag `json:"present"`
		Asleep  flag `json:"asleep"`
	}
)

// flag is a JSON field that is normally a boolean; anything else, null
// included, reads as absent, for the same reason as text.
type flag struct{ value, set bool }

func (f *flag) UnmarshalJSON(b []byte) error {
	f.set = string(b) != "null" && json.Unmarshal(b, &f.value) == nil
	if !f.set {
		f.value = false
	}
	return nil
}

// ParseSnapshot reads a snapshot leniently: devices, capabilities and users
// with an invalid or repeated ID are left out, texts are cut to 200
// characters, values that are not a number, boolean or text become null,
// and only the first 500 devices, 64 capabilities of each (by ID) and 50
// users are kept. It returns ErrNotSnapshot when raw is not a JSON object
// with a list of devices.
func ParseSnapshot(raw []byte) (Snapshot, error) {
	var in snapshotIn
	if err := json.Unmarshal(raw, &in); err != nil || in.Devices == nil {
		return Snapshot{}, ErrNotSnapshot
	}
	out := Snapshot{Devices: []SnapshotDevice{}, Users: []SnapshotUser{}}
	seenDevices, seenUsers := map[string]bool{}, map[string]bool{}
	for _, item := range *in.Devices {
		if len(out.Devices) == maxSnapshotDevices {
			break
		}
		var d deviceIn
		if json.Unmarshal(item, &d) != nil {
			continue
		}
		id := string(d.ID)
		// A device ID has the same form as a user ID: a UUID.
		if !ValidUserID(id) || seenDevices[id] {
			continue
		}
		seenDevices[id] = true
		out.Devices = append(out.Devices, SnapshotDevice{
			ID: id, Name: clip(string(d.Name)), Zone: clip(string(d.Zone)),
			Available: !d.Available.set || d.Available.value, Caps: snapshotCaps(id, d.Caps),
		})
	}
	for _, item := range in.Users {
		if len(out.Users) == maxSnapshotUsers {
			break
		}
		var u userIn
		if json.Unmarshal(item, &u) != nil {
			continue
		}
		id := string(u.ID)
		if !ValidUserID(id) || seenUsers[id] {
			continue
		}
		seenUsers[id] = true
		out.Users = append(out.Users, SnapshotUser{
			ID: id, Name: clip(string(u.Name)), Present: u.Present.value, Asleep: u.Asleep.value,
		})
	}
	return out, nil
}

// snapshotCaps reads the capabilities of device id: the first ones by ID
// whose key is valid.
func snapshotCaps(id string, caps map[string]json.RawMessage) map[string]SnapshotCapability {
	ids := make([]string, 0, len(caps))
	for capID := range caps {
		if ValidKey(id + keySeparator + capID) {
			ids = append(ids, capID)
		}
	}
	slices.Sort(ids)
	out := make(map[string]SnapshotCapability, min(len(ids), maxSnapshotCaps))
	for _, capID := range ids {
		if len(out) == maxSnapshotCaps {
			break
		}
		var c capabilityIn
		if json.Unmarshal(caps[capID], &c) != nil {
			continue
		}
		value := plainValue(c.Value)
		if s, ok := value.(string); ok {
			value = clip(s)
		}
		out[capID] = SnapshotCapability{Title: clip(string(c.Title)), Value: value, Units: clip(string(c.Units))}
	}
	return out
}

// clip cuts s to maxSnapshotText characters.
func clip(s string) string {
	if utf8.RuneCountInString(s) <= maxSnapshotText {
		return s
	}
	return string([]rune(s)[:maxSnapshotText])
}

// devices returns the snapshot's devices as the Web API describes them,
// with the zone's name in Zone, so both share how a state is made.
func (s Snapshot) devices() map[string]apiDevice {
	out := make(map[string]apiDevice, len(s.Devices))
	for _, d := range s.Devices {
		available := d.Available
		caps := make(map[string]apiCapability, len(d.Caps))
		for id, c := range d.Caps {
			caps[id] = apiCapability{Title: text(c.Title), Value: c.Value, Units: text(c.Units)}
		}
		out[d.ID] = apiDevice{Name: text(d.Name), Zone: text(d.Zone), Available: &available, Caps: caps}
	}
	return out
}

// States returns the state of every capability by its key, as the polled
// States does for the keys it is asked for.
func (s Snapshot) States() map[string]State {
	out := map[string]State{}
	for deviceID, device := range s.devices() {
		for capID, capability := range device.Caps {
			out[deviceID+keySeparator+capID] = device.state(capID, capability)
		}
	}
	return out
}

// Capabilities lists every capability for the widget editor, like the
// polled Capabilities.
func (s Snapshot) Capabilities() []Capability {
	return capabilityList(s.devices(), func(d apiDevice) string { return string(d.Zone) })
}

// UserList lists the users, sorted by name, like the polled Users.
func (s Snapshot) UserList() []User {
	all := make(map[string]apiUser, len(s.Users))
	for _, u := range s.Users {
		present := u.Present
		all[u.ID] = apiUser{Name: text(u.Name), Present: &present}
	}
	return userList(all)
}

// Presence returns whether each user is home, by ID.
func (s Snapshot) Presence() map[string]bool {
	out := make(map[string]bool, len(s.Users))
	for _, u := range s.Users {
		out[u.ID] = u.Present
	}
	return out
}
