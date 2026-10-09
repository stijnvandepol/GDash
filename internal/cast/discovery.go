package cast

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	"golang.org/x/net/ipv4"
)

const (
	castService = "_googlecast._tcp.local."
	// The querier asks again after this long, because a single mDNS packet
	// on Wi-Fi is easily lost.
	discoveryResendInterval = time.Second
	defaultDiscoveryTime    = 3 * time.Second
	// maxMDNSPacket fits the largest mDNS message over Ethernet with jumbo
	// frames (RFC 6762 section 17 allows up to 9000 bytes).
	maxMDNSPacket = 9000
)

var mdnsGroup = &net.UDPAddr{IP: net.IPv4(224, 0, 0, 251), Port: 5353}

// Device is a Cast device found on the local network.
type Device struct {
	// ID is the stable device ID. The name can be changed in the Google
	// Home app, so displays are linked to the ID.
	ID    string `json:"id"`
	Name  string `json:"name"`
	Model string `json:"model"`
	Addr  string `json:"addr"`
	// HasScreen is false for speakers such as a Nest Mini or a soundbar,
	// which can run a Cast session but cannot show a dashboard.
	HasScreen bool `json:"hasScreen"`
}

// capabilityVideoOut is bit 0 of the "ca" TXT field: the device has video
// output.
const capabilityVideoOut = 1

func hasVideoOut(ca string) bool {
	flags, err := strconv.ParseUint(ca, 10, 64)
	return err == nil && flags&capabilityVideoOut != 0
}

// Discover asks the local network for Cast devices and collects answers
// until ctx ends, or for three seconds if ctx has no deadline.
//
// It queries from an ephemeral port, so devices answer by unicast (RFC 6762
// section 6.7) and Nookboard never needs port 5353, which Avahi on a NAS
// usually holds.
func Discover(ctx context.Context) ([]Device, error) {
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultDiscoveryTime)
		defer cancel()
	}
	deadline, _ := ctx.Deadline()

	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero})
	if err != nil {
		return nil, fmt.Errorf("open mDNS socket: %w", err)
	}
	defer conn.Close()

	query, err := buildQuery()
	if err != nil {
		return nil, err
	}

	ifaces, err := multicastInterfaces()
	if err != nil {
		return nil, err
	}
	sender := ipv4.NewPacketConn(conn)

	acc := newCollector()
	buf := make([]byte, maxMDNSPacket)
	for ctx.Err() == nil {
		if err := sendQuery(sender, ifaces, query); err != nil {
			return nil, err
		}
		readUntil := time.Now().Add(discoveryResendInterval)
		if deadline.Before(readUntil) {
			readUntil = deadline
		}
		if err := readResponses(conn, buf, readUntil, acc); err != nil {
			return nil, err
		}
	}
	return acc.devices(), nil
}

// readResponses collects answers until the read deadline passes.
func readResponses(conn *net.UDPConn, buf []byte, until time.Time, acc *collector) error {
	if err := conn.SetReadDeadline(until); err != nil {
		return err
	}
	for {
		n, from, err := conn.ReadFromUDP(buf)
		if errors.Is(err, os.ErrDeadlineExceeded) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read mDNS response: %w", err)
		}
		// A malformed packet from one device must not stop discovery of the
		// others, so its parse error is dropped on purpose.
		_ = acc.add(buf[:n], from.IP)
	}
}

// multicastInterfaces returns the interfaces a query must go out on. A home
// server often has several (Docker bridges, VPNs, VM adapters) and the
// routing table picks only one of them for multicast, often the wrong one.
func multicastInterfaces() ([]net.Interface, error) {
	all, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("list network interfaces: %w", err)
	}
	var ifaces []net.Interface
	for _, ifc := range all {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagMulticast == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		if hasIPv4(ifc) {
			ifaces = append(ifaces, ifc)
		}
	}
	return ifaces, nil
}

func hasIPv4(ifc net.Interface) bool {
	addrs, err := ifc.Addrs()
	if err != nil {
		return false
	}
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && ipnet.IP.To4() != nil {
			return true
		}
	}
	return false
}

// sendQuery sends the query on every interface. It only fails when no
// interface could send it, because one adapter without a route to the LAN
// (a VPN, for example) must not stop discovery on the others.
func sendQuery(conn *ipv4.PacketConn, ifaces []net.Interface, query []byte) error {
	var sent int
	var lastErr error
	for i := range ifaces {
		if err := conn.SetMulticastInterface(&ifaces[i]); err != nil {
			lastErr = err
			continue
		}
		if _, err := conn.WriteTo(query, nil, mdnsGroup); err != nil {
			lastErr = err
			continue
		}
		sent++
	}
	if sent == 0 {
		return fmt.Errorf("send mDNS query on any of %d interfaces: %w", len(ifaces), lastErr)
	}
	return nil
}
