package main

import (
	"context"
	"fmt"
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
