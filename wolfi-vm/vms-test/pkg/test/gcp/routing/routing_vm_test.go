//go:build vmtest

package routing

import (
	"net"
	"testing"

	"github.com/vishvananda/netlink"
)

var (
	googleMDSIP = net.IPv4(169, 254, 169, 254)
)

func TestRoutePresence(t *testing.T) {
	routes, err := netlink.RouteGet(googleMDSIP)
	if err != nil {
		t.Errorf("netlink.RouteGet(%v) = err %v, want nil", googleMDSIP, err)
	} else if len(routes) < 1 {
		t.Errorf("found no routes to %v, want at least one", googleMDSIP)
	}
}
