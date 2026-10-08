package cast

import (
	"net"
	"testing"
)

func ipNet(t *testing.T, cidr string) *net.IPNet {
	t.Helper()
	ip, network, err := net.ParseCIDR(cidr)
	if err != nil {
		t.Fatal(err)
	}
	network.IP = ip
	return network
}

func TestReachableIPPrefersInterfaceOnDeviceSubnet(t *testing.T) {
	// The interfaces of the laptop the bug was found on: VMware, Tailscale
	// and the home Wi-Fi.
	addrs := []net.Addr{
		ipNet(t, "127.0.0.1/8"),
		ipNet(t, "192.168.119.1/24"),
		ipNet(t, "100.75.74.66/32"),
		ipNet(t, "192.168.1.109/24"),
	}
	got := reachableIP(net.ParseIP("192.168.1.160"), addrs, "100.75.74.66")
	if got != "192.168.1.109" {
		t.Fatalf("reachableIP = %q, want the Wi-Fi address on the TV's subnet", got)
	}
}

func TestReachableIPFallsBackWithoutSharedSubnet(t *testing.T) {
	addrs := []net.Addr{ipNet(t, "10.0.0.5/24")}
	if got := reachableIP(net.ParseIP("192.168.1.160"), addrs, "10.0.0.5"); got != "10.0.0.5" {
		t.Fatalf("reachableIP = %q, want the fallback", got)
	}
}

func TestReachableIPHandlesBadInput(t *testing.T) {
	if got := ReachableIP("not-an-address", "fallback"); got != "fallback" {
		t.Fatalf("ReachableIP = %q, want the fallback", got)
	}
}
