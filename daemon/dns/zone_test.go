package dns

import (
	"strings"
	"testing"
	"time"
)

func TestRender(t *testing.T) {
	z := Zone{
		Domain: "home.example",
		Now:    time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC),
		Records: []Record{
			{Name: "plex", IP: "10.6.1.6"},
			{Name: "jellyfin", IP: "10.6.1.5"},
		},
	}
	out, err := Render(z)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(out, "$ORIGIN home.example.") {
		t.Fatalf("missing $ORIGIN: %s", out)
	}
	if !strings.Contains(out, "2605030000") {
		t.Fatalf("missing serial: %s", out)
	}
	jPos := strings.Index(out, "jellyfin")
	pPos := strings.Index(out, "plex")
	if jPos < 0 || pPos < 0 || jPos > pPos {
		t.Fatalf("expected sorted records, got:\n%s", out)
	}
}

func TestLeafLabel(t *testing.T) {
	if LeafLabel("jellyfin.home.example") != "jellyfin" {
		t.Fatal("expected jellyfin")
	}
	if LeafLabel("solo") != "solo" {
		t.Fatal("expected solo")
	}
	if LeafLabel("") != "" {
		t.Fatal("expected empty for empty input")
	}
	if LeafLabel("host.") != "host" {
		t.Fatal("expected host for trailing dot")
	}
	if LeafLabel(".host") != "" {
		t.Fatal("expected empty leaf label for leading dot")
	}
}

func TestRenderEmptyDomain(t *testing.T) {
	if _, err := Render(Zone{Domain: ""}); err == nil {
		t.Fatal("expected error for empty domain")
	}
}

func TestRenderEmptyRecordsHasSOAOnly(t *testing.T) {
	z := Zone{
		Domain: "home.example",
		Now:    time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
	}
	out, err := Render(z)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(out, "$ORIGIN home.example.\n") {
		t.Fatalf("missing $ORIGIN: %q", out)
	}
	if !strings.Contains(out, "$TTL 300") {
		t.Fatalf("missing $TTL: %q", out)
	}
	if !strings.Contains(out, "IN SOA") {
		t.Fatalf("missing SOA: %q", out)
	}
	if !strings.Contains(out, "2601020000") {
		t.Fatalf("expected serial YYMMDDHHMM: %q", out)
	}
	if strings.Contains(out, "IN A") {
		t.Fatalf("expected no A records: %q", out)
	}
}

func TestRenderTrailingDotInOrigin(t *testing.T) {
	out, err := Render(Zone{
		Domain: "home.example.",
		Now:    time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(out, "$ORIGIN home.example.\n") {
		t.Fatalf("expected single trailing dot: %q", out)
	}
	if strings.Contains(out, "home.example..") {
		t.Fatalf("trailing dot should be normalized, not doubled: %q", out)
	}
	if !strings.Contains(out, "ns.home.example. admin.home.example.") {
		t.Fatalf("SOA should use normalized domain: %q", out)
	}
}

func TestRenderSkipsEmptyNameOrIP(t *testing.T) {
	z := Zone{
		Domain: "home.example",
		Now:    time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC),
		Records: []Record{
			{Name: "", IP: "10.6.1.1"},
			{Name: "noip", IP: ""},
			{Name: "good", IP: "10.6.1.2"},
		},
	}
	out, err := Render(z)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(out, "good") {
		t.Fatalf("expected good record: %q", out)
	}
	if strings.Contains(out, "noip") {
		t.Fatalf("expected noip record skipped: %q", out)
	}
	if strings.Contains(out, "10.6.1.1") {
		t.Fatalf("expected empty-name record skipped: %q", out)
	}
	if c := strings.Count(out, "IN A"); c != 1 {
		t.Fatalf("expected exactly 1 A record, got %d: %q", c, out)
	}
}

func TestRenderSortsByName(t *testing.T) {
	z := Zone{
		Domain: "home.example",
		Now:    time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC),
		Records: []Record{
			{Name: "zebra", IP: "10.6.1.3"},
			{Name: "alpha", IP: "10.6.1.1"},
			{Name: "mango", IP: "10.6.1.2"},
		},
	}
	out, err := Render(z)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	a := strings.Index(out, "alpha")
	m := strings.Index(out, "mango")
	zb := strings.Index(out, "zebra")
	if !(a >= 0 && m >= 0 && zb >= 0 && a < m && m < zb) {
		t.Fatalf("records not sorted by name: a=%d m=%d z=%d\n%s", a, m, zb, out)
	}
}

func TestRenderSerialPassthrough(t *testing.T) {
	z := Zone{
		Domain: "home.example",
		Serial: "9999999999",
		Now:    time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC),
	}
	out, err := Render(z)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(out, "9999999999") {
		t.Fatalf("expected provided serial passthrough: %q", out)
	}
	if strings.Contains(out, "2026050301") {
		t.Fatalf("did not expect generated serial when Serial provided: %q", out)
	}
}
