package secret

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSealAndOpen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secret.key")
	box, err := LoadOrCreateKey(path)
	if err != nil {
		t.Fatal(err)
	}
	const url = "https://calendar.example/private-abc/basic.ics"
	sealed, err := box.Seal(url)
	if err != nil {
		t.Fatal(err)
	}
	if sealed == url || len(sealed) < len(url) {
		t.Fatalf("sealed = %q, want ciphertext", sealed)
	}
	again, _ := box.Seal(url)
	if again == sealed {
		t.Fatal("two seals of the same value are equal; the nonce is not fresh")
	}

	// The same key file opens it after a restart.
	reloaded, err := LoadOrCreateKey(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := reloaded.Open(sealed); err != nil || got != url {
		t.Fatalf("Open = %q, %v", got, err)
	}
	if info, err := os.Stat(path); err != nil || info.Size() != keySize {
		t.Fatalf("key file = %v, %v", info, err)
	}
}

func TestOpenRejectsOtherKeyAndChanges(t *testing.T) {
	box, _ := LoadOrCreateKey(filepath.Join(t.TempDir(), "a.key"))
	other, _ := LoadOrCreateKey(filepath.Join(t.TempDir(), "b.key"))
	sealed, _ := box.Seal("secret")
	for name, value := range map[string]string{
		"other key":  sealed,
		"not base64": "%%%",
		"too short":  "AAAA",
	} {
		if _, err := other.Open(value); !errors.Is(err, ErrUnreadable) {
			t.Errorf("%s: err = %v, want ErrUnreadable", name, err)
		}
	}
	tampered := []byte(sealed)
	tampered[len(tampered)-2] ^= 1
	if _, err := box.Open(string(tampered)); !errors.Is(err, ErrUnreadable) {
		t.Fatalf("tampered: err = %v, want ErrUnreadable", err)
	}
}

func TestNewBoxRejectsWrongKeySize(t *testing.T) {
	if _, err := NewBox(make([]byte, 16)); err == nil {
		t.Fatal("NewBox accepted a 16-byte key")
	}
}

func TestFingerprintDependsOnTheKey(t *testing.T) {
	one, _ := NewBox(make([]byte, keySize))
	other, _ := NewBox(append(make([]byte, keySize-1), 1))
	data := []byte("https://calendar.example.org/private-0123456789/basic.ics")
	first, again := one.Fingerprint(data), one.Fingerprint(data)
	if first != again {
		t.Fatal("a fingerprint changes between calls")
	}
	if one.Fingerprint(data) == other.Fingerprint(data) {
		t.Fatal("two keys give the same fingerprint, so it could be computed without the key file")
	}
	if one.Fingerprint(data) == one.Fingerprint([]byte("something else")) {
		t.Fatal("different data gives the same fingerprint")
	}
	if got := len(one.Fingerprint(data)); got != 2*fingerprintBytes {
		t.Fatalf("fingerprint length = %d, want %d hex characters", got, 2*fingerprintBytes)
	}
}
