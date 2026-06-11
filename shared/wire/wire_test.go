package wire

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

func roundTrip[T any](t *testing.T, v T) (string, T) {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out T
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return string(raw), out
}

func TestUserRoundTrip(t *testing.T) {
	u := User{
		ID: 1, Name: "alice", Email: "a@b.c",
		PublicKey: "PUB==", PresharedKey: "PSK==", IPAddress: "10.6.0.5",
		Endpoint: "vpn:51820", Migrated: true,
		CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-02T00:00:00Z",
	}
	raw, got := roundTrip(t, u)
	if !reflect.DeepEqual(u, got) {
		t.Errorf("round trip mismatch:\n got %+v\nwant %+v", got, u)
	}
	for _, key := range []string{`"public_key"`, `"preshared_key"`, `"ip_address"`, `"created_at"`, `"updated_at"`, `"migrated"`} {
		if !strings.Contains(raw, key) {
			t.Errorf("JSON missing key %s:\n%s", key, raw)
		}
	}
}

func TestUserOmitemptyEmailEndpoint(t *testing.T) {
	raw, _ := roundTrip(t, User{Name: "bob", PublicKey: "P", IPAddress: "10.6.0.6"})
	if strings.Contains(raw, "email") {
		t.Errorf("empty email should be omitted: %s", raw)
	}
	if strings.Contains(raw, "endpoint") {
		t.Errorf("empty endpoint should be omitted: %s", raw)
	}

	if !strings.Contains(raw, `"migrated":false`) {
		t.Errorf("migrated:false must be present: %s", raw)
	}
}

func TestServiceRoundTripWithPorts(t *testing.T) {
	sv := Service{
		ID: 2, Name: "jellyfin", Hostname: "jellyfin.lightscale.local",
		Origin: "host", IPAddress: "10.6.1.5", Description: "media",
		Ports:     []ServicePort{{Port: 8096, Protocol: "tcp"}, {Port: 9000, Protocol: "udp"}},
		CreatedAt: "t1", UpdatedAt: "t2",
	}
	raw, got := roundTrip(t, sv)
	if !reflect.DeepEqual(sv, got) {
		t.Errorf("round trip mismatch:\n got %+v\nwant %+v", got, sv)
	}
	for _, key := range []string{`"ip_address"`, `"protocol"`, `"hostname"`} {
		if !strings.Contains(raw, key) {
			t.Errorf("JSON missing key %s:\n%s", key, raw)
		}
	}
}

func TestServiceDescriptionOmitempty(t *testing.T) {
	raw, _ := roundTrip(t, Service{Name: "x", Ports: []ServicePort{}})
	if strings.Contains(raw, "description") {
		t.Errorf("empty description should be omitted: %s", raw)
	}
}

func TestUserGroupSnakeCase(t *testing.T) {
	raw, got := roundTrip(t, UserGroup{ID: 3, Name: "team", LANMode: true, CreatedAt: "a", UpdatedAt: "b"})
	if !got.LANMode {
		t.Error("LANMode lost in round trip")
	}
	if !strings.Contains(raw, `"lan_mode"`) {
		t.Errorf("JSON missing lan_mode: %s", raw)
	}
}

func TestPolicyRoundTrip(t *testing.T) {
	p := Policy{
		ID: 4, SubjectType: "user", SubjectID: 1, SubjectName: "alice",
		ObjectType: "service", ObjectID: 2, ObjectName: "jellyfin",
		Action: "allow", CreatedAt: "a", UpdatedAt: "b",
	}
	raw, got := roundTrip(t, p)
	if !reflect.DeepEqual(p, got) {
		t.Errorf("round trip mismatch:\n got %+v\nwant %+v", got, p)
	}
	for _, key := range []string{`"subject_type"`, `"subject_id"`, `"subject_name"`, `"object_type"`, `"object_id"`, `"object_name"`} {
		if !strings.Contains(raw, key) {
			t.Errorf("JSON missing key %s:\n%s", key, raw)
		}
	}
}

func TestStatusSnapshotRoundTrip(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	s := StatusSnapshot{
		Running: true, Peers: 3, ActiveFlows: 7,
		StartedAt: now, UptimeSec: 42,
		WireGuardUDP: "0.0.0.0:51820", SocketPath: "/run/lightscale/lightscale.sock",
	}
	raw, got := roundTrip(t, s)
	if !reflect.DeepEqual(s, got) {
		t.Errorf("round trip mismatch:\n got %+v\nwant %+v", got, s)
	}
	for _, key := range []string{`"running"`, `"peers"`, `"active_flows"`, `"started_at"`, `"uptime_sec"`} {
		if !strings.Contains(raw, key) {
			t.Errorf("JSON missing key %s:\n%s", key, raw)
		}
	}
}

func TestStatusSnapshotOmitempty(t *testing.T) {
	raw, _ := roundTrip(t, StatusSnapshot{Running: true})
	if strings.Contains(raw, "wireguard_udp") {
		t.Errorf("empty wireguard_udp should be omitted: %s", raw)
	}
	if strings.Contains(raw, "socket_path") {
		t.Errorf("empty socket_path should be omitted: %s", raw)
	}
}

func TestPeerRoundTrip(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	p := Peer{
		Name: "alice", UserID: 1, IPAddress: "10.6.0.5",
		PublicKey: "PUB==", PresharedKey: "PSK==",
		AllowedIPs: []string{"10.6.0.5/32"}, Endpoint: "1.2.3.4:51820",
		LastHandshake: now, LastHandshakeAgoS: 30,
		KeepaliveInterval: 25, RxBytes: 100, TxBytes: 200,
	}
	raw, got := roundTrip(t, p)
	if !reflect.DeepEqual(p, got) {
		t.Errorf("round trip mismatch:\n got %+v\nwant %+v", got, p)
	}
	for _, key := range []string{`"allowed_ips"`, `"public_key"`, `"keepalive_interval"`, `"rx_bytes"`, `"tx_bytes"`} {
		if !strings.Contains(raw, key) {
			t.Errorf("JSON missing key %s:\n%s", key, raw)
		}
	}
}

func TestConnectionRoundTrip(t *testing.T) {
	c := Connection{
		ID: 9, SrcUserID: 1, SrcName: "alice", SrcIP: "10.6.0.5",
		ObjectType: "service", ObjectID: 2, ObjectName: "jellyfin", ObjectIP: "10.6.1.5",
		Port: 8096, Protocol: "tcp",
	}
	raw, got := roundTrip(t, c)
	if !reflect.DeepEqual(c, got) {
		t.Errorf("round trip mismatch:\n got %+v\nwant %+v", got, c)
	}
	for _, key := range []string{`"src_user_id"`, `"object_type"`, `"object_id"`} {
		if !strings.Contains(raw, key) {
			t.Errorf("JSON missing key %s:\n%s", key, raw)
		}
	}
}

func TestCreateReqsRoundTrip(t *testing.T) {
	if _, got := roundTrip(t, CreateUserReq{Name: "a", Email: "e", IP: "10.6.0.5", Endpoint: "x"}); got.Name != "a" {
		t.Error("CreateUserReq lost data")
	}
	if _, got := roundTrip(t, CreateServiceReq{Name: "s", Origin: "host", Ports: "8096/tcp", Hostname: "h", IP: "10.6.1.5", Description: "d"}); got.Origin != "host" {
		t.Error("CreateServiceReq lost data")
	}
	if _, got := roundTrip(t, CreatePolicyReq{SubjectName: "a", ObjectName: "b", Action: "allow"}); got.Action != "allow" {
		t.Error("CreatePolicyReq lost data")
	}
	if _, got := roundTrip(t, UserGroupMemberReq{UserName: "alice"}); got.UserName != "alice" {
		t.Error("UserGroupMemberReq lost data")
	}
}

func TestUpdateReqPointerSemantics(t *testing.T) {

	raw, _ := roundTrip(t, UpdateUserReq{})
	if strings.Contains(raw, "name") || strings.Contains(raw, "email") || strings.Contains(raw, "endpoint") {
		t.Errorf("nil pointers must omit keys: %s", raw)
	}

	empty := ""
	raw2, got2 := roundTrip(t, UpdateUserReq{Name: &empty})
	if !strings.Contains(raw2, `"name":""`) {
		t.Errorf("pointer to empty string must emit key: %s", raw2)
	}
	if got2.Name == nil || *got2.Name != "" {
		t.Errorf("decoded Name pointer = %v, want non-nil empty string", got2.Name)
	}
	if got2.Email != nil {
		t.Errorf("Email should decode to nil, got %v", got2.Email)
	}

	f := false
	raw3, got3 := roundTrip(t, UpdateUserGroupReq{LANMode: &f})
	if !strings.Contains(raw3, `"lan_mode":false`) {
		t.Errorf("pointer to false bool must emit key: %s", raw3)
	}
	if got3.LANMode == nil || *got3.LANMode != false {
		t.Errorf("decoded LANMode = %v, want non-nil false", got3.LANMode)
	}
	rawNil, _ := roundTrip(t, UpdateUserGroupReq{})
	if strings.Contains(rawNil, "lan_mode") {
		t.Errorf("nil *bool must omit lan_mode: %s", rawNil)
	}
}
