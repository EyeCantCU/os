//go:build vmtest

package waagent

import (
	"context"
	"net"
	"testing"

	"chainguard.dev/wolfi-vm/vm-test/pkg/systemd"
	"chainguard.dev/wolfi-vm/vm-test/pkg/vmtest"
	"github.com/vishvananda/netlink"
)

var (
	azurePlatformIP = net.IPv4(168, 63, 129, 16)
	azureMDSIP      = net.IPv4(169, 254, 169, 254)
)

func waitForServiceIfPresent(ctx context.Context, t *testing.T, svc string) {
	t.Helper()
	_, err := systemd.SystemctlShow(ctx, svc)
	if err != nil {
		// Assume service doesn't exist on this image by design.
		return
	}
	// Don't care about start time but, this function
	// also waits for the service to be started.
	_, err = systemd.GetServiceStartMonotonic(ctx, svc)
	if err != nil {
		t.Errorf("systemd.GetServiceStartMonotonic(ctx, %q) = err %v, want nil", svc, err)
	}
}

func TestRoutePresence(t *testing.T) {
	ctx := vmtest.Context(t)
	waitForServiceIfPresent(ctx, t, "waagent.service")
	waitForServiceIfPresent(ctx, t, "cloud-init.service")
	for _, target := range []net.IP{azurePlatformIP, azureMDSIP} {
		routes, err := netlink.RouteGet(target)
		if err != nil {
			t.Errorf("netlink.GetRoute(%v) = err %v, want nil", target, err)
		} else if len(routes) < 1 {
			t.Errorf("found no routes to %v, want at least one", target)
		}
	}
}
