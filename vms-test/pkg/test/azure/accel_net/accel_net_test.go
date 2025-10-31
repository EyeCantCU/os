//go:build vmtest

package accel_net

import (
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"chainguard.dev/wolfi-vm/vms-test/pkg/artifacts"
	"chainguard.dev/wolfi-vm/vms-test/pkg/artifacts/metrics"
	"github.com/safchain/ethtool"
	"github.com/vishvananda/netlink"
)

/* Maybe there are better ways of testing accelerated networking.
 * Right now, this is just a reimplemntation of what Azure docs say to do to
 * verify accelerated networking works (except using an API instead of
 * cli).
 * https://learn.microsoft.com/en-us/azure/virtual-network/accelerated-networking-how-it-works#application-usage
 */

var (
	acceleratedDeviceDriverRe = regexp.MustCompile(`^mlx[0-9]_`)
)

func getDriver(t *testing.T, ifname string) string {
	devicepath := filepath.Join("/sys/class/net", ifname, "device")
	_, err := os.Stat(devicepath)
	if os.IsNotExist(err) {
		// This is like docker0 or something, a virtual link
		// with no real device.
		return ""
	}
	if err != nil {
		t.Fatalf("os.Stat(%q) = err %v, want nil or ErrNotExist", devicepath, err)
	}
	driverpath := filepath.Join("/sys/class/net", ifname, "device/driver")
	target, err := os.Readlink(driverpath)
	if err != nil {
		t.Fatalf("os.Readlink(%q) = err %v, want nil", driverpath, err)
	}
	return strings.TrimSpace(filepath.Base(filepath.Clean(target)))
}

func TestVFPackets(t *testing.T) {
	links, err := netlink.LinkList()
	if err != nil {
		t.Fatalf("netlink.LinkList() = err %v want nil", err)
	}
	eth, err := ethtool.NewEthtool()
	if err != nil {
		t.Fatalf("ethtool.New() = err %v want nil", err)
	}
	defer eth.Close()

	// netlink doesn't handle checking for a master reasonably, it shows master
	// index zero if the link has a master with index zero, or if has no master.
	// We need some other method of identifying accelerated networking VF devices.
	// Conventiently, the driver works.
	linkByIndex := make(map[int]netlink.Link)
	var masterIndexes []int

	for _, link := range links {
		if link.Attrs() == nil {
			continue
		}
		linkByIndex[link.Attrs().Index] = link
		if link.Attrs().Flags&net.FlagLoopback != 0 && link.Type() == "device" {
			continue
		}

		driver := getDriver(t, link.Attrs().Name)
		if !acceleratedDeviceDriverRe.MatchString(driver) {
			continue
		}
		t.Logf("identified %s with driver %s as accelerated networking device", link.Attrs().Name, driver)

		masterIndexes = append(masterIndexes, link.Attrs().MasterIndex)
	}

	if len(masterIndexes) < 1 {
		t.Fatalf("could not identify any master links for accelerated networking")
	}

	for _, i := range masterIndexes {
		link, ok := linkByIndex[i]
		if !ok {
			t.Errorf("No link with index %d found", i)
			continue
		}
		t.Logf("identified %s as accelerated networking master", link.Attrs().Name)
		stats, err := eth.Stats(link.Attrs().Name)
		if err != nil {
			t.Errorf("eth.Stats(%s) = err %v want nil", link.Attrs().Name, err)
		}
		stats["index"] = uint64(i)
		artifacts.Log(t, metrics.LinkStatistics, link.Attrs().Name, artifacts.MapOfAny(stats))

		for _, stat := range []string{"vf_tx_packets", "vf_rx_packets"} {
			t.Logf("%s %s: %d", link.Attrs().Name, stat, stats[stat])
			if stats[stat] == 0 {
				t.Errorf("fail, want non-zero statistic for %s", stat)
			}
		}
	}
}
