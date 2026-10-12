package cast

import "time"

// claim is what a keeper knows about who controls the screen. It belongs to
// the Keeper, not to one connection, so it survives a reconnect; a server
// restart starts with a new, empty claim.
//
// The policy: GDash keeps the screen it launched, and when someone takes
// the TV away from it, it waits until the TV has been in standby before it
// launches again. See docs/guide.md, "When GDash is not on the TV".
type claim struct {
	// session is the app session GDash launched and believes is on
	// screen; empty when it holds nothing.
	session string
	// stopping is a session GDash sent STOP for itself. When the reply was
	// lost, its disappearance is still expected and not a sign that
	// someone took the TV.
	stopping string
	// sawInput: the TV showed the Cast device's input, or did not say,
	// when GDash launched, or has reported it since. Only then does "not
	// active" mean that someone switched to another input: after a launch
	// over another input, the TV takes a moment to follow through HDMI-CEC.
	sawInput bool
	// pageSent is set once a receiver that takes only its first message
	// (DashCast) got that message and all its resends. A session without
	// it may still show the receiver's empty page, so it is restarted.
	pageSent bool
	// yielded: someone took the TV from GDash. The keeper launches nothing
	// and restarts nothing until the TV reports standby, or an admin takes
	// the TV back.
	yielded bool
	// started is set by the first status the keeper handles. A keeper that
	// starts (with the server) and finds something on the TV cannot know
	// whether GDash launched it before the restart, so it yields.
	started bool
	// replace allows stopping a session of the receiver app that GDash did
	// not launch, as GDash always did: after a standby nobody watches it.
	replace bool
	// force: an admin asked to show the dashboard now, over whatever is on
	// the screen.
	force bool
	// idleSince is when a yielded keeper first saw the device's idle
	// screen in an unbroken run of such statuses; zero when not counting.
	idleSince time.Time
}

// idleYieldTimeout is how long the device must show its own idle screen,
// awake and on its input, before a yield ends. A Nest Hub and TVs without
// HDMI-CEC never report standby, the usual end of a yield, and their idle
// screen means that the radio or cast that took the TV has ended.
const idleYieldTimeout = 10 * time.Minute

// holds reports whether app is the session GDash launched.
func (c *claim) holds(app Application, running bool) bool {
	return running && c.session != "" && app.SessionID == c.session
}

// observe follows the session GDash holds through a fresh status. It returns
// why GDash lost the screen to someone, or "" when it still holds it, held
// nothing, or stopped it itself. A lost session is let go.
func (c *claim) observe(status ReceiverStatus, appID string) string {
	if c.session == "" {
		return ""
	}
	app, running := status.App(appID)
	if !c.holds(app, running) {
		expected := c.stopping == c.session
		c.session, c.stopping = "", ""
		switch {
		case expected:
			return ""
		case running:
			return "another sender started the receiver again"
		case status.hasOtherApp(appID):
			return "another app took the screen"
		default:
			return "the dashboard was closed on the TV"
		}
	}
	c.stopping = ""
	if status.OnThisInput() {
		c.sawInput = true
	}
	switch {
	case status.hasOtherApp(appID):
		return "another app took the screen"
	case c.sawInput && status.OnOtherInput():
		return "the TV switched to another input"
	}
	return ""
}

// idleLongEnough follows, while GDash yields, how long the device has shown
// its own idle screen without a break, and reports whether that lasted
// idleYieldTimeout. Any other status starts the count again.
func (c *claim) idleLongEnough(status ReceiverStatus, now time.Time) bool {
	if !status.showsIdleScreen() {
		c.idleSince = time.Time{}
		return false
	}
	if c.idleSince.IsZero() {
		c.idleSince = now
	}
	if now.Sub(c.idleSince) < idleYieldTimeout {
		return false
	}
	c.idleSince = time.Time{}
	return true
}

// endYield is called when the TV reports standby: whoever used it switched
// it off, so the next time it is on, the screen is GDash's again.
func (c *claim) endYield() {
	c.idleSince = time.Time{}
	c.yielded = false
	c.replace = true
	c.force = false
}

// mayResume reports whether a yielded keeper whose own session still runs
// is back on screen: the TV switched back to the Cast device's input and
// nothing else runs.
func (c *claim) mayResume(status ReceiverStatus, appID string) bool {
	return status.OnThisInput() && !status.hasOtherApp(appID)
}

// mayRestart reports whether the periodic restart may stop and launch the
// receiver: a fresh status shows GDash's own session and nothing else, on
// an awake TV that shows the Cast device's input.
func (c *claim) mayRestart(status ReceiverStatus, appID string) bool {
	app, running := status.App(appID)
	return !c.yielded && c.holds(app, running) && !status.hasOtherApp(appID) &&
		!status.InStandby() && !status.OnOtherInput()
}

// launched records a session GDash just started, on a TV that showed another
// input or not.
func (c *claim) launched(sessionID string, otherInput bool) {
	c.session, c.stopping = sessionID, ""
	c.sawInput, c.pageSent = !otherInput, false
	c.yielded, c.replace, c.force = false, false, false
	c.idleSince = time.Time{}
}
