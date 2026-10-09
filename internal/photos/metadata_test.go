package photos

import (
	"bytes"
	"encoding/binary"
	"errors"
	imagejpeg "image/jpeg"
	imagepng "image/png"
	"testing"
)

// segment is one JPEG marker segment with its length.
func segment(marker byte, payload []byte) []byte {
	out := []byte{0xFF, marker, 0, 0}
	binary.BigEndian.PutUint16(out[2:], uint16(len(payload)+2))
	return append(out, payload...)
}

// exifWithOrientation is an EXIF payload with Orientation and a GPS-like
// marker string, as a phone writes it, little-endian.
func exifWithOrientation(orientation uint16) []byte {
	var b bytes.Buffer
	b.WriteString("Exif\x00\x00II\x2A\x00\x08\x00\x00\x00")
	b.Write([]byte{1, 0}) // one entry
	b.Write([]byte{0x12, 0x01, 3, 0, 1, 0, 0, 0})
	b.Write([]byte{byte(orientation), 0, 0, 0})
	b.Write([]byte{0, 0, 0, 0})
	b.WriteString("GPS 51.7650N 5.5180E")
	return b.Bytes()
}

// withSegments inserts segments right after a JPEG's SOI marker.
func withSegments(jpg []byte, segments ...[]byte) []byte {
	out := append([]byte(nil), jpg[:2]...)
	for _, s := range segments {
		out = append(out, s...)
	}
	return append(out, jpg[2:]...)
}

func TestJPEGLosesMetadataButKeepsOrientationAndColour(t *testing.T) {
	icc := segment(markerAPP2, []byte("ICC_PROFILE\x00\x01\x01profile-data"))
	in := withSegments(jpeg,
		segment(markerAPP1, exifWithOrientation(6)),
		segment(markerAPP1, []byte("http://ns.adobe.com/xap/1.0/\x00<x:xmpmeta>secret</x:xmpmeta>")),
		segment(0xED, []byte("Photoshop 3.0\x00IPTC city")),
		segment(markerCOM, []byte("taken at home")),
		icc,
	)
	out, err := stripMetadata(".jpg", in)
	if err != nil {
		t.Fatal(err)
	}
	for _, leak := range []string{"GPS", "xmpmeta", "IPTC", "taken at home"} {
		if bytes.Contains(out, []byte(leak)) {
			t.Errorf("stripped JPEG still contains %q", leak)
		}
	}
	if !bytes.Contains(out, []byte("profile-data")) {
		t.Error("the colour profile was removed")
	}
	if got := orientationOf(t, out); got != 6 {
		t.Errorf("orientation = %d, want 6 so a portrait photo keeps standing", got)
	}
	if _, err := imagejpeg.Decode(bytes.NewReader(out)); err != nil {
		t.Fatalf("stripped JPEG does not decode: %v", err)
	}
}

func TestJPEGWithoutRotationGetsNoExif(t *testing.T) {
	out, err := stripMetadata(".jpg", withSegments(jpeg, segment(markerAPP1, exifWithOrientation(1))))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(out, []byte("Exif")) {
		t.Error("an upright photo kept an EXIF block")
	}
}

// orientationOf finds the orientation in the APP1 segments of a JPEG.
func orientationOf(t *testing.T, jpg []byte) uint16 {
	t.Helper()
	for i := 2; i+4 <= len(jpg) && jpg[i+1] != markerSOS; {
		length := int(binary.BigEndian.Uint16(jpg[i+2:]))
		if jpg[i+1] == markerAPP1 {
			if o := exifOrientation(jpg[i+4 : i+2+length]); o != 0 {
				return o
			}
		}
		i += 2 + length
	}
	return 0
}

func TestPNGLosesTextAndExifChunks(t *testing.T) {
	iend := len(png) - 12
	text := pngChunk("tEXt", []byte("Location\x00Our street 12"))
	exif := pngChunk("eXIf", []byte("MM\x00\x2AGPS"))
	in := append(append(append(append([]byte(nil), png[:iend]...), text...), exif...), png[iend:]...)
	out, err := stripMetadata(".png", in)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(out, []byte("Our street")) || bytes.Contains(out, []byte("eXIf")) {
		t.Error("stripped PNG still holds text or EXIF")
	}
	if _, err := imagepng.Decode(bytes.NewReader(out)); err != nil {
		t.Fatalf("stripped PNG does not decode: %v", err)
	}
}

func pngChunk(kind string, data []byte) []byte {
	out := make([]byte, 4, 12+len(data))
	binary.BigEndian.PutUint32(out, uint32(len(data)))
	out = append(out, kind...)
	out = append(out, data...)
	return append(out, 0, 0, 0, 0) // the decoder checks no CRC of chunks it skips
}

func TestWebPLosesExifAndXMP(t *testing.T) {
	vp8x := make([]byte, 10)
	vp8x[0] = webpEXIFFlag | webpXMPFlag
	in := webpContainer(chunk("VP8X", vp8x), chunk("VP8 ", make([]byte, 40)), chunk("EXIF", []byte("GPS here")), chunk("XMP ", []byte("<x/>")))
	out, err := stripMetadata(".webp", in)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(out, []byte("GPS here")) || bytes.Contains(out, []byte("XMP ")) {
		t.Error("stripped WebP still holds EXIF or XMP")
	}
	if flags := out[20]; flags&(webpEXIFFlag|webpXMPFlag) != 0 {
		t.Errorf("VP8X flags = %08b, want the EXIF and XMP bits cleared", flags)
	}
	if size := binary.LittleEndian.Uint32(out[4:]); int(size) != len(out)-8 {
		t.Errorf("RIFF size = %d, want %d", size, len(out)-8)
	}
}

func TestBrokenContainersAreRefused(t *testing.T) {
	for _, tt := range []struct{ ext, data string }{
		{".jpg", "\xFF\xD8\xFF\xE1\xFF\xFF"},
		{".jpg", "\xFF\xD8"},
		{".png", "\x89PNG\r\n\x1a\n\x7F\xFF\xFF\xFFIDAT"},
		{".webp", "RIFF\x00\x00\x00\x00WEBPVP8 \xFF\xFF\xFF\x7F"},
		{".gif", "GIF89a"},
	} {
		if _, err := stripMetadata(tt.ext, []byte(tt.data)); !errors.Is(err, errMalformed) {
			t.Errorf("stripMetadata(%s, %q) = %v, want errMalformed", tt.ext, tt.data, err)
		}
	}
}

func FuzzStripMetadata(f *testing.F) {
	for _, seed := range [][]byte{jpeg, png, webp, withSegments(jpeg, segment(markerAPP1, exifWithOrientation(8)))} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		for _, ext := range []string{".jpg", ".png", ".webp"} {
			out, err := stripMetadata(ext, data)
			if err == nil && len(out) > len(data)+64 {
				t.Fatalf("%s grew from %d to %d bytes", ext, len(data), len(out))
			}
		}
	})
}
