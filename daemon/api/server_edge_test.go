package api

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/devcutler/lightscale/daemon/store"
	"github.com/devcutler/lightscale/shared/config"
	"github.com/devcutler/lightscale/shared/wire"
)

func TestHealth(t *testing.T) {
	srv := newTestServer(t)
	rr := do(t, srv, "GET", "/api/health", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("health: %d", rr.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" {
		t.Errorf("status = %q, want ok", body["status"])
	}
}

func TestPrincipalsAndObjects(t *testing.T) {
	srv := newTestServer(t)

	if rr := do(t, srv, "POST", "/api/users", map[string]any{"name": "alice"}); rr.Code != http.StatusCreated {
		t.Fatal(rr.Body.String())
	}
	if rr := do(t, srv, "POST", "/api/user-groups", map[string]any{"name": "team"}); rr.Code != http.StatusCreated {
		t.Fatal(rr.Body.String())
	}
	if rr := do(t, srv, "POST", "/api/services", map[string]any{"name": "jelly", "origin": "host", "ports": "8096/tcp"}); rr.Code != http.StatusCreated {
		t.Fatal(rr.Body.String())
	}
	if rr := do(t, srv, "POST", "/api/service-groups", map[string]any{"name": "media"}); rr.Code != http.StatusCreated {
		t.Fatal(rr.Body.String())
	}

	rr := do(t, srv, "GET", "/api/principals", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("principals: %d", rr.Code)
	}
	var principals []wire.Principal
	if err := json.Unmarshal(rr.Body.Bytes(), &principals); err != nil {
		t.Fatal(err)
	}
	types := map[string]string{}
	for _, p := range principals {
		types[p.Name] = p.Type
	}
	if types["alice"] != "user" {
		t.Errorf("alice principal type = %q, want user", types["alice"])
	}
	if types["team"] != "user_group" {
		t.Errorf("team principal type = %q, want user_group", types["team"])
	}
	if _, ok := types["jelly"]; ok {
		t.Error("service should not appear in principals")
	}

	rr = do(t, srv, "GET", "/api/objects", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("objects: %d", rr.Code)
	}
	var objects []wire.Object
	if err := json.Unmarshal(rr.Body.Bytes(), &objects); err != nil {
		t.Fatal(err)
	}
	otypes := map[string]string{}
	for _, o := range objects {
		otypes[o.Name] = o.Type
	}
	wantObjects := map[string]string{
		"alice": "user", "team": "user_group", "jelly": "service", "media": "service_group",
	}
	for name, typ := range wantObjects {
		if otypes[name] != typ {
			t.Errorf("object %q type = %q, want %q", name, otypes[name], typ)
		}
	}
}

func createUserGroup(t *testing.T, srv *Server, name string) int64 {
	t.Helper()
	rr := do(t, srv, "POST", "/api/user-groups", map[string]any{"name": name})
	if rr.Code != http.StatusCreated {
		t.Fatalf("create group %s: %d %s", name, rr.Code, rr.Body.String())
	}
	var g wire.UserGroup
	if err := json.Unmarshal(rr.Body.Bytes(), &g); err != nil {
		t.Fatal(err)
	}
	return g.ID
}

func createUser(t *testing.T, srv *Server, name string) int64 {
	t.Helper()
	rr := do(t, srv, "POST", "/api/users", map[string]any{"name": name})
	if rr.Code != http.StatusCreated {
		t.Fatalf("create user %s: %d %s", name, rr.Code, rr.Body.String())
	}
	var u wire.User
	if err := json.Unmarshal(rr.Body.Bytes(), &u); err != nil {
		t.Fatal(err)
	}
	return u.ID
}

func TestUserGroupMembers(t *testing.T) {
	srv := newTestServer(t)
	gid := createUserGroup(t, srv, "team")
	aliceID := createUser(t, srv, "alice")
	createUser(t, srv, "bob")

	gp := "/api/user-groups/" + itoa(gid) + "/members"

	if rr := do(t, srv, "POST", gp, map[string]any{"user_id": aliceID}); rr.Code != http.StatusNoContent {
		t.Fatalf("add by id: %d %s", rr.Code, rr.Body.String())
	}
	if rr := do(t, srv, "POST", gp, map[string]any{"user_name": "bob"}); rr.Code != http.StatusNoContent {
		t.Fatalf("add by name: %d %s", rr.Code, rr.Body.String())
	}
	if rr := do(t, srv, "POST", gp, map[string]any{}); rr.Code != http.StatusBadRequest {
		t.Fatalf("add with neither: %d %s", rr.Code, rr.Body.String())
	}

	rr := do(t, srv, "GET", gp, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("list members: %d", rr.Code)
	}
	var members []wire.User
	if err := json.Unmarshal(rr.Body.Bytes(), &members); err != nil {
		t.Fatal(err)
	}
	if len(members) != 2 {
		t.Fatalf("members = %d, want 2", len(members))
	}

	if rr := do(t, srv, "DELETE", gp+"/"+itoa(aliceID), nil); rr.Code != http.StatusNoContent {
		t.Fatalf("remove: %d %s", rr.Code, rr.Body.String())
	}
	rr = do(t, srv, "GET", gp, nil)
	if err := json.Unmarshal(rr.Body.Bytes(), &members); err != nil {
		t.Fatal(err)
	}
	if len(members) != 1 || members[0].Name != "bob" {
		t.Fatalf("after remove, members = %+v, want [bob]", members)
	}
}

func TestServiceGroupMembers(t *testing.T) {
	srv := newTestServer(t)
	rr := do(t, srv, "POST", "/api/service-groups", map[string]any{"name": "media"})
	if rr.Code != http.StatusCreated {
		t.Fatal(rr.Body.String())
	}
	var sg wire.ServiceGroup
	if err := json.Unmarshal(rr.Body.Bytes(), &sg); err != nil {
		t.Fatal(err)
	}

	rr = do(t, srv, "POST", "/api/services", map[string]any{"name": "jelly", "origin": "host", "ports": "8096/tcp"})
	if rr.Code != http.StatusCreated {
		t.Fatal(rr.Body.String())
	}
	var sv wire.Service
	if err := json.Unmarshal(rr.Body.Bytes(), &sv); err != nil {
		t.Fatal(err)
	}

	gp := "/api/service-groups/" + itoa(sg.ID) + "/members"

	if rr := do(t, srv, "POST", gp, map[string]any{"service_id": sv.ID}); rr.Code != http.StatusNoContent {
		t.Fatalf("add by id: %d %s", rr.Code, rr.Body.String())
	}
	if rr := do(t, srv, "POST", gp, map[string]any{}); rr.Code != http.StatusBadRequest {
		t.Fatalf("add with neither: %d %s", rr.Code, rr.Body.String())
	}

	rr = do(t, srv, "GET", gp, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("list: %d", rr.Code)
	}
	var members []wire.Service
	if err := json.Unmarshal(rr.Body.Bytes(), &members); err != nil {
		t.Fatal(err)
	}
	if len(members) != 1 || members[0].Name != "jelly" {
		t.Fatalf("members = %+v, want [jelly]", members)
	}

	if rr := do(t, srv, "DELETE", gp+"/"+itoa(sv.ID), nil); rr.Code != http.StatusNoContent {
		t.Fatalf("remove: %d %s", rr.Code, rr.Body.String())
	}
}

type fakeStatus struct{ snap wire.StatusSnapshot }

func (f fakeStatus) Snapshot() StatusSnapshot { return f.snap }

func TestStatusWithDep(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	cfg := config.Defaults()
	want := wire.StatusSnapshot{Running: true, Peers: 5, ActiveFlows: 2, StartedAt: time.Now(), UptimeSec: 99}
	srv := New(Deps{Store: s, Config: &cfg, Status: fakeStatus{snap: want}})

	rr := do(t, srv, "GET", "/api/status", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d", rr.Code)
	}
	var snap wire.StatusSnapshot
	if err := json.Unmarshal(rr.Body.Bytes(), &snap); err != nil {
		t.Fatal(err)
	}
	if snap.Peers != 5 || snap.ActiveFlows != 2 || snap.UptimeSec != 99 {
		t.Errorf("snapshot = %+v, want peers=5 flows=2 uptime=99", snap)
	}
}

func TestConflictNameTaken(t *testing.T) {
	srv := newTestServer(t)
	if rr := do(t, srv, "POST", "/api/users", map[string]any{"name": "alice"}); rr.Code != http.StatusCreated {
		t.Fatal(rr.Body.String())
	}
	rr := do(t, srv, "POST", "/api/users", map[string]any{"name": "alice"})
	if rr.Code != http.StatusConflict {
		t.Fatalf("duplicate user: %d %s, want 409", rr.Code, rr.Body.String())
	}
}

func TestConflictIPInUse(t *testing.T) {
	srv := newTestServer(t)
	body := map[string]any{"name": "svc1", "origin": "host", "ports": "80/tcp", "ip": "10.6.1.50"}
	if rr := do(t, srv, "POST", "/api/services", body); rr.Code != http.StatusCreated {
		t.Fatal(rr.Body.String())
	}
	body2 := map[string]any{"name": "svc2", "origin": "host", "ports": "81/tcp", "ip": "10.6.1.50"}
	rr := do(t, srv, "POST", "/api/services", body2)
	if rr.Code != http.StatusConflict {
		t.Fatalf("duplicate IP: %d %s, want 409", rr.Code, rr.Body.String())
	}
}

func TestUserConfigRendering(t *testing.T) {
	srv := newTestServer(t)
	rr := do(t, srv, "POST", "/api/users", map[string]any{"name": "alice"})
	if rr.Code != http.StatusCreated {
		t.Fatal(rr.Body.String())
	}
	var u wire.User
	if err := json.Unmarshal(rr.Body.Bytes(), &u); err != nil {
		t.Fatal(err)
	}

	rr = do(t, srv, "GET", "/api/users/"+itoa(u.ID)+"/config", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("config: %d %s", rr.Code, rr.Body.String())
	}
	conf := rr.Body.String()
	for _, want := range []string{"[Interface]", "PrivateKey", "Address", "[Peer]", "PublicKey", "Endpoint", "AllowedIPs", "vpn.example.com:51820"} {
		if !strings.Contains(conf, want) {
			t.Errorf("config missing %q:\n%s", want, conf)
		}
	}
}

func TestUserConfigEndpointOverride(t *testing.T) {
	srv := newTestServer(t)
	rr := do(t, srv, "POST", "/api/users", map[string]any{"name": "alice", "endpoint": "custom.example:99"})
	if rr.Code != http.StatusCreated {
		t.Fatal(rr.Body.String())
	}
	var u wire.User
	if err := json.Unmarshal(rr.Body.Bytes(), &u); err != nil {
		t.Fatal(err)
	}
	rr = do(t, srv, "GET", "/api/users/"+itoa(u.ID)+"/config", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("config: %d %s", rr.Code, rr.Body.String())
	}
	conf := rr.Body.String()
	if !strings.Contains(conf, "custom.example:99") {
		t.Errorf("per-user endpoint override missing:\n%s", conf)
	}
	if strings.Contains(conf, "vpn.example.com:51820") {
		t.Errorf("override should replace default endpoint:\n%s", conf)
	}
}

func TestUserConfigNoEndpointConfigured(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	cfg := config.Defaults()
	cfg.PublicEndpoint = ""
	if err := s.SetSetting(context.Background(), "server_public_key", "FAKE=="); err != nil {
		t.Fatal(err)
	}
	srv := New(Deps{Store: s, Config: &cfg})

	rr := do(t, srv, "POST", "/api/users", map[string]any{"name": "alice"})
	if rr.Code != http.StatusCreated {
		t.Fatal(rr.Body.String())
	}
	var u wire.User
	if err := json.Unmarshal(rr.Body.Bytes(), &u); err != nil {
		t.Fatal(err)
	}
	rr = do(t, srv, "GET", "/api/users/"+itoa(u.ID)+"/config", nil)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("config without endpoint: %d %s, want 503", rr.Code, rr.Body.String())
	}
}

func TestUserConfigNoServerKey(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	cfg := config.Defaults()
	cfg.PublicEndpoint = "vpn.example.com:51820"

	srv := New(Deps{Store: s, Config: &cfg})

	rr := do(t, srv, "POST", "/api/users", map[string]any{"name": "alice"})
	if rr.Code != http.StatusCreated {
		t.Fatal(rr.Body.String())
	}
	var u wire.User
	if err := json.Unmarshal(rr.Body.Bytes(), &u); err != nil {
		t.Fatal(err)
	}
	rr = do(t, srv, "GET", "/api/users/"+itoa(u.ID)+"/config", nil)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("config without server key: %d %s, want 503", rr.Code, rr.Body.String())
	}
}
