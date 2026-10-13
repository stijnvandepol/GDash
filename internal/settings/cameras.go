package settings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/stijnvandepol/gdash/internal/camera"
	"github.com/stijnvandepol/gdash/internal/custom"
	"github.com/stijnvandepol/gdash/internal/store"
)

// keyCameras holds every camera in one sealed value: an address on the
// home network says where the cameras are, and the secrets open them.
const keyCameras = "cameras"

// Camera is a camera as the web interface sees it: everything but the
// secret, which only says whether it is set.
type Camera struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
	Auth struct {
		Kind      string `json:"kind"`
		Name      string `json:"name"`
		SecretSet bool   `json:"secretSet"`
	} `json:"auth"`
}

// Cameras is the list for the web interface.
type Cameras struct {
	Cameras []Camera `json:"cameras"`
	// Unreadable is true when the stored cameras cannot be decrypted, for
	// example after the database was moved without its secret.key. Adding
	// a camera then starts a new list.
	Unreadable bool `json:"unreadable"`
}

// storedCameras is what keyCameras holds, sealed. Next is the ID of the
// next camera: an ID is never reused, so a widget that showed a removed
// camera never shows another one.
type storedCameras struct {
	Next    int64          `json:"next"`
	Cameras []storedCamera `json:"cameras"`
}

type storedCamera struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	URL        string `json:"url"`
	AuthKind   string `json:"authKind"`
	AuthName   string `json:"authName,omitempty"`
	AuthSecret string `json:"authSecret,omitempty"`
}

func (c storedCamera) camera() camera.Camera {
	return camera.Camera{ID: c.ID, Name: c.Name, URL: c.URL, Auth: custom.Auth{Kind: c.AuthKind, Name: c.AuthName, Secret: c.AuthSecret}}
}

func toStored(cam camera.Camera) storedCamera {
	return storedCamera{ID: cam.ID, Name: cam.Name, URL: cam.URL, AuthKind: cam.Auth.Kind, AuthName: cam.Auth.Name, AuthSecret: cam.Auth.Secret}
}

func (c storedCamera) view() Camera {
	v := Camera{ID: c.ID, Name: c.Name, URL: c.URL}
	v.Auth.Kind, v.Auth.Name, v.Auth.SecretSet = c.AuthKind, c.AuthName, c.AuthSecret != ""
	return v
}

// CameraList returns the cameras for the web interface, without secrets.
func (s *Service) CameraList(ctx context.Context) (Cameras, error) {
	stored, err := s.loadCameras(ctx)
	if err != nil {
		if errUnreadable(err) {
			s.Logger.Error("cameras cannot be decrypted; add them again", "error", err)
			return Cameras{Cameras: []Camera{}, Unreadable: true}, nil
		}
		return Cameras{}, err
	}
	out := Cameras{Cameras: make([]Camera, len(stored.Cameras))}
	for i, c := range stored.Cameras {
		out.Cameras[i] = c.view()
	}
	return out, nil
}

// Camera returns one camera with its secret, for fetching its image; it
// returns store.ErrNotFound for an unknown ID, and also when the cameras
// cannot be decrypted.
func (s *Service) Camera(ctx context.Context, id int64) (camera.Camera, error) {
	stored, err := s.loadCameras(ctx)
	if err != nil {
		if errUnreadable(err) {
			return camera.Camera{}, store.ErrNotFound
		}
		return camera.Camera{}, err
	}
	i := slices.IndexFunc(stored.Cameras, func(c storedCamera) bool { return c.ID == id })
	if i < 0 {
		return camera.Camera{}, store.ErrNotFound
	}
	return stored.Cameras[i].camera(), nil
}

// AddCamera stores a new camera. It returns a *validation.Error for invalid
// input and store.ErrConflict when camera.MaxCameras exist.
func (s *Service) AddCamera(ctx context.Context, in camera.Input) (Camera, error) {
	cam, err := camera.Check(in, nil)
	if err != nil {
		return Camera{}, err
	}
	s.camerasMu.Lock()
	defer s.camerasMu.Unlock()
	stored, err := s.loadCameras(ctx)
	if err != nil && !errUnreadable(err) {
		return Camera{}, err
	}
	if len(stored.Cameras) >= camera.MaxCameras {
		return Camera{}, store.ErrConflict
	}
	stored.Next = max(stored.Next, 1)
	cam.ID = stored.Next
	stored.Next++
	stored.Cameras = append(stored.Cameras, toStored(cam))
	if err := s.saveCameras(ctx, stored); err != nil {
		return Camera{}, err
	}
	return toStored(cam).view(), nil
}

// UpdateCamera changes a camera; an empty secret keeps the stored one. It
// returns store.ErrNotFound for an unknown ID.
func (s *Service) UpdateCamera(ctx context.Context, id int64, in camera.Input) (Camera, error) {
	s.camerasMu.Lock()
	defer s.camerasMu.Unlock()
	stored, i, err := s.findCamera(ctx, id)
	if err != nil {
		return Camera{}, err
	}
	old := stored.Cameras[i].camera()
	cam, err := camera.Check(in, &old)
	if err != nil {
		return Camera{}, err
	}
	stored.Cameras[i] = toStored(cam)
	if err := s.saveCameras(ctx, stored); err != nil {
		return Camera{}, err
	}
	return stored.Cameras[i].view(), nil
}

// DeleteCamera removes a camera, or returns store.ErrNotFound. Widgets
// that show it then say that their camera is not available.
func (s *Service) DeleteCamera(ctx context.Context, id int64) error {
	s.camerasMu.Lock()
	defer s.camerasMu.Unlock()
	stored, i, err := s.findCamera(ctx, id)
	if err != nil {
		return err
	}
	stored.Cameras = slices.Delete(stored.Cameras, i, i+1)
	return s.saveCameras(ctx, stored)
}

func (s *Service) findCamera(ctx context.Context, id int64) (storedCameras, int, error) {
	stored, err := s.loadCameras(ctx)
	if errUnreadable(err) {
		return storedCameras{}, 0, store.ErrNotFound
	}
	if err != nil {
		return storedCameras{}, 0, err
	}
	i := slices.IndexFunc(stored.Cameras, func(c storedCamera) bool { return c.ID == id })
	if i < 0 {
		return storedCameras{}, 0, store.ErrNotFound
	}
	return stored, i, nil
}

// errCamerasUnreadable is returned by loadCameras when the stored cameras
// cannot be decrypted.
var errCamerasUnreadable = errors.New("the stored cameras cannot be decrypted")

func errUnreadable(err error) bool { return errors.Is(err, errCamerasUnreadable) }

func (s *Service) loadCameras(ctx context.Context) (storedCameras, error) {
	sealed, ok, err := s.Store.Setting(ctx, keyCameras)
	if err != nil || !ok {
		return storedCameras{}, err
	}
	plain, err := s.Box.Open(sealed)
	if err != nil {
		return storedCameras{}, fmt.Errorf("%w: %v", errCamerasUnreadable, err)
	}
	var stored storedCameras
	if err := json.Unmarshal([]byte(plain), &stored); err != nil {
		return storedCameras{}, fmt.Errorf("stored cameras are invalid: %w", err)
	}
	return stored, nil
}

func (s *Service) saveCameras(ctx context.Context, stored storedCameras) error {
	plain, err := json.Marshal(stored)
	if err != nil {
		return err
	}
	sealed, err := s.Box.Seal(string(plain))
	if err != nil {
		return err
	}
	return s.Store.SetSetting(ctx, keyCameras, sealed)
}
