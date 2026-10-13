// Package camera shows still images of cameras on the home network on the
// TVs. The GDash server fetches a camera's snapshot itself, so the
// camera's address and secrets never reach a screen; it keeps only the
// latest image of each camera, in memory, and only fetches while a screen
// asks for it.
package camera

import (
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/stijnvandepol/gdash/internal/custom"
	"github.com/stijnvandepol/gdash/internal/validation"
)

const (
	// MaxCameras bounds the list; every camera on a screen is a request
	// every few seconds.
	MaxCameras    = 10
	maxNameLength = 60
	maxURLLength  = 2048
)

// Camera is a camera with its secret, for the server only.
type Camera struct {
	ID   int64
	Name string
	// URL is the snapshot address, such as Frigate's
	// http://frigate:5000/api/front_door/latest.jpg.
	URL  string
	Auth custom.Auth
}

// Input is a camera as the web interface sends it. An empty secret keeps
// the one already stored, so the form never needs to show it.
type Input struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Auth struct {
		Kind   string `json:"kind"`
		Name   string `json:"name"`
		Secret string `json:"secret"`
	} `json:"auth"`
}

// Check validates an input, keeps the secret of old (the stored camera
// when it is edited, or nil) when the input leaves it empty, and returns
// the camera to store. It returns a *validation.Error naming every invalid
// field; messages never contain a secret.
func Check(in Input, old *Camera) (Camera, error) {
	var errs validation.Collector
	cam := Camera{Name: in.Name, URL: checkURL(&errs, in.URL)}
	var oldAuth custom.Auth
	if old != nil {
		cam.ID, oldAuth = old.ID, old.Auth
	}
	trimmed := strings.TrimSpace(in.Name)
	if trimmed == "" || trimmed != in.Name || utf8.RuneCountInString(in.Name) > maxNameLength || validation.HasControlCharacter(in.Name) {
		errs.Add("name", "use 1 to 60 characters without leading or trailing spaces")
	}
	cam.Auth = custom.CheckAuth(&errs, custom.Auth{Kind: in.Auth.Kind, Name: in.Auth.Name, Secret: in.Auth.Secret}, oldAuth, cam.URL)
	if err := errs.Err(); err != nil {
		return Camera{}, err
	}
	return cam, nil
}

func checkURL(errs *validation.Collector, raw string) string {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	switch {
	case err != nil || u.Host == "" || len(raw) > maxURLLength:
		errs.Add("url", "paste the full snapshot address, starting with http://")
	case u.Scheme != "http" && u.Scheme != "https":
		errs.Add("url", "use an http or https address")
	case u.User != nil:
		// The address is shown in the web interface; a password belongs
		// in the authentication, where it is sealed.
		errs.Add("url", "use basic auth instead of a user name and password in the address")
	case u.Fragment != "":
		errs.Add("url", "use the address without a # part")
	default:
		return u.String()
	}
	return ""
}
