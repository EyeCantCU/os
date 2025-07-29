//go:build vmtest

package waagent

import (
	"net"
	"testing"

	"github.com/vishvananda/netlink"
)

var (
	azurePlatformIP = net.IPv4(168, 63, 129, 16)
	azureMDSIP      = net.IPv4(169, 254, 169, 254)
)

func TestRoutePresence(t *testing.T) {
	for _, target := range []net.IP{azurePlatformIP, azureMDSIP} {
		routes, err := netlink.RouteGet(target)
		if err != nil {
			t.Errorf("netlink.GetRoute(%v) = err %v, want nil", target, err)
		} else if len(routes) < 1 {
			t.Errorf("found no routes to %v, want at least one", target)
		}
	}
}
