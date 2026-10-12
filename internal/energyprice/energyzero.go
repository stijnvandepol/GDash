// Package energyprice fetches the dynamic electricity price in the
// Netherlands per hour from EnergyZero, which needs no API key. The prices
// are in euro per kWh, including VAT, and come grouped by the Dutch day
// with the cheapest stretch of hours found for each.
package energyprice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"slices"
	"time"
)

// DefaultBaseURL is the public EnergyZero price endpoint.
const DefaultBaseURL = "https://api.energyzero.nl/v1/energyprices"

// A day of hourly prices is a few kilobytes; anything far larger is not a
// price list and is not read into memory.
const maxResponseBytes = 1 << 20

const (
	// hourlyInterval and electricity are EnergyZero's codes for the
	// "interval" and "usageType" parameters.
	hourlyInterval = "4"
	electricity    = "1"
	// requestTime is how EnergyZero wants its range: UTC with milliseconds.
	requestTime = "2006-01-02T15:04:05.000Z"
	dateLayout  = "2006-01-02"
	// A price outside this range (euro per kWh) is a broken answer, not a
	// market: the highest Dutch price in years stayed under 1 euro.
	minPrice = -5.0
	maxPrice = 10.0
)

// Hour is the price for one hour, starting at Start.
type Hour struct {
	// Start is the beginning of the hour, written in the Dutch time zone.
	Start time.Time `json:"start"`
	// Price is in euro per kWh, including VAT. It can be negative.
	Price float64 `json:"price"`
}

// Day is the hourly prices of one Dutch calendar day.
type Day struct {
	// Date is the Dutch calendar day, YYYY-MM-DD.
	Date  string `json:"date"`
	Hours []Hour `json:"hours"`
	// Cheapest is the cheapest stretch of consecutive hours of the day, or
	// nil when the day has too few hours to tell.
	Cheapest *Window `json:"cheapest"`
}

// Report is the prices of today and, once EnergyZero has published them in
// the afternoon, tomorrow.
type Report struct {
	Days []Day `json:"days"`
}

// Client fetches prices from EnergyZero.
type Client struct {
	BaseURL string
	// HTTP must have a timeout; the caller owns that choice.
	HTTP *http.Client
	// Location is the Dutch time zone, which decides where a day starts.
	Location *time.Location
	Now      func() time.Time
}

type priceResponse struct {
	Prices []struct {
		ReadingDate string   `json:"readingDate"`
		Price       *float64 `json:"price"`
	} `json:"Prices"`
}

var errNoPrices = errors.New("energyzero response has no prices for today")

// Prices returns the hourly prices from the start of today to the end of
// tomorrow, in one request. It fails when today has no prices, so the
// dashboard keeps the last good list; a missing tomorrow is normal before
// the afternoon.
func (c Client) Prices(ctx context.Context) (Report, error) {
	now := c.Now().In(c.Location)
	from := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, c.Location)
	till := from.AddDate(0, 0, 2).Add(-time.Millisecond)
	query := url.Values{
		"fromDate":  {from.UTC().Format(requestTime)},
		"tillDate":  {till.UTC().Format(requestTime)},
		"interval":  {hourlyInterval},
		"usageType": {electricity},
		"inclBtw":   {"true"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"?"+query.Encode(), nil)
	if err != nil {
		return Report{}, fmt.Errorf("build energyzero request: %w", err)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return Report{}, fmt.Errorf("request energyzero: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Report{}, fmt.Errorf("energyzero answered %s", resp.Status)
	}
	report, err := parse(io.LimitReader(resp.Body, maxResponseBytes), c.Location)
	if err != nil {
		return Report{}, err
	}
	if len(report.Days) == 0 || report.Days[0].Date != now.Format(dateLayout) {
		return Report{}, errNoPrices
	}
	return report, nil
}

// parse reads an EnergyZero answer into days in loc. An hour with a price
// that is missing, absurd or not on the hour is left out, and a repeated
// hour counts once, so one broken entry does not hide the rest.
func parse(r io.Reader, loc *time.Location) (Report, error) {
	var body priceResponse
	if err := json.NewDecoder(r).Decode(&body); err != nil {
		return Report{}, fmt.Errorf("decode energyzero response: %w", err)
	}
	var hours []Hour
	for _, p := range body.Prices {
		start, err := time.Parse(time.RFC3339, p.ReadingDate)
		if err != nil || p.Price == nil || !validPrice(*p.Price) || !start.Equal(start.Truncate(time.Hour)) {
			continue
		}
		hours = append(hours, Hour{Start: start.In(loc), Price: *p.Price})
	}
	slices.SortFunc(hours, func(a, b Hour) int { return a.Start.Compare(b.Start) })
	hours = slices.CompactFunc(hours, func(a, b Hour) bool { return a.Start.Equal(b.Start) })
	return Report{Days: groupByDay(hours)}, nil
}

func validPrice(price float64) bool {
	return !math.IsNaN(price) && price >= minPrice && price <= maxPrice
}

// groupByDay splits sorted hours at the Dutch midnight.
func groupByDay(hours []Hour) []Day {
	days := []Day{}
	for _, h := range hours {
		date := h.Start.Format(dateLayout)
		if len(days) == 0 || days[len(days)-1].Date != date {
			days = append(days, Day{Date: date})
		}
		day := &days[len(days)-1]
		day.Hours = append(day.Hours, h)
	}
	for i := range days {
		if w, ok := CheapestWindow(days[i].Hours, WindowHours); ok {
			days[i].Cheapest = &w
		}
	}
	return days
}
