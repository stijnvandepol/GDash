// Package outbound builds the HTTP client for addresses that an admin types
// in, such as news feeds and calendars. Nookboard runs on home servers, so
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

// ErrBlockedAddress is returned for a connection to an address Nookboard does
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

var (
	// awsMetadata holds the IPv6 address of the AWS instance metadata
	// service (fd00:ec2::254). It is a unique local address, which the
	// LAN rule would otherwise let through.
	awsMetadata = netip.MustParsePrefix("fd00:ec2::/32")
	// nat64 addresses carry an IPv4 address in their last four bytes;
	// 64:ff9b::a9fe:a9fe reaches 169.254.169.254.
	nat64 = netip.MustParsePrefix("64:ff9b::/96")
	// sixToFour (6to4) addresses carry an IPv4 address in bytes 2 to 5.
	sixToFour = netip.MustParsePrefix("2002::/16")
	// teredo addresses carry the client's IPv4 address, inverted, in their
	// last four bytes.
	teredo = netip.MustParsePrefix("2001::/32")
	// ipv4Compatible is the deprecated ::a.b.c.d form; :: and ::1 in it
	// are handled as themselves.
	ipv4Compatible = netip.MustParsePrefix("::/96")
	// alibabaMetadata is Alibaba Cloud's instance metadata service, on a
	// shared address that is not link-local.
	alibabaMetadata = netip.MustParseAddr("100.100.100.200")
)

// embeddedIPv4 returns the IPv4 address that an IPv6 transition address
// reaches, or addr itself.
func embeddedIPv4(addr netip.Addr) netip.Addr {
	b := addr.As16()
	switch {
	case !addr.Is6():
		return addr
	case nat64.Contains(addr):
		return netip.AddrFrom4([4]byte{b[12], b[13], b[14], b[15]})
	case sixToFour.Contains(addr):
		return netip.AddrFrom4([4]byte{b[2], b[3], b[4], b[5]})
	case teredo.Contains(addr):
		return netip.AddrFrom4([4]byte{^b[12], ^b[13], ^b[14], ^b[15]})
	case ipv4Compatible.Contains(addr) && !addr.IsLoopback() && !addr.IsUnspecified():
		return netip.AddrFrom4([4]byte{b[12], b[13], b[14], b[15]})
	}
	return addr
}

// Allowed reports whether Nookboard may fetch from addr.
func Allowed(addr netip.Addr) bool {
	addr = embeddedIPv4(addr.Unmap())
	return addr.IsValid() &&
		addr != alibabaMetadata &&
		!awsMetadata.Contains(addr) &&
		!addr.IsUnspecified() &&
		!addr.IsLinkLocalUnicast() &&
		!addr.IsLinkLocalMulticast() &&
		!addr.IsInterfaceLocalMulticast() &&
		!addr.IsMulticast()
}
