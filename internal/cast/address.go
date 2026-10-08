package cast

import "net"

// reachableIP returns the address of this machine that a device at remote
// can reach: the address on an interface that shares remote's subnet.
//
// The address of the connection itself is not enough. With a VPN such as
// Tailscale advertising the home subnet, connections to the TV leave
// through the VPN interface, and a TV cannot reach that address. fallback
// is used when no interface shares the subnet.
func reachableIP(remote net.IP, addrs []net.Addr, fallback string) string {
	for _, a := range addrs {
		ipnet, ok := a.(*net.IPNet)
		if !ok || ipnet.IP.To4() == nil || ipnet.IP.IsLoopback() {
			continue
		}
		if ipnet.Contains(remote) {
			return ipnet.IP.String()
		}
	}
	return fallback
}

// ReachableIP is reachableIP for the interfaces of this machine.
func ReachableIP(remoteAddr, fallback string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return fallback
	}
	remote := net.ParseIP(host)
	addrs, err := net.InterfaceAddrs()
	if remote == nil || err != nil {
		return fallback
	}
	return reachableIP(remote, addrs, fallback)
}
