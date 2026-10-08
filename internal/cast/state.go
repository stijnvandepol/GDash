package cast

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
	// StateOtherApp: another app is on screen, which the keeper leaves
	// alone (see ShouldRelaunch).
	StateOtherApp State = "other_app"
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
