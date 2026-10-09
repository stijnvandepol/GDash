package display

import (
	"fmt"
	"time"
)

// Template is a ready-made layout a new user can start from instead of an
// empty grid. Creating a dashboard from one is a plain dashboard create with
// the template's theme and widgets.
type Template struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Theme       string   `json:"theme"`
	Widgets     []Widget `json:"widgets"`
}

// Templates returns the built-in templates. now only dates the countdown:
// it counts to the next New Year, so the template never starts with a date
// that has passed. Home Assistant widgets are left out because they need
// entities that only the user's own installation has.
func Templates(now time.Time) []Template {
	newYear := fmt.Sprintf("%d-01-01", now.Year()+1)
	templates := []Template{
		{
			ID: "kitchen", Name: "Kitchen", Theme: "warm-light",
			Description: "Clock, weather and forecast on top, the calendar and the news below.",
			Widgets: []Widget{
				{ID: "clock", Type: "clock", X: 0, Y: 0, W: 9, H: 5},
				{ID: "weather", Type: "weather", X: 9, Y: 0, W: 6, H: 5},
				{ID: "forecast", Type: "forecast", X: 15, Y: 0, W: 9, H: 5, Settings: map[string]any{"days": 4}},
				{ID: "calendar", Type: "calendar", X: 0, Y: 5, W: 12, H: 9, Settings: map[string]any{"maxItems": 6}},
				{ID: "news", Type: "news", X: 12, Y: 5, W: 12, H: 9, Settings: map[string]any{"maxItems": 5}},
			},
		},
		{
			ID: "hallway", Name: "Hallway", Theme: "high-contrast",
			Description: "A big clock and the weather for the way out, with the next appointments and a reminder.",
			Widgets: []Widget{
				{ID: "clock", Type: "clock", X: 0, Y: 0, W: 14, H: 7},
				{ID: "weather", Type: "weather", X: 14, Y: 0, W: 10, H: 7},
				{ID: "calendar", Type: "calendar", X: 0, Y: 7, W: 14, H: 7, Settings: map[string]any{"maxItems": 4}},
				{ID: "note", Type: "text", X: 14, Y: 7, W: 10, H: 7, Settings: map[string]any{"text": "Keys, wallet, phone?\nBins out on Thursday."}},
			},
		},
		{
			ID: "bedroom", Name: "Bedroom / night", Theme: "night",
			Description: "A calm clock and date in dim amber, with the weather for today and tomorrow below.",
			Widgets: []Widget{
				{ID: "clock", Type: "clock", X: 4, Y: 0, W: 16, H: 9},
				{ID: "forecast", Type: "forecast", X: 6, Y: 9, W: 12, H: 5, Settings: map[string]any{"days": 2}},
			},
		},
		{
			ID: "small-screen", Name: "Small screen", Theme: "minimal-dark",
			Description: "Three large widgets that stay readable on a Nest Hub or a tablet.",
			Widgets: []Widget{
				{ID: "clock", Type: "clock", X: 0, Y: 0, W: 12, H: 7},
				{ID: "weather", Type: "weather", X: 12, Y: 0, W: 12, H: 7},
				{ID: "calendar", Type: "calendar", X: 0, Y: 7, W: 24, H: 7, Settings: map[string]any{"maxItems": 3}},
			},
		},
		{
			ID: "family", Name: "Family", Theme: "glass",
			Description: "A large calendar, a photo slideshow, a countdown to the next New Year and a note.",
			Widgets: []Widget{
				{ID: "calendar", Type: "calendar", X: 0, Y: 0, W: 10, H: 14, Settings: map[string]any{"maxItems": 8}},
				{ID: "photos", Type: "photos", X: 10, Y: 0, W: 14, H: 8, Settings: map[string]any{"seconds": 20}},
				{ID: "countdown", Type: "countdown", X: 10, Y: 8, W: 7, H: 6, Settings: map[string]any{"label": "New Year", "date": newYear}},
				{ID: "note", Type: "text", X: 17, Y: 8, W: 7, H: 6, Settings: map[string]any{"text": "Dinner at 18:00\nSports bags by the door"}},
			},
		},
	}
	for i, t := range templates {
		// Normalize fills in each widget's data source, as for any dashboard.
		templates[i].Widgets = Normalize(Dashboard{Widgets: t.Widgets}).Widgets
	}
	return templates
}
