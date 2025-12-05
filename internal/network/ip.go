package network

import (
	"fmt"
	"net"
)

// GetLocalIP returns the first non-loopback IPv4 address found on the system.
// This is used for Nomad's advertise block to allow other nodes to connect.
func GetLocalIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", fmt.Errorf("failed to get interface addresses: %w", err)
	}

	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok {
			continue
		}

		ip := ipNet.IP
		if ip.IsLoopback() || ip.To4() == nil {
			continue
		}

		return ip.String(), nil
	}

	return "", fmt.Errorf("no suitable IP address found")
}

// GetInterfaceIP returns the IPv4 address of a specific network interface.
// Returns empty string if interface not found or has no IPv4 address.
func GetInterfaceIP(name string) string {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return ""
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return ""
	}

	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok {
			continue
		}

		if ip := ipNet.IP.To4(); ip != nil {
			return ip.String()
		}
	}

	return ""
}

// GetPreferredIP tries common macOS interfaces (en0, en1) first,
// then falls back to GetLocalIP.
func GetPreferredIP() (string, error) {
	// Try common macOS interfaces first
	for _, iface := range []string{"en0", "en1", "en2"} {
		if ip := GetInterfaceIP(iface); ip != "" {
			return ip, nil
		}
	}

	// Fall back to any available IP
	return GetLocalIP()
}
