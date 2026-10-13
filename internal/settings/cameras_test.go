package settings

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stijnvandepol/gdash/internal/camera"
	"github.com/stijnvandepol/gdash/internal/secret"
	"github.com/stijnvandepol/gdash/internal/store"
)

// cameraPassword is made up.
const cameraPassword = "made-up-camera-password"

func cameraInput(name, password string) camera.Input {
	in := camera.Input{Name: name, URL: "http://192.168.1.30/cgi-bin/api.cgi?cmd=Snap&channel=0"}
	in.Auth.Kind, in.Auth.Name, in.Auth.Secret = "basic", "admin", password
	return in
}

func TestCamerasAreSealedAndTheirSecretIsNeverShown(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	added, err := s.AddCamera(ctx, cameraInput("Front door", cameraPassword))
	if err != nil || added.ID != 1 || !added.Auth.SecretSet {
		t.Fatalf("AddCamera = %+v, %v", added, err)
	}
	stored, _, _ := s.Store.Setting(ctx, keyCameras)
	if strings.Contains(stored, cameraPassword) || strings.Contains(stored, "192.168") {
		t.Fatal("the camera is stored in plain text")
	}
	list, err := s.CameraList(ctx)
	shown, _ := json.Marshal(list)
	if err != nil || len(list.Cameras) != 1 || strings.Contains(string(shown), cameraPassword) {
		t.Fatalf("CameraList = %s, %v", shown, err)
	}

	// An empty password keeps the stored one.
	if _, err := s.UpdateCamera(ctx, 1, cameraInput("Front door, left", "")); err != nil {
		t.Fatal(err)
	}
	cam, err := s.Camera(ctx, 1)
	if err != nil || cam.Name != "Front door, left" || cam.Auth.Secret != cameraPassword {
		t.Fatalf("Camera = %+v, %v", cam, err)
	}

	// IDs are never reused, so a widget of a removed camera shows no other.
	if err := s.DeleteCamera(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if again, _ := s.AddCamera(ctx, cameraInput("Garden", cameraPassword)); again.ID != 2 {
		t.Fatalf("new camera after a removal got ID %d, want 2", again.ID)
	}
	if _, err := s.Camera(ctx, 1); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("removed camera = %v", err)
	}
}

func TestCamerasHaveALimit(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	for range camera.MaxCameras {
		if _, err := s.AddCamera(ctx, cameraInput("Camera", cameraPassword)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.AddCamera(ctx, cameraInput("One too many", cameraPassword)); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("camera over the limit = %v", err)
	}
}

func TestUnreadableCamerasAreReportedAndReplaced(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	if _, err := s.AddCamera(ctx, cameraInput("Front door", cameraPassword)); err != nil {
		t.Fatal(err)
	}
	other, _ := secret.LoadOrCreateKey(filepath.Join(t.TempDir(), "other.key"))
	s.Box = other
	if list, err := s.CameraList(ctx); err != nil || !list.Unreadable || len(list.Cameras) != 0 {
		t.Fatalf("CameraList with another key = %+v, %v", list, err)
	}
	if _, err := s.Camera(ctx, 1); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("Camera with another key = %v", err)
	}
	if added, err := s.AddCamera(ctx, cameraInput("Front door", cameraPassword)); err != nil || added.ID != 1 {
		t.Fatalf("AddCamera over unreadable cameras = %+v, %v", added, err)
	}
}
