package displays

import (
	"strings"
	"testing"

	"github.com/stijnvandepol/daycast/internal/validation"
)

func TestValidate(t *testing.T) {
	valid := Display{ID: "keuken", Name: "Keuken TV", CastDeviceID: "e542c41e4d94baa86e46f36dfdebf26e"}
	tests := []struct {
		name       string
		change     func(*Display)
		wantFields []string
	}{
		{"valid", func(*Display) {}, nil},
		{"browser display without cast device", func(d *Display) { d.CastDeviceID = "" }, nil},
		{"uppercase id", func(d *Display) { d.ID = "Keuken" }, []string{"id"}},
		{"id with slash", func(d *Display) { d.ID = "a/b" }, []string{"id"}},
		{"empty name", func(d *Display) { d.Name = "" }, []string{"name"}},
		{"name with padding", func(d *Display) { d.Name = " Keuken " }, []string{"name"}},
		{"name too long", func(d *Display) { d.Name = strings.Repeat("é", 61) }, []string{"name"}},
		{"name of 60 multibyte chars", func(d *Display) { d.Name = strings.Repeat("é", 60) }, nil},
		{"cast id is an IP", func(d *Display) { d.CastDeviceID = "192.168.1.160" }, []string{"castDeviceId"}},
		{"everything wrong", func(d *Display) { d.ID = ""; d.Name = ""; d.CastDeviceID = "x" }, []string{"id", "name", "castDeviceId"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := valid
			tt.change(&d)
			err := Validate(d)
			if tt.wantFields == nil {
				if err != nil {
					t.Fatalf("Validate = %v, want nil", err)
				}
				return
			}
			fields := validation.Fields(err)
			if fields == nil {
				t.Fatalf("Validate = %v, want a validation error", err)
			}
			var got []string
			for _, f := range fields {
				got = append(got, f.Field)
			}
			if strings.Join(got, ",") != strings.Join(tt.wantFields, ",") {
				t.Fatalf("invalid fields = %v, want %v", got, tt.wantFields)
			}
		})
	}
}
