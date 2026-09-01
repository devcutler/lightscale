package cli

import (
	"context"
	"fmt"
)

func principalNameTaken(ctx context.Context, p *previewer, name string) (string, bool) {
	var principals []struct {
		Type string `json:"type"`
		Name string `json:"name"`
	}
	if !p.get(ctx, "/api/principals", &principals) {
		return "", false
	}
	for _, pr := range principals {
		if pr.Name == name {
			return pr.Type, true
		}
	}
	return "", false
}

func lookupUser(ctx context.Context, p *previewer, name string) (userJSON, bool) {
	var users []userJSON
	if !p.get(ctx, "/api/users", &users) {
		return userJSON{}, false
	}
	for _, u := range users {
		if u.Name == name {
			return u, true
		}
	}
	return userJSON{}, false
}

func previewUserCreate(ctx context.Context, p *previewer, args []string) ([]string, error) {
	name := missingArg(args, 0, "name")
	out := []string{fmt.Sprintf("create user %q", name)}

	if email := p.flag("email"); email != "" {
		out = append(out, "email:       "+email)
	}
	if ip := p.flag("ip"); ip != "" {
		out = append(out, "mesh ip:     "+ip)
	} else {
		out = append(out, "mesh ip:     next free address in the client subnet")
	}
	if ep := p.flag("endpoint"); ep != "" {
		out = append(out, "endpoint:    "+ep+" (overrides the server default)")
	}
	out = append(out,
		"generates a keypair and preshared key, stored by the daemon",
		fmt.Sprintf("afterwards: lightscale user config %s prints the client .conf", name))

	if kind, ok := principalNameTaken(ctx, p, name); ok {
		out = append(out, fmt.Sprintf("conflict: the name %q is already used by a %s", name, kind))
	}
	if ip := p.flag("ip"); ip != "" {
		var users []userJSON
		if p.get(ctx, "/api/users", &users) {
			for _, u := range users {
				if u.IPAddress == ip {
					out = append(out, fmt.Sprintf("conflict: ip %s already belongs to user %q", ip, u.Name))
				}
			}
		}
	}
	return out, nil
}

func previewUserUpdate(ctx context.Context, p *previewer, args []string) ([]string, error) {
	name := missingArg(args, 0, "name")
	u, ok := lookupUser(ctx, p, name)
	if p.offline {
		return nil, nil
	}
	if !ok {
		return []string{fmt.Sprintf("no user named %q exists, so this would fail", name)}, nil
	}

	out := []string{fmt.Sprintf("update user %q (id %d)", u.Name, u.ID)}
	changed := false
	for _, f := range []struct{ flag, label, current string }{
		{"name", "name", u.Name},
		{"email", "email", u.Email},
		{"endpoint", "endpoint", u.Endpoint},
	} {
		if p.changed(f.flag) {
			changed = true
			cur := f.current
			if cur == "" {
				cur = "(unset)"
			}
			val := p.flag(f.flag)
			if val == "" {
				val = "(cleared)"
			}
			out = append(out, fmt.Sprintf("%-10s %s -> %s", f.label+":", cur, val))
		}
	}
	if !changed {
		out = append(out, "nothing to update: no fields were given")
	}
	return out, nil
}

func previewUserDelete(ctx context.Context, p *previewer, args []string) ([]string, error) {
	name := missingArg(args, 0, "name")
	u, ok := lookupUser(ctx, p, name)
	if p.offline {
		return nil, nil
	}
	if !ok {
		return []string{fmt.Sprintf("no user named %q exists, so this would fail", name)}, nil
	}

	out := []string{
		fmt.Sprintf("delete user %q (id %d, mesh ip %s)", u.Name, u.ID, u.IPAddress),
		"their device can no longer connect, and the mesh ip is freed for reuse",
	}
	var policies []policyJSON
	if p.get(ctx, "/api/policies", &policies) {
		n := 0
		for _, pol := range policies {
			if pol.SubjectType == "user" && pol.SubjectID == u.ID {
				n++
				out = append(out, fmt.Sprintf("also removes policy #%d: %s -> %s = %s",
					pol.ID, pol.SubjectName, pol.ObjectName, pol.Action))
			}
		}
		if n == 0 {
			out = append(out, "no policies reference this user")
		}
	}
	return out, nil
}
