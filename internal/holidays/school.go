package holidays

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"
)

// The school holidays the Dutch government sets, by the kind the screen
// translates. Other is a holiday the feed names in a way GDash does not
// know; the screen shows its Dutch name.
const (
	Autumn          = "autumn"
	ChristmasBreak  = "christmas"
	Spring          = "spring"
	May             = "may"
	Summer          = "summer"
	OtherSchoolKind = "other"
)

// The school holiday regions, as widgets name them and as the keys of a
// Report.
const (
	North  = "north"
	Middle = "middle"
	South  = "south"
)

// Regions are the school holiday regions a widget can choose.
var Regions = []string{North, Middle, South}

// schoolKinds maps the feed's names, lower case, to kinds.
var schoolKinds = map[string]string{
	"herfstvakantie":    Autumn,
	"kerstvakantie":     ChristmasBreak,
	"voorjaarsvakantie": Spring,
	"meivakantie":       May,
	"zomervakantie":     Summer,
}

// feedRegions maps the feed's regions, lower case, to the regions they
// cover: "heel Nederland" is a holiday of every region.
var feedRegions = map[string][]string{
	"noord":          {North},
	"midden":         {Middle},
	"zuid":           {South},
	"heel nederland": Regions,
}

// SchoolHoliday is one school holiday in one region.
type SchoolHoliday struct {
	Kind string `json:"kind"`
	// Name is the holiday's Dutch name as the feed has it, which the screen
	// shows for OtherSchoolKind.
	Name string `json:"name"`
	// Start and End are the first and last day off, YYYY-MM-DD, End
	// included.
	Start string `json:"start"`
	End   string `json:"end"`
}

// schoolFeed is the part of the government's open data that GDash reads:
// one document per school year. The text fields carry the whitespace of
// the XML they were made from.
type schoolFeed []struct {
	Content []struct {
		Vacations []struct {
			Type    string `json:"type"`
			Regions []struct {
				Region    string `json:"region"`
				StartDate string `json:"startdate"`
				EndDate   string `json:"enddate"`
			} `json:"regions"`
		} `json:"vacations"`
	} `json:"content"`
}

var errNoSchoolHolidays = errors.New("the school holiday feed lists no holidays")

// parseSchool reads the feed into the holidays of each region that have not
// ended before today (YYYY-MM-DD), in order of their start. The feed writes
// a first day as its midnight in UTC and a last day as 23:59 Dutch time in
// UTC, so both are read as Dutch days in loc. A holiday with a region or
// dates GDash cannot read is left out, so one broken entry does not hide
// the rest; a feed without any holiday is an error, so the screen keeps the
// last good list.
func parseSchool(r io.Reader, loc *time.Location, today string) (map[string][]SchoolHoliday, error) {
	var feed schoolFeed
	if err := json.NewDecoder(r).Decode(&feed); err != nil {
		return nil, fmt.Errorf("decode school holidays: %w", err)
	}
	byRegion := map[string][]SchoolHoliday{}
	found := false
	for _, year := range feed {
		for _, content := range year.Content {
			for _, v := range content.Vacations {
				name := strings.TrimSpace(v.Type)
				kind, known := schoolKinds[strings.ToLower(name)]
				if !known {
					kind = OtherSchoolKind
				}
				for _, r := range v.Regions {
					regions := feedRegions[strings.ToLower(strings.TrimSpace(r.Region))]
					start, okStart := dutchDay(r.StartDate, loc)
					end, okEnd := dutchDay(r.EndDate, loc)
					if len(regions) == 0 || name == "" || !okStart || !okEnd || end < start {
						continue
					}
					found = true
					if end < today {
						continue
					}
					for _, region := range regions {
						byRegion[region] = append(byRegion[region], SchoolHoliday{Kind: kind, Name: name, Start: start, End: end})
					}
				}
			}
		}
	}
	if !found {
		return nil, errNoSchoolHolidays
	}
	for region, list := range byRegion {
		slices.SortFunc(list, func(a, b SchoolHoliday) int {
			return cmp.Or(cmp.Compare(a.Start, b.Start), cmp.Compare(a.Kind, b.Kind))
		})
		byRegion[region] = slices.Compact(list)
	}
	return byRegion, nil
}

// dutchDay reads a feed time as the Dutch day it falls on.
func dutchDay(text string, loc *time.Location) (string, bool) {
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(text))
	if err != nil {
		return "", false
	}
	return t.In(loc).Format(dateLayout), true
}
