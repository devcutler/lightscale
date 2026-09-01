package cli

import (
	"context"
	"fmt"
	"net/netip"
	"strings"

	"github.com/devcutler/lightscale/shared/origin"
)

func objectNameTaken(ctx context.Context, p *previewer, name string) (string, bool) {
	var objects []struct {
		Type string `json:"type"`
		Name string `json:"name"`
	}
	if !p.get(ctx, "/api/objects", &objects) {
		return "", false
	}
	for _, o := range objects {
		if o.Name == name {
			return o.Type, true
		}
	}
	return "", false
}

func previewServiceCreate(ctx context.Context, p *previewer, args []string) ([]string, error) {
	name := missingArg(args, 0, "name")

	kind, value, changed, err := (&originFlags{
		container: p.flag("container"),
		ip:        p.flag("ip"),
		hostname:  p.flag("hostname"),
		host:      p.flag("host") == "true",
		network:   p.flag("network"),
	}).resolve(p.cmd)
	if err != nil {
		return []string{"error: " + err.Error()}, nil
	}

	var out []string
	label := "<none given>"
	if changed {
		label = kind
		if value != "" {
			label = kind + " " + value
		}
	}
	out = append(out, fmt.Sprintf("create service %q backed by %s", name, label))

	if !changed {
		out = append(out, "error: an origin is required (--container, --ip, --hostname, or --host)")
	} else if err := validateOrigin(kind, value, p.flag("network")); err != nil {
		out = append(out, "error: "+err.Error())
	}

	var services []serviceJSON
	if !p.get(ctx, "/api/services", &services) {
		return nil, nil
	}

	domain := p.flag("domain")
	if domain == "" {
		domain = p.flag("internal-hostname")
	}
	if domain == "" {
		if suffix := commonDomainSuffix(services); suffix != "" {
			domain = name + "." + suffix
			out = append(out, "domain:      "+domain+"  (auto)")
		} else {
			out = append(out, "domain:      <name>.<the daemon's configured domain>  (auto)")
		}
	} else {
		out = append(out, "domain:      "+domain)
	}

	if ip := p.flag("internal-ip"); ip != "" {
		out = append(out, "internal ip: "+ip)
	} else if next := nextFreeIP(services); next != "" {
		out = append(out, "internal ip: "+next+"  (auto, next free)")
	} else {
		out = append(out, "internal ip: next free address in the service subnet  (auto)")
	}

	ports := p.flag("ports")
	switch {
	case ports != "":
		out = append(out, "ports:       "+ports)
	case kind == "host":
		out = append(out, "error: a host service must declare --ports")
	default:
		out = append(out, "ports:       all (no --ports given)")
	}

	if taken, ok := objectNameTaken(ctx, p, name); ok {
		out = append(out, fmt.Sprintf("conflict: the name %q is already used by a %s", name, taken))
	}
	for _, sv := range services {
		if domain != "" && sv.Hostname == domain {
			out = append(out, fmt.Sprintf("conflict: domain %s already belongs to service %q", domain, sv.Name))
		}
		if ip := p.flag("internal-ip"); ip != "" && sv.IPAddress == ip {
			out = append(out, fmt.Sprintf("conflict: internal ip %s already belongs to service %q", ip, sv.Name))
		}
	}

	if kind == "container" && value != "" {
		var containers []struct {
			Name     string   `json:"name"`
			Shared   bool     `json:"shared"`
			Networks []string `json:"networks"`
		}
		if p.get(ctx, "/api/discover/containers", &containers) {
			found := false
			for _, c := range containers {
				if c.Name != value {
					continue
				}
				found = true
				if c.Shared {
					out = append(out, fmt.Sprintf("backend:     container %q is on a shared network (%s), reachable by name",
						value, strings.Join(c.Networks, ", ")))
				} else {
					out = append(out, fmt.Sprintf("backend:     container %q shares no network with the daemon, "+
						"so it would be resolved through the runtime socket", value))
				}
			}
			if !found {
				out = append(out, fmt.Sprintf("warning: no running container named %q is visible", value))
			}
		}
	}
	return out, nil
}

func previewServiceUpdate(ctx context.Context, p *previewer, args []string) ([]string, error) {
	name := missingArg(args, 0, "name")
	sv, err := lookupServiceByName(ctx, p.opts, name)
	if err != nil {
		return []string{fmt.Sprintf("no service named %q exists, so this would fail", name)}, nil
	}

	out := []string{fmt.Sprintf("update service %q (id %d)", sv.Name, sv.ID)}
	changedAny := false

	kind, value, originChanged, oerr := (&originFlags{
		container: p.flag("container"),
		ip:        p.flag("ip"),
		hostname:  p.flag("hostname"),
		host:      p.flag("host") == "true",
		network:   p.flag("network"),
	}).resolve(p.cmd)
	if oerr != nil {
		return []string{"error: " + oerr.Error()}, nil
	}
	if originChanged {
		changedAny = true
		from := sv.OriginKind
		if sv.OriginValue != "" {
			from += " " + sv.OriginValue
		}
		to := kind
		if value != "" {
			to += " " + value
		}
		out = append(out, fmt.Sprintf("backend:     %s -> %s", from, to))
		if err := validateOrigin(kind, value, p.flag("network")); err != nil {
			out = append(out, "error: "+err.Error())
		}
	}
	for _, f := range []struct{ flag, label, current string }{
		{"name", "name", sv.Name},
		{"domain", "domain", sv.Hostname},
		{"internal-hostname", "domain", sv.Hostname},
		{"internal-ip", "internal ip", sv.IPAddress},
		{"ports", "ports", formatPorts(sv.Ports)},
		{"description", "description", sv.Description},
	} {
		if p.changed(f.flag) {
			changedAny = true
			out = append(out, fmt.Sprintf("%-12s %s -> %s", f.label+":", f.current, p.flag(f.flag)))
		}
	}
	if !changedAny {
		out = append(out, "nothing to update: no fields were given")
	}
	return out, nil
}

func previewServiceDelete(ctx context.Context, p *previewer, args []string) ([]string, error) {
	name := missingArg(args, 0, "name")
	sv, err := lookupServiceByName(ctx, p.opts, name)
	if err != nil {
		return []string{fmt.Sprintf("no service named %q exists, so this would fail", name)}, nil
	}
	out := []string{
		fmt.Sprintf("delete service %q (id %d, internal ip %s)", sv.Name, sv.ID, sv.IPAddress),
	}

	var policies []policyJSON
	if p.get(ctx, "/api/policies", &policies) {
		n := 0
		for _, pol := range policies {
			if pol.ObjectType == "service" && pol.ObjectID == sv.ID {
				n++
				out = append(out, fmt.Sprintf("also removes policy #%d: %s -> %s = %s",
					pol.ID, pol.SubjectName, pol.ObjectName, pol.Action))
			}
		}
		if n == 0 {
			out = append(out, "no policies reference this service")
		}
	}
	out = append(out, "its VIP stops accepting connections and active flows are closed")
	return out, nil
}

func commonDomainSuffix(services []serviceJSON) string {
	for _, sv := range services {
		if _, rest, ok := strings.Cut(sv.Hostname, "."); ok && rest != "" {
			return rest
		}
	}
	return ""
}

func nextFreeIP(services []serviceJSON) string {
	taken := map[netip.Addr]bool{}
	var any netip.Addr
	for _, sv := range services {
		a, err := netip.ParseAddr(sv.IPAddress)
		if err != nil || !a.Is4() {
			continue
		}
		taken[a] = true
		if !any.IsValid() || a.Less(any) {
			any = a
		}
	}
	if !any.IsValid() {
		return ""
	}
	for c := any; c.IsValid(); c = c.Next() {
		if !taken[c] {
			return c.String()
		}
	}
	return ""
}

func validateOrigin(kind, value, network string) error {
	_, err := origin.Validate(origin.Spec{
		Kind:    origin.Kind(kind),
		Value:   value,
		Network: network,
	})
	return err
}
