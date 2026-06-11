package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	daemon "github.com/devcutler/lightscale/daemon/runtime"
	"github.com/devcutler/lightscale/shared/config"
)

func main() {
	root := newRoot()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "lightscaled:", err)
		os.Exit(1)
	}
}

func newRoot() *cobra.Command {
	var configPath string

	root := &cobra.Command{
		Use:           "lightscaled",
		Short:         "Lightscale VPN gateway daemon",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDaemon(configPath)
		},
	}
	root.PersistentFlags().StringVar(&configPath, "config", "", "path to lightscale.toml")

	var force bool
	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Write a default lightscale.toml at the resolved config path",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(configPath, force)
		},
	}
	initCmd.Flags().BoolVar(&force, "force", false, "overwrite an existing config file")
	root.AddCommand(initCmd)

	return root
}

func runDaemon(configPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	logger := newLogger(cfg.Log)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reloadCh := make(chan struct{}, 1)
	go handleSignals(cancel, reloadCh)

	resolveFn := func() (config.Config, error) {
		return config.Load(configPath)
	}

	return daemon.Run(ctx, cfg, logger, reloadCh, resolveFn)
}

func handleSignals(cancel context.CancelFunc, reloadCh chan<- struct{}) {
	c := make(chan os.Signal, 4)
	registerSignals(c)
	for sig := range c {
		switch sig {
		case syscall.SIGINT, syscall.SIGTERM:
			cancel()
			return
		default:
			select {
			case reloadCh <- struct{}{}:
			default:
			}
		}
	}
}
func runInit(configPath string, force bool) error {
	target := configPath
	if target == "" {
		if env := os.Getenv("LIGHTSCALE_CONFIG"); env != "" {
			target = env
		}
	}
	if target == "" {
		target = defaultInitPath()
	}
	if target == "" {
		return errors.New("init: cannot determine target path; pass --config")
	}
	if _, err := os.Stat(target); err == nil && !force {
		return fmt.Errorf("init: %s already exists (use --force to overwrite)", target)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("init: mkdir: %w", err)
	}
	if err := os.WriteFile(target, []byte(defaultConfigTOML()), 0o640); err != nil {
		return fmt.Errorf("init: write: %w", err)
	}
	fmt.Printf("wrote default config to %s\n", target)
	return nil
}
func defaultInitPath() string {
	return "/etc/lightscale/lightscale.toml"
}
func defaultConfigTOML() string {
	return strings.TrimLeft(`
# lightscale daemon configuration. see the README for details

# where clients reach your server, wireguard format.
# I've been using a domain pointed at my house, and I have :51820 forwarding to my server
public_endpoint = "vpn.example.com:51820"

# DNS domain suffix used for service hostnames
domain = "lightscale.local"

[wireguard]
port            = 51820
subnet          = "10.6.0.0/23"
client_subnet   = "10.6.0.0/24"
service_subnet  = "10.6.1.0/24"
server_ip       = "10.6.0.1"

[socket]
path  = "/run/lightscale/lightscale.sock"
mode  = "0660"
group = "lightscale"

[storage]
database = "/var/lib/lightscale/lightscale.db"

[log]
format = "text"
level  = "info"
# file = "/var/log/lightscale/lightscale.log"

[docker]
# you need this if you want to use automatic docker mapping
# --origin will not work without it
# socket = "/var/run/docker.sock"
`, "\n")
}

func newLogger(logCfg config.LogConfig) *slog.Logger {
	level := slog.LevelInfo
	switch logCfg.Level {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	out := os.Stdout
	if logCfg.File != "" {
		f, err := os.OpenFile(logCfg.File, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o640)
		if err == nil {
			out = f
		}
	}
	if logCfg.Format == "json" {
		return slog.New(slog.NewJSONHandler(out, &slog.HandlerOptions{Level: level}))
	}
	return slog.New(slog.NewTextHandler(out, &slog.HandlerOptions{Level: level}))
}
