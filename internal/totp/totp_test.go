package totp

import (
	"net/url"
	"testing"
	"time"
)

// rfcSecret is the SHA-1 seed of the RFC 6238 test vectors.
var rfcSecret = []byte("12345678901234567890")

func TestCodeMatchesRFC6238Vectors(t *testing.T) {
	// RFC 6238, appendix B, gives 8 digits; a 6-digit code is the last six.
	vectors := []struct {
		unix int64
		code string
	}{
		{59, "287082"},
		{1111111109, "081804"},
		{1111111111, "050471"},
		{1234567890, "005924"},
		{2000000000, "279037"},
		{20000000000, "353130"},
	}
	for _, v := range vectors {
		if got := Code(rfcSecret, Step(time.Unix(v.unix, 0))); got != v.code {
			t.Errorf("code at %d = %s, want %s", v.unix, got, v.code)
		}
	}
}

func TestCodeMatchesRFC4226Vectors(t *testing.T) {
	want := []string{"755224", "287082", "359152", "969429", "338314", "254676", "287922", "162583", "399871", "520489"}
	for counter, code := range want {
		if got := Code(rfcSecret, int64(counter)); got != code {
			t.Errorf("HOTP(%d) = %s, want %s", counter, got, code)
		}
	}
}

func TestMatchAcceptsOneStepEitherSide(t *testing.T) {
	now := time.Unix(1111111111, 0)
	current := Step(now)
	for _, step := range []int64{current - 1, current, current + 1} {
		got, ok := Match(rfcSecret, Code(rfcSecret, step), now, 0)
		if !ok || got != step {
			t.Errorf("code of step %d (current %d): Match = %d, %v", step, current, got, ok)
		}
	}
	for _, step := range []int64{current - 2, current + 2} {
		if _, ok := Match(rfcSecret, Code(rfcSecret, step), now, 0); ok {
			t.Errorf("code of step %d accepted at step %d", step, current)
		}
	}
}

func TestMatchRefusesAUsedStep(t *testing.T) {
	now := time.Unix(1111111111, 0)
	current := Step(now)
	code := Code(rfcSecret, current)
	if _, ok := Match(rfcSecret, code, now, current); ok {
		t.Fatal("a code was accepted again after its step was used")
	}
	// The next code is still fine once the previous one was used.
	if step, ok := Match(rfcSecret, Code(rfcSecret, current+1), now, current); !ok || step != current+1 {
		t.Fatalf("next code: Match = %d, %v", step, ok)
	}
	if _, ok := Match(rfcSecret, Code(rfcSecret, current-1), now, current); ok {
		t.Fatal("an older code was accepted after a newer one was used")
	}
}

func TestMatchRefusesMalformedCodes(t *testing.T) {
	now := time.Unix(1111111111, 0)
	for _, code := range []string{"", "05047", "0504711", "abcdef", " 050471"} {
		if _, ok := Match(rfcSecret, code, now, 0); ok {
			t.Errorf("code %q accepted", code)
		}
	}
}

func TestNewSecretIsRandom(t *testing.T) {
	a, err := NewSecret()
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewSecret()
	if err != nil {
		t.Fatal(err)
	}
	if len(a) != SecretBytes || string(a) == string(b) {
		t.Fatalf("secrets %x and %x: want two different %d-byte secrets", a, b, SecretBytes)
	}
}

func TestURI(t *testing.T) {
	const encoded = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	if got := EncodeSecret(rfcSecret); got != encoded {
		t.Fatalf("EncodeSecret = %s, want %s", got, encoded)
	}
	uri := URI("GDash", "stijn", rfcSecret)
	u, err := url.Parse(uri)
	if err != nil {
		t.Fatal(err)
	}
	if u.Scheme != "otpauth" || u.Host != "totp" || u.Path != "/GDash:stijn" {
		t.Fatalf("URI %s: wrong scheme, type or label", uri)
	}
	q := u.Query()
	if q.Get("secret") != encoded || q.Get("issuer") != "GDash" ||
		q.Get("algorithm") != "SHA1" || q.Get("digits") != "6" || q.Get("period") != "30" {
		t.Fatalf("URI %s: wrong parameters", uri)
	}
}
