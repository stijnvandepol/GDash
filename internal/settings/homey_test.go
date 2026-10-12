package settings

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stijnvandepol/gdash/internal/secret"
)

// homeyKey has the shape of a Homey API key, but is made up.
const homeyKey = "made-up-client:made-up-user:secret-part"

func TestHomeyKeyIsSecret(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	conn, err := CheckHomey("http://192.168.1.20/", homeyKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetHomey(ctx, conn); err != nil {
		t.Fatal(err)
	}
	stored, _, _ := s.Store.Setting(ctx, keyHomey)
	if strings.Contains(stored, "secret-part") || strings.Contains(stored, "192.168") {
		t.Fatal("the connection is stored in plain text")
	}
	got, err := s.Get(ctx)
	if err != nil || got.Homey == nil || got.Homey.URL != "http://192.168.1.20" || got.Homey.Unreadable {
		t.Fatalf("Get homey = %+v, %v", got.Homey, err)
	}
	src, _ := s.Sources(ctx)
	if src.Homey == nil || src.Homey.Key != homeyKey {
		t.Fatalf("Sources homey = %+v; want the key", src.Homey)
	}

	// With another key the connection is reported, not used.
	other, _ := secret.LoadOrCreateKey(filepath.Join(t.TempDir(), "other.key"))
	s.Box = other
	if got, _ := s.Get(ctx); got.Homey == nil || !got.Homey.Unreadable {
		t.Fatalf("Get with another key = %+v; want it marked unreadable", got.Homey)
	}
	if src, err := s.Sources(ctx); err != nil || src.Homey != nil {
		t.Fatalf("Sources with another key = %+v, %v; want no connection", src.Homey, err)
	}

	if err := s.ClearHomey(ctx); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Get(ctx); got.Homey != nil {
		t.Fatal("connection still shown after removing it")
	}
}

func TestCheckHomey(t *testing.T) {
	if _, err := CheckHomey("192.168.1.20", "two words"); fieldNames(err) != "url,key" {
		t.Fatalf("fields = %q, want url,key", fieldNames(err))
	}
	if _, err := CheckHomey("http://192.168.1.20", strings.Repeat("k", maxTokenLength+1)); fieldNames(err) != "key" {
		t.Fatalf("fields for a long key = %q, want key", fieldNames(err))
	}
	conn, err := CheckHomey("https://192.168.1.20:4859", "  "+homeyKey+"\n")
	if err != nil || conn.Key != homeyKey || conn.URL != "https://192.168.1.20:4859" {
		t.Fatalf("CheckHomey = %+v, %v; want the key trimmed", conn, err)
	}
}
