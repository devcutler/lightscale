package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/devcutler/lightscale/shared/origin"
)

func openTest(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestUserCRUDAndNamespaceCollision(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()

	u, err := s.CreateUser(ctx, CreateUserInput{
		Name: "alice", Email: "a@example.com",
		PublicKey: "pk", PrivateKey: "sk", PresharedKey: "psk",
		IPAddress: "10.6.0.2",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if u.ID == 0 {
		t.Fatalf("expected id")
	}

	if _, err := s.CreateUserGroup(ctx, "alice", false); !errors.Is(err, ErrNameTaken) {
		t.Fatalf("want ErrNameTaken, got %v", err)
	}

	got, err := s.GetUser(ctx, u.ID)
	if err != nil || got.Email != "a@example.com" {
		t.Fatalf("get: %v %#v", err, got)
	}

	email := "alice@new.example"
	if _, err := s.UpdateUser(ctx, u.ID, UpdateUserInput{Email: &email}); err != nil {
		t.Fatalf("update: %v", err)
	}

	if err := s.DeleteUser(ctx, u.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.GetUser(ctx, u.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestServiceCRUDAndPorts(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()

	svc, err := s.CreateService(ctx, CreateServiceInput{
		Name: "jellyfin", Hostname: "jellyfin.lightscale.local",
		Origin: origin.Spec{Kind: origin.Host}, IPAddress: "10.6.1.5",
		Ports: []ServicePort{{Port: 8096, Protocol: "tcp"}},
	})
	if err != nil {
		t.Fatalf("create svc: %v", err)
	}
	if len(svc.Ports) != 1 {
		t.Fatalf("want 1 port, got %d", len(svc.Ports))
	}

	got, err := s.GetService(ctx, svc.ID)
	if err != nil || len(got.Ports) != 1 {
		t.Fatalf("get svc: %v %#v", err, got)
	}
}

func TestPolicyAllowDeleteRoundtrip(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()

	u, err := s.CreateUser(ctx, CreateUserInput{
		Name: "alice", PublicKey: "pk", PrivateKey: "sk", PresharedKey: "psk",
		IPAddress: "10.6.0.2",
	})
	if err != nil {
		t.Fatal(err)
	}
	svc, err := s.CreateService(ctx, CreateServiceInput{
		Name: "jellyfin", Hostname: "jellyfin.local", Origin: origin.Spec{Kind: origin.Host}, IPAddress: "10.6.1.5",
	})
	if err != nil {
		t.Fatal(err)
	}

	rule, err := s.CreatePolicy(ctx, CreatePolicyInput{
		SubjectType: "user", SubjectID: u.ID,
		ObjectType: "service", ObjectID: svc.ID,
		Action: "allow",
	})
	if err != nil {
		t.Fatalf("create policy: %v", err)
	}

	if err := s.DeletePolicy(ctx, rule.ID); err != nil {
		t.Fatalf("delete policy: %v", err)
	}

	if err := s.DeletePolicy(ctx, rule.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected not-found on second delete, got %v", err)
	}
}

func TestUserGroupMembershipRoundtrip(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	u, _ := s.CreateUser(ctx, CreateUserInput{
		Name: "alice", PublicKey: "p", PrivateKey: "s", PresharedKey: "k",
		IPAddress: "10.6.0.2",
	})
	g, err := s.CreateUserGroup(ctx, "admins", true)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AddUserToGroup(ctx, g.ID, u.ID); err != nil {
		t.Fatalf("add: %v", err)
	}
	members, err := s.UserGroupMembers(ctx, g.ID)
	if err != nil || len(members) != 1 {
		t.Fatalf("members: %v %v", err, members)
	}
	gids, err := s.UserGroupIDsForUser(ctx, u.ID)
	if err != nil || len(gids) != 1 || gids[0] != g.ID {
		t.Fatalf("ids: %v %v", err, gids)
	}
}

func TestListenerFiresOnCommit(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	var got ChangeKind
	s.Subscribe(func(k ChangeKind) { got = k })
	if _, err := s.CreateUser(ctx, CreateUserInput{
		Name: "x", PublicKey: "p", PrivateKey: "s", PresharedKey: "k",
		IPAddress: "10.6.0.2",
	}); err != nil {
		t.Fatal(err)
	}
	if got != ChangeUsers {
		t.Fatalf("want ChangeUsers, got %s", got)
	}
}
