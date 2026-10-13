// Package holidays knows the coming school holidays and public holidays of
// the Netherlands. The public holidays are computed (Easter by the Gregorian
// computus, the rest from it or from a fixed date); the school holidays are
// read from the government's open data, which needs no key.
package holidays

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DefaultSchoolURL is the open data of the Dutch government with the school
// holidays of the current and coming school years (CC0).
const DefaultSchoolURL = "https://opendata.rijksoverheid.nl/v1/infotypes/schoolholidays?output=json"

// PublicKey is the key of the public holidays in a Report; the other keys
// are the Regions.
const PublicKey = "public"

// The feed holds about five school years in some 30 kB; anything far larger
// is not the feed and is not read into memory.
const maxResponseBytes = 1 << 20

// Report is what the holidays source holds. Every list is in date order and
// starts today. The regions are only there when the school holidays were
// asked for; an empty list means the feed has no holiday left for it.
type Report struct {
	Public []PublicHoliday `json:"public"`
	North  []SchoolHoliday `json:"north,omitzero"`
	Middle []SchoolHoliday `json:"middle,omitzero"`
	South  []SchoolHoliday `json:"south,omitzero"`
}

// Client builds reports.
type Client struct {
	// SchoolURL is where the school holidays are read, DefaultSchoolURL
	// outside tests.
	SchoolURL string
	// HTTP must have a timeout; the caller owns that choice.
	HTTP      *http.Client
	UserAgent string
	// Location is the Dutch time zone, which decides where a day starts.
	Location *time.Location
	Now      func() time.Time
}

// Report returns the coming public holidays and, when withSchool is set,
// the coming school holidays of every region. Without the school holidays
// it needs no network and cannot fail; with them it fails when the feed
// cannot be read, so the screens keep the last good report.
func (c Client) Report(ctx context.Context, withSchool bool) (Report, error) {
	today := c.Now().In(c.Location)
	report := Report{Public: UpcomingPublicHolidays(today)}
	if !withSchool {
		return report, nil
	}
	school, err := c.school(ctx, today.Format(dateLayout))
	if err != nil {
		return Report{}, err
	}
	// Not nil, so that a region without holidays left is still in the
	// report, and the screen can tell it from one not fetched yet.
	report.North = append([]SchoolHoliday{}, school[North]...)
	report.Middle = append([]SchoolHoliday{}, school[Middle]...)
	report.South = append([]SchoolHoliday{}, school[South]...)
	return report, nil
}

func (c Client) school(ctx context.Context, today string) (map[string][]SchoolHoliday, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.SchoolURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build school holiday request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request school holidays: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("school holiday feed answered %s", resp.Status)
	}
	return parseSchool(io.LimitReader(resp.Body, maxResponseBytes), c.Location, today)
}
