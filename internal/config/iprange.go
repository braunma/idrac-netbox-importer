package config

import (
	"fmt"
	"net"
	"strings"
)

// ParseIPRange parses an IP range string like "10.10.10.1-10.10.10.25"
// and returns a slice of individual IP addresses.
func ParseIPRange(rangeStr string) ([]string, error) {
	rangeStr = strings.TrimSpace(rangeStr)

	// Check if it's a range (contains '-')
	if !strings.Contains(rangeStr, "-") {
		// Single IP address
		if ip := net.ParseIP(rangeStr); ip == nil {
			return nil, fmt.Errorf("invalid IP address: %s", rangeStr)
		}
		return []string{rangeStr}, nil
	}

	// Split range into start and end
	parts := strings.Split(rangeStr, "-")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid IP range format (expected 'start-end'): %s", rangeStr)
	}

	startIP := strings.TrimSpace(parts[0])
	endIP := strings.TrimSpace(parts[1])

	// Parse start IP
	start := net.ParseIP(startIP)
	if start == nil {
		return nil, fmt.Errorf("invalid start IP address: %s", startIP)
	}
	start = start.To4()
	if start == nil {
		return nil, fmt.Errorf("only IPv4 ranges are supported: %s", startIP)
	}

	// Parse end IP
	end := net.ParseIP(endIP)
	if end == nil {
		return nil, fmt.Errorf("invalid end IP address: %s", endIP)
	}
	end = end.To4()
	if end == nil {
		return nil, fmt.Errorf("only IPv4 ranges are supported: %s", endIP)
	}

	// Validate that start <= end
	if compareIPs(start, end) > 0 {
		return nil, fmt.Errorf("start IP must be <= end IP: %s-%s", startIP, endIP)
	}

	// Generate all IPs in range, with an early-exit safety limit.
	var ips []string
	for ip := copyIP(start); compareIPs(ip, end) <= 0; incrementIP(ip) {
		ips = append(ips, ip.String())
		if len(ips) > 10000 {
			return nil, fmt.Errorf("IP range too large (max 10000 IPs): %s", rangeStr)
		}
	}

	return ips, nil
}

// ExpandIPRanges takes a slice of IP range strings and expands them all
func ExpandIPRanges(ranges []string) ([]string, error) {
	var allIPs []string
	seen := make(map[string]bool)

	for _, rangeStr := range ranges {
		ips, err := ParseIPRange(rangeStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse range '%s': %w", rangeStr, err)
		}

		// Deduplicate IPs
		for _, ip := range ips {
			if !seen[ip] {
				seen[ip] = true
				allIPs = append(allIPs, ip)
			}
		}
	}

	return allIPs, nil
}

// compareIPs compares two IPv4 addresses
// Returns: -1 if a < b, 0 if a == b, 1 if a > b
func compareIPs(a, b net.IP) int {
	a = a.To4()
	b = b.To4()

	for i := 0; i < 4; i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return 0
}

// incrementIP increments an IPv4 address by 1 (modifies in place)
func incrementIP(ip net.IP) {
	for i := len(ip) - 1; i >= 0; i-- {
		ip[i]++
		if ip[i] > 0 {
			break
		}
	}
}

// copyIP creates a copy of an IP address
func copyIP(ip net.IP) net.IP {
	dup := make(net.IP, len(ip))
	copy(dup, ip)
	return dup
}

// ParseCIDR parses a CIDR notation like "192.168.1.0/24" and returns all IPs
func ParseCIDR(cidr string) ([]string, error) {
	ip, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, fmt.Errorf("invalid CIDR: %w", err)
	}

	// Only support IPv4
	if ip.To4() == nil {
		return nil, fmt.Errorf("only IPv4 CIDR is supported: %s", cidr)
	}

	var ips []string
	for ip := ip.Mask(ipNet.Mask); ipNet.Contains(ip); incrementIP(ip) {
		// Skip network and broadcast addresses for /24 and smaller
		ones, bits := ipNet.Mask.Size()
		if ones < bits {
			ipCopy := copyIP(ip)
			// Skip network address (first) and broadcast (last) for proper subnets
			if !ip.Equal(ipNet.IP) && !isBroadcast(ip, ipNet) {
				ips = append(ips, ipCopy.String())
			}
		} else {
			ips = append(ips, copyIP(ip).String())
		}

		// Safety check
		if len(ips) > 10000 {
			return nil, fmt.Errorf("CIDR range too large (max 10000 IPs): %s", cidr)
		}
	}

	return ips, nil
}

// isBroadcast checks if an IP is the broadcast address for a network
func isBroadcast(ip net.IP, ipNet *net.IPNet) bool {
	broadcast := make(net.IP, len(ipNet.IP))
	for i := range ipNet.IP {
		broadcast[i] = ipNet.IP[i] | ^ipNet.Mask[i]
	}
	return ip.Equal(broadcast)
}

// ValidateIPOrRange validates if a string is a valid IP, IP range, or CIDR
func ValidateIPOrRange(input string) error {
	input = strings.TrimSpace(input)

	// Try as CIDR
	if strings.Contains(input, "/") {
		_, err := ParseCIDR(input)
		return err
	}

	// Try as IP range
	_, err := ParseIPRange(input)
	return err
}

// ExpandServerInput handles all IP input formats: single IP, range, or CIDR
func ExpandServerInput(input string) ([]string, error) {
	input = strings.TrimSpace(input)

	// Check if it's a CIDR notation
	if strings.Contains(input, "/") {
		return ParseCIDR(input)
	}

	// Otherwise treat as IP or IP range
	return ParseIPRange(input)
}

// CountIPsInRange returns how many IPs would be in a range without expanding
func CountIPsInRange(rangeStr string) (int, error) {
	rangeStr = strings.TrimSpace(rangeStr)

	// CIDR notation
	if strings.Contains(rangeStr, "/") {
		_, ipNet, err := net.ParseCIDR(rangeStr)
		if err != nil {
			return 0, err
		}
		ones, bits := ipNet.Mask.Size()
		return 1 << uint(bits-ones), nil
	}

	// Single IP
	if !strings.Contains(rangeStr, "-") {
		if net.ParseIP(rangeStr) == nil {
			return 0, fmt.Errorf("invalid IP: %s", rangeStr)
		}
		return 1, nil
	}

	// IP range
	parts := strings.Split(rangeStr, "-")
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid range format: %s", rangeStr)
	}

	start := net.ParseIP(strings.TrimSpace(parts[0])).To4()
	end := net.ParseIP(strings.TrimSpace(parts[1])).To4()

	if start == nil || end == nil {
		return 0, fmt.Errorf("invalid IP addresses in range: %s", rangeStr)
	}

	// Convert to uint32 for easy counting
	startNum := uint32(start[0])<<24 | uint32(start[1])<<16 | uint32(start[2])<<8 | uint32(start[3])
	endNum := uint32(end[0])<<24 | uint32(end[1])<<16 | uint32(end[2])<<8 | uint32(end[3])

	if startNum > endNum {
		return 0, fmt.Errorf("start IP must be <= end IP: %s", rangeStr)
	}

	return int(endNum - startNum + 1), nil
}
