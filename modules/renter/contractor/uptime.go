package contractor

import (
	"sort"
	"time"

	"github.com/NebulousLabs/Sia/build"
	"github.com/NebulousLabs/Sia/modules"
)

// uptimeMinScans is the minimum number of scans required to judge whether a
// host is offline or not.
const uptimeMinScans = 3

// uptimeWindow specifies the duration in which host uptime is checked.
var uptimeWindow = func() time.Duration {
	switch build.Release {
	case "dev":
		return 30 * time.Minute
	case "standard":
		return 7 * 24 * time.Hour // 1 week
	case "testing":
		return 15 * time.Second
	}
	panic("undefined uptimeWindow")
}()

// IsOffline indicates whether a contract's host should be considered offline,
// based on its scan metrics.
func (c *Contractor) IsOffline(addr modules.NetAddress) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.isOffline(addr)
}

// isOffline indicates whether a contract's host should be considered offline,
// based on its scan metrics.
func (c *Contractor) isOffline(addr modules.NetAddress) bool {
	// lookup the contract's host
	host, ok := c.hdb.Host(addr)
	if !ok {
		return false
	}

	// NOTE: ScanHistory should always be ordered from oldest to newest.
	if build.DEBUG && !sort.IsSorted(host.ScanHistory) {
		sort.Sort(host.ScanHistory)
		build.Critical("host's scan history was not sorted")
	}

	// consider a host offline if:
	// 1) The host has been scanned at least three times, and
	// 2) The three most recent scans have all failed, and
	// 3) The time between the most recent scan and the last successful scan
	//    (or first scan) is at least uptimeWindow
	numScans := len(host.ScanHistory)
	if numScans < uptimeMinScans {
		// not enough data to make a fair judgment
		return false
	}
	recent := host.ScanHistory[numScans-uptimeMinScans:]
	for _, scan := range recent {
		if scan.Success {
			// one of the scans succeeded
			return false
		}
	}
	// initialize window bounds
	windowStart, windowEnd := host.ScanHistory[0].Timestamp, host.ScanHistory[numScans-1].Timestamp
	// iterate from newest-oldest, seeking to last successful scan
	for i := numScans - 1; i >= 0; i-- {
		if scan := host.ScanHistory[i]; scan.Success {
			windowStart = scan.Timestamp
			break
		}
	}
	return windowEnd.Sub(windowStart) >= uptimeWindow
}

// onlineContracts returns the subset of the Contractor's contracts whose
// hosts are considered online.
func (c *Contractor) onlineContracts() []modules.RenterContract {
	var cs []modules.RenterContract
	for _, contract := range c.contracts {
		if !c.isOffline(contract.NetAddress) {
			cs = append(cs, contract)
		}
	}
	return cs
}
