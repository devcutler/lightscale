package config

import (
	"path/filepath"
	"testing"
)

func TestLoadInvalidTOMLSyntax(t *testing.T) {
	dir := t.TempDir()
	p := writeTOML(t, dir, `this is = not [valid toml`)
	if _, err := Load(p); err == nil {
		t.Fatal("expected error for invalid TOML syntax")
	}
}

func TestLoadTypeMismatch(t *testing.T) {
	dir := t.TempDir()
	p := writeTOML(t, dir, `
[wireguard]
port = "abc"
`)
	if _, err := Load(p); err == nil {
		t.Fatal("expected error for port type mismatch (string into int)")
	}
}

func TestLoadPartialMergeKeepsDefaults(t *testing.T) {
	dir := t.TempDir()
	p := writeTOML(t, dir, `domain = "merge.example"`)
	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Domain != "merge.example" {
		t.Errorf("Domain = %q, want merge.example", cfg.Domain)
	}

	if cfg.WireGuard.Port != 51820 {
		t.Errorf("WireGuard.Port = %d, want default 51820", cfg.WireGuard.Port)
	}
	if cfg.WireGuard.Subnet != "10.6.0.0/23" {
		t.Errorf("WireGuard.Subnet = %q, want default", cfg.WireGuard.Subnet)
	}
	if cfg.WireGuard.ServerIP != "10.6.0.1" {
		t.Errorf("WireGuard.ServerIP = %q, want default", cfg.WireGuard.ServerIP)
	}
	if cfg.Socket.Path != defaultSocket {
		t.Errorf("Socket.Path = %q, want default %q", cfg.Socket.Path, defaultSocket)
	}
}

func TestDefaultsExactValues(t *testing.T) {
	d := Defaults()
	checks := []struct {
		name string
		got  any
		want any
	}{
		{"Domain", d.Domain, "lightscale.local"},
		{"WireGuard.Port", d.WireGuard.Port, 51820},
		{"WireGuard.Subnet", d.WireGuard.Subnet, "10.6.0.0/23"},
		{"WireGuard.ClientSubnet", d.WireGuard.ClientSubnet, "10.6.0.0/24"},
		{"WireGuard.ServiceSubnet", d.WireGuard.ServiceSubnet, "10.6.1.0/24"},
		{"WireGuard.ServerIP", d.WireGuard.ServerIP, "10.6.0.1"},
		{"Socket.Path", d.Socket.Path, defaultSocket},
		{"Socket.Mode", d.Socket.Mode, "0660"},
		{"Socket.Group", d.Socket.Group, "lightscale"},
		{"Storage.Database", d.Storage.Database, defaultDatabase},
		{"Log.Format", d.Log.Format, ""},
		{"Log.Level", d.Log.Level, "info"},
		{"Docker.Socket", d.Docker.Socket, defaultDocker},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
	if d.Path != "" {
		t.Errorf("Path = %q, want empty", d.Path)
	}
	if defaultSocket != "/run/lightscale/lightscale.sock" {
		t.Errorf("defaultSocket = %q", defaultSocket)
	}
	if defaultDatabase != "/var/lib/lightscale/lightscale.db" {
		t.Errorf("defaultDatabase = %q", defaultDatabase)
	}
	if defaultConfig != "/etc/lightscale/lightscale.toml" {
		t.Errorf("defaultConfig = %q", defaultConfig)
	}
	if defaultDocker != "/var/run/docker.sock" {
		t.Errorf("defaultDocker = %q", defaultDocker)
	}
}

func TestResolvePathExplicitBeatsEnvAndMissingEnvIgnored(t *testing.T) {
	dir := t.TempDir()
	explicit := writeTOML(t, dir, `domain = "explicit"`)

	t.Setenv("LIGHTSCALE_CONFIG", filepath.Join(dir, "does-not-exist.toml"))

	got, err := resolvePath(explicit)
	if err != nil {
		t.Fatalf("resolvePath: %v", err)
	}
	if got != explicit {
		t.Errorf("resolvePath = %q, want %q", got, explicit)
	}
}

func TestResolvePathExplicitMissing(t *testing.T) {
	_, err := resolvePath(filepath.Join(t.TempDir(), "nope.toml"))
	if err == nil {
		t.Fatal("expected error for missing explicit path")
	}
}
