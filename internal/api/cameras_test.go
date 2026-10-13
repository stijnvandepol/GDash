package api

import (
	"bytes"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// cameraPassword is made up; the tests check that it never leaves the server.
const cameraPassword = "made-up-camera-password-77"

var pngImage = func() []byte {
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		panic(err)
	}
	return b.Bytes()
}()

// cameraStandIn is a camera on the LAN with basic auth. It answers with
// the PNG, or with what reply holds when set.
type cameraStandIn struct {
	srv      *httptest.Server
	requests atomic.Int32
	reply    atomic.Value // []byte
	broken   atomic.Bool
}

func newCameraStandIn(t *testing.T) *cameraStandIn {
	c := &cameraStandIn{}
	c.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.requests.Add(1)
		if user, pass, ok := r.BasicAuth(); !ok || user != "admin" || pass != cameraPassword {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if c.broken.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if reply, ok := c.reply.Load().([]byte); ok {
			w.Write(reply)
			return
		}
		w.Write(pngImage)
	}))
	t.Cleanup(c.srv.Close)
	return c
}

// setUpCamera adds the stand-in as camera 1 and shows it on the kitchen
// display, refreshed every 10 seconds; the hall display shows no camera.
func setUpCamera(t *testing.T, env *testEnv, cam *cameraStandIn) {
	t.Helper()
	env.setUp(t)
	env.createKitchen(t)
	if status, body := env.do(t, "POST", "/api/displays", `{"id":"hal","name":"Hal"}`); status != http.StatusCreated {
		t.Fatalf("create hall = %d %s", status, body)
	}
	status, body := env.do(t, "POST", "/api/cameras",
		`{"name":"Front door","url":"`+cam.srv.URL+`/snap.jpg","auth":{"kind":"basic","name":"admin","secret":"`+cameraPassword+`"}}`)
	if status != http.StatusCreated || !strings.Contains(body, `"id":1`) || strings.Contains(body, cameraPassword) {
		t.Fatalf("add camera = %d %s", status, body)
	}
	if status, body := env.do(t, "POST", "/api/dashboards", `{"id":"cams","name":"Cams","widgets":[
		{"id":"door","type":"camera","x":0,"y":0,"w":8,"h":6,"settings":{"camera":1,"seconds":10,"fit":"cover"}}]}`); status != http.StatusCreated {
		t.Fatalf("create dashboard = %d %s", status, body)
	}
	if status, body := env.do(t, "PATCH", "/api/displays/keuken", `{"dashboardId":"cams"}`); status != http.StatusOK {
		t.Fatalf("show dashboard = %d %s", status, body)
	}
}

func getImage(t *testing.T, client *http.Client, url string) (int, http.Header, []byte) {
	t.Helper()
	resp, err := client.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header, body
}

func TestCameraImageIsOnlyForAdminsAndScreensThatShowIt(t *testing.T) {
	env := newTestEnv(t, nil)
	cam := newCameraStandIn(t)
	setUpCamera(t, env, cam)
	url := env.srv.URL + "/api/cameras/1/image"

	if status, _, body := getImage(t, tv(), url); status != http.StatusNotFound || bytes.Equal(body, pngImage) {
		t.Fatalf("image for a stranger on the network = %d, want 404", status)
	}
	if status, _, _ := getImage(t, pairedScreenOf(t, env, "hal"), url); status != http.StatusNotFound {
		t.Fatalf("image for a screen of a display without the camera = %d, want 404", status)
	}
	kitchen := pairedScreen(t, env)
	status, header, body := getImage(t, kitchen, url)
	if status != http.StatusOK || !bytes.Equal(body, pngImage) || header.Get("Content-Type") != "image/png" ||
		header.Get("Cache-Control") != "no-store" || header.Get("X-Image-Stale") != "" || header.Get("Last-Modified") == "" {
		t.Fatalf("image for the kitchen screen = %d %v", status, header)
	}
	if status, _, _ := getImage(t, env.client, url); status != http.StatusOK {
		t.Fatalf("image for the admin = %d, want 200", status)
	}
	if status, _, _ := getImage(t, kitchen, env.srv.URL+"/api/cameras/2/image"); status != http.StatusNotFound {
		t.Fatalf("an unknown camera = %d, want 404", status)
	}

	// The display stops showing it: its screen loses the image.
	env.do(t, "POST", "/api/dashboards", `{"id":"plain","name":"Plain","widgets":[]}`)
	if status, body := env.do(t, "PATCH", "/api/displays/keuken", `{"dashboardId":"plain"}`); status != http.StatusOK {
		t.Fatalf("show another dashboard = %d %s", status, body)
	}
	if status, _, _ := getImage(t, kitchen, url); status != http.StatusNotFound {
		t.Fatalf("image after the dashboard changed = %d, want 404", status)
	}
}

func TestCameraIsAskedOncePerRefreshAndGoesStale(t *testing.T) {
	env := newTestEnv(t, nil)
	cam := newCameraStandIn(t)
	setUpCamera(t, env, cam)
	url := env.srv.URL + "/api/cameras/1/image"
	kitchen, second := pairedScreen(t, env), pairedScreen(t, env)

	for _, screen := range []*http.Client{kitchen, second, kitchen} {
		if status, _, _ := getImage(t, screen, url); status != http.StatusOK {
			t.Fatalf("image = %d", status)
		}
	}
	if n := cam.requests.Load(); n != 1 {
		t.Fatalf("the camera was asked %d times for 3 screen requests within 10 seconds, want 1", n)
	}

	cam.broken.Store(true)
	env.cameraClock.add(10 * time.Second)
	status, header, body := getImage(t, kitchen, url)
	if status != http.StatusOK || !bytes.Equal(body, pngImage) || header.Get("X-Image-Stale") != "true" {
		t.Fatalf("image of a failing camera = %d stale %q", status, header.Get("X-Image-Stale"))
	}
	if lastModified, _ := http.ParseTime(header.Get("Last-Modified")); !lastModified.Equal(env.cameraClock.Now().Add(-10 * time.Second)) {
		t.Fatalf("Last-Modified = %v, want when the last image arrived", lastModified)
	}
}

func TestCameraWithoutAnImageAnswers502AndTheTestSaysWhy(t *testing.T) {
	env := newTestEnv(t, nil)
	cam := newCameraStandIn(t)
	cam.reply.Store([]byte("<html>login</html>"))
	setUpCamera(t, env, cam)
	if status, _, _ := getImage(t, env.client, env.srv.URL+"/api/cameras/1/image"); status != http.StatusBadGateway {
		t.Fatalf("image of a camera that sends HTML = %d, want 502", status)
	}
	status, body := env.do(t, "POST", "/api/cameras/1/tests", "")
	if status != http.StatusBadGateway || !strings.Contains(body, "JPEG, PNG or WebP") {
		t.Fatalf("test of a camera that sends HTML = %d %s", status, body)
	}
	cam.reply.Store(append(append([]byte{}, pngImage...), make([]byte, 5<<20)...))
	if status, body := env.do(t, "POST", "/api/cameras/1/tests", ""); status != http.StatusBadGateway || !strings.Contains(body, "more than 5 MB") {
		t.Fatalf("test of a camera that sends too much = %d %s", status, body)
	}
	cam.reply.Store(pngImage)
	if status, body := env.do(t, "POST", "/api/cameras/1/tests", ""); status != http.StatusOK || !strings.Contains(body, `"contentType":"image/png"`) {
		t.Fatalf("test = %d %s", status, body)
	}
}

func TestCamerasAreManagedByAdminsWithoutShowingSecrets(t *testing.T) {
	env := newTestEnv(t, nil)
	if status, _ := env.do(t, "GET", "/api/cameras", ""); status != http.StatusUnauthorized {
		t.Fatalf("list without login = %d, want 401", status)
	}
	cam := newCameraStandIn(t)
	setUpCamera(t, env, cam)
	if status, body := env.do(t, "PUT", "/api/cameras/1", `{"name":"Door","url":"`+cam.srv.URL+`/snap.jpg","auth":{"kind":"basic","name":"admin","secret":""}}`); status != http.StatusOK {
		t.Fatalf("rename keeping the password = %d %s", status, body)
	}
	if status, _, _ := getImage(t, env.client, env.srv.URL+"/api/cameras/1/image"); status != http.StatusOK {
		t.Fatalf("image after the rename = %d", status)
	}
	status, list := env.do(t, "GET", "/api/cameras", "")
	if status != http.StatusOK || !strings.Contains(list, `"name":"Door"`) || !strings.Contains(list, `"secretSet":true`) || strings.Contains(list, cameraPassword) {
		t.Fatalf("list = %d %s", status, list)
	}
	if status, body := env.do(t, "POST", "/api/cameras", `{"name":"Bad","url":"rtsp://cam/stream","auth":{"kind":"none"}}`); status != http.StatusUnprocessableEntity || !strings.Contains(body, `"field":"url"`) {
		t.Fatalf("RTSP address = %d %s", status, body)
	}
	if status, _ := env.do(t, "DELETE", "/api/cameras/1", ""); status != http.StatusNoContent {
		t.Fatalf("delete = %d", status)
	}
	if status, _, _ := getImage(t, env.client, env.srv.URL+"/api/cameras/1/image"); status != http.StatusNotFound {
		t.Fatalf("image of a removed camera = %d, want 404", status)
	}
	if strings.Contains(env.logs.String(), cameraPassword) {
		t.Fatal("the password is in the log")
	}
}
