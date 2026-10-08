// Package outbound builds the HTTP client for addresses that an admin types
// in, such as news feeds and calendars. Daycast runs on home servers, so
// LAN and localhost addresses must keep working (a Nextcloud or Radicale at
// home); what it refuses are link-local addresses, where cloud and container
// hosts serve instance metadata and credentials, and addresses that are no
// real destination.
package outbound

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"syscall"
	"time"
)

// ErrBlockedAddress is returned for a connection to an address Daycast does
// not fetch from.
var ErrBlockedAddress = errors.New("this address is not allowed for a data source")

const (
	dialTimeout = 10 * time.Second
	// maxRedirects is lower than Go's default of 10: feeds and calendars
	// redirect once or twice at most.
	maxRedirects = 5
)

// NewClient returns an HTTP client with the given total timeout that only
// connects to allowed addresses. The check runs on the resolved address at
// connect time, so a DNS name cannot point it elsewhere after validation.
// With a proxy from the environment, the proxy decides instead.
func NewClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{Timeout: dialTimeout, Control: control}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = dialer.DialContext
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(_ *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("stopped after %d redirects", maxRedirects)
			}
			return nil
		},
	}
}

func control(_, address string, _ syscall.RawConn) error {
	addrPort, err := netip.ParseAddrPort(address)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrBlockedAddress, err)
	}
	if !Allowed(addrPort.Addr()) {
		return ErrBlockedAddress
	}
	return nil
}

// Allowed reports whether Daycast may fetch from addr.
func Allowed(addr netip.Addr) bool {
	addr = addr.Unmap()
	return addr.IsValid() &&
		!addr.IsUnspecified() &&
		!addr.IsLinkLocalUnicast() &&
		!addr.IsLinkLocalMulticast() &&
		!addr.IsInterfaceLocalMulticast() &&
		!addr.IsMulticast()
}
