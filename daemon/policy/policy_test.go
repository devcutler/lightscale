package policy

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devcutler/lightscale/daemon/store"

	"github.com/devcutler/lightscale/shared/origin"
)

type fixture struct {
	store *store.Store
	alice store.User
	bob   store.User
	jelly store.Service
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "policy.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()

	alice, err := s.CreateUser(ctx, store.CreateUserInput{
		Name: "alice", PublicKey: "p", PrivateKey: "s", PresharedKey: "k",
		IPAddress: "10.6.0.2",
	})
	if err != nil {
		t.Fatal(err)
	}
	bob, err := s.CreateUser(ctx, store.CreateUserInput{
		Name: "bob", PublicKey: "p2", PrivateKey: "s2", PresharedKey: "k2",
		IPAddress: "10.6.0.3",
	})
	if err != nil {
		t.Fatal(err)
	}
	jelly, err := s.CreateService(ctx, store.CreateServiceInput{
		Name: "jellyfin", Hostname: "jellyfin.local", Origin: origin.Spec{Kind: origin.Host},
		IPAddress: "10.6.1.5",
		Ports:     []store.ServicePort{{Port: 8096, Protocol: "tcp"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return &fixture{store: s, alice: alice, bob: bob, jelly: jelly}
}

func TestServiceAllow(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if _, err := f.store.CreatePolicy(ctx, store.CreatePolicyInput{
		SubjectType: "user", SubjectID: f.alice.ID,
		ObjectType: "service", ObjectID: f.jelly.ID,
		Action: "allow",
	}); err != nil {
		t.Fatal(err)
	}

	idx, err := Build(ctx, f.store)
	if err != nil {
		t.Fatal(err)
	}

	d, _, _ := idx.CheckService("10.6.0.2", "10.6.1.5", 8096, "tcp")
	if d != Allow {
		t.Fatalf("want allow, got %s", d)
	}
	d, _, _ = idx.CheckService("10.6.0.2", "10.6.1.5", 80, "tcp")
	if d != Deny {
		t.Fatalf("want deny on undeclared port, got %s", d)
	}
	d, _, _ = idx.CheckService("10.6.0.99", "10.6.1.5", 8096, "tcp")
	if d != Deny {
		t.Fatalf("want deny for unknown peer, got %s", d)
	}
}

func TestWildcardPortsAuthorization(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	open, err := f.store.CreateService(ctx, store.CreateServiceInput{
		Name: "open", Hostname: "open.local", Origin: origin.Spec{Kind: origin.IP, Value: "192.168.1.10"},
		IPAddress: "10.6.1.9",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.CreatePolicy(ctx, store.CreatePolicyInput{
		SubjectType: "user", SubjectID: f.alice.ID,
		ObjectType: "service", ObjectID: open.ID,
		Action: "allow",
	}); err != nil {
		t.Fatal(err)
	}

	idx, _ := Build(ctx, f.store)
	for _, port := range []int{80, 443, 9999} {
		if d, _, _ := idx.CheckService("10.6.0.2", "10.6.1.9", port, "tcp"); d != Allow {
			t.Fatalf("wildcard service should allow port %d, got %s", port, d)
		}
	}
	if d, _, _ := idx.CheckService("10.6.0.3", "10.6.1.9", 80, "tcp"); d != Deny {
		t.Fatalf("wildcard service must still require a policy; got %s for bob", d)
	}
}

func TestServiceGroupAuthorization(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	grp, err := f.store.CreateServiceGroup(ctx, "media")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.store.AddServiceToGroup(ctx, grp.ID, f.jelly.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.CreatePolicy(ctx, store.CreatePolicyInput{
		SubjectType: "user", SubjectID: f.alice.ID,
		ObjectType: "service_group", ObjectID: grp.ID,
		Action: "allow",
	}); err != nil {
		t.Fatal(err)
	}

	idx, _ := Build(ctx, f.store)
	if d, _, _ := idx.CheckService("10.6.0.2", "10.6.1.5", 8096, "tcp"); d != Allow {
		t.Fatalf("member service should be allowed via group policy, got %s", d)
	}
	other, err := f.store.CreateService(ctx, store.CreateServiceInput{
		Name: "other", Hostname: "other.local", Origin: origin.Spec{Kind: origin.IP, Value: "192.168.1.20"},
		IPAddress: "10.6.1.20",
		Ports:     []store.ServicePort{{Port: 8096, Protocol: "tcp"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	idx, _ = Build(ctx, f.store)
	if d, _, _ := idx.CheckService("10.6.0.2", "10.6.1.20", 8096, "tcp"); d != Deny {
		t.Fatalf("non-member service must not be allowed by group policy, got %s", d)
	}
	_ = other
}

func TestLastWriteWins(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if _, err := f.store.CreatePolicy(ctx, store.CreatePolicyInput{
		SubjectType: "user", SubjectID: f.alice.ID,
		ObjectType: "service", ObjectID: f.jelly.ID,
		Action: "deny",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.CreatePolicy(ctx, store.CreatePolicyInput{
		SubjectType: "user", SubjectID: f.alice.ID,
		ObjectType: "service", ObjectID: f.jelly.ID,
		Action: "allow",
	}); err != nil {
		t.Fatal(err)
	}

	idx, _ := Build(ctx, f.store)
	d, _, _ := idx.CheckService("10.6.0.2", "10.6.1.5", 8096, "tcp")
	if d != Allow {
		t.Fatalf("highest id should win allow, got %s", d)
	}
}

func TestLANModePeerToPeer(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	g, err := f.store.CreateUserGroup(ctx, "household", true)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.store.AddUserToGroup(ctx, g.ID, f.alice.ID); err != nil {
		t.Fatal(err)
	}
	if err := f.store.AddUserToGroup(ctx, g.ID, f.bob.ID); err != nil {
		t.Fatal(err)
	}

	idx, _ := Build(ctx, f.store)
	d, _, _ := idx.CheckPeer("10.6.0.2", "10.6.0.3")
	if d != Allow {
		t.Fatalf("LAN-mode group should allow peer-to-peer, got %s", d)
	}
	if err := f.store.RemoveUserFromGroup(ctx, g.ID, f.bob.ID); err != nil {
		t.Fatal(err)
	}
	idx, _ = Build(ctx, f.store)
	d, _, _ = idx.CheckPeer("10.6.0.2", "10.6.0.3")
	if d != Deny {
		t.Fatalf("expected deny, got %s", d)
	}
}

func TestExplicitPeerPolicyOverridesLanMode(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	g, _ := f.store.CreateUserGroup(ctx, "household", true)
	_ = f.store.AddUserToGroup(ctx, g.ID, f.alice.ID)
	_ = f.store.AddUserToGroup(ctx, g.ID, f.bob.ID)
	if _, err := f.store.CreatePolicy(ctx, store.CreatePolicyInput{
		SubjectType: "user", SubjectID: f.alice.ID,
		ObjectType: "user", ObjectID: f.bob.ID,
		Action: "deny",
	}); err != nil {
		t.Fatal(err)
	}

	idx, _ := Build(ctx, f.store)
	d, _, _ := idx.CheckPeer("10.6.0.2", "10.6.0.3")
	if d != Deny {
		t.Fatalf("explicit deny should win, got %s", d)
	}
}

func TestFlowTableReaper(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if _, err := f.store.CreatePolicy(ctx, store.CreatePolicyInput{
		SubjectType: "user", SubjectID: f.alice.ID,
		ObjectType: "service", ObjectID: f.jelly.ID,
		Action: "allow",
	}); err != nil {
		t.Fatal(err)
	}

	idx, _ := Build(ctx, f.store)
	tbl := NewFlowTable()
	closed := false
	tbl.Add(Flow{
		SrcUserID: f.alice.ID, ObjectType: "service", ObjectID: f.jelly.ID,
		Port: 8096, Protocol: "tcp",
		Close: func() { closed = true },
	})
	tbl.Reap(idx)
	if closed {
		t.Fatal("flow killed prematurely")
	}
	policies, _ := f.store.ListPolicies(ctx)
	if err := f.store.DeletePolicy(ctx, policies[0].ID); err != nil {
		t.Fatal(err)
	}
	idx2, _ := Build(ctx, f.store)
	tbl.Reap(idx2)
	if !closed {
		t.Fatal("expected flow to be closed after policy revoked")
	}
}
