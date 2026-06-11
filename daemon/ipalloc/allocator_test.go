package ipalloc

import (
	"strings"
	"testing"
)

func TestAllocateSkipsTakenAndReserved(t *testing.T) {
	taken := map[string]struct{}{
		"10.6.0.2": {},
		"10.6.0.3": {},
	}
	got, err := Allocate("10.6.0.0/24", taken, "10.6.0.1")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if got != "10.6.0.4" {
		t.Fatalf("want 10.6.0.4, got %s", got)
	}
}

func TestAllocateExhaustion(t *testing.T) {
	taken := map[string]struct{}{
		"10.0.0.1": {}, "10.0.0.2": {},
	}
	if _, err := Allocate("10.0.0.0/30", taken); err != ErrSubnetExhausted {
		t.Fatalf("want exhaustion, got %v", err)
	}
}

func TestIsInPrefix(t *testing.T) {
	ok, err := IsInPrefix("10.6.0.0/23", "10.6.1.7")
	if err != nil || !ok {
		t.Fatalf("expected match, got ok=%v err=%v", ok, err)
	}
	ok, _ = IsInPrefix("10.6.0.0/23", "10.7.0.1")
	if ok {
		t.Fatalf("expected miss")
	}
}

func TestAllocateTable(t *testing.T) {
	cases := []struct {
		name     string
		cidr     string
		taken    map[string]struct{}
		reserved []string
		want     string
	}{
		{
			name: "empty taken allocates first usable",
			cidr: "10.6.0.0/24",
			want: "10.6.0.1",
		},
		{
			name:  "skips network and starts at first usable",
			cidr:  "192.168.1.0/24",
			taken: map[string]struct{}{},
			want:  "192.168.1.1",
		},
		{
			name:  "deterministic lowest free first",
			cidr:  "10.0.0.0/24",
			taken: map[string]struct{}{"10.0.0.1": {}, "10.0.0.2": {}},
			want:  "10.0.0.3",
		},
		{

			name: "/30 first usable",
			cidr: "10.0.0.0/30",
			want: "10.0.0.1",
		},
		{
			name:  "/30 second usable",
			cidr:  "10.0.0.0/30",
			taken: map[string]struct{}{"10.0.0.1": {}},
			want:  "10.0.0.2",
		},
		{
			name:     "reserved with empty strings skipped",
			cidr:     "10.6.0.0/24",
			reserved: []string{"", "", "10.6.0.1"},
			want:     "10.6.0.2",
		},
		{
			name:     "reserved skips lowest",
			cidr:     "10.6.0.0/24",
			reserved: []string{"10.6.0.1", "10.6.0.2"},
			want:     "10.6.0.3",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Allocate(tc.cidr, tc.taken, tc.reserved...)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}
}

func TestAllocateExhaustionCases(t *testing.T) {
	cases := []struct {
		name  string
		cidr  string
		taken map[string]struct{}
	}{
		{

			name:  "/30 exhausted",
			cidr:  "10.0.0.0/30",
			taken: map[string]struct{}{"10.0.0.1": {}, "10.0.0.2": {}},
		},
		{

			name: "/31 has no usable",
			cidr: "10.0.0.0/31",
		},
		{

			name: "/32 has no usable",
			cidr: "10.0.0.0/32",
		},
		{
			name: "/24 fully exhausted",
			cidr: "10.0.0.0/30",
			taken: map[string]struct{}{
				"10.0.0.1": {}, "10.0.0.2": {},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Allocate(tc.cidr, tc.taken)
			if err != ErrSubnetExhausted {
				t.Fatalf("got %v, want ErrSubnetExhausted", err)
			}
		})
	}
}

func TestAllocateExhaustedSlash24(t *testing.T) {
	taken := make(map[string]struct{}, 254)
	for i := 1; i <= 254; i++ {
		taken[netByte("10.0.0.", i)] = struct{}{}
	}
	if _, err := Allocate("10.0.0.0/24", taken); err != ErrSubnetExhausted {
		t.Fatalf("got %v, want ErrSubnetExhausted", err)
	}
}

func netByte(prefix string, n int) string {
	return prefix + itoa(n)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [4]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func TestAllocateInvalidCIDR(t *testing.T) {
	if _, err := Allocate("not-a-cidr", nil); err == nil {
		t.Fatal("expected error for invalid CIDR")
	}
	if _, err := Allocate("10.0.0.0/33", nil); err == nil {
		t.Fatal("expected error for invalid prefix length")
	}
}

func TestAllocateRejectsIPv6(t *testing.T) {
	_, err := Allocate("fd00::/64", nil)
	if err == nil {
		t.Fatal("expected IPv6 to be rejected")
	}
	if !strings.Contains(err.Error(), "only IPv4") {
		t.Fatalf("expected IPv4-only error, got %v", err)
	}
}

func TestAllocateInvalidReserved(t *testing.T) {
	_, err := Allocate("10.0.0.0/24", nil, "not-an-ip")
	if err == nil {
		t.Fatal("expected error for invalid reserved IP")
	}
	if !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("expected reserved parse error, got %v", err)
	}
}
