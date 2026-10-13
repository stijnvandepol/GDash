package settings

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stijnvandepol/gdash/internal/secret"
)

func TestTrafficInfoIsOffUntilAcknowledged(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	if info, err := s.TrafficInfo(ctx); err != nil || info.Enabled {
		t.Fatalf("TrafficInfo = %+v, %v; want off", info, err)
	}
	if _, err := s.SetTrafficInfo(ctx, true, false); fieldNames(err) != "acknowledged" {
		t.Fatalf("on without acknowledging: %v", err)
	}
	if src, _ := s.Sources(ctx); src.TrafficInfo {
		t.Fatal("refused switch turned traffic info on")
	}

	info, err := s.SetTrafficInfo(ctx, true, true)
	if err != nil || !info.Enabled || info.AcknowledgedAt == nil || !info.AcknowledgedAt.Equal(s.Now()) {
		t.Fatalf("SetTrafficInfo = %+v, %v", info, err)
	}
	if src, _ := s.Sources(ctx); !src.TrafficInfo {
		t.Fatal("Sources does not have traffic info on")
	}

	off, err := s.SetTrafficInfo(ctx, false, false)
	if err != nil || off.Enabled || off.AcknowledgedAt == nil {
		t.Fatalf("off = %+v, %v; want off with the acknowledgement kept", off, err)
	}
	if src, _ := s.Sources(ctx); src.TrafficInfo {
		t.Fatal("still on after turning it off")
	}
	// On again needs the acknowledgement again.
	if _, err := s.SetTrafficInfo(ctx, true, false); err == nil {
		t.Fatal("turned on again without acknowledging")
	}
}

func TestTrafficInfoNeedsTheAcknowledgementStored(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	// A hand-edited setting that is on without an acknowledgement is off.
	if err := s.Store.SetSetting(ctx, keyTraffic, `{"enabled":true}`); err != nil {
		t.Fatal(err)
	}
	if info, err := s.TrafficInfo(ctx); err != nil || info.Enabled {
		t.Fatalf("TrafficInfo = %+v, %v", info, err)
	}
}

const knmiKey = "made-up-knmi-settings-key"

func TestKNMIKeyIsSecret(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	if _, err := CheckKNMIKey("two words"); fieldNames(err) != "key" {
		t.Fatalf("a key with a space: %v", err)
	}
	key, err := CheckKNMIKey("  " + knmiKey + "\n")
	if err != nil || key != knmiKey {
		t.Fatalf("CheckKNMIKey = %q, %v", key, err)
	}
	if err := s.SetKNMIKey(ctx, key); err != nil {
		t.Fatal(err)
	}
	stored, _, _ := s.Store.Setting(ctx, keyKNMI)
	if strings.Contains(stored, knmiKey) {
		t.Fatal("the key is stored in plain text")
	}
	if view, err := s.KNMIView(ctx); err != nil || !view.Connected || view.Unreadable {
		t.Fatalf("KNMIView = %+v, %v", view, err)
	}
	if src, _ := s.Sources(ctx); src.KNMIKey != knmiKey {
		t.Fatal("Sources does not have the key")
	}

	other, _ := secret.LoadOrCreateKey(filepath.Join(t.TempDir(), "other.key"))
	s.Box = other
	if view, _ := s.KNMIView(ctx); !view.Unreadable {
		t.Fatalf("KNMIView with another secret = %+v; want unreadable", view)
	}
	if src, err := s.Sources(ctx); err != nil || src.KNMIKey != "" {
		t.Fatalf("Sources with another secret = %q, %v", src.KNMIKey, err)
	}

	if err := s.ClearKNMIKey(ctx); err != nil {
		t.Fatal(err)
	}
	if view, _ := s.KNMIView(ctx); view.Connected {
		t.Fatal("key still shown after removing it")
	}
}
