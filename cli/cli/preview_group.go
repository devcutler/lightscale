package cli

import (
	"context"
	"fmt"
)

type groupPreview struct {
	noun       string
	memberNoun string
	listPath   string
	memberPath string
	taken      func(ctx context.Context, p *previewer, name string) (string, bool)
}

var userGroups = groupPreview{
	noun: "user group", memberNoun: "user",
	listPath: "/api/user-groups", memberPath: "/api/users",
	taken: principalNameTaken,
}

var serviceGroups = groupPreview{
	noun: "service group", memberNoun: "service",
	listPath: "/api/service-groups", memberPath: "/api/services",
	taken: objectNameTaken,
}

type namedRow struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	LANMode bool   `json:"lan_mode"`
}

func (g groupPreview) find(ctx context.Context, p *previewer, path, name string) (namedRow, bool) {
	var rows []namedRow
	if !p.get(ctx, path, &rows) {
		return namedRow{}, false
	}
	for _, r := range rows {
		if r.Name == name {
			return r, true
		}
	}
	return namedRow{}, false
}

func (g groupPreview) previewCreate(ctx context.Context, p *previewer, args []string) ([]string, error) {
	name := missingArg(args, 0, "name")
	out := []string{fmt.Sprintf("create %s %q", g.noun, name)}
	if p.changed("lan-mode") && p.flag("lan-mode") == "true" {
		out = append(out, "lan mode: on, so members can reach each other directly")
	}
	out = append(out, fmt.Sprintf("add members with %q", "join "+name+" <"+g.memberNoun+">"))
	if kind, ok := g.taken(ctx, p, name); ok {
		out = append(out, fmt.Sprintf("conflict: the name %q is already used by a %s", name, kind))
	}
	return out, nil
}

func (g groupPreview) previewDelete(ctx context.Context, p *previewer, args []string) ([]string, error) {
	name := missingArg(args, 0, "name")
	row, ok := g.find(ctx, p, g.listPath, name)
	if p.offline {
		return nil, nil
	}
	if !ok {
		return []string{fmt.Sprintf("no %s named %q exists, so this would fail", g.noun, name)}, nil
	}
	out := []string{
		fmt.Sprintf("delete %s %q (id %d)", g.noun, row.Name, row.ID),
		"members are not deleted, they just stop being in this group",
	}
	var policies []policyJSON
	if p.get(ctx, "/api/policies", &policies) {
		want := "user_group"
		if g.memberNoun == "service" {
			want = "service_group"
		}
		n := 0
		for _, pol := range policies {
			if (pol.SubjectType == want && pol.SubjectID == row.ID) ||
				(pol.ObjectType == want && pol.ObjectID == row.ID) {
				n++
				out = append(out, fmt.Sprintf("also removes policy #%d: %s -> %s = %s",
					pol.ID, pol.SubjectName, pol.ObjectName, pol.Action))
			}
		}
		if n == 0 {
			out = append(out, "no policies reference this group")
		}
	}
	return out, nil
}

func (g groupPreview) previewUpdate(ctx context.Context, p *previewer, args []string) ([]string, error) {
	name := missingArg(args, 0, "name")
	row, ok := g.find(ctx, p, g.listPath, name)
	if p.offline {
		return nil, nil
	}
	if !ok {
		return []string{fmt.Sprintf("no %s named %q exists, so this would fail", g.noun, name)}, nil
	}
	out := []string{fmt.Sprintf("update %s %q (id %d)", g.noun, row.Name, row.ID)}
	changed := false
	if p.changed("name") {
		changed = true
		out = append(out, fmt.Sprintf("name:     %s -> %s", row.Name, p.flag("name")))
	}
	if p.changed("lan-mode") {
		changed = true
		out = append(out, fmt.Sprintf("lan mode: %v -> %s", row.LANMode, p.flag("lan-mode")))
	}
	if !changed {
		out = append(out, "nothing to update: no fields were given")
	}
	return out, nil
}

func (g groupPreview) previewJoin(ctx context.Context, p *previewer, args []string) ([]string, error) {
	group := missingArg(args, 0, "group")
	member := missingArg(args, 1, g.memberNoun)

	out := []string{fmt.Sprintf("add %s %q to %s %q", g.memberNoun, member, g.noun, group)}
	if _, ok := g.find(ctx, p, g.listPath, group); !ok && !p.offline {
		out = append(out, fmt.Sprintf("conflict: no %s named %q exists", g.noun, group))
	}
	if _, ok := g.find(ctx, p, g.memberPath, member); !ok && !p.offline {
		out = append(out, fmt.Sprintf("conflict: no %s named %q exists", g.memberNoun, member))
	}
	if p.offline {
		return nil, nil
	}
	out = append(out, fmt.Sprintf("any policy granting access to %q then applies to %q", group, member))
	return out, nil
}

func (g groupPreview) previewLeave(ctx context.Context, p *previewer, args []string) ([]string, error) {
	group := missingArg(args, 0, "group")
	member := missingArg(args, 1, g.memberNoun)

	out := []string{fmt.Sprintf("remove %s %q from %s %q", g.memberNoun, member, g.noun, group)}
	if _, ok := g.find(ctx, p, g.listPath, group); !ok && !p.offline {
		out = append(out, fmt.Sprintf("note: no %s named %q exists; leaving is a no-op", g.noun, group))
	}
	if p.offline {
		return nil, nil
	}
	out = append(out,
		fmt.Sprintf("access granted only through %q is lost, and open flows are closed", group))
	return out, nil
}
