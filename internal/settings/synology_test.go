package settings

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stijnvandepol/gdash/internal/secret"
)

// Made up: a share link's passphrase and password.
const (
	sharePassphrase = "MadeUpShare42"
	sharePassword   = "made-up-share-password"
)

func TestSynologyAlbumIsSealedAndKeepsItsPassword(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	link := "https://192.168.1.2:5001/mo/sharing/" + sharePassphrase
	album, err := s.CheckSynology(ctx, link+"/", sharePassword)
	if err != nil || album.Link != link {
		t.Fatalf("CheckSynology = %+v, %v", album, err)
	}
	album.Pin[0], album.Pin[31] = 0xAB, 0x01
	if err := s.SetSynology(ctx, album); err != nil {
		t.Fatal(err)
	}
	stored, _, _ := s.Store.Setting(ctx, keySynology)
	if strings.Contains(stored, sharePassphrase) || strings.Contains(stored, sharePassword) {
		t.Fatal("the album is stored in plain text")
	}
	got, err := s.Get(ctx)
	shown, _ := json.Marshal(got)
	if err != nil || got.Synology == nil || got.Synology.Host != "192.168.1.2:5001" || !got.Synology.PasswordSet ||
		!strings.HasPrefix(got.Synology.Certificate, "AB:00:") || strings.Contains(string(shown), sharePassphrase) {
		t.Fatalf("Get synology = %s, %v", shown, err)
	}
	src, _ := s.Sources(ctx)
	if src.Synology == nil || *src.Synology != album {
		t.Fatalf("Sources synology = %+v, want %+v", src.Synology, album)
	}

	// An empty password keeps the stored one for the same link only.
	if again, _ := s.CheckSynology(ctx, link, ""); again.Password != sharePassword {
		t.Fatal("an empty password lost the stored one")
	}
	if other, _ := s.CheckSynology(ctx, "https://192.168.1.3:5001/mo/sharing/Other1", ""); other.Password != "" {
		t.Fatal("another link got the stored password")
	}

	otherKey, _ := secret.LoadOrCreateKey(filepath.Join(t.TempDir(), "other.key"))
	s.Box = otherKey
	if got, _ := s.Get(ctx); got.Synology == nil || !got.Synology.Unreadable {
		t.Fatalf("Get with another key = %+v; want it marked unreadable", got.Synology)
	}
	if src, err := s.Sources(ctx); err != nil || src.Synology != nil {
		t.Fatalf("Sources with another key = %+v, %v", src.Synology, err)
	}
	if err := s.ClearSynology(ctx); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Get(ctx); got.Synology != nil {
		t.Fatal("album still shown after removing it")
	}
}

func TestCheckSynologyNamesTheField(t *testing.T) {
	s := newService(t)
	if _, err := s.CheckSynology(context.Background(), "https://nas/photo/", "a\nb"); fieldNames(err) != "link,password" {
		t.Fatalf("fields = %q, want link,password", fieldNames(err))
	}
}
