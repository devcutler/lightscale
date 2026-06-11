package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/netip"
	"strings"
)

func emitJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func table(w io.Writer, headers []string, rows [][]string) {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, r := range rows {
		for i, cell := range r {
			if len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}
	formatRow(w, widths, headers)
	separators := make([]string, len(headers))
	for i := range headers {
		separators[i] = strings.Repeat("-", widths[i])
	}
	formatRow(w, widths, separators)
	for _, r := range rows {
		formatRow(w, widths, r)
	}
}

func formatRow(w io.Writer, widths []int, cells []string) {
	parts := make([]string, len(cells))
	for i, c := range cells {
		parts[i] = padRight(c, widths[i])
	}
	fmt.Fprintln(w, strings.Join(parts, "  "))
}

func padRight(s string, w int) string {
	if len(s) >= w {
		return s
	}
	return s + strings.Repeat(" ", w-len(s))
}

// ip sorter (natural sorting, not character based)
func ipLess(a, b string) bool {
	ipA, errA := netip.ParseAddr(a)
	ipB, errB := netip.ParseAddr(b)
	switch {
	case errA == nil && errB == nil:
		return ipA.Less(ipB)
	case errA == nil:
		return true
	case errB == nil:
		return false
	default:
		return a < b
	}
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
