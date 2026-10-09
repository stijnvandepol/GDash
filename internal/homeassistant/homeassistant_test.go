package homeassistant

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const testToken = "test-token"

// fakeHomeAssistant answers like Home Assistant's REST API for two entities.
func fakeHomeAssistant(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+testToken {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/":
			w.Write([]byte(`{"message":"API running."}`))
		case "/api/states":
			w.Write([]byte(`[{"entity_id":"sensor.temp","state":"21.5","attributes":{"friendly_name":"Living room","unit_of_measurement":"°C","device_class":"temperature"}},
				{"entity_id":"binary_sensor.door","state":"off","attributes":{"device_class":"door"}},
				{"entity_id":"Not valid","state":"x","attributes":{}}]`))
		case "/api/states/sensor.temp":
			w.Write([]byte(`{"entity_id":"sensor.temp","state":"21.5","attributes":{"friendly_name":"Living room","unit_of_measurement":"°C","device_class":"temperature"}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func client() *Client { return &Client{HTTP: &http.Client{Timeout: 5 * time.Second}} }

func TestCheck(t *testing.T) {
	srv := fakeHomeAssistant(t)
	if err := client().Check(context.Background(), Connection{URL: srv.URL, Token: testToken}); err != nil {
		t.Fatalf("Check with the right token: %v", err)
	}
	err := client().Check(context.Background(), Connection{URL: srv.URL, Token: "wrong"})
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("Check with a wrong token = %v, want ErrUnauthorized", err)
	}
}

func TestCheckRejectsAnotherServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`<html>router login</html>`))
	}))
	defer srv.Close()
	err := client().Check(context.Background(), Connection{URL: srv.URL, Token: testToken})
	if !errors.Is(err, ErrNotHomeAssistant) {
		t.Fatalf("Check = %v, want ErrNotHomeAssistant", err)
	}
}

func TestStatesLeavesOutUnknownEntities(t *testing.T) {
	srv := fakeHomeAssistant(t)
	states, err := client().States(context.Background(), Connection{URL: srv.URL, Token: testToken},
		[]string{"sensor.temp", "sensor.gone", "../api/config"})
	if err != nil {
		t.Fatal(err)
	}
	want := State{State: "21.5", Name: "Living room", Unit: "°C", DeviceClass: "temperature"}
	if len(states) != 1 || states["sensor.temp"] != want {
		t.Fatalf("States = %+v, want only sensor.temp as %+v", states, want)
	}
}

func TestStatesFailOnAWrongToken(t *testing.T) {
	srv := fakeHomeAssistant(t)
	_, err := client().States(context.Background(), Connection{URL: srv.URL, Token: "wrong"}, []string{"sensor.temp"})
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("States = %v, want ErrUnauthorized", err)
	}
	if strings.Contains(err.Error(), "wrong") {
		t.Fatalf("error %q contains the token", err)
	}
}

func TestEntitiesAreSortedAndNamed(t *testing.T) {
	srv := fakeHomeAssistant(t)
	list, err := client().Entities(context.Background(), Connection{URL: srv.URL, Token: testToken})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].ID != "binary_sensor.door" || list[1].ID != "sensor.temp" {
		t.Fatalf("Entities = %+v, want the two valid entities sorted", list)
	}
	if list[0].Name != "binary_sensor.door" {
		t.Errorf("an entity without a friendly name is named %q, want its ID", list[0].Name)
	}
}

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		in, want string
		ok       bool
	}{
		{" http://192.168.1.10:8123/ ", "http://192.168.1.10:8123", true},
		{"https://ha.example.org/sub/", "https://ha.example.org/sub", true},
		{"homeassistant.local:8123", "", false},
		{"ftp://ha", "", false},
		{"http://user:pw@ha:8123", "", false},
		{"http://ha:8123/?x=1", "", false},
	}
	for _, tt := range tests {
		got, msg := NormalizeURL(tt.in)
		if got != tt.want || (msg == "") != tt.ok {
			t.Errorf("NormalizeURL(%q) = %q, %q; want %q, ok %v", tt.in, got, msg, tt.want, tt.ok)
		}
	}
}
