package homey

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const testKey = "test-key"

const (
	zonesAnswer = `{"z1": {"id": "z1", "name": "Kitchen", "parent": null}}`
	// A thermostat in the kitchen, and a motion sensor that is offline,
	// in a zone that is gone and with a title per language.
	devicesAnswer = `{
		"d1": {"id": "d1", "name": "Thermostat", "zone": "z1", "available": true, "capabilitiesObj": {
			"measure_temperature": {"id": "measure_temperature", "type": "number", "title": "Temperature", "value": 21.46, "units": "°C", "decimals": 1, "getable": true},
			"onoff": {"id": "onoff", "type": "boolean", "title": "Turned on", "value": true, "getable": true},
			"button": {"id": "button", "type": "boolean", "title": "Button", "value": null, "getable": false}
		}},
		"d2": {"id": "d2", "name": "Hall motion", "zone": "gone", "available": false, "capabilitiesObj": {
			"alarm_motion": {"id": "alarm_motion", "type": "boolean", "title": {"en": "Motion"}, "value": true, "decimals": 1.5}
		}}
	}`
	usersAnswer = `{
		"u1": {"id": "u1", "name": "Anna", "present": true, "asleep": false},
		"u2": {"id": "u2", "name": "Bram", "present": false},
		"u3": {"id": "u3", "name": "Cas", "present": null},
		"not valid!": {"name": "X", "present": true}
	}`
)

// fakeHomey answers like Homey's local Web API.
func fakeHomey(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+testKey {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		answers := map[string]string{zonesPath: zonesAnswer, devicesPath: devicesAnswer, usersPath: usersAnswer}
		answer, ok := answers[r.URL.Path]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(answer))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func client() *Client { return &Client{HTTP: &http.Client{Timeout: 5 * time.Second}} }

func TestCheck(t *testing.T) {
	srv := fakeHomey(t)
	if err := client().Check(context.Background(), Connection{URL: srv.URL, Key: testKey}); err != nil {
		t.Fatalf("Check with the right key: %v", err)
	}
	err := client().Check(context.Background(), Connection{URL: srv.URL, Key: "wrong"})
	if !errors.Is(err, ErrUnauthorized) || strings.Contains(err.Error(), "wrong") {
		t.Fatalf("Check with a wrong key = %v, want ErrUnauthorized without the key", err)
	}
}

func TestCheckRejectsAnotherServer(t *testing.T) {
	for name, answer := range map[string]string{
		"a router page":    `<html>router login</html>`,
		"another JSON API": `{"message": "API running."}`,
		"no zones":         `{}`,
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Write([]byte(answer))
		}))
		err := client().Check(context.Background(), Connection{URL: srv.URL, Key: testKey})
		srv.Close()
		if !errors.Is(err, ErrNotHomey) {
			t.Errorf("Check against %s = %v, want ErrNotHomey", name, err)
		}
	}
}

func TestCheckNeedsTheDevicesPermission(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == devicesPath {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Write([]byte(zonesAnswer))
	}))
	defer srv.Close()
	if err := client().Check(context.Background(), Connection{URL: srv.URL, Key: testKey}); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("Check with a key without View devices = %v, want ErrUnauthorized", err)
	}
}

func TestStates(t *testing.T) {
	srv := fakeHomey(t)
	states, err := client().States(context.Background(), Connection{URL: srv.URL, Key: testKey},
		[]string{"d1/measure_temperature", "d1/onoff", "d2/alarm_motion", "d1/gone", "gone/onoff", "../api"})
	if err != nil {
		t.Fatal(err)
	}
	one := 1
	want := map[string]State{
		"d1/measure_temperature": {Device: "Thermostat", Title: "Temperature", Capability: "measure_temperature", Value: 21.46, Units: "°C", Decimals: &one},
		"d1/onoff":               {Device: "Thermostat", Title: "Turned on", Capability: "onoff", Value: true},
		// Offline: no value. The title per language and the odd decimals
		// are left out instead of failing the answer.
		"d2/alarm_motion": {Device: "Hall motion", Title: "alarm_motion", Capability: "alarm_motion"},
	}
	if !reflect.DeepEqual(states, want) {
		t.Fatalf("States = %+v,\nwant %+v", states, want)
	}
}

func TestCapabilitiesAreReadableAndSorted(t *testing.T) {
	srv := fakeHomey(t)
	list, err := client().Capabilities(context.Background(), Connection{URL: srv.URL, Key: testKey})
	if err != nil {
		t.Fatal(err)
	}
	var keys, zones []string
	for _, c := range list {
		keys = append(keys, c.Key)
		zones = append(zones, c.Zone)
	}
	// The button cannot be read; a zone that is gone sorts first, empty.
	if want := []string{"d2/alarm_motion", "d1/measure_temperature", "d1/onoff"}; !reflect.DeepEqual(keys, want) {
		t.Fatalf("Capabilities = %v, want %v", keys, want)
	}
	if want := []string{"", "Kitchen", "Kitchen"}; !reflect.DeepEqual(zones, want) {
		t.Fatalf("zones = %v, want %v", zones, want)
	}
}

func TestUsersAndPresence(t *testing.T) {
	srv := fakeHomey(t)
	conn := Connection{URL: srv.URL, Key: testKey}
	users, err := client().Users(context.Background(), conn)
	if err != nil {
		t.Fatal(err)
	}
	want := []User{{ID: "u1", Name: "Anna", Present: true}, {ID: "u2", Name: "Bram"}, {ID: "u3", Name: "Cas"}}
	if !reflect.DeepEqual(users, want) {
		t.Fatalf("Users = %+v, want %+v", users, want)
	}
	presence, err := client().Presence(context.Background(), conn, []string{"u1", "u2", "nobody"})
	if err != nil {
		t.Fatal(err)
	}
	// Cas is not asked for, and an unknown user is left out.
	if want := map[string]bool{"u1": true, "u2": false}; !reflect.DeepEqual(presence, want) {
		t.Fatalf("Presence = %v, want %v", presence, want)
	}
}

func TestKeyDoesNotFollowRedirects(t *testing.T) {
	var leaked atomic.Bool
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			leaked.Store(true)
		}
	}))
	defer other.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL+r.URL.Path, http.StatusFound)
	}))
	defer srv.Close()
	err := client().Check(context.Background(), Connection{URL: srv.URL, Key: testKey})
	if !errors.Is(err, ErrNotHomey) || leaked.Load() {
		t.Fatalf("Check = %v, key sent on: %v; want ErrNotHomey and no request to the target", err, leaked.Load())
	}
}

func TestValidKey(t *testing.T) {
	for key, want := range map[string]bool{
		"5f2c1e9a-0b1d-4c8e-9f00-1a2b3c4d5e6f/measure_temperature": true,
		"d1/alarm_contact.window":                                  true,
		"d1/measure_power.L1":                                      true,
		"d1":                                                       false,
		"d1/":                                                      false,
		"../api/manager/users/user":                                false,
		"d1/onoff/x":                                               false,
		"d1/on off":                                                false,
	} {
		if got := ValidKey(key); got != want {
			t.Errorf("ValidKey(%q) = %v, want %v", key, got, want)
		}
	}
}
