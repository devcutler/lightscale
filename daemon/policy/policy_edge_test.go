package policy

import "testing"

func newIndex() *Index {
	return &Index{
		PeerByIP:        map[string]UserSnapshot{},
		UserByID:        map[int64]UserSnapshot{},
		ServiceByVIP:    map[string]ServiceSnapshot{},
		ServiceByID:     map[int64]ServiceSnapshot{},
		GroupsByUser:    map[int64][]int64{},
		GroupsByService: map[int64][]int64{},
		UserGroups:      map[int64]UserGroupSnapshot{},
	}
}

func (idx *Index) addUser(id int64, ip string) {
	u := UserSnapshot{ID: id, IPAddress: ip}
	idx.PeerByIP[ip] = u
	idx.UserByID[id] = u
}

func (idx *Index) addService(id int64, vip string, ports []PortSpec) {
	s := ServiceSnapshot{ID: id, IPAddress: vip, Ports: ports}
	idx.ServiceByVIP[vip] = s
	idx.ServiceByID[id] = s
}

func TestCheckServiceUnknownSrc(t *testing.T) {
	idx := newIndex()
	idx.addService(1, "10.6.1.5", []PortSpec{{8096, "tcp"}})
	d, u, s := idx.CheckService("10.6.0.99", "10.6.1.5", 8096, "tcp")
	if d != Deny || u != nil || s != nil {
		t.Fatalf("unknown src: want Deny/nil/nil, got %s/%v/%v", d, u, s)
	}
}

func TestCheckServiceUnknownDst(t *testing.T) {
	idx := newIndex()
	idx.addUser(1, "10.6.0.2")
	d, u, s := idx.CheckService("10.6.0.2", "10.6.9.9", 8096, "tcp")
	if d != Deny || u == nil || s != nil {
		t.Fatalf("unknown dst: want Deny/user/nil, got %s/%v/%v", d, u, s)
	}
}

func TestCheckServicePortNotInService(t *testing.T) {
	idx := newIndex()
	idx.addUser(1, "10.6.0.2")
	idx.addService(1, "10.6.1.5", []PortSpec{{8096, "tcp"}})
	idx.Rules = []RuleSnapshot{{ID: 1, SubjectType: "user", SubjectID: 1, ObjectType: "service", ObjectID: 1, Action: "allow"}}
	d, u, s := idx.CheckService("10.6.0.2", "10.6.1.5", 80, "tcp")
	if d != Deny || u == nil || s == nil {
		t.Fatalf("port not declared: want Deny/user/svc, got %s/%v/%v", d, u, s)
	}
}

func TestCheckServiceEmptyPortsWildcard(t *testing.T) {
	idx := newIndex()
	idx.addUser(1, "10.6.0.2")
	idx.addService(1, "10.6.1.5", nil)
	idx.Rules = []RuleSnapshot{{ID: 1, SubjectType: "user", SubjectID: 1, ObjectType: "service", ObjectID: 1, Action: "allow"}}
	for _, port := range []int{1, 80, 443, 65535} {
		if d, _, _ := idx.CheckService("10.6.0.2", "10.6.1.5", port, "tcp"); d != Allow {
			t.Fatalf("wildcard ports should allow %d, got %s", port, d)
		}
	}
}

func TestCheckServiceProtocolCaseSensitive(t *testing.T) {
	idx := newIndex()
	idx.addUser(1, "10.6.0.2")
	idx.addService(1, "10.6.1.5", []PortSpec{{8096, "tcp"}})
	idx.Rules = []RuleSnapshot{{ID: 1, SubjectType: "user", SubjectID: 1, ObjectType: "service", ObjectID: 1, Action: "allow"}}

	if d, _, _ := idx.CheckService("10.6.0.2", "10.6.1.5", 8096, "TCP"); d != Deny {
		t.Fatalf("protocol match is case-sensitive: TCP should Deny, got %s", d)
	}
	if d, _, _ := idx.CheckService("10.6.0.2", "10.6.1.5", 8096, "tcp"); d != Allow {
		t.Fatalf("lowercase tcp should Allow, got %s", d)
	}
}

func TestCheckServiceEmptyProtocol(t *testing.T) {
	idx := newIndex()
	idx.addUser(1, "10.6.0.2")
	idx.addService(1, "10.6.1.5", []PortSpec{{8096, "tcp"}})
	idx.Rules = []RuleSnapshot{{ID: 1, SubjectType: "user", SubjectID: 1, ObjectType: "service", ObjectID: 1, Action: "allow"}}

	if d, _, _ := idx.CheckService("10.6.0.2", "10.6.1.5", 8096, ""); d != Deny {
		t.Fatalf("empty protocol should Deny, got %s", d)
	}
}

func TestCheckServicePortBoundaries(t *testing.T) {
	idx := newIndex()
	idx.addUser(1, "10.6.0.2")
	idx.addService(1, "10.6.1.5", []PortSpec{{0, "tcp"}, {65535, "tcp"}})
	idx.Rules = []RuleSnapshot{{ID: 1, SubjectType: "user", SubjectID: 1, ObjectType: "service", ObjectID: 1, Action: "allow"}}
	if d, _, _ := idx.CheckService("10.6.0.2", "10.6.1.5", 0, "tcp"); d != Allow {
		t.Fatalf("port 0 should Allow, got %s", d)
	}
	if d, _, _ := idx.CheckService("10.6.0.2", "10.6.1.5", 65535, "tcp"); d != Allow {
		t.Fatalf("port 65535 should Allow, got %s", d)
	}
	if d, _, _ := idx.CheckService("10.6.0.2", "10.6.1.5", 1, "tcp"); d != Deny {
		t.Fatalf("undeclared port 1 should Deny, got %s", d)
	}
}

func TestCheckServiceMismatchedProtocol(t *testing.T) {
	idx := newIndex()
	idx.addUser(1, "10.6.0.2")
	idx.addService(1, "10.6.1.5", []PortSpec{{8080, "tcp"}})
	idx.Rules = []RuleSnapshot{{ID: 1, SubjectType: "user", SubjectID: 1, ObjectType: "service", ObjectID: 1, Action: "allow"}}
	if d, _, _ := idx.CheckService("10.6.0.2", "10.6.1.5", 8080, "udp"); d != Deny {
		t.Fatalf("udp:8080 against tcp:8080 service should Deny, got %s", d)
	}
}

func TestSubjectSetUserNoGroups(t *testing.T) {
	idx := newIndex()
	idx.addUser(1, "10.6.0.2")
	idx.addService(1, "10.6.1.5", nil)
	idx.Rules = []RuleSnapshot{{ID: 1, SubjectType: "user_group", SubjectID: 99, ObjectType: "service", ObjectID: 1, Action: "allow"}}
	if d, _, _ := idx.CheckService("10.6.0.2", "10.6.1.5", 80, "tcp"); d != Deny {
		t.Fatalf("user in no groups should not match group rule, got %s", d)
	}
}

func TestSubjectSetUserMultipleGroups(t *testing.T) {
	idx := newIndex()
	idx.addUser(1, "10.6.0.2")
	idx.addService(1, "10.6.1.5", nil)
	idx.GroupsByUser[1] = []int64{10, 20, 30}
	idx.Rules = []RuleSnapshot{{ID: 1, SubjectType: "user_group", SubjectID: 20, ObjectType: "service", ObjectID: 1, Action: "allow"}}
	if d, _, _ := idx.CheckService("10.6.0.2", "10.6.1.5", 80, "tcp"); d != Allow {
		t.Fatalf("rule on any of user's groups should apply, got %s", d)
	}
}

func TestObjectSetServiceMultipleGroups(t *testing.T) {
	idx := newIndex()
	idx.addUser(1, "10.6.0.2")
	idx.addService(1, "10.6.1.5", nil)
	idx.GroupsByService[1] = []int64{40, 50}
	idx.Rules = []RuleSnapshot{{ID: 1, SubjectType: "user", SubjectID: 1, ObjectType: "service_group", ObjectID: 50, Action: "allow"}}
	if d, _, _ := idx.CheckService("10.6.0.2", "10.6.1.5", 80, "tcp"); d != Allow {
		t.Fatalf("rule on any service-group should apply, got %s", d)
	}
}

func TestGroupSubjectToGroupObject(t *testing.T) {
	idx := newIndex()
	idx.addUser(1, "10.6.0.2")
	idx.addService(1, "10.6.1.5", nil)
	idx.GroupsByUser[1] = []int64{10}
	idx.GroupsByService[1] = []int64{40}
	idx.Rules = []RuleSnapshot{{ID: 1, SubjectType: "user_group", SubjectID: 10, ObjectType: "service_group", ObjectID: 40, Action: "allow"}}
	if d, _, _ := idx.CheckService("10.6.0.2", "10.6.1.5", 80, "tcp"); d != Allow {
		t.Fatalf("group->group rule should apply, got %s", d)
	}
}

func TestEvaluateNoRules(t *testing.T) {
	idx := newIndex()
	d, found := idx.evaluateExplicit([]ref{{"user", 1}}, []ref{{"service", 1}})
	if d != Deny || found {
		t.Fatalf("no rules: want Deny/false, got %s/%v", d, found)
	}
}

func TestEvaluateAllow1Deny2(t *testing.T) {
	idx := newIndex()
	subj := []ref{{"user", 1}}
	obj := []ref{{"service", 1}}
	idx.Rules = []RuleSnapshot{
		{ID: 1, SubjectType: "user", SubjectID: 1, ObjectType: "service", ObjectID: 1, Action: "allow"},
		{ID: 2, SubjectType: "user", SubjectID: 1, ObjectType: "service", ObjectID: 1, Action: "deny"},
	}
	if d, found := idx.evaluateExplicit(subj, obj); d != Deny || !found {
		t.Fatalf("allow#1+deny#2: want Deny/true, got %s/%v", d, found)
	}
}

func TestEvaluateDeny1Allow2(t *testing.T) {
	idx := newIndex()
	subj := []ref{{"user", 1}}
	obj := []ref{{"service", 1}}
	idx.Rules = []RuleSnapshot{
		{ID: 1, SubjectType: "user", SubjectID: 1, ObjectType: "service", ObjectID: 1, Action: "deny"},
		{ID: 2, SubjectType: "user", SubjectID: 1, ObjectType: "service", ObjectID: 1, Action: "allow"},
	}
	if d, found := idx.evaluateExplicit(subj, obj); d != Allow || !found {
		t.Fatalf("deny#1+allow#2: want Allow/true, got %s/%v", d, found)
	}
}

func TestEvaluateHighestIdWins(t *testing.T) {
	idx := newIndex()
	subj := []ref{{"user", 1}}
	obj := []ref{{"service", 1}}

	idx.Rules = []RuleSnapshot{
		{ID: 3, SubjectType: "user", SubjectID: 1, ObjectType: "service", ObjectID: 1, Action: "allow"},
		{ID: 1, SubjectType: "user", SubjectID: 1, ObjectType: "service", ObjectID: 1, Action: "allow"},
		{ID: 2, SubjectType: "user", SubjectID: 1, ObjectType: "service", ObjectID: 1, Action: "deny"},
	}
	if d, found := idx.evaluateExplicit(subj, obj); d != Allow || !found {
		t.Fatalf("highest id (3=allow) should win, got %s/%v", d, found)
	}
}

func TestCheckPeerUnknownSrc(t *testing.T) {
	idx := newIndex()
	idx.addUser(2, "10.6.0.3")
	d, a, b := idx.CheckPeer("10.6.0.99", "10.6.0.3")
	if d != Deny || a != nil || b != nil {
		t.Fatalf("unknown src: want Deny/nil/nil, got %s/%v/%v", d, a, b)
	}
}

func TestCheckPeerUnknownDst(t *testing.T) {
	idx := newIndex()
	idx.addUser(1, "10.6.0.2")
	d, a, b := idx.CheckPeer("10.6.0.2", "10.6.0.99")
	if d != Deny || a == nil || b != nil {
		t.Fatalf("unknown dst: want Deny/src/nil, got %s/%v/%v", d, a, b)
	}
}

func TestCheckPeerNoRuleNoLAN(t *testing.T) {
	idx := newIndex()
	idx.addUser(1, "10.6.0.2")
	idx.addUser(2, "10.6.0.3")
	if d, _, _ := idx.CheckPeer("10.6.0.2", "10.6.0.3"); d != Deny {
		t.Fatalf("no rule, no shared LAN group should Deny, got %s", d)
	}
}

func TestCheckPeerExplicitAllow(t *testing.T) {
	idx := newIndex()
	idx.addUser(1, "10.6.0.2")
	idx.addUser(2, "10.6.0.3")
	idx.Rules = []RuleSnapshot{{ID: 1, SubjectType: "user", SubjectID: 1, ObjectType: "user", ObjectID: 2, Action: "allow"}}
	if d, _, _ := idx.CheckPeer("10.6.0.2", "10.6.0.3"); d != Allow {
		t.Fatalf("explicit allow should Allow, got %s", d)
	}
}

func TestCheckPeerExplicitDenyOverridesLAN(t *testing.T) {
	idx := newIndex()
	idx.addUser(1, "10.6.0.2")
	idx.addUser(2, "10.6.0.3")
	idx.GroupsByUser[1] = []int64{10}
	idx.GroupsByUser[2] = []int64{10}
	idx.UserGroups[10] = UserGroupSnapshot{ID: 10, LANMode: true, Members: []int64{1, 2}}
	idx.Rules = []RuleSnapshot{{ID: 1, SubjectType: "user", SubjectID: 1, ObjectType: "user", ObjectID: 2, Action: "deny"}}
	if d, _, _ := idx.CheckPeer("10.6.0.2", "10.6.0.3"); d != Deny {
		t.Fatalf("explicit deny should override shared LAN group, got %s", d)
	}
}

func TestCheckPeerSharedGroupLANModeFalse(t *testing.T) {
	idx := newIndex()
	idx.addUser(1, "10.6.0.2")
	idx.addUser(2, "10.6.0.3")
	idx.GroupsByUser[1] = []int64{10}
	idx.GroupsByUser[2] = []int64{10}
	idx.UserGroups[10] = UserGroupSnapshot{ID: 10, LANMode: false, Members: []int64{1, 2}}
	if d, _, _ := idx.CheckPeer("10.6.0.2", "10.6.0.3"); d != Deny {
		t.Fatalf("shared group with LANMode=false should Deny, got %s", d)
	}
}

func TestCheckPeerSharedLANModeTrue(t *testing.T) {
	idx := newIndex()
	idx.addUser(1, "10.6.0.2")
	idx.addUser(2, "10.6.0.3")
	idx.GroupsByUser[1] = []int64{10}
	idx.GroupsByUser[2] = []int64{10}
	idx.UserGroups[10] = UserGroupSnapshot{ID: 10, LANMode: true, Members: []int64{1, 2}}
	if d, _, _ := idx.CheckPeer("10.6.0.2", "10.6.0.3"); d != Allow {
		t.Fatalf("shared LAN-mode group should Allow, got %s", d)
	}
}

func TestCheckPeerDifferentGroupsNotShared(t *testing.T) {
	idx := newIndex()
	idx.addUser(1, "10.6.0.2")
	idx.addUser(2, "10.6.0.3")
	idx.GroupsByUser[1] = []int64{10}
	idx.GroupsByUser[2] = []int64{20}
	idx.UserGroups[10] = UserGroupSnapshot{ID: 10, LANMode: true, Members: []int64{1}}
	idx.UserGroups[20] = UserGroupSnapshot{ID: 20, LANMode: true, Members: []int64{2}}
	if d, _, _ := idx.CheckPeer("10.6.0.2", "10.6.0.3"); d != Deny {
		t.Fatalf("src in A, dst in B (not shared) should Deny, got %s", d)
	}
}
