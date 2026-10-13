package cast

import (
	"context"
	"fmt"
)

// Application is one app running on a Cast device, as reported in a
// RECEIVER_STATUS message.
type Application struct {
	AppID        string `json:"appId"`
	DisplayName  string `json:"displayName"`
	SessionID    string `json:"sessionId"`
	TransportID  string `json:"transportId"`
	IsIdleScreen bool   `json:"isIdleScreen"`
	// StatusText is what the app says it does, such as "Casting: Song".
	StatusText string `json:"statusText"`
	// Namespaces are the message namespaces the app listens on.
	Namespaces []Namespace `json:"namespaces"`
}

// Namespace is one entry of an application's namespaces.
type Namespace struct {
	Name string `json:"name"`
}

// speaks reports whether the app listens on namespace.
func (a Application) speaks(namespace string) bool {
	for _, ns := range a.Namespaces {
		if ns.Name == namespace {
			return true
		}
	}
	return false
}

// ReceiverStatus is the state of a Cast device. IsActiveInput and IsStandBy
// come from HDMI-CEC; they are nil when the device does not report them,
// which is not the same as false.
type ReceiverStatus struct {
	Applications  []Application `json:"applications"`
	IsActiveInput *bool         `json:"isActiveInput"`
	IsStandBy     *bool         `json:"isStandBy"`
}

// InStandby reports whether the TV says it is in standby. A device that
// does not say counts as awake.
func (s ReceiverStatus) InStandby() bool { return s.IsStandBy != nil && *s.IsStandBy }

// OnThisInput reports whether the TV says it shows the Cast device's input.
func (s ReceiverStatus) OnThisInput() bool { return s.IsActiveInput != nil && *s.IsActiveInput }

// OnOtherInput reports whether the TV says it shows another input, such as
// a game console. A device that does not say is not on another input.
func (s ReceiverStatus) OnOtherInput() bool { return s.IsActiveInput != nil && !*s.IsActiveInput }

// showsIdleScreen reports whether the device says it shows its own idle
// screen, such as Backdrop, on an awake TV that is not on another input.
// No apps at all does not count: Google TV reports that while Netflix runs.
func (s ReceiverStatus) showsIdleScreen() bool {
	if len(s.Applications) == 0 || s.InStandby() || s.OnOtherInput() {
		return false
	}
	for _, app := range s.Applications {
		if !app.IsIdleScreen {
			return false
		}
	}
	return true
}

// hasOtherApp reports whether an app other than appID, and other than the
// device's idle screen, is running.
func (s ReceiverStatus) hasOtherApp(appID string) bool {
	for _, app := range s.Applications {
		if app.AppID != appID && !app.IsIdleScreen {
			return true
		}
	}
	return false
}

// App returns the running application with the given app ID, if any.
func (s ReceiverStatus) App(appID string) (Application, bool) {
	for _, app := range s.Applications {
		if app.AppID == appID {
			return app, true
		}
	}
	return Application{}, false
}

// Status asks the device which apps are running.
func (c *Client) Status(ctx context.Context) (ReceiverStatus, error) {
	resp, err := c.request(ctx, map[string]any{"type": "GET_STATUS"})
	if err != nil {
		return ReceiverStatus{}, fmt.Errorf("get receiver status: %w", err)
	}
	return resp.Status, nil
}

// Launch starts appID on the device and returns the running application.
func (c *Client) Launch(ctx context.Context, appID string) (Application, error) {
	resp, err := c.request(ctx, map[string]any{"type": "LAUNCH", "appId": appID})
	if err != nil {
		return Application{}, fmt.Errorf("launch app %s: %w", appID, err)
	}
	if resp.Type == "LAUNCH_ERROR" {
		return Application{}, fmt.Errorf("launch app %s: device refused: %s", appID, resp.Reason)
	}
	app, ok := resp.Status.App(appID)
	if !ok {
		return Application{}, fmt.Errorf("launch app %s: app not running after launch", appID)
	}
	return app, nil
}

// Stop ends the app session with the given ID. The device returns to its
// idle screen, unless a new app is launched right after.
func (c *Client) Stop(ctx context.Context, sessionID string) error {
	resp, err := c.request(ctx, map[string]any{"type": "STOP", "sessionId": sessionID})
	if err != nil {
		return fmt.Errorf("stop session %s: %w", sessionID, err)
	}
	if resp.Type != "RECEIVER_STATUS" {
		return fmt.Errorf("stop session %s: device answered %s", sessionID, resp.Type)
	}
	return nil
}

// Connect opens a virtual connection to a running app, which is required
// before sending it messages.
func (c *Client) Connect(transportID string) error {
	return c.sendJSON(transportID, nsConnection, map[string]any{"type": "CONNECT"})
}

// Send delivers a JSON payload to a running app on the given namespace.
func (c *Client) Send(transportID, namespace string, payload []byte) error {
	return c.write(Message{SourceID: senderID, DestinationID: transportID, Namespace: namespace, Payload: string(payload)})
}
