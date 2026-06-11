package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

type Config struct {
	PublicEndpoint string          `toml:"public_endpoint"`
	Domain         string          `toml:"domain"`
	WireGuard      WireGuardConfig `toml:"wireguard"`
	Socket         SocketConfig    `toml:"socket"`
	Storage        StorageConfig   `toml:"storage"`
	Log            LogConfig       `toml:"log"`
	Docker         DockerConfig    `toml:"docker"`
	Path           string          `toml:"-"`
}

type WireGuardConfig struct {
	Port          int    `toml:"port"`
	Subnet        string `toml:"subnet"`
	ClientSubnet  string `toml:"client_subnet"`
	ServiceSubnet string `toml:"service_subnet"`
	ServerIP      string `toml:"server_ip"`
}

type SocketConfig struct {
	Path  string `toml:"path"`
	Mode  string `toml:"mode"`
	Group string `toml:"group"`
}

type StorageConfig struct {
	Database string `toml:"database"`
}

type LogConfig struct {
	Format string `toml:"format"`
	Level  string `toml:"level"`
	File   string `toml:"file"`
}

type DockerConfig struct {
	Socket string `toml:"socket"`
}

func Defaults() Config {
	return Config{
		Domain: "lightscale.local",
		WireGuard: WireGuardConfig{
			Port:          51820,
			Subnet:        "10.6.0.0/23",
			ClientSubnet:  "10.6.0.0/24",
			ServiceSubnet: "10.6.1.0/24",
			ServerIP:      "10.6.0.1",
		},
		Socket: SocketConfig{
			Path:  defaultSocketPath(),
			Mode:  "0660",
			Group: "lightscale",
		},
		Storage: StorageConfig{
			Database: defaultDatabasePath(),
		},
		Log: LogConfig{
			Format: "",
			Level:  "info",
		},
		Docker: DockerConfig{
			Socket: defaultDockerSocket(),
		},
	}
}

func Load(explicitPath string) (Config, error) {
	cfg := Defaults()

	path, err := resolvePath(explicitPath)
	if err != nil {
		return cfg, err
	}
	if path == "" {
		return cfg, nil
	}

	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return cfg, fmt.Errorf("config: decode %s: %w", path, err)
	}
	cfg.Path = path
	return cfg, nil
}

func resolvePath(explicitPath string) (string, error) {
	if explicitPath != "" {
		if _, err := os.Stat(explicitPath); err != nil {
			return "", fmt.Errorf("config: --config %s: %w", explicitPath, err)
		}
		return explicitPath, nil
	}

	if env := os.Getenv("LIGHTSCALE_CONFIG"); env != "" {
		if _, err := os.Stat(env); err != nil {
			return "", fmt.Errorf("config: LIGHTSCALE_CONFIG %s: %w", env, err)
		}
		return env, nil
	}

	for _, candidate := range defaultPathCandidates() {
		if candidate == "" {
			continue
		}
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("config: stat %s: %w", candidate, err)
		}
	}
	return "", nil
}

const (
	defaultSocket   = "/run/lightscale/lightscale.sock"
	defaultDatabase = "/var/lib/lightscale/lightscale.db"
	defaultConfig   = "/etc/lightscale/lightscale.toml"
	defaultDocker   = "/var/run/docker.sock"
)

func defaultPathCandidates() []string {
	return []string{defaultConfig}
}

func defaultSocketPath() string   { return defaultSocket }
func defaultDatabasePath() string { return defaultDatabase }
func defaultDockerSocket() string { return defaultDocker }
