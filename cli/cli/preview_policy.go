package cli

import (
	"context"
	"fmt"
	"strconv"
)

func previewPolicyAdd(ctx context.Context, p *previewer, args []string) ([]string, error) {
	subject := missingArg(args, 0, "subject")
	object := missingArg(args, 1, "object")
	action := missingArg(args, 2, "allow|deny")

	out := []string{fmt.Sprintf("add policy: %s -> %s = %s", subject, object, action)}
	if action != "allow" && action != "deny" && len(args) > 2 {
		out = append(out, fmt.Sprintf("error: action must be 'allow' or 'deny', got %q", action))
	}

	var principals []struct {
		Type string `json:"type"`
		Name string `json:"name"`
	}
	if p.get(ctx, "/api/principals", &principals) {
		found := false
		for _, pr := range principals {
			if pr.Name == subject {
				found = true
				out = append(out, fmt.Sprintf("subject:  %s %q", pr.Type, subject))
			}
		}
		if !found {
			out = append(out, fmt.Sprintf("conflict: no user or user group named %q exists", subject))
		}
	}
	var objects []struct {
		Type string `json:"type"`
		Name string `json:"name"`
	}
	if p.get(ctx, "/api/objects", &objects) {
		found := false
		for _, o := range objects {
			if o.Name == object {
				found = true
				out = append(out, fmt.Sprintf("object:   %s %q", o.Type, object))
			}
		}
		if !found {
			out = append(out, fmt.Sprintf("conflict: nothing named %q exists to grant access to", object))
		}
	}

	var policies []policyJSON
	if p.get(ctx, "/api/policies", &policies) {
		for _, pol := range policies {
			if pol.SubjectName == subject && pol.ObjectName == object {
				if pol.Action == action {
					out = append(out, fmt.Sprintf(
						"policy #%d already says exactly this, so nothing would change", pol.ID))
				} else {
					out = append(out, fmt.Sprintf(
						"replaces policy #%d, which currently says %s", pol.ID, pol.Action))
				}
			}
		}
	}
	return out, nil
}

func previewPolicyDelete(ctx context.Context, p *previewer, args []string) ([]string, error) {
	raw := missingArg(args, 0, "id")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return []string{fmt.Sprintf("delete policy %s", raw),
			"error: the id must be a number (see: lightscale policy list)"}, nil
	}

	var policies []policyJSON
	if !p.get(ctx, "/api/policies", &policies) {
		return nil, nil
	}
	for _, pol := range policies {
		if pol.ID == id {
			return []string{
				fmt.Sprintf("delete policy #%d: %s -> %s = %s",
					pol.ID, pol.SubjectName, pol.ObjectName, pol.Action),
				fmt.Sprintf("%q loses whatever access this rule granted, and open flows are closed",
					pol.SubjectName),
			}, nil
		}
	}
	return []string{fmt.Sprintf("no policy #%d exists, so this would fail", id)}, nil
}
