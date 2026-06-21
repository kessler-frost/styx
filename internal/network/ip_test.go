package network

import (
	"net"
	"testing"
)

// TestGetLocalIP verifies that, when an IP is found, it is a parseable,
// non-loopback IPv4 address (and otherwise returns a descriptive error).
// This runs against the host's real interfaces, so it tolerates the
// no-suitable-IP case rather than asserting a specific address.
func TestGetLocalIP(t *testing.T) {
	ip, err := GetLocalIP()
	if err != nil {
		// Acceptable on hosts without a routable IPv4 interface.
		t.Logf("GetLocalIP returned no address: %v", err)
		return
	}

	parsed := net.ParseIP(ip)
	if parsed == nil {
		t.Fatalf("GetLocalIP returned unparseable IP %q", ip)
	}
	if parsed.To4() == nil {
		t.Errorf("GetLocalIP returned non-IPv4 address %q", ip)
	}
	if parsed.IsLoopback() {
		t.Errorf("GetLocalIP returned loopback address %q", ip)
	}
}

func TestGetInterfaceIP(t *testing.T) {
	// A definitely-nonexistent interface must return an empty string, not panic.
	if got := GetInterfaceIP("this-iface-does-not-exist-0"); got != "" {
		t.Errorf("GetInterfaceIP(nonexistent) = %q, want empty", got)
	}

	// The loopback interface (lo0/lo) has no non-loopback IPv4 we care about,
	// but the call must still be safe and return a string.
	for _, name := range []string{"lo0", "lo"} {
		if ip := GetInterfaceIP(name); ip != "" {
			if net.ParseIP(ip) == nil {
				t.Errorf("GetInterfaceIP(%q) = %q which is not a valid IP", name, ip)
			}
		}
	}
}

// TestStyxNetworkConstants validates the well-known container network identity.
func TestStyxNetworkConstants(t *testing.T) {
	if StyxNetworkName != "styx" {
		t.Errorf("StyxNetworkName = %q, want styx", StyxNetworkName)
	}

	_, subnet, err := net.ParseCIDR(StyxNetworkSubnet)
	if err != nil {
		t.Fatalf("StyxNetworkSubnet %q is not valid CIDR: %v", StyxNetworkSubnet, err)
	}
	if ones, _ := subnet.Mask.Size(); ones != 24 {
		t.Errorf("StyxNetworkSubnet mask = /%d, want /24", ones)
	}
}
