package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestOpenIdempotentMigrations(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "reopen.db")

	s1, err := Open(path)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	u, err := s1.CreateUser(ctx, CreateUserInput{
		Name: "alice", PublicKey: "pk", PrivateKey: "sk", PresharedKey: "psk",
		IPAddress: "10.6.0.2",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := s1.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	s2, err := Open(path)
	if err != nil {
		t.Fatalf("re-open: %v", err)
	}
	defer s2.Close()

	got, err := s2.GetUser(ctx, u.ID)
	if err != nil || got.Name != "alice" {
		t.Fatalf("data did not persist: %v %#v", err, got)
	}

	var n int
	if err := s2.DB().QueryRowContext(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&n); err != nil {
		t.Fatalf("count schema_migrations: %v", err)
	}
	if n != 1 {
		t.Fatalf("want 1 migration recorded, got %d", n)
	}

	var id string
	if err := s2.DB().QueryRowContext(ctx, `SELECT id FROM schema_migrations`).Scan(&id); err != nil {
		t.Fatalf("read migration id: %v", err)
	}
	if id != "0001_initial.sql" {
		t.Fatalf("want 0001_initial.sql recorded, got %q", id)
	}
}

func TestCreateUserValidation(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()

	base := CreateUserInput{
		Name: "alice", PublicKey: "pk", PrivateKey: "sk", PresharedKey: "psk",
		IPAddress: "10.6.0.2",
	}
	cases := map[string]func(CreateUserInput) CreateUserInput{
		"empty name":          func(in CreateUserInput) CreateUserInput { in.Name = ""; return in },
		"empty public key":    func(in CreateUserInput) CreateUserInput { in.PublicKey = ""; return in },
		"empty private key":   func(in CreateUserInput) CreateUserInput { in.PrivateKey = ""; return in },
		"empty preshared key": func(in CreateUserInput) CreateUserInput { in.PresharedKey = ""; return in },
		"empty ip":            func(in CreateUserInput) CreateUserInput { in.IPAddress = ""; return in },
	}
	for name, mut := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := s.CreateUser(ctx, mut(base)); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("want ErrInvalidInput, got %v", err)
			}
		})
	}
}

func TestCreateUserDuplicateNameAndIP(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	if _, err := s.CreateUser(ctx, CreateUserInput{
		Name: "alice", PublicKey: "pk", PrivateKey: "sk", PresharedKey: "psk", IPAddress: "10.6.0.2",
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := s.CreateUser(ctx, CreateUserInput{
		Name: "alice", PublicKey: "pk", PrivateKey: "sk", PresharedKey: "psk", IPAddress: "10.6.0.3",
	}); !errors.Is(err, ErrNameTaken) {
		t.Fatalf("dup name: want ErrNameTaken, got %v", err)
	}

	if _, err := s.CreateUser(ctx, CreateUserInput{
		Name: "bob", PublicKey: "pk", PrivateKey: "sk", PresharedKey: "psk", IPAddress: "10.6.0.2",
	}); !errors.Is(err, ErrIPInUse) {
		t.Fatalf("dup ip: want ErrIPInUse, got %v", err)
	}
}

func TestUpdateUserErrors(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	a, _ := s.CreateUser(ctx, CreateUserInput{
		Name: "alice", PublicKey: "pk", PrivateKey: "sk", PresharedKey: "psk", IPAddress: "10.6.0.2",
	})
	_, _ = s.CreateUser(ctx, CreateUserInput{
		Name: "bob", PublicKey: "pk", PrivateKey: "sk", PresharedKey: "psk", IPAddress: "10.6.0.3",
	})

	bob := "bob"
	if _, err := s.UpdateUser(ctx, a.ID, UpdateUserInput{Name: &bob}); !errors.Is(err, ErrNameTaken) {
		t.Fatalf("update to dup name: want ErrNameTaken, got %v", err)
	}

	any := "ghost"
	if _, err := s.UpdateUser(ctx, 99999, UpdateUserInput{Name: &any}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("update missing: want ErrNotFound, got %v", err)
	}
}

func TestDeleteUserMissing(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	if err := s.DeleteUser(ctx, 12345); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestDeleteUserCascades(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()

	u, _ := s.CreateUser(ctx, CreateUserInput{
		Name: "alice", PublicKey: "pk", PrivateKey: "sk", PresharedKey: "psk", IPAddress: "10.6.0.2",
	})
	g, _ := s.CreateUserGroup(ctx, "admins", false)
	if err := s.AddUserToGroup(ctx, g.ID, u.ID); err != nil {
		t.Fatal(err)
	}
	svc, _ := s.CreateService(ctx, CreateServiceInput{
		Name: "jelly", Hostname: "jelly.local", Origin: "host", IPAddress: "10.6.1.5",
	})
	if _, err := s.CreatePolicy(ctx, CreatePolicyInput{
		SubjectType: "user", SubjectID: u.ID, ObjectType: "service", ObjectID: svc.ID, Action: "allow",
	}); err != nil {
		t.Fatal(err)
	}

	if err := s.DeleteUser(ctx, u.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	members, _ := s.UserGroupMembers(ctx, g.ID)
	if len(members) != 0 {
		t.Fatalf("membership not cascaded: %v", members)
	}

	pols, _ := s.ListPolicies(ctx)
	if len(pols) != 0 {
		t.Fatalf("policies not scrubbed: %v", pols)
	}
}

func TestServicePortConflictAndProtocols(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()

	_, err := s.CreateService(ctx, CreateServiceInput{
		Name: "dup", Hostname: "dup.local", Origin: "host", IPAddress: "10.6.1.10",
		Ports: []ServicePort{{Port: 8080, Protocol: "tcp"}, {Port: 8080, Protocol: "tcp"}},
	})
	if err == nil {
		t.Fatalf("want error on duplicate port+protocol, got nil")
	}

	svc, err := s.CreateService(ctx, CreateServiceInput{
		Name: "ok", Hostname: "ok.local", Origin: "host", IPAddress: "10.6.1.11",
		Ports: []ServicePort{{Port: 8080, Protocol: "tcp"}, {Port: 8080, Protocol: "udp"}},
	})
	if err != nil {
		t.Fatalf("tcp+udp same port: %v", err)
	}
	if len(svc.Ports) != 2 {
		t.Fatalf("want 2 ports, got %d", len(svc.Ports))
	}
}

func TestServiceEmptyPortsAllowed(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	svc, err := s.CreateService(ctx, CreateServiceInput{
		Name: "wild", Hostname: "wild.local", Origin: "host", IPAddress: "10.6.1.12",
	})
	if err != nil {
		t.Fatalf("empty ports: %v", err)
	}
	if len(svc.Ports) != 0 {
		t.Fatalf("want 0 ports, got %d", len(svc.Ports))
	}
}

func TestServiceValidationErrors(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	base := CreateServiceInput{Name: "a", Hostname: "a.local", Origin: "host", IPAddress: "10.6.1.20"}

	cases := map[string]func(CreateServiceInput) CreateServiceInput{
		"empty name":     func(in CreateServiceInput) CreateServiceInput { in.Name = ""; return in },
		"empty hostname": func(in CreateServiceInput) CreateServiceInput { in.Hostname = ""; return in },
		"empty origin":   func(in CreateServiceInput) CreateServiceInput { in.Origin = ""; return in },
		"empty ip":       func(in CreateServiceInput) CreateServiceInput { in.IPAddress = ""; return in },
	}
	for name, mut := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := s.CreateService(ctx, mut(base)); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("want ErrInvalidInput, got %v", err)
			}
		})
	}
}

func TestServiceDuplicateNameVIPHostname(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	_, err := s.CreateService(ctx, CreateServiceInput{
		Name: "a", Hostname: "a.local", Origin: "host", IPAddress: "10.6.1.20",
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.CreateService(ctx, CreateServiceInput{
		Name: "a", Hostname: "b.local", Origin: "host", IPAddress: "10.6.1.21",
	}); !errors.Is(err, ErrNameTaken) {
		t.Fatalf("dup name: want ErrNameTaken, got %v", err)
	}
	if _, err := s.CreateService(ctx, CreateServiceInput{
		Name: "b", Hostname: "b.local", Origin: "host", IPAddress: "10.6.1.20",
	}); !errors.Is(err, ErrIPInUse) {
		t.Fatalf("dup vip: want ErrIPInUse, got %v", err)
	}
	if _, err := s.CreateService(ctx, CreateServiceInput{
		Name: "c", Hostname: "a.local", Origin: "host", IPAddress: "10.6.1.22",
	}); !errors.Is(err, ErrNameTaken) {
		t.Fatalf("dup hostname: want ErrNameTaken, got %v", err)
	}
}

func TestUpdateServiceFields(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	svc, _ := s.CreateService(ctx, CreateServiceInput{
		Name: "a", Hostname: "a.local", Origin: "host", IPAddress: "10.6.1.20",
		Ports: []ServicePort{{Port: 80, Protocol: "tcp"}},
	})

	newOrigin := "container1"
	newHost := "renamed.local"
	newDesc := "hello"
	out, err := s.UpdateService(ctx, svc.ID, UpdateServiceInput{
		Origin: &newOrigin, Hostname: &newHost, Description: &newDesc,
		Ports: []ServicePort{{Port: 443, Protocol: "tcp"}}, ReplacePorts: true,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if out.Origin != newOrigin || out.Hostname != newHost || out.Description != newDesc {
		t.Fatalf("fields not updated: %#v", out)
	}
	if len(out.Ports) != 1 || out.Ports[0].Port != 443 {
		t.Fatalf("ports not replaced: %#v", out.Ports)
	}
}

func TestDeleteServiceCascades(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	svc, _ := s.CreateService(ctx, CreateServiceInput{
		Name: "a", Hostname: "a.local", Origin: "host", IPAddress: "10.6.1.20",
		Ports: []ServicePort{{Port: 80, Protocol: "tcp"}},
	})
	sg, _ := s.CreateServiceGroup(ctx, "grp")
	if err := s.AddServiceToGroup(ctx, sg.ID, svc.ID); err != nil {
		t.Fatal(err)
	}

	if err := s.DeleteService(ctx, svc.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	var nPorts int
	if err := s.DB().QueryRowContext(ctx,
		`SELECT count(*) FROM service_ports WHERE service_id=?`, svc.ID).Scan(&nPorts); err != nil {
		t.Fatal(err)
	}
	if nPorts != 0 {
		t.Fatalf("ports not cascaded: %d", nPorts)
	}

	mem, _ := s.ServiceGroupMembers(ctx, sg.ID)
	if len(mem) != 0 {
		t.Fatalf("group membership not cascaded: %v", mem)
	}
}

func TestDeleteServiceMissing(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	if err := s.DeleteService(ctx, 999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestPolicyInvalidInputs(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	u, _ := s.CreateUser(ctx, CreateUserInput{
		Name: "alice", PublicKey: "pk", PrivateKey: "sk", PresharedKey: "psk", IPAddress: "10.6.0.2",
	})
	svc, _ := s.CreateService(ctx, CreateServiceInput{
		Name: "a", Hostname: "a.local", Origin: "host", IPAddress: "10.6.1.20",
	})

	cases := map[string]CreatePolicyInput{
		"bad subject type": {SubjectType: "service", SubjectID: svc.ID, ObjectType: "service", ObjectID: svc.ID, Action: "allow"},
		"bad object type":  {SubjectType: "user", SubjectID: u.ID, ObjectType: "bogus", ObjectID: 1, Action: "allow"},
		"bad action":       {SubjectType: "user", SubjectID: u.ID, ObjectType: "service", ObjectID: svc.ID, Action: "maybe"},
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := s.CreatePolicy(ctx, in); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("want ErrInvalidInput, got %v", err)
			}
		})
	}
}

func TestPolicyMissingSubjectObject(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	svc, _ := s.CreateService(ctx, CreateServiceInput{
		Name: "a", Hostname: "a.local", Origin: "host", IPAddress: "10.6.1.20",
	})
	if _, err := s.CreatePolicy(ctx, CreatePolicyInput{
		SubjectType: "user", SubjectID: 999, ObjectType: "service", ObjectID: svc.ID, Action: "allow",
	}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing subject: want ErrNotFound, got %v", err)
	}
}

func TestPolicyUpsertOnSameSubjectObject(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	u, _ := s.CreateUser(ctx, CreateUserInput{
		Name: "alice", PublicKey: "pk", PrivateKey: "sk", PresharedKey: "psk", IPAddress: "10.6.0.2",
	})
	svc, _ := s.CreateService(ctx, CreateServiceInput{
		Name: "a", Hostname: "a.local", Origin: "host", IPAddress: "10.6.1.20",
	})
	first, err := s.CreatePolicy(ctx, CreatePolicyInput{
		SubjectType: "user", SubjectID: u.ID, ObjectType: "service", ObjectID: svc.ID, Action: "allow",
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.CreatePolicy(ctx, CreatePolicyInput{
		SubjectType: "user", SubjectID: u.ID, ObjectType: "service", ObjectID: svc.ID, Action: "deny",
	})
	if err != nil {
		t.Fatalf("re-policy on same subject/object should upsert, got %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("upsert should reuse the same row id: first=%d second=%d", first.ID, second.ID)
	}
	pols, _ := s.ListPolicies(ctx)
	if len(pols) != 1 {
		t.Fatalf("want 1 policy after upsert, got %d", len(pols))
	}
	if pols[0].Action != "deny" {
		t.Fatalf("upsert should update action to deny, got %q", pols[0].Action)
	}
}

func TestDeletePolicyMissing(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	if err := s.DeletePolicy(ctx, 777); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestAddUserToGroupIdempotent(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	u, _ := s.CreateUser(ctx, CreateUserInput{
		Name: "alice", PublicKey: "pk", PrivateKey: "sk", PresharedKey: "psk", IPAddress: "10.6.0.2",
	})
	g, _ := s.CreateUserGroup(ctx, "admins", false)
	if err := s.AddUserToGroup(ctx, g.ID, u.ID); err != nil {
		t.Fatal(err)
	}

	if err := s.AddUserToGroup(ctx, g.ID, u.ID); err != nil {
		t.Fatalf("second add: %v", err)
	}
	members, _ := s.UserGroupMembers(ctx, g.ID)
	if len(members) != 1 {
		t.Fatalf("want 1 member, got %d", len(members))
	}
}

func TestAddUserToGroupMissingRefs(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	u, _ := s.CreateUser(ctx, CreateUserInput{
		Name: "alice", PublicKey: "pk", PrivateKey: "sk", PresharedKey: "psk", IPAddress: "10.6.0.2",
	})
	g, _ := s.CreateUserGroup(ctx, "admins", false)

	if err := s.AddUserToGroup(ctx, 999, u.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing group: want ErrNotFound, got %v", err)
	}
	if err := s.AddUserToGroup(ctx, g.ID, 999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing user: want ErrNotFound, got %v", err)
	}
}

func TestRemoveUserFromGroupNonMember(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	u, _ := s.CreateUser(ctx, CreateUserInput{
		Name: "alice", PublicKey: "pk", PrivateKey: "sk", PresharedKey: "psk", IPAddress: "10.6.0.2",
	})
	g, _ := s.CreateUserGroup(ctx, "admins", false)

	if err := s.RemoveUserFromGroup(ctx, g.ID, u.ID); err != nil {
		t.Fatalf("remove non-member: %v", err)
	}
}

func TestServiceGroupMembership(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	svc, _ := s.CreateService(ctx, CreateServiceInput{
		Name: "a", Hostname: "a.local", Origin: "host", IPAddress: "10.6.1.20",
	})
	g, _ := s.CreateServiceGroup(ctx, "grp")

	if err := s.AddServiceToGroup(ctx, 999, svc.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing group: want ErrNotFound, got %v", err)
	}
	if err := s.AddServiceToGroup(ctx, g.ID, 999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing service: want ErrNotFound, got %v", err)
	}
	if err := s.AddServiceToGroup(ctx, g.ID, svc.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.AddServiceToGroup(ctx, g.ID, svc.ID); err != nil {
		t.Fatalf("idempotent add: %v", err)
	}
	members, _ := s.ServiceGroupMembers(ctx, g.ID)
	if len(members) != 1 {
		t.Fatalf("want 1 member, got %d", len(members))
	}
	ids, _ := s.ServiceGroupIDsForService(ctx, svc.ID)
	if len(ids) != 1 || ids[0] != g.ID {
		t.Fatalf("ids: %v", ids)
	}
	if err := s.RemoveServiceFromGroup(ctx, g.ID, 999); err != nil {
		t.Fatalf("remove non-member: %v", err)
	}
}

func TestNamespaceCollisions(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()

	if _, err := s.CreateUserGroup(ctx, "alice", false); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateUser(ctx, CreateUserInput{
		Name: "alice", PublicKey: "pk", PrivateKey: "sk", PresharedKey: "psk", IPAddress: "10.6.0.2",
	}); !errors.Is(err, ErrNameTaken) {
		t.Fatalf("user vs user_group: want ErrNameTaken, got %v", err)
	}

	if _, err := s.CreateService(ctx, CreateServiceInput{
		Name: "store", Hostname: "store.local", Origin: "host", IPAddress: "10.6.1.5",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateServiceGroup(ctx, "store"); !errors.Is(err, ErrNameTaken) {
		t.Fatalf("service_group vs service: want ErrNameTaken, got %v", err)
	}

	if _, err := s.CreateService(ctx, CreateServiceInput{
		Name: "alice", Hostname: "alice.local", Origin: "host", IPAddress: "10.6.1.6",
	}); err != nil {
		t.Fatalf("user/service same name should be allowed, got %v", err)
	}
}

func TestRenameFreesName(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	u, _ := s.CreateUser(ctx, CreateUserInput{
		Name: "alice", PublicKey: "pk", PrivateKey: "sk", PresharedKey: "psk", IPAddress: "10.6.0.2",
	})
	newName := "alice2"
	if _, err := s.UpdateUser(ctx, u.ID, UpdateUserInput{Name: &newName}); err != nil {
		t.Fatal(err)
	}

	if _, err := s.CreateUserGroup(ctx, "alice", false); err != nil {
		t.Fatalf("old name not freed: %v", err)
	}
}

func TestSettings(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()

	if _, err := s.GetSetting(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing key: want ErrNotFound, got %v", err)
	}
	if err := s.SetSetting(ctx, "k", "v1"); err != nil {
		t.Fatal(err)
	}
	if v, _ := s.GetSetting(ctx, "k"); v != "v1" {
		t.Fatalf("want v1, got %q", v)
	}
	if err := s.SetSetting(ctx, "k", "v2"); err != nil {
		t.Fatal(err)
	}
	if v, _ := s.GetSetting(ctx, "k"); v != "v2" {
		t.Fatalf("overwrite: want v2, got %q", v)
	}

	if err := s.SetSetting(ctx, "empty", ""); err != nil {
		t.Fatalf("empty value: %v", err)
	}
	if v, err := s.GetSetting(ctx, "empty"); err != nil || v != "" {
		t.Fatalf("empty value get: %v %q", err, v)
	}
}

func TestListenerDoesNotFireOnRollback(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	fired := 0
	s.Subscribe(func(ChangeKind) { fired++ })

	if _, err := s.CreateUser(ctx, CreateUserInput{
		Name: "alice", PublicKey: "pk", PrivateKey: "sk", PresharedKey: "psk", IPAddress: "10.6.0.2",
	}); err != nil {
		t.Fatal(err)
	}
	if fired != 1 {
		t.Fatalf("want 1 fire after success, got %d", fired)
	}
	if _, err := s.CreateUser(ctx, CreateUserInput{
		Name: "bob", PublicKey: "pk", PrivateKey: "sk", PresharedKey: "psk", IPAddress: "10.6.0.2",
	}); !errors.Is(err, ErrIPInUse) {
		t.Fatalf("want ErrIPInUse, got %v", err)
	}
	if fired != 1 {
		t.Fatalf("listener fired on rolled-back tx: count=%d", fired)
	}
}

func TestMultipleSubscribersFire(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	var a, b ChangeKind
	s.Subscribe(func(k ChangeKind) { a = k })
	s.Subscribe(func(k ChangeKind) { b = k })
	if _, err := s.CreateUserGroup(ctx, "g", false); err != nil {
		t.Fatal(err)
	}
	if a != ChangeUserGroups || b != ChangeUserGroups {
		t.Fatalf("both subscribers should fire ChangeUserGroups, got %q %q", a, b)
	}
}
