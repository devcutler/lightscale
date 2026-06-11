package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTOML(t *testing.T, dir, body string) string {
	t.Helper()
	p := filepath.Join(dir, "lightscale.toml")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadExplicitPath(t *testing.T) {
	dir := t.TempDir()
	p := writeTOML(t, dir, `
public_endpoint = "vpn.example.com:51820"
[wireguard]
port = 51999
`)
	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Path != p {
		t.Errorf("Path = %q, want %q", cfg.Path, p)
	}
	if cfg.PublicEndpoint != "vpn.example.com:51820" {
		t.Errorf("PublicEndpoint = %q", cfg.PublicEndpoint)
	}
	if cfg.WireGuard.Port != 51999 {
		t.Errorf("Port = %d, want 51999 (from file)", cfg.WireGuard.Port)
	}
	if cfg.Domain != "lightscale.local" {
		t.Errorf("Domain = %q, want default lightscale.local", cfg.Domain)
	}
	if cfg.WireGuard.ServerIP != "10.6.0.1" {
		t.Errorf("ServerIP = %q, want default", cfg.WireGuard.ServerIP)
	}
}
func TestLoadExplicitMissing(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "nope.toml"))
	if err == nil {
		t.Fatal("expected error for missing explicit path")
	}
}

func TestLoadEnvPath(t *testing.T) {
	dir := t.TempDir()
	p := writeTOML(t, dir, `domain = "env.example"`)
	t.Setenv("LIGHTSCALE_CONFIG", p)

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Domain != "env.example" {
		t.Errorf("Domain = %q, want env.example", cfg.Domain)
	}

	t.Setenv("LIGHTSCALE_CONFIG", filepath.Join(dir, "missing.toml"))
	if _, err := Load(""); err == nil {
		t.Fatal("expected error for missing LIGHTSCALE_CONFIG path")
	}
}
func TestLoadExplicitBeatsEnv(t *testing.T) {
	dir := t.TempDir()
	explicit := writeTOML(t, dir, `domain = "explicit.example"`)
	envDir := t.TempDir()
	envPath := filepath.Join(envDir, "lightscale.toml")
	if err := os.WriteFile(envPath, []byte(`domain = "env.example"`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LIGHTSCALE_CONFIG", envPath)

	cfg, err := Load(explicit)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Domain != "explicit.example" {
		t.Errorf("Domain = %q, want explicit.example (explicit must win)", cfg.Domain)
	}
}

func TestLoadDefaultsWhenNoFile(t *testing.T) {
	t.Setenv("LIGHTSCALE_CONFIG", "")

	if _, err := os.Stat(defaultConfig); err == nil {
		t.Skipf("%s exists on this host; skipping defaults-only check", defaultConfig)
	}
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Path != "" {
		t.Errorf("Path = %q, want empty (defaults-only)", cfg.Path)
	}
	if cfg.Socket.Path != defaultSocket {
		t.Errorf("Socket.Path = %q, want default %q", cfg.Socket.Path, defaultSocket)
	}
}
