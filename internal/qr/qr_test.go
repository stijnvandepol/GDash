package qr

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestReedSolomonMatchesTheStandardExample(t *testing.T) {
	// "HELLO WORLD" at 1-M, the worked example of the standard's annex
	// (and of the Thonky QR tutorial): data codewords and their 10 error
	// correction codewords.
	data := []byte{32, 91, 11, 120, 209, 114, 220, 77, 67, 64, 236, 17, 236, 17, 236, 17}
	want := []byte{196, 35, 39, 119, 235, 215, 231, 226, 93, 23}
	if got := rsRemainder(data, rsDivisor(10)); !bytes.Equal(got, want) {
		t.Fatalf("error correction = %v, want %v", got, want)
	}
}

func TestFormatBitsMatchTheStandardTable(t *testing.T) {
	want := []int{
		0b101010000010010, 0b101000100100101, 0b101111001111100, 0b101101101001011,
		0b100010111111001, 0b100000011001110, 0b100111110010111, 0b100101010100000,
	}
	for mask, bits := range want {
		if got := formatBits(mask); got != bits {
			t.Errorf("format bits of M, mask %d = %015b, want %015b", mask, got, bits)
		}
	}
}

func TestVersionBitsMatchTheStandardTable(t *testing.T) {
	want := map[int]int{7: 0x07C94, 8: 0x085BC, 9: 0x09A99, 10: 0x0A4D3}
	for version, bits := range want {
		if got := versionBits(version); got != bits {
			t.Errorf("version bits of %d = %018b, want %018b", version, got, bits)
		}
	}
}

func TestEncodePicksTheSmallestVersion(t *testing.T) {
	cases := []struct{ length, version int }{
		{0, 1}, {14, 1}, {15, 2}, {26, 2}, {27, 3}, {106, 6}, {107, 7}, {152, 8}, {180, 9}, {181, 10}, {MaxBytes, 10},
	}
	for _, c := range cases {
		code, err := Encode(bytes.Repeat([]byte("a"), c.length))
		if err != nil {
			t.Fatalf("%d bytes: %v", c.length, err)
		}
		if code.Version != c.version || code.Size != 17+4*c.version {
			t.Errorf("%d bytes: version %d, size %d; want version %d", c.length, code.Version, code.Size, c.version)
		}
	}
	if _, err := Encode(bytes.Repeat([]byte("a"), MaxBytes+1)); !errors.Is(err, ErrTooLong) {
		t.Fatalf("%d bytes: err = %v, want ErrTooLong", MaxBytes+1, err)
	}
}

func TestFinderTimingAndDarkModule(t *testing.T) {
	code, err := Encode([]byte("otpauth://totp/GDash:stijn?secret=GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ&issuer=GDash"))
	if err != nil {
		t.Fatal(err)
	}
	n := code.Size
	for _, corner := range [][2]int{{0, 0}, {n - 7, 0}, {0, n - 7}} {
		for dy := range 7 {
			for dx := range 7 {
				ring := max(abs(dx-3), abs(dy-3))
				if want := ring != 2; code.Dark(corner[0]+dx, corner[1]+dy) != want {
					t.Fatalf("finder at %v: module %d,%d dark = %v", corner, dx, dy, !want)
				}
			}
		}
	}
	for i := 8; i < n-8; i++ {
		if code.Dark(i, 6) != (i%2 == 0) || code.Dark(6, i) != (i%2 == 0) {
			t.Fatalf("timing pattern broken at %d", i)
		}
	}
	if !code.Dark(8, n-8) {
		t.Fatal("the dark module is light")
	}
}

func TestCodesReadBack(t *testing.T) {
	inputs := []string{
		"",
		"GDash",
		"otpauth://totp/GDash:stijn?algorithm=SHA1&digits=6&issuer=GDash&period=30&secret=GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ",
		"otpauth://totp/GDash:" + strings.Repeat("u", 32) + "?algorithm=SHA1&digits=6&issuer=GDash&period=30&secret=" + strings.Repeat("A", 32),
		strings.Repeat("\x00\xff", MaxBytes/2),
	}
	for _, input := range inputs {
		code, err := Encode([]byte(input))
		if err != nil {
			t.Fatal(err)
		}
		if got := readBack(t, code); got != input {
			t.Errorf("version %d read back %q, want %q", code.Version, got, input)
		}
	}
}

func TestSVG(t *testing.T) {
	code, err := Encode([]byte("GDash"))
	if err != nil {
		t.Fatal(err)
	}
	svg := code.SVG()
	if !strings.HasPrefix(svg, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 29 29"`) || !strings.HasSuffix(svg, "</svg>") {
		t.Fatalf("SVG of a 21-module code with its quiet zone: %s", svg)
	}
	// The top left finder's first row: seven dark modules from 4,4.
	if !strings.Contains(svg, `d="M4 4h7v1h-7z`) {
		t.Fatalf("SVG does not start with the finder row: %s", svg)
	}
}
