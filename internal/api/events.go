package api

import (
	"fmt"
	"net/http"
	"time"
)

// displayEvents handles GET /api/displays/{display}/events: a Server-Sent
// Events stream with a "snapshot" event for every change of the display.
func (s *Server) displayEvents(w http.ResponseWriter, r *http.Request) {
	displayID := r.PathValue("display")
	if !validDisplayID.MatchString(displayID) {
		http.NotFound(w, r)
		return
	}
	allowed, err := s.mayShowDisplay(r, displayID)
	if err != nil {
		s.internalError(w, "authorize display stream", err)
		return
	}
	if !allowed {
		writeError(w, http.StatusUnauthorized, "not_paired", "Pair this screen with the display first.")
		return
	}
	rc := http.NewResponseController(w)
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	// nginx buffers responses by default, which holds events back.
	h.Set("X-Accel-Buffering", "no")

	if _, err := fmt.Fprintf(w, "retry: %d\n\n", sseRetryMillis); err != nil {
		return
	}
	if err := rc.Flush(); err != nil {
		s.Logger.Error("event stream cannot flush", "error", err)
		return
	}

	s.streamSnapshots(w, r, rc, displayID)
}

// streamSnapshots writes the display's snapshots as events until the client
// leaves or may no longer see the display.
func (s *Server) streamSnapshots(w http.ResponseWriter, r *http.Request, rc *http.ResponseController, displayID string) {
	messages := s.Snapshots.Subscribe(r.Context(), displayID)
	interval := s.PingInterval
	if interval == 0 {
		interval = defaultPingInterval
	}
	ping := time.NewTicker(interval)
	defer ping.Stop()
	for {
		err := rc.SetWriteDeadline(time.Now().Add(streamWriteTimeout))
		if err != nil {
			return
		}
		select {
		case msg, ok := <-messages:
			if !ok {
				return
			}
			// Snapshots are compact JSON without newlines, so one data
			// line is a complete event.
			_, err = fmt.Fprintf(w, "event: snapshot\ndata: %s\n\n", msg)
		case <-ping.C:
			if !s.stillAllowed(r, displayID) {
				return
			}
			_, err = fmt.Fprint(w, ": ping\n\n")
		}
		if err == nil {
			err = rc.Flush()
		}
		if err != nil {
			return
		}
	}
}

// stillAllowed re-checks an open stream: a screen that was unpaired, or an
// admin whose session ended, must stop receiving data now, not at its next
// reconnect.
func (s *Server) stillAllowed(r *http.Request, displayID string) bool {
	allowed, err := s.mayShowDisplay(r, displayID)
	if err != nil {
		s.Logger.Error("could not re-check display stream", "display", displayID, "error", err)
		return false
	}
	return allowed
}
