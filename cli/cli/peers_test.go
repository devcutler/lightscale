package cli

import (
	"testing"
	"time"

	"github.com/devcutler/lightscale/shared/wire"
)

func TestNonEmpty(t *testing.T) {
	if got := nonEmpty("", "fb"); got != "fb" {
		t.Errorf("nonEmpty empty => %q want fb", got)
	}
	if got := nonEmpty("x", "fb"); got != "x" {
		t.Errorf("nonEmpty non-empty => %q want x", got)
	}
}

func TestShortKey(t *testing.T) {
	if got := shortKey("short"); got != "short" {
		t.Errorf("len<=10 => %q", got)
	}
	if got := shortKey("0123456789"); got != "0123456789" {
		t.Errorf("len==10 unchanged => %q", got)
	}
	key := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQR"
	if len(key) != 44 {
		t.Fatalf("test key wrong len %d", len(key))
	}
	want := "abcdef" + "…" + "OPQR"
	if got := shortKey(key); got != want {
		t.Errorf("shortKey(44)=%q want %q", got, want)
	}
}

func TestFormatHandshake(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name string
		t    time.Time
		ago  int64
		want string
	}{
		{"zero", time.Time{}, 999, "(never)"},
		{"negative-clamped", now, -5, "0s ago"},
		{"seconds", now, 42, "42s ago"},
		{"minutes", now, 125, "2m 5s ago"},
		{"hours", now, 3725, "1h 2m ago"},
		{"days", now, 90061, "1d 1h ago"},
	}
	for _, c := range cases {
		if got := formatHandshake(c.t, c.ago); got != c.want {
			t.Errorf("%s: formatHandshake(%v)=%q want %q", c.name, c.ago, got, c.want)
		}
	}
}

func TestFormatBytes(t *testing.T) {
	cases := []struct {
		b    uint64
		want string
	}{
		{0, "0 B"},
		{1023, "1023 B"},
		{1024, "1.00 KiB"},
		{1536, "1.50 KiB"},
		{1048576, "1.00 MiB"},
		{1073741824, "1.00 GiB"},
		{1099511627776, "1.00 TiB"},
	}
	for _, c := range cases {
		if got := formatBytes(c.b); got != c.want {
			t.Errorf("formatBytes(%d)=%q want %q", c.b, got, c.want)
		}
	}
}

func TestFormatPorts(t *testing.T) {
	if got := formatPorts(nil); got != "all" {
		t.Errorf("nil => %q want all", got)
	}
	if got := formatPorts([]wire.ServicePort{}); got != "all" {
		t.Errorf("empty => %q want all", got)
	}
	one := []wire.ServicePort{{Port: 8096, Protocol: "tcp"}}
	if got := formatPorts(one); got != "8096/tcp" {
		t.Errorf("one => %q", got)
	}
	multi := []wire.ServicePort{
		{Port: 8096, Protocol: "tcp"},
		{Port: 19132, Protocol: "udp"},
	}
	if got := formatPorts(multi); got != "8096/tcp,19132/udp" {
		t.Errorf("multi => %q", got)
	}
}
