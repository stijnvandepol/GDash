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

// sharePassphrase is made up; the tests check that it never leaves the server.
const sharePassphrase = "MadeUpShare77"

const sharedPhoto = "11-11_1633859830-xl"

// synologyStandIn answers like Synology Photos for an album shared by link
// without a password, with one photo.
func synologyStandIn() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		switch r.Form.Get("api") {
		case "SYNO.Core.Sharing.Login":
			if r.Form.Get("sharing_id") != sharePassphrase {
				json.NewEncoder(w).Encode(map[string]any{"success": false})
				return
			}
			http.SetCookie(w, &http.Cookie{Name: "sharing_sid", Value: "s", Path: "/"})
			json.NewEncoder(w).Encode(map[string]any{"success": true})
		case "SYNO.Foto.Browse.Item":
			json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{"list": []any{
				map[string]any{"id": 11, "type": "photo", "additional": map[string]any{"thumbnail": map[string]any{"cache_key": "11_1633859830", "unit_id": 11, "xl": "ready"}}},
			}}})
		case "SYNO.Foto.Thumbnail":
			w.Write(jpegBytes)
		}
	})
}

func TestSynologyAlbumIsConnectedWithoutShowingTheLink(t *testing.T) {
	env := newTestEnv(t, nil)
	nas := httptest.NewServer(synologyStandIn())
	defer nas.Close()
	env.setUp(t)
	link := nas.URL + "/mo/sharing/" + sharePassphrase
	if status, body := env.do(t, "PUT", "/api/synology", `{"link":"`+nas.URL+`/photo/","password":""}`); status != http.StatusUnprocessableEntity || !strings.Contains(body, `"field":"link"`) {
		t.Fatalf("not a share link = %d %s", status, body)
	}
	if status, body := env.do(t, "PUT", "/api/synology", `{"link":"`+nas.URL+`/mo/sharing/Wrong1","password":""}`); status != http.StatusUnprocessableEntity || !strings.Contains(body, `"field":"password"`) {
		t.Fatalf("a link the NAS refuses = %d %s", status, body)
	}
	if status, body := env.do(t, "PUT", "/api/synology", `{"link":"`+link+`","password":"","acceptCertificate":true}`); status != http.StatusUnprocessableEntity || !strings.Contains(body, `"field":"acceptCertificate"`) {
		t.Fatalf("accepting the certificate of an http link = %d %s", status, body)
	}
	status, body := env.do(t, "PUT", "/api/synology", `{"link":"`+link+`","password":""}`)
	if status != http.StatusOK || !strings.Contains(body, `"photos":1`) || strings.Contains(body, sharePassphrase) {
		t.Fatalf("connect = %d %s", status, body)
	}
	if _, settings := env.do(t, "GET", "/api/settings", ""); strings.Contains(settings, sharePassphrase) || !strings.Contains(settings, `"synologyPhotos":{"host"`) {
		t.Fatalf("settings = %s", settings)
	}
	if status, _ := env.do(t, "DELETE", "/api/synology", ""); status != http.StatusNoContent {
		t.Fatalf("disconnect = %d", status)
	}
}

func TestASelfSignedNASIsUsedOnlyWhenItsCertificateIsAccepted(t *testing.T) {
	env := newTestEnv(t, nil)
	nas := httptest.NewTLSServer(synologyStandIn())
	defer nas.Close()
	env.setUp(t)
	link := nas.URL + "/mo/sharing/" + sharePassphrase
	if status, body := env.do(t, "PUT", "/api/synology", `{"link":"`+link+`","password":""}`); status != http.StatusUnprocessableEntity || !strings.Contains(body, `"field":"acceptCertificate"`) {
		t.Fatalf("self-signed without accepting = %d %s", status, body)
	}
	status, body := env.do(t, "PUT", "/api/synology", `{"link":"`+link+`","password":"","acceptCertificate":true}`)
	if status != http.StatusOK || !strings.Contains(body, `"certificate":"`) {
		t.Fatalf("self-signed, accepted = %d %s", status, body)
	}
}

func TestSynologyPhotosAreOnlyForScreensThatShowTheAlbum(t *testing.T) {
	env := newTestEnv(t, nil)
	nas := httptest.NewServer(synologyStandIn())
	defer nas.Close()
	env.setUp(t)
	env.createKitchen(t)
	env.do(t, "POST", "/api/displays", `{"id":"hal","name":"Hal"}`)
	env.do(t, "PUT", "/api/synology", `{"link":"`+nas.URL+`/mo/sharing/`+sharePassphrase+`","password":""}`)
	if status, body := env.do(t, "POST", "/api/dashboards", `{"id":"nas","name":"NAS","widgets":[
		{"id":"photos","type":"photos","x":0,"y":0,"w":8,"h":6,"settings":{"source":"synology"}}]}`); status != http.StatusCreated {
		t.Fatalf("create dashboard = %d %s", status, body)
	}
	env.do(t, "PATCH", "/api/displays/keuken", `{"dashboardId":"nas"}`)
	env.sources.set(nil, nil, map[string]datahub.Entry{display.SynologySource: {Value: json.RawMessage(`["` + sharedPhoto + `"]`)}})

	url := env.srv.URL + "/api/synology/photos/" + sharedPhoto
	if status, _, _ := getImage(t, tv(), url); status != http.StatusNotFound {
		t.Fatalf("photo for a stranger = %d, want 404", status)
	}
	if status, _, _ := getImage(t, pairedScreenOf(t, env, "hal"), url); status != http.StatusNotFound {
		t.Fatalf("photo for a screen without the album = %d, want 404", status)
	}
	if status, header, _ := getImage(t, pairedScreen(t, env), url); status != http.StatusOK || header.Get("Content-Type") != "image/jpeg" {
		t.Fatalf("photo for the kitchen screen = %d %v", status, header)
	}
	if status, _, _ := getImage(t, env.client, env.srv.URL+"/api/synology/photos/12-12_1633859831-xl"); status != http.StatusNotFound {
		t.Fatalf("a photo outside the album's list = %d, want 404", status)
	}
}
