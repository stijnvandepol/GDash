package homey

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// scriptSnapshot is what the HomeyScript in the guide sends.
const scriptSnapshot = `{"at": "2026-10-13T08:00:00.000Z",
	"devices": [
		{"id": "d1", "name": "Thermostat", "zone": "Kitchen", "available": true,
		 "caps": {"measure_temperature": {"title": "Temperature", "value": 21.5, "units": "°C"},
		          "target_temperature": {"title": "Target", "value": 20, "units": null}}},
		{"id": "d2", "name": "Back door", "zone": null, "available": false,
		 "caps": {"alarm_contact": {"title": "Contact", "value": true, "units": null}}}
	],
	"users": [
		{"id": "u1", "name": "Anna", "present": true, "asleep": false},
		{"id": "u2", "name": "Bram", "present": false, "asleep": false}
	]}`

func TestParseSnapshotReadsWhatTheScriptSends(t *testing.T) {
	s, err := ParseSnapshot([]byte(scriptSnapshot))
	if err != nil {
		t.Fatal(err)
	}
	states := s.States()
	if got := states["d1/measure_temperature"]; got.Value != 21.5 || got.Units != "°C" || got.Device != "Thermostat" || got.Title != "Temperature" {
		t.Errorf("temperature = %+v", got)
	}
	// An unavailable device shows no value, as when Homey is asked.
	if got, ok := states["d2/alarm_contact"]; !ok || got.Value != nil {
		t.Errorf("unavailable door = %+v, %v; want it without a value", got, ok)
	}
	caps := s.Capabilities()
	if len(caps) != 3 || caps[0].Key != "d2/alarm_contact" || caps[0].Zone != "" || caps[1].Zone != "Kitchen" {
		t.Errorf("capabilities = %+v, sorted by zone, device and title", caps)
	}
	if users := s.UserList(); len(users) != 2 || users[0].Name != "Anna" || !users[0].Present {
		t.Errorf("users = %+v", users)
	}
	if p := s.Presence(); !p["u1"] || p["u2"] || len(p) != 2 {
		t.Errorf("presence = %v", p)
	}

	// The stored form reads back the same.
	stored, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	again, err := ParseSnapshot(stored)
	if err != nil {
		t.Fatal(err)
	}
	if again2, _ := json.Marshal(again); string(again2) != string(stored) {
		t.Errorf("stored snapshot changed on reading:\n%s\n%s", stored, again2)
	}
}

func TestParseSnapshotLeavesOutOddEntries(t *testing.T) {
	long := strings.Repeat("é", 300)
	raw := `{"devices": [
		"not a device",
		{"id": "bad id/with slash", "name": "X", "caps": {}},
		{"id": "d1", "name": "` + long + `", "caps": {"onoff": {"value": {"nested": true}}, "bad cap!": {"value": 1}, "dim": "odd"}},
		{"id": "d1", "name": "Again", "caps": {}},
		{"id": "d3", "name": "Caps not an object", "caps": [1]}
	], "users": [{"id": "u1", "name": 7, "present": "yes"}, {"name": "No id"}, 3]}`
	s, err := ParseSnapshot([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Devices) != 1 || s.Devices[0].ID != "d1" || len([]rune(s.Devices[0].Name)) != 200 {
		t.Fatalf("devices = %+v, want d1 only with its name cut", s.Devices)
	}
	if caps := s.Devices[0].Caps; len(caps) != 1 || caps["onoff"].Value != nil {
		t.Errorf("caps = %+v, want onoff without its odd value", caps)
	}
	if len(s.Users) != 1 || s.Users[0].Name != "" || s.Users[0].Present {
		t.Errorf("users = %+v, want u1, not present", s.Users)
	}
	if users := s.UserList(); users[0].Name != "u1" {
		t.Errorf("a user without a name = %+v, want the ID as name", users)
	}
}

func TestParseSnapshotCapsWhatItKeeps(t *testing.T) {
	var devices, users []string
	caps := make([]string, 0, 70)
	for i := range 70 {
		caps = append(caps, fmt.Sprintf(`"c%02d": {"value": %d}`, i, i))
	}
	for i := range 520 {
		devices = append(devices, fmt.Sprintf(`{"id": "d%d", "name": "D", "caps": {%s}}`, i, strings.Join(caps, ",")))
	}
	for i := range 60 {
		users = append(users, fmt.Sprintf(`{"id": "u%d", "name": "U"}`, i))
	}
	s, err := ParseSnapshot([]byte(`{"devices": [` + strings.Join(devices, ",") + `], "users": [` + strings.Join(users, ",") + `]}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Devices) != maxSnapshotDevices || len(s.Users) != maxSnapshotUsers || len(s.Devices[0].Caps) != maxSnapshotCaps {
		t.Fatalf("kept %d devices, %d users, %d caps", len(s.Devices), len(s.Users), len(s.Devices[0].Caps))
	}
	if _, ok := s.Devices[0].Caps["c63"]; !ok {
		t.Error("the first capabilities by ID were not the ones kept")
	}
}

func TestParseSnapshotRefusesOtherJSON(t *testing.T) {
	for _, raw := range []string{`[]`, `{}`, `{"devices": {}}`, `{"devices": "x"}`, `"text"`, `{"home": {"Anna": true}}`} {
		if _, err := ParseSnapshot([]byte(raw)); !errors.Is(err, ErrNotSnapshot) {
			t.Errorf("ParseSnapshot(%s) = %v, want ErrNotSnapshot", raw, err)
		}
	}
	if s, err := ParseSnapshot([]byte(`{"devices": []}`)); err != nil || len(s.Devices) != 0 || s.Users == nil {
		t.Errorf("an empty home = %+v, %v", s, err)
	}
}
