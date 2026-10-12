package cast

import "errors"

// ErrDeviceNotFound is wrapped by a keeper's Resolve when discovery did not
// find the device, so the web interface can say so instead of "no answer".
var ErrDeviceNotFound = errors.New("cast device not found on the network")

// State is what a keeper last found on its device, for the web interface.
type State string

// States a keeper reports.
const (
	// StateConnecting: looking for the device or connecting to it.
	StateConnecting State = "connecting"
	// StateUnreachable: the device was not found or did not answer; the
	// keeper keeps trying.
	StateUnreachable State = "unreachable"
	// StateShowing: the receiver runs and has the dashboard.
	StateShowing State = "showing"
	// StateStandby: the receiver runs, but the TV reports standby.
	StateStandby State = "standby"
	// StateOtherApp: another app is on screen, or the TV shows another
	// input; the keeper launches when the screen is free (see
	// ShouldRelaunch).
	StateOtherApp State = "other_app"
	// StateYielded: someone took the TV from GDash, or used it when GDash
	// started. The keeper launches nothing until the TV has been in
	// standby, or an admin takes the TV back.
	StateYielded State = "yielded"
	// StateResting: outside the display's active hours.
	StateResting State = "resting"
)

// report passes a state to the keeper's Status callback, if any. err
// explains StateUnreachable and is nil otherwise.
func (k *Keeper) report(state State, err error) {
	if k.Status != nil {
		k.Status(state, err)
	}
}
