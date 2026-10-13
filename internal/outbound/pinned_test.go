package outbound

import (
	"context"
	"crypto/sha256"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPinnedClientAcceptsOnlyTheAcceptedCertificate(t *testing.T) {
	nas := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	defer nas.Close()
	other := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	defer other.Close()

	// A self-signed certificate fails the normal check.
	if _, err := NewClient(5 * time.Second).Get(nas.URL); err == nil {
		t.Fatal("a self-signed certificate was accepted without a pin")
	}
	pin, err := CertificateFingerprint(context.Background(), strings.TrimPrefix(nas.URL, "https://"), 5*time.Second)
	if err != nil || pin != sha256.Sum256(nas.Certificate().Raw) {
		t.Fatalf("CertificateFingerprint = %x, %v", pin, err)
	}
	resp, err := NewPinnedClient(5*time.Second, pin).Get(nas.URL)
	if err != nil {
		t.Fatalf("GET with the pin: %v", err)
	}
	resp.Body.Close()

	// httptest gives every server the same certificate; another one is
	// what a NAS presents after a renewal, or someone in between.
	changed := pin
	changed[0] ^= 0xFF
	if _, err := NewPinnedClient(5*time.Second, changed).Get(other.URL); !errors.Is(err, ErrCertificateChanged) {
		t.Fatalf("GET with another certificate: %v, want ErrCertificateChanged", err)
	}
	if _, err := CertificateFingerprint(context.Background(), "169.254.169.254:443", time.Second); !errors.Is(err, ErrBlockedAddress) {
		t.Fatalf("certificate of a metadata address: %v, want ErrBlockedAddress", err)
	}
}
