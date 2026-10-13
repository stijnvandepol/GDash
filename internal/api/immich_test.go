package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stijnvandepol/gdash/internal/datahub"
	"github.com/stijnvandepol/gdash/internal/display"
)

// immichKey is made up; the tests check that it never leaves the server.
const immichKey = "made-up-immich-api-key-31"

const (
	immichAlbum = "4b1c5d6e-7f80-4912-a3b4-c5d6e7f80912"
	inAlbum     = "11111111-2222-4333-8444-555555555555"
	notInAlbum  = "66666666-7777-4888-9999-aaaaaaaaaaaa"
)

// immichStandIn answers like Immich for one album; every asset is the
// same small JPEG.
func immichStandIn(t *testing.T) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Header.Get("x-api-key") != immichKey:
			w.WriteHeader(http.StatusUnauthorized)
		case r.URL.Path == "/api/albums":
			json.NewEncoder(w).Encode([]map[string]any{{"id": immichAlbum, "albumName": "Holidays", "assetCount": 1}})
		case strings.HasPrefix(r.URL.Path, "/api/assets/") && strings.HasSuffix(r.URL.Path, "/thumbnail"):
			w.Write(jpegBytes)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestImmichIsConnectedWithoutShowingTheKey(t *testing.T) {
	env := newTestEnv(t, nil)
	immich := immichStandIn(t)
	env.setUp(t)
	if status, body := env.do(t, "PUT", "/api/immich", `{"url":"`+immich.URL+`","key":"wrong-key"}`); status != http.StatusUnprocessableEntity || !strings.Contains(body, `"field":"key"`) {
		t.Fatalf("wrong key = %d %s", status, body)
	}
	if status, body := env.do(t, "GET", "/api/immich/albums", ""); status != http.StatusConflict {
		t.Fatalf("albums without a connection = %d %s", status, body)
	}
	if status, body := env.do(t, "PUT", "/api/immich", `{"url":"`+immich.URL+`/api","key":"`+immichKey+`"}`); status != http.StatusOK || strings.Contains(body, immichKey) {
		t.Fatalf("connect = %d %s", status, body)
	}
	status, body := env.do(t, "GET", "/api/immich/albums", "")
	if status != http.StatusOK || !strings.Contains(body, `"name":"Holidays"`) {
		t.Fatalf("albums = %d %s", status, body)
	}
	if status, body := env.do(t, "PUT", "/api/immich/album", `{"albumId":"`+notInAlbum+`"}`); status != http.StatusUnprocessableEntity {
		t.Fatalf("an album Immich does not list = %d %s", status, body)
	}
	if status, body := env.do(t, "PUT", "/api/immich/album", `{"albumId":"`+immichAlbum+`"}`); status != http.StatusOK || !strings.Contains(body, `"albumName":"Holidays"`) {
		t.Fatalf("choose album = %d %s", status, body)
	}
	_, settings := env.do(t, "GET", "/api/settings", "")
	if !strings.Contains(settings, `"albumId":"`+immichAlbum+`"`) || strings.Contains(settings, immichKey) {
		t.Fatalf("settings = %s", settings)
	}
	if status, _ := env.do(t, "DELETE", "/api/immich", ""); status != http.StatusNoContent {
		t.Fatalf("disconnect = %d", status)
	}
}

func TestImmichPhotosAreOnlyForScreensThatShowTheAlbum(t *testing.T) {
	env := newTestEnv(t, nil)
	immich := immichStandIn(t)
	env.setUp(t)
	env.createKitchen(t)
	env.do(t, "POST", "/api/displays", `{"id":"hal","name":"Hal"}`)
	env.do(t, "PUT", "/api/immich", `{"url":"`+immich.URL+`","key":"`+immichKey+`"}`)
	env.do(t, "PUT", "/api/immich/album", `{"albumId":"`+immichAlbum+`"}`)
	if status, body := env.do(t, "POST", "/api/dashboards", `{"id":"album","name":"Album","widgets":[
		{"id":"photos","type":"photos","x":0,"y":0,"w":8,"h":6,"settings":{"source":"immich"}}]}`); status != http.StatusCreated {
		t.Fatalf("create dashboard = %d %s", status, body)
	}
	env.do(t, "PATCH", "/api/displays/keuken", `{"dashboardId":"album"}`)
	env.sources.set(nil, nil, map[string]datahub.Entry{display.ImmichSource: {Value: json.RawMessage(`["` + inAlbum + `"]`)}})

	url := env.srv.URL + "/api/immich/assets/" + inAlbum
	if status, _, _ := getImage(t, tv(), url); status != http.StatusNotFound {
		t.Fatalf("photo for a stranger = %d, want 404", status)
	}
	if status, _, _ := getImage(t, pairedScreenOf(t, env, "hal"), url); status != http.StatusNotFound {
		t.Fatalf("photo for a screen without the album = %d, want 404", status)
	}
	kitchen := pairedScreen(t, env)
	if status, header, _ := getImage(t, kitchen, url); status != http.StatusOK || header.Get("Content-Type") != "image/jpeg" {
		t.Fatalf("photo for the kitchen screen = %d %v", status, header)
	}
	if status, _, _ := getImage(t, env.client, url); status != http.StatusOK {
		t.Fatalf("photo for the admin = %d", status)
	}
	// Immich would serve it, but it is not one of the album's photos.
	if status, _, _ := getImage(t, env.client, env.srv.URL+"/api/immich/assets/"+notInAlbum); status != http.StatusNotFound {
		t.Fatalf("a photo outside the album = %d, want 404", status)
	}
}
