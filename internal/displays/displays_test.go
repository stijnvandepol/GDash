package displays

import (
	"strings"
	"testing"

	"github.com/stijnvandepol/gdash/internal/validation"
)

func TestValidate(t *testing.T) {
	valid := Display{ID: "keuken", Name: "Keuken TV", CastDeviceID: "e542c41e4d94baa86e46f36dfdebf26e", TextScale: 100}
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
		{"morning slot", func(d *Display) { d.Schedule = []Slot{{From: "06:30", Until: "09:00", DashboardID: "morning"}} }, nil},
		{"slot past midnight", func(d *Display) { d.Schedule = []Slot{{From: "22:00", Until: "01:00", DashboardID: "night"}} }, nil},
		{"slot without times", func(d *Display) { d.Schedule = []Slot{{DashboardID: "morning"}} }, []string{"schedule[0]"}},
		{"slot with one time", func(d *Display) { d.Schedule = []Slot{{From: "06:30", DashboardID: "morning"}} }, []string{"schedule[0]"}},
		{"weekend slot", func(d *Display) {
			d.Schedule = []Slot{{From: "08:00", Until: "12:00", Days: []string{"sat", "sun"}, DashboardID: "weekend"}}
		}, nil},
		{"slot with an unknown day", func(d *Display) {
			d.Schedule = []Slot{{From: "08:00", Until: "12:00", Days: []string{"zaterdag"}, DashboardID: "weekend"}}
		}, []string{"schedule[0]"}},
		{"slot without dashboard", func(d *Display) { d.Schedule = []Slot{{From: "06:30", Until: "09:00"}} }, []string{"schedule[0]"}},
		{"too many slots", func(d *Display) { d.Schedule = make([]Slot, MaxSlots+1) }, []string{"schedule"}},
		{"rotation", func(d *Display) { d.Rotation, d.RotateSeconds = []string{"news", "calendar"}, 30 }, nil},
		{"rotation without interval", func(d *Display) { d.Rotation = []string{"news"} }, []string{"rotation"}},
		{"interval without rotation", func(d *Display) { d.RotateSeconds = 30 }, []string{"rotation"}},
		{"rotation too fast", func(d *Display) { d.Rotation, d.RotateSeconds = []string{"news"}, 2 }, []string{"rotation"}},
		{"rotation twice the same", func(d *Display) { d.Rotation, d.RotateSeconds = []string{"news", "news"}, 30 }, []string{"rotation"}},
		{"too many pages", func(d *Display) {
			d.Rotation, d.RotateSeconds = []string{"a", "b", "c", "d", "e", "f"}, 30
		}, []string{"rotation"}},
		{"larger text", func(d *Display) { d.TextScale = 150 }, nil},
		{"odd text size", func(d *Display) { d.TextScale = 130 }, []string{"textScale"}},
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
