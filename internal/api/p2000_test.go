package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestP2000PlaceTest(t *testing.T) {
	env := newTestEnv(t, nil)
	if status, _ := env.do(t, "POST", "/api/p2000-place-tests", `{"name":"Nuenen"}`); status != http.StatusUnauthorized {
		t.Fatalf("test without login = %d, want 401", status)
	}
	env.setUp(t)

	status, body := env.do(t, "POST", "/api/p2000-place-tests", `{"name":"Nuenen"}`)
	if status != http.StatusOK {
		t.Fatalf("test = %d %s", status, body)
	}
	var place map[string]string
	if err := json.Unmarshal([]byte(body), &place); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"name": "Nuenen", "slug": "nuenen", "region": "brabant-zuidoost", "regionName": "Brabant-Zuidoost"}
	for k, v := range want {
		if place[k] != v {
			t.Fatalf("place = %v, want %v", place, want)
		}
	}

	for _, tc := range []struct {
		name   string
		status int
		code   string
	}{
		{"Nergenshuizen", http.StatusBadGateway, "place_unknown"},
		{"Stilledorp", http.StatusBadGateway, "place_without_alerts"},
		{"Storing", http.StatusBadGateway, "fetch_failed"},
		{"N", http.StatusUnprocessableEntity, "invalid_input"},
	} {
		status, body := env.do(t, "POST", "/api/p2000-place-tests", `{"name":"`+tc.name+`"}`)
		var answer struct {
			Error struct{ Code string }
		}
		_ = json.Unmarshal([]byte(body), &answer)
		if status != tc.status || answer.Error.Code != tc.code {
			t.Errorf("test of %s = %d %s, want %d %s", tc.name, status, body, tc.status, tc.code)
		}
	}
}
