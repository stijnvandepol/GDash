package settings

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stijnvandepol/nookboard/internal/secret"
)

const haToken = "eyJhbGciOiJIUzI1NiJ9.secret-part"

func TestHomeAssistantTokenIsSecret(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	conn, err := CheckHomeAssistant("http://192.168.1.10:8123/", haToken)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetHomeAssistant(ctx, conn); err != nil {
		t.Fatal(err)
	}
	stored, _, _ := s.Store.Setting(ctx, keyHomeAssistant)
	if strings.Contains(stored, "secret-part") || strings.Contains(stored, "192.168") {
		t.Fatal("the connection is stored in plain text")
	}
	got, err := s.Get(ctx)
	if err != nil || got.HomeAssistant == nil || got.HomeAssistant.URL != "http://192.168.1.10:8123" || got.HomeAssistant.Unreadable {
		t.Fatalf("Get home assistant = %+v, %v", got.HomeAssistant, err)
	}
	src, _ := s.Sources(ctx)
	if src.HomeAssistant == nil || src.HomeAssistant.Token != haToken {
		t.Fatalf("Sources home assistant = %+v; want the token", src.HomeAssistant)
	}

	// With another key the connection is reported, not used.
	other, _ := secret.LoadOrCreateKey(filepath.Join(t.TempDir(), "other.key"))
	s.Box = other
	if got, _ := s.Get(ctx); got.HomeAssistant == nil || !got.HomeAssistant.Unreadable {
		t.Fatalf("Get with another key = %+v; want it marked unreadable", got.HomeAssistant)
	}
	if src, err := s.Sources(ctx); err != nil || src.HomeAssistant != nil {
		t.Fatalf("Sources with another key = %+v, %v; want no connection", src.HomeAssistant, err)
	}

	if err := s.ClearHomeAssistant(ctx); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Get(ctx); got.HomeAssistant != nil {
		t.Fatal("connection still shown after removing it")
	}
}

func TestCheckHomeAssistant(t *testing.T) {
	if _, err := CheckHomeAssistant("homeassistant.local", "two words"); fieldNames(err) != "url,token" {
		t.Fatalf("fields = %q, want url,token", fieldNames(err))
	}
	conn, err := CheckHomeAssistant("https://ha.example.org", "  "+haToken+"\n")
	if err != nil || conn.Token != haToken {
		t.Fatalf("CheckHomeAssistant = %+v, %v; want the token trimmed", conn, err)
	}
}
