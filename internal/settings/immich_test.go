package settings

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stijnvandepol/gdash/internal/immich"
	"github.com/stijnvandepol/gdash/internal/secret"
)

// immichKey has the shape of an Immich API key, but is made up.
const immichKey = "madeUpImmichApiKey0123456789abcdefABCDEF"

const testAlbumID = "4b1c5d6e-7f80-4912-a3b4-c5d6e7f80912"

func TestImmichKeyIsSecretAndTheAlbumFollowsTheAddress(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	conn, err := CheckImmich("http://192.168.1.10:2283/api/", immichKey)
	if err != nil || conn.URL != "http://192.168.1.10:2283" {
		t.Fatalf("CheckImmich = %+v, %v", conn, err)
	}
	if err := s.SetImmich(ctx, conn); err != nil {
		t.Fatal(err)
	}
	if src, _ := s.Sources(ctx); src.Immich != nil {
		t.Fatal("Sources has an Immich album before one is chosen")
	}
	if err := s.SetImmichAlbum(ctx, immich.Album{ID: testAlbumID, Name: "Holidays"}); err != nil {
		t.Fatal(err)
	}
	stored, _, _ := s.Store.Setting(ctx, keyImmich)
	if strings.Contains(stored, immichKey) || strings.Contains(stored, "192.168") {
		t.Fatal("the connection is stored in plain text")
	}
	got, err := s.Get(ctx)
	if err != nil || got.Immich == nil || got.Immich.AlbumName != "Holidays" || got.Immich.URL != conn.URL {
		t.Fatalf("Get immich = %+v, %v", got.Immich, err)
	}
	src, _ := s.Sources(ctx)
	if src.Immich == nil || src.Immich.Connection.Key != immichKey || src.Immich.AlbumID != testAlbumID {
		t.Fatalf("Sources immich = %+v", src.Immich)
	}

	// A new key for the same server keeps the album; another server does not.
	conn.Key = immichKey + "2"
	s.SetImmich(ctx, conn)
	if got, _ := s.Get(ctx); got.Immich.AlbumID != testAlbumID {
		t.Fatal("a new key lost the album")
	}
	s.SetImmich(ctx, immich.Connection{URL: "http://192.168.1.11:2283", Key: immichKey})
	if got, _ := s.Get(ctx); got.Immich.AlbumID != "" {
		t.Fatal("another server kept the album of the first")
	}

	other, _ := secret.LoadOrCreateKey(filepath.Join(t.TempDir(), "other.key"))
	s.Box = other
	if got, _ := s.Get(ctx); got.Immich == nil || !got.Immich.Unreadable {
		t.Fatalf("Get with another key = %+v; want it marked unreadable", got.Immich)
	}
	if src, err := s.Sources(ctx); err != nil || src.Immich != nil {
		t.Fatalf("Sources with another key = %+v, %v", src.Immich, err)
	}
	if err := s.ClearImmich(ctx); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Get(ctx); got.Immich != nil {
		t.Fatal("connection still shown after removing it")
	}
}

func TestCheckImmich(t *testing.T) {
	if _, err := CheckImmich("192.168.1.10:2283", "two words"); fieldNames(err) != "url,key" {
		t.Fatalf("fields = %q, want url,key", fieldNames(err))
	}
}
