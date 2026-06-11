package policy

import "slices"

type Decision int

const (
	Deny Decision = iota
	Allow
)

func (d Decision) String() string {
	if d == Allow {
		return "allow"
	}
	return "deny"
}

func (idx *Index) CheckService(srcIP, dstVIP string, port int, proto string) (Decision, *UserSnapshot, *ServiceSnapshot) {
	user, ok := idx.PeerByIP[srcIP]
	if !ok {
		return Deny, nil, nil
	}
	svc, ok := idx.ServiceByVIP[dstVIP]
	if !ok {
		return Deny, &user, nil
	}
	if !portAllowed(svc.Ports, port, proto) {
		return Deny, &user, &svc
	}

	subjects := idx.subjectSet(user.ID)
	objects := idx.serviceObjectSet(svc.ID)
	return idx.evaluate(subjects, objects), &user, &svc
}

func (idx *Index) CheckPeer(srcIP, dstIP string) (Decision, *UserSnapshot, *UserSnapshot) {
	src, ok := idx.PeerByIP[srcIP]
	if !ok {
		return Deny, nil, nil
	}
	dst, ok := idx.PeerByIP[dstIP]
	if !ok {
		return Deny, &src, nil
	}

	subjects := idx.subjectSet(src.ID)
	objects := idx.userObjectSet(dst.ID)

	if d, found := idx.evaluateExplicit(subjects, objects); found {
		return d, &src, &dst
	}
	if idx.shareLANGroup(src.ID, dst.ID) {
		return Allow, &src, &dst
	}
	return Deny, &src, &dst
}
func portAllowed(ports []PortSpec, port int, proto string) bool {
	if len(ports) == 0 {
		return true
	}
	for _, p := range ports {
		if p.Port == port && p.Protocol == proto {
			return true
		}
	}
	return false
}

func (idx *Index) subjectSet(userID int64) []ref {
	out := []ref{{"user", userID}}
	for _, gid := range idx.GroupsByUser[userID] {
		out = append(out, ref{"user_group", gid})
	}
	return out
}

func (idx *Index) serviceObjectSet(serviceID int64) []ref {
	out := []ref{{"service", serviceID}}
	for _, gid := range idx.GroupsByService[serviceID] {
		out = append(out, ref{"service_group", gid})
	}
	return out
}

func (idx *Index) userObjectSet(userID int64) []ref {
	out := []ref{{"user", userID}}
	for _, gid := range idx.GroupsByUser[userID] {
		out = append(out, ref{"user_group", gid})
	}
	return out
}

type ref struct {
	Type string
	ID   int64
}

func (idx *Index) evaluate(subjects, objects []ref) Decision {
	d, _ := idx.evaluateExplicit(subjects, objects)
	return d
}

func (idx *Index) evaluateExplicit(subjects, objects []ref) (Decision, bool) {
	var bestID int64 = -1
	var bestAction string
	for _, r := range idx.Rules {
		if !matchesAny(r.SubjectType, r.SubjectID, subjects) {
			continue
		}
		if !matchesAny(r.ObjectType, r.ObjectID, objects) {
			continue
		}
		if r.ID > bestID {
			bestID = r.ID
			bestAction = r.Action
		}
	}
	if bestID < 0 {
		return Deny, false
	}
	if bestAction == "allow" {
		return Allow, true
	}
	return Deny, true
}

func matchesAny(t string, id int64, set []ref) bool {
	for _, r := range set {
		if r.Type == t && r.ID == id {
			return true
		}
	}
	return false
}

func (idx *Index) shareLANGroup(srcID, dstID int64) bool {
	srcGroups := idx.GroupsByUser[srcID]
	for _, gid := range srcGroups {
		g, ok := idx.UserGroups[gid]
		if !ok || !g.LANMode {
			continue
		}
		if slices.Contains(g.Members, dstID) {
			return true
		}
	}
	return false
}
