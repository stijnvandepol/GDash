package calendar

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"strings"
	"time"
)

const (
	// MaxBytes bounds what is read of one feed, unpacked. The iCal
	// address of a Google calendar holds its whole history: ten busy years
	// pass 10 MB. trim drops what it does not need while it reads, so the
	// memory a feed takes is bounded by maxKeptBytes, not by this.
	MaxBytes = 64 << 20
	// maxKeptBytes bounds what trim keeps of one feed for the parser: the
	// recurring events and the events near the window, with only the
	// properties that Parse reads.
	maxKeptBytes = 8 << 20
	// maxLineBytes bounds one kept line, folded lines joined.
	maxLineBytes = 1 << 20
	// maxFirstLineBytes is more than "BEGIN:VCALENDAR" with a byte order
	// mark and spaces; a longer first line is not the start of a calendar.
	maxFirstLineBytes = 64
	// trimMargin keeps events whose dates are this close to the window. The
	// dates are compared without their time and zone, which move an event
	// by at most a day and fourteen hours.
	trimMargin = 48 * time.Hour
	// maxCachedZones bounds the TZID lookups trim remembers for one feed.
	maxCachedZones = 100
)

var (
	// ErrNotCalendar is returned for an address that answers with
	// something else than an iCal calendar, such as the web page of a
	// calendar or a login page.
	ErrNotCalendar = errors.New("the address did not return an iCal calendar")
	// ErrTooLarge is returned for a feed above MaxBytes, or one
	// that keeps more than maxKeptBytes for the window.
	ErrTooLarge = errors.New("calendar is larger than the limit")

	errLongLine = errors.New("calendar has a line that is too long")
	errNesting  = errors.New("malformed calendar: BEGIN and END do not match")
)

// eventProps are the properties of an event that Parse reads. Descriptions,
// attendees and alarms, often most of a feed, are dropped while reading.
var eventProps = map[string]bool{
	"UID": true, "SUMMARY": true, "STATUS": true, "DTSTART": true, "DTEND": true, "DURATION": true,
	"RRULE": true, "RDATE": true, "EXDATE": true, "RECURRENCE-ID": true,
}

// zonedProps are the event properties that may name a time zone in TZID.
var zonedProps = map[string]bool{"DTSTART": true, "DTEND": true, "RDATE": true, "EXDATE": true, "RECURRENCE-ID": true}

// windowsZones maps the Windows time zone names that Outlook and Exchange
// write in TZID, instead of the IANA names Go knows, to an IANA zone with
// the same rules; the common ones in Western Europe and North America.
var windowsZones = map[string]string{
	"W. Europe Standard Time":         "Europe/Berlin",
	"Romance Standard Time":           "Europe/Paris",
	"GMT Standard Time":               "Europe/London",
	"Greenwich Standard Time":         "Atlantic/Reykjavik",
	"Central Europe Standard Time":    "Europe/Budapest",
	"Central European Standard Time":  "Europe/Warsaw",
	"E. Europe Standard Time":         "Europe/Chisinau",
	"FLE Standard Time":               "Europe/Kiev",
	"GTB Standard Time":               "Europe/Bucharest",
	"Turkey Standard Time":            "Europe/Istanbul",
	"Russian Standard Time":           "Europe/Moscow",
	"Eastern Standard Time":           "America/New_York",
	"Central Standard Time":           "America/Chicago",
	"Mountain Standard Time":          "America/Denver",
	"Pacific Standard Time":           "America/Los_Angeles",
	"AUS Eastern Standard Time":       "Australia/Sydney",
	"India Standard Time":             "Asia/Kolkata",
	"China Standard Time":             "Asia/Shanghai",
	"Tokyo Standard Time":             "Asia/Tokyo",
	"tzone://Microsoft/Utc":           "UTC",
	"Coordinated Universal Time":      "UTC",
	"Central Standard Time (Mexico)":  "America/Mexico_City",
	"E. South America Standard Time":  "America/Sao_Paulo",
	"South Africa Standard Time":      "Africa/Johannesburg",
	"W. Central Africa Standard Time": "Africa/Lagos",
}

// trim reads an ICS document and returns the part that Parse needs for w:
// the calendar's name, every recurring event, and the single events and
// overrides with a date near w, each with only eventProps. It reads the
// feed as a stream, so a calendar with years of history takes little
// memory. A TZID that Go does not know is replaced by its IANA name from
// windowsZones, or dropped, so the time counts in w.Location instead of
// the event being lost.
func trim(r io.Reader, w Window) ([]byte, error) {
	limited := &io.LimitedReader{R: r, N: MaxBytes + 1}
	lines := bufio.NewReader(limited)
	t := &trimmer{window: w, zones: map[string]string{}}
	continued := false
	for !t.done {
		chunk, isPrefix, err := lines.ReadLine()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if err := t.read(chunk, continued); err != nil {
			return nil, err
		}
		continued = isPrefix
	}
	if !t.done {
		if limited.N <= 0 {
			return nil, ErrTooLarge
		}
		// A feed cut short keeps its last line; the parser reports the
		// missing END.
		if err := t.flush(); err != nil {
			return nil, err
		}
	}
	if !t.started {
		return nil, ErrNotCalendar
	}
	return t.out.Bytes(), nil
}

type trimmer struct {
	window  Window
	out     bytes.Buffer
	started bool
	done    bool
	// stack holds the names of the open components, VCALENDAR first.
	stack []string
	// event collects the event that is open directly under VCALENDAR.
	event *pendingEvent
	// line is the logical line being read, folded lines joined, and keep
	// whether it is kept at all; a dropped line is not collected.
	line []byte
	keep bool
	// zones holds the TZID that is kept for each TZID read, "" to drop it.
	zones map[string]string
}

// read takes the next piece of a physical line; continued is true when it
// is the rest of a line longer than the reader's buffer.
func (t *trimmer) read(chunk []byte, continued bool) error {
	switch {
	case continued:
	case len(chunk) > 0 && (chunk[0] == ' ' || chunk[0] == '\t'):
		// A folded line continues the logical line before it.
		chunk = chunk[1:]
	default:
		if err := t.flush(); err != nil {
			return err
		}
		t.keep = t.wanted(propertyName(chunk))
	}
	if !t.keep {
		return nil
	}
	if !t.started && len(t.line)+len(chunk) > maxFirstLineBytes {
		return ErrNotCalendar
	}
	if len(t.line)+len(chunk) > maxLineBytes {
		return errLongLine
	}
	t.line = append(t.line, chunk...)
	return nil
}

// wanted reports whether a line with this property name is kept.
func (t *trimmer) wanted(name string) bool {
	switch {
	case !t.started || name == "BEGIN" || name == "END":
		return true
	case len(t.stack) == 2 && t.event != nil:
		return eventProps[name]
	default:
		return len(t.stack) == 1 && name == "X-WR-CALNAME"
	}
}

// flush handles the logical line that was read completely.
func (t *trimmer) flush() error {
	line := string(t.line)
	t.line = t.line[:0]
	if !t.keep || t.done {
		return nil
	}
	if !t.started {
		return t.begin(line)
	}
	name := propertyName([]byte(line))
	_, value, _ := cutValue(line)
	switch {
	case name == "BEGIN":
		t.open(strings.ToUpper(value))
		return nil
	case name == "END":
		return t.close(strings.ToUpper(value))
	case t.event != nil:
		return t.addToEvent(name, line)
	default:
		return t.write(line)
	}
}

func (t *trimmer) begin(line string) error {
	line = strings.TrimSpace(strings.TrimPrefix(line, "\ufeff"))
	if line == "" {
		return nil
	}
	if !strings.EqualFold(line, "BEGIN:VCALENDAR") {
		return ErrNotCalendar
	}
	t.started = true
	t.stack = append(t.stack, "VCALENDAR")
	return t.write("BEGIN:VCALENDAR")
}

func (t *trimmer) open(component string) {
	t.stack = append(t.stack, component)
	if len(t.stack) == 2 && component == "VEVENT" {
		t.event = &pendingEvent{}
	}
}

func (t *trimmer) close(component string) error {
	if len(t.stack) == 0 || t.stack[len(t.stack)-1] != component {
		return errNesting
	}
	t.stack = t.stack[:len(t.stack)-1]
	switch {
	case len(t.stack) == 0:
		t.done = true
		return t.write("END:VCALENDAR")
	case len(t.stack) == 1 && t.event != nil:
		event := t.event
		t.event = nil
		if !event.near(t.window) {
			return nil
		}
		lines := append(append([]string{"BEGIN:VEVENT"}, event.lines...), "END:VEVENT")
		for _, l := range lines {
			if err := t.write(l); err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *trimmer) addToEvent(name, line string) error {
	if zonedProps[name] {
		line = t.withKnownZone(line)
	}
	_, value, _ := cutValue(line)
	t.event.note(name, value)
	t.event.lines = append(t.event.lines, line)
	t.event.size += len(line) + len("\r\n")
	if t.out.Len()+t.event.size > maxKeptBytes {
		return ErrTooLarge
	}
	return nil
}

func (t *trimmer) write(line string) error {
	if t.out.Len()+len(line) > maxKeptBytes {
		return ErrTooLarge
	}
	t.out.WriteString(line)
	t.out.WriteString("\r\n")
	return nil
}

// withKnownZone returns the line with its TZID replaced by one Go knows,
// or without TZID when there is none.
func (t *trimmer) withKnownZone(line string) string {
	head, value, ok := cutValue(line)
	if !ok {
		return line
	}
	parts := splitUnquoted(head, ';')
	kept := parts[:1]
	changed := false
	for _, param := range parts[1:] {
		key, written, _ := strings.Cut(param, "=")
		if !strings.EqualFold(key, "TZID") {
			kept = append(kept, param)
			continue
		}
		tzid := strings.Trim(written, `"`)
		zone := t.zone(tzid)
		changed = changed || zone != tzid
		if zone != "" {
			kept = append(kept, "TZID="+zone)
		}
	}
	if !changed {
		return line
	}
	return strings.Join(kept, ";") + ":" + value
}

func (t *trimmer) zone(tzid string) string {
	if zone, ok := t.zones[tzid]; ok {
		return zone
	}
	zone := knownZone(tzid)
	if len(t.zones) < maxCachedZones {
		t.zones[tzid] = zone
	}
	return zone
}

// knownZone returns tzid when Go knows it, its IANA name when it is a
// Windows name in windowsZones, or "".
func knownZone(tzid string) string {
	if _, err := time.LoadLocation(tzid); err == nil {
		return tzid
	}
	return windowsZones[tzid]
}

// pendingEvent is an event being read, with what trim needs to decide
// whether to keep it.
type pendingEvent struct {
	lines []string
	size  int
	// recurring events are always kept: their occurrences can fall in the
	// window whatever their first date.
	recurring   bool
	hasEnd      bool
	hasDuration bool
	// undated is true when a date could not be read; the parser decides.
	undated     bool
	dated       bool
	first, last time.Time
}

func (e *pendingEvent) note(name, value string) {
	switch name {
	case "RRULE", "RDATE":
		e.recurring = true
	case "DURATION":
		e.hasDuration = true
	case "DTSTART", "DTEND", "RECURRENCE-ID":
		e.hasEnd = e.hasEnd || name == "DTEND"
		day, ok := dayOf(value)
		if !ok {
			e.undated = true
			return
		}
		if !e.dated || day.Before(e.first) {
			e.first = day
		}
		if !e.dated || day.After(e.last) {
			e.last = day
		}
		e.dated = true
	}
}

// near reports whether the event may overlap w. An override is kept when
// either its own dates or the occurrence it replaces are near.
func (e *pendingEvent) near(w Window) bool {
	if e.recurring || e.undated || !e.dated || (e.hasDuration && !e.hasEnd) {
		return true
	}
	return !e.last.Before(w.From.Add(-trimMargin)) && !e.first.After(w.To.Add(trimMargin))
}

// dayOf reads the date at the start of an iCalendar DATE or DATE-TIME
// value, such as 20261009 or 20261009T080000Z, as midnight UTC.
func dayOf(value string) (time.Time, bool) {
	const digits = len("20060102")
	if len(value) < digits {
		return time.Time{}, false
	}
	day, err := time.Parse("20060102", value[:digits])
	return day, err == nil
}

// propertyName is the name of a content line, upper case.
func propertyName(line []byte) string {
	if i := bytes.IndexAny(line, ";:"); i >= 0 {
		line = line[:i]
	}
	return strings.ToUpper(string(line))
}

// cutValue splits a content line at the colon before its value, which is
// the first one outside a quoted parameter value.
func cutValue(line string) (head, value string, ok bool) {
	quoted := false
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case '"':
			quoted = !quoted
		case ':':
			if !quoted {
				return line[:i], line[i+1:], true
			}
		}
	}
	return line, "", false
}

// splitUnquoted splits s at every sep outside double quotes.
func splitUnquoted(s string, sep byte) []string {
	var parts []string
	quoted, start := false, 0
	for i := 0; i < len(s); i++ {
		switch {
		case s[i] == '"':
			quoted = !quoted
		case s[i] == sep && !quoted:
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	return append(parts, s[start:])
}
