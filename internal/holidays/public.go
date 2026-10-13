package holidays

import (
	"slices"
	"strings"
	"time"
)

// The public holidays of the Netherlands, by the kind the screen translates.
const (
	NewYear       = "newYear"
	GoodFriday    = "goodFriday"
	Easter        = "easter"
	EasterMonday  = "easterMonday"
	KingsDay      = "kingsDay"
	LiberationDay = "liberationDay"
	Ascension     = "ascension"
	WhitSunday    = "whitSunday"
	WhitMonday    = "whitMonday"
	Christmas     = "christmas"
	BoxingDay     = "boxingDay"
)

const dateLayout = "2006-01-02"

// PublicHoliday is one Dutch public holiday.
type PublicHoliday struct {
	Kind string `json:"kind"`
	// Date is the day, YYYY-MM-DD.
	Date string `json:"date"`
}

// EasterSunday returns the day of Easter in the Gregorian calendar, by the
// anonymous Gregorian algorithm (Meeus, Astronomical Algorithms, chapter 8),
// at midnight UTC.
func EasterSunday(year int) time.Time {
	a := year % 19
	b, c := year/100, year%100
	d, e := b/4, b%4
	f := (b + 8) / 25
	g := (b - f + 1) / 3
	h := (19*a + b - d - g + 15) % 30
	i, k := c/4, c%4
	l := (32 + 2*e + 2*i - h - k) % 7
	m := (a + 11*h + 22*l) / 451
	month := (h + l - 7*m + 114) / 31
	day := (h+l-7*m+114)%31 + 1
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}

// kingsDay is 27 April, the King's birthday, or the Saturday before when
// that is a Sunday.
func kingsDay(year int) time.Time {
	day := time.Date(year, time.April, 27, 0, 0, 0, 0, time.UTC)
	if day.Weekday() == time.Sunday {
		return day.AddDate(0, 0, -1)
	}
	return day
}

// PublicHolidays returns the Dutch public holidays of a year in date order.
// Liberation Day is listed every year; many collective agreements only
// give the day off once every five years, which the screen does not try to
// know.
func PublicHolidays(year int) []PublicHoliday {
	easter := EasterSunday(year)
	day := func(kind string, date time.Time) PublicHoliday {
		return PublicHoliday{Kind: kind, Date: date.Format(dateLayout)}
	}
	holidays := []PublicHoliday{
		day(NewYear, time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC)),
		day(GoodFriday, easter.AddDate(0, 0, -2)),
		day(Easter, easter),
		day(EasterMonday, easter.AddDate(0, 0, 1)),
		day(KingsDay, kingsDay(year)),
		day(LiberationDay, time.Date(year, time.May, 5, 0, 0, 0, 0, time.UTC)),
		day(Ascension, easter.AddDate(0, 0, 39)),
		day(WhitSunday, easter.AddDate(0, 0, 49)),
		day(WhitMonday, easter.AddDate(0, 0, 50)),
		day(Christmas, time.Date(year, time.December, 25, 0, 0, 0, 0, time.UTC)),
		day(BoxingDay, time.Date(year, time.December, 26, 0, 0, 0, 0, time.UTC)),
	}
	// Stable: Ascension and Liberation Day can fall on the same day. Dates
	// as YYYY-MM-DD sort as text.
	slices.SortStableFunc(holidays, func(a, b PublicHoliday) int { return strings.Compare(a.Date, b.Date) })
	return holidays
}

// UpcomingPublicHolidays returns the public holidays from the day of today
// (its date as it is, in its own location) to the end of next year: always
// at least a year ahead.
func UpcomingPublicHolidays(today time.Time) []PublicHoliday {
	from := today.Format(dateLayout)
	var upcoming []PublicHoliday
	for _, h := range append(PublicHolidays(today.Year()), PublicHolidays(today.Year()+1)...) {
		if h.Date >= from {
			upcoming = append(upcoming, h)
		}
	}
	return upcoming
}
