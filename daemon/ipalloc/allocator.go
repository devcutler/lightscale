package ipalloc

import (
	"errors"
	"fmt"
	"net/netip"
)

var ErrSubnetExhausted = errors.New("ipalloc: subnet exhausted")

func Allocate(cidr string, taken map[string]struct{}, reserved ...string) (string, error) {
	prefix, err := netip.ParsePrefix(cidr)
	if err != nil {
		return "", fmt.Errorf("ipalloc: parse %q: %w", cidr, err)
	}
	if !prefix.Addr().Is4() {
		return "", fmt.Errorf("ipalloc: only IPv4 prefixes supported, got %s", cidr)
	}

	reservedSet := make(map[string]struct{}, len(reserved)+1)
	for _, r := range reserved {
		if r == "" {
			continue
		}
		addr, err := netip.ParseAddr(r)
		if err != nil {
			return "", fmt.Errorf("ipalloc: parse reserved %q: %w", r, err)
		}
		reservedSet[addr.String()] = struct{}{}
	}

	network := prefix.Masked().Addr()
	broadcast := lastAddr(prefix)
	candidate := network.Next()

	for candidate.Less(broadcast) {
		key := candidate.String()
		if _, ok := taken[key]; ok {
			candidate = candidate.Next()
			continue
		}
		if _, ok := reservedSet[key]; ok {
			candidate = candidate.Next()
			continue
		}
		return key, nil
	}
	return "", ErrSubnetExhausted
}
func IsInPrefix(cidr, ip string) (bool, error) {
	prefix, err := netip.ParsePrefix(cidr)
	if err != nil {
		return false, fmt.Errorf("ipalloc: parse cidr %q: %w", cidr, err)
	}
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return false, fmt.Errorf("ipalloc: parse ip %q: %w", ip, err)
	}
	return prefix.Contains(addr), nil
}
func lastAddr(p netip.Prefix) netip.Addr {
	bytes := p.Masked().Addr().As4()
	bits := p.Bits()
	hostBits := 32 - bits
	for i := range hostBits {
		idx := 3 - i/8
		bytes[idx] |= 1 << (i % 8)
	}
	return netip.AddrFrom4(bytes)
}
