package photos

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
)

// Enough of each format for content sniffing; the store never decodes.
var (
	jpeg = append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 100)...)
	png  = append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 100)...)
	webp = append([]byte("RIFF\x00\x00\x00\x00WEBPVP8 "), make([]byte, 100)...)
)

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
