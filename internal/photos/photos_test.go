package photos

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	imagejpeg "image/jpeg"
	imagepng "image/png"
	"io"
	"os"
	"strings"
	"testing"
)

// Small real images of each format: the store walks their containers to
// remove metadata, so a few magic bytes are not enough.
var (
	jpeg = encodeTestImage(func(w io.Writer, m image.Image) error { return imagejpeg.Encode(w, m, nil) })
	png  = encodeTestImage(imagepng.Encode)
	webp = webpContainer(chunk("VP8 ", make([]byte, 40)))
)

func encodeTestImage(encode func(io.Writer, image.Image) error) []byte {
	m := image.NewRGBA(image.Rect(0, 0, 8, 8))
	m.Set(1, 1, color.RGBA{R: 200, A: 255})
	var b bytes.Buffer
	if err := encode(&b, m); err != nil {
		panic(err)
	}
	return b.Bytes()
}

// chunk is one RIFF chunk, padded to an even size.
func chunk(kind string, payload []byte) []byte {
	out := append([]byte(kind), 0, 0, 0, 0)
	binary.LittleEndian.PutUint32(out[4:], uint32(len(payload)))
	out = append(out, payload...)
	if len(payload)%2 == 1 {
		out = append(out, 0)
	}
	return out
}

// webpContainer wraps chunks in a RIFF WEBP header with the right size.
func webpContainer(chunks ...[]byte) []byte {
	out := []byte("RIFF\x00\x00\x00\x00WEBP")
	for _, c := range chunks {
		out = append(out, c...)
	}
	binary.LittleEndian.PutUint32(out[4:], uint32(len(out)-8))
	return out
}

func newStore(t *testing.T) *Store {
	t.Helper()
	s, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestAddListOpenDelete(t *testing.T) {
	s := newStore(t)
	for _, tt := range []struct {
		data []byte
		ext  string
		typ  string
	}{{jpeg, ".jpg", "image/jpeg"}, {png, ".png", "image/png"}, {webp, ".webp", "image/webp"}} {
		p, err := s.Add(bytes.NewReader(tt.data))
		if err != nil || !strings.HasSuffix(p.Name, tt.ext) || p.Size != int64(len(tt.data)) {
			t.Fatalf("Add(%s) = %+v, %v", tt.typ, p, err)
		}
		f, typ, err := s.Open(p.Name)
		if err != nil || typ != tt.typ {
			t.Fatalf("Open = %q, %v", typ, err)
		}
		got, _ := io.ReadAll(f)
		f.Close()
		if !bytes.Equal(got, tt.data) {
			t.Fatal("the stored photo differs from the upload")
		}
	}
	list, err := s.List()
	if err != nil || len(list) != 3 {
		t.Fatalf("List = %+v, %v", list, err)
	}
	if err := s.Delete(list[0].Name); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(list[0].Name); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second delete: err = %v, want ErrNotFound", err)
	}
}

func TestAddRejectsWhatIsNotAPhoto(t *testing.T) {
	s := newStore(t)
	for name, data := range map[string][]byte{
		"text":  []byte("<html><script>alert(1)</script></html>"),
		"empty": nil,
		"gif":   append([]byte("GIF89a"), make([]byte, 50)...),
	} {
		if _, err := s.Add(bytes.NewReader(data)); !errors.Is(err, ErrNotAPhoto) {
			t.Errorf("%s: err = %v, want ErrNotAPhoto", name, err)
		}
	}
}

func TestAddRejectsOversizedPhotoAndLeavesNothing(t *testing.T) {
	s := newStore(t)
	big := io.MultiReader(bytes.NewReader(jpeg), bytes.NewReader(make([]byte, MaxBytes)))
	if _, err := s.Add(big); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("err = %v, want ErrTooLarge", err)
	}
	entries, _ := os.ReadDir(s.dir)
	if len(entries) != 0 {
		t.Fatalf("files left after a rejected upload: %v", entries)
	}
}

func TestAddStopsAtTheLimit(t *testing.T) {
	s := newStore(t)
	for range MaxPhotos {
		if _, err := s.Add(bytes.NewReader(png)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Add(bytes.NewReader(png)); !errors.Is(err, ErrFull) {
		t.Fatalf("err = %v, want ErrFull", err)
	}
}

func TestOpenAndDeleteRefuseOtherNames(t *testing.T) {
	s := newStore(t)
	if err := os.WriteFile(s.dir+"/notes.txt", []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"notes.txt", "../gdash.db", "..%2fgdash.db", ""} {
		if _, _, err := s.Open(name); !errors.Is(err, ErrNotFound) {
			t.Errorf("Open(%q): err = %v, want ErrNotFound", name, err)
		}
		if err := s.Delete(name); !errors.Is(err, ErrNotFound) {
			t.Errorf("Delete(%q): err = %v, want ErrNotFound", name, err)
		}
	}
	if list, _ := s.List(); len(list) != 0 {
		t.Fatalf("List shows other files: %+v", list)
	}
}
