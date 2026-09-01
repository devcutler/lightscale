package origin

import (
	"fmt"
	"net/netip"
	"strings"
)

type Kind string

const (
	Host      Kind = "host"
	Container Kind = "container"
	IP        Kind = "ip"
	Hostname  Kind = "hostname"
)

func Valid(k Kind) bool {
	switch k {
	case Host, Container, IP, Hostname:
		return true
	}
	return false
}

type Spec struct {
	Kind    Kind
	Value   string
	Network string
}

func (s Spec) String() string {
	if s.Kind == Host {
		return string(Host)
	}
	return fmt.Sprintf("%s:%s", s.Kind, s.Value)
}

func (s Spec) CacheKey() string {
	return string(s.Kind) + "\x00" + s.Value + "\x00" + s.Network
}

func Validate(s Spec) (Spec, error) {
	s.Kind = Kind(strings.ToLower(strings.TrimSpace(string(s.Kind))))
	s.Value = strings.TrimSpace(s.Value)
	s.Network = strings.TrimSpace(s.Network)

	if !Valid(s.Kind) {
		return Spec{}, fmt.Errorf("origin: unknown kind %q (want one of host, container, ip, hostname)", s.Kind)
	}

	if s.Network != "" && s.Kind != Container {
		return Spec{}, fmt.Errorf("origin: network is only valid with kind %q", Container)
	}

	if s.Kind == Host {
		if s.Value != "" {
			return Spec{}, fmt.Errorf("origin: kind %q takes no value (got %q)", Host, s.Value)
		}
		return s, nil
	}

	if s.Value == "" {
		return Spec{}, fmt.Errorf("origin: kind %q requires a value", s.Kind)
	}

	if s.Kind == IP {
		addr, err := netip.ParseAddr(s.Value)
		if err != nil {
			return Spec{}, fmt.Errorf("origin: %q is not a valid ip address", s.Value)
		}
		if err := SafeBackendAddr(addr); err != nil {
			return Spec{}, err
		}
	}

	return s, nil
}

func SafeBackendAddr(addr netip.Addr) error {
	switch {
	case addr.IsLoopback():
		return fmt.Errorf("origin: refusing loopback backend %s (use kind %q to target the gateway intentionally)", addr, Host)
	case addr.IsLinkLocalUnicast(), addr.IsLinkLocalMulticast():
		return fmt.Errorf("origin: refusing link-local backend %s", addr)
	case addr.IsUnspecified():
		return fmt.Errorf("origin: refusing unspecified backend %s", addr)
	case addr.IsMulticast():
		return fmt.Errorf("origin: refusing multicast backend %s", addr)
	}
	return nil
}

func SafeBackendIP(ip string) error {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return fmt.Errorf("origin: unparseable backend ip %q: %w", ip, err)
	}
	return SafeBackendAddr(addr)
}
