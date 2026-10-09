package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/stijnvandepol/nookboard/internal/announcement"
	"github.com/stijnvandepol/nookboard/internal/validation"
)

// announcementPostWindow is how long a sender that posted too much waits.
const announcementPostWindow = time.Minute

// postAnnouncement handles POST /api/announcements {message, title?,
// seconds?, displays?} with an API token or an admin session. The message
// shows over the dashboards of the named displays (all by default) for
// 5 to 3600 seconds (60 by default); a newer one replaces the older one on
// the displays it targets. 201 with {title?, message, expiresAt, displays};
// 401, 422 naming the invalid fields, 429 after 30 a minute.
func (s *Server) postAnnouncement(w http.ResponseWriter, r *http.Request) {
	from := senderFrom(r.Context())
	if !s.Announcements.Allow(from.key) {
		w.Header().Set("Retry-After", strconv.Itoa(int(announcementPostWindow.Seconds())))
		writeError(w, http.StatusTooManyRequests, "rate_limited", "Too many announcements. Wait a minute and try again.")
		return
	}
	var body struct {
		Title    string   `json:"title"`
		Message  string   `json:"message"`
		Seconds  *int     `json:"seconds"`
		Displays []string `json:"displays"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	list, err := s.Store.Displays(r.Context())
	if err != nil {
		s.internalError(w, "list displays for an announcement", err)
		return
	}
	known := make([]string, len(list))
	for i, d := range list {
		known[i] = d.ID
	}
	posted, err := s.Announcements.Post(announcement.Request{
		Title: body.Title, Message: body.Message, Seconds: body.Seconds, Displays: body.Displays,
	}, known, s.Now())
	if err != nil {
		writeValidationError(w, validation.Fields(err))
		return
	}
	// The sender and the number of displays only: a message can be personal.
	s.Logger.Info("announcement posted", "sender", from.name, "displays", len(posted.Displays))
	s.AnnouncementsChanged(posted.ExpiresAt)
	writeJSON(w, http.StatusCreated, posted)
}
