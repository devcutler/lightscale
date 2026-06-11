package dns

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type Record struct {
	Name string
	IP   string
}
type Zone struct {
	Domain  string
	Serial  string
	Records []Record
	Now     time.Time
}

func Render(z Zone) (string, error) {
	if z.Domain == "" {
		return "", fmt.Errorf("dns: empty domain")
	}
	now := z.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	serial := z.Serial
	if serial == "" {
		serial = now.Format("0601021504")
	}

	domain := strings.TrimSuffix(z.Domain, ".")

	var b strings.Builder
	fmt.Fprintf(&b, "$ORIGIN %s.\n", domain)
	fmt.Fprintf(&b, "$TTL 300\n")
	fmt.Fprintf(&b,
		"@           IN SOA   ns.%s. admin.%s. ( %s 3600 600 86400 60 )\n",
		domain, domain, serial)

	records := append([]Record(nil), z.Records...)
	sort.Slice(records, func(i, j int) bool { return records[i].Name < records[j].Name })

	for _, r := range records {
		if r.Name == "" || r.IP == "" {
			continue
		}
		fmt.Fprintf(&b, "%-12s IN A     %s\n", r.Name, r.IP)
	}
	return b.String(), nil
}

func LeafLabel(fqdn string) string {
	if before, _, ok := strings.Cut(fqdn, "."); ok {
		return before
	}
	return fqdn
}
