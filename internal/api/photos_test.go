package api

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"github.com/stijnvandepol/gdash/internal/pairing"
	"github.com/stijnvandepol/gdash/internal/photos"
)

var jpegBytes = append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 200)...)

// upload sends data as the "photo" field of a multipart form.
func upload(t *testing.T, client *http.Client, url string, data []byte) (int, string) {
	t.Helper()
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	part, err := form.CreateFormFile("photo", "holiday.jpg")
	if err != nil {
		t.Fatal(err)
	}
	part.Write(data)
	form.Close()
	req, err := http.NewRequest("POST", url, &body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", form.FormDataContentType())
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func get(t *testing.T, client *http.Client, url string) (int, http.Header) {
	t.Helper()
	resp, err := client.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp.StatusCode, resp.Header
}

// pairedScreen returns a client paired with the kitchen display.
func pairedScreen(t *testing.T, env *testEnv) *http.Client {
	t.Helper()
	screen := tv()
	_, body := post(t, screen, env.srv.URL+"/api/pairing-requests", `{"displayId":"keuken"}`)
	var req pairing.Request
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatal(err)
	}
	if status, body := env.do(t, "POST", "/api/displays/keuken/pairings", `{"code":"`+req.Code+`"}`); status != http.StatusCreated {
		t.Fatalf("approve = %d %s", status, body)
	}
	if status, body := post(t, screen, env.srv.URL+"/api/display-sessions", `{"pollToken":"`+req.PollToken+`"}`); status != http.StatusCreated {
		t.Fatalf("claim = %d %s", status, body)
	}
	return screen
}

func TestPhotosAreOnlyForAdminsAndPairedScreens(t *testing.T) {
	env := newTestEnv(t, nil)
	if status, _ := upload(t, tv(), env.srv.URL+"/api/photos", jpegBytes); status != http.StatusUnauthorized {
		t.Fatalf("upload without login = %d, want 401", status)
	}
	env.setUp(t)
	env.createKitchen(t)
	status, body := upload(t, env.client, env.srv.URL+"/api/photos", jpegBytes)
	var photo photos.Photo
	if status != http.StatusCreated || json.Unmarshal([]byte(body), &photo) != nil || !strings.HasSuffix(photo.Name, ".jpg") {
		t.Fatalf("upload = %d %s", status, body)
	}
	url := env.srv.URL + "/api/photos/" + photo.Name

	if status, _ := get(t, tv(), url); status != http.StatusNotFound {
		t.Fatalf("photo for a stranger on the network = %d, want 404", status)
	}
	if status, header := get(t, env.client, url); status != http.StatusOK || header.Get("Content-Type") != "image/jpeg" {
		t.Fatalf("photo for the admin = %d %s", status, header.Get("Content-Type"))
	}
	if status, _ := get(t, pairedScreen(t, env), url); status != http.StatusOK {
		t.Fatalf("photo for a paired screen = %d, want 200", status)
	}

	if _, list := env.do(t, "GET", "/api/photos", ""); !strings.Contains(list, photo.Name) {
		t.Fatalf("list = %s", list)
	}
	if status, _ := env.do(t, "DELETE", "/api/photos/"+photo.Name, ""); status != http.StatusNoContent {
		t.Fatalf("delete = %d", status)
	}
	if status, _ := get(t, env.client, url); status != http.StatusNotFound {
		t.Fatalf("deleted photo = %d, want 404", status)
	}
}

func TestPhotoUploadRejectsOtherFiles(t *testing.T) {
	env := newTestEnv(t, nil)
	env.setUp(t)
	status, body := upload(t, env.client, env.srv.URL+"/api/photos", []byte("<svg onload=alert(1)>"))
	if status != http.StatusUnprocessableEntity || !strings.Contains(body, `"field":"photo"`) {
		t.Fatalf("SVG upload = %d %s", status, body)
	}
	big := append(append([]byte{}, jpegBytes...), make([]byte, photos.MaxBytes)...)
	if status, body = upload(t, env.client, env.srv.URL+"/api/photos", big); status != http.StatusUnprocessableEntity {
		t.Fatalf("oversized upload = %d %s", status, body)
	}
	if _, list := env.do(t, "GET", "/api/photos", ""); list != `{"photos":[]}`+"\n" {
		t.Fatalf("photos after rejected uploads = %s", list)
	}
}
