package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"os"
	"text/tabwriter"
	"time"

	"github.com/stijnvandepol/gdash/internal/cast"
)

// discover lists the Cast devices on the network, to find a TV's ID.
func discover(ctx context.Context) error {
	devices, err := cast.Discover(ctx)
	if err != nil {
		return err
	}
	if len(devices) == 0 {
		fmt.Println("No Cast devices found. Is this machine on the same network and VLAN as the TV?")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tMODEL\tSCREEN\tADDRESS\tID")
	for _, d := range devices {
		fmt.Fprintf(w, "%s\t%s\t%v\t%s\t%s\n", d.Name, d.Model, d.HasScreen, d.Addr, d.ID)
	}
	return w.Flush()
}

// status shows what a Cast device is doing without changing anything on it,
// to check the CASTV2 connection and to see how a device reports its state.
func status(ctx context.Context, device string) error {
	addr := withDefaultPort(device)
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	client, err := cast.Dial(ctx, addr)
	if err != nil {
		return err
	}
	defer client.Close()
	st, err := client.Status(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("standby: %v  active input: %v  gdash may launch: %v\n", st.IsStandBy, st.IsActiveInput, cast.ShouldRelaunch(st))
	if len(st.Applications) == 0 {
		fmt.Println("no apps running")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "APP ID\tNAME\tIDLE SCREEN")
	for _, app := range st.Applications {
		fmt.Fprintf(w, "%s\t%s\t%v\n", app.AppID, app.DisplayName, app.IsIdleScreen)
	}
	return w.Flush()
}

// healthTimeout bounds the check: a server that does not answer is not healthy.
const healthTimeout = 3 * time.Second

// healthcheck asks the server on this machine whether it is up, for the
// container's HEALTHCHECK: the image has no shell, curl or wget. It reads
// the address from the same GDASH_ADDR the server listens on.
func healthcheck(ctx context.Context, getenv func(string) string) error {
	url, err := healthURL(valueOr(getenv("GDASH_ADDR"), defaultAddr))
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, healthTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("gdash is not answering: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("gdash is unhealthy: %s", resp.Status)
	}
	return nil
}

// healthURL turns a listen address such as ":8420" into the URL of the
// health route on this machine. A server that listens on all addresses is
// reached through the loopback address.
func healthURL(listenAddr string) (string, error) {
	host, port, err := net.SplitHostPort(listenAddr)
	if err != nil {
		return "", fmt.Errorf("GDASH_ADDR %q is not a host:port address: %w", listenAddr, err)
	}
	if ip, err := netip.ParseAddr(host); host == "" || (err == nil && ip.IsUnspecified()) {
		host = "127.0.0.1"
	}
	return "http://" + net.JoinHostPort(host, port) + "/healthz", nil
}
