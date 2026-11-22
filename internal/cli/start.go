package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/hardhacker/hylo/internal/agent"
	"github.com/hardhacker/hylo/internal/config"
	"github.com/hardhacker/hylo/internal/logger"
	"github.com/hardhacker/hylo/internal/relaunch"
	"github.com/hardhacker/hylo/internal/server"
	"github.com/hardhacker/hylo/internal/storage"
	"github.com/spf13/cobra"
)

func defaultServerPIDFile() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".hylo", "pid")
	}
	return filepath.Join(home, ".hylo", "pid")
}

// set by `hylo start server --pid-file`
var servePIDFile string

func newStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "start",
		Short:        "Start a service",
		SilenceUsage: true,
	}

	serverCmd := &cobra.Command{
		Use:   "server",
		Short: "Start the database server",
		Long: `Start the database server.

The listen address and timeouts come from the config file, not from flags.
HYLO_API_KEY overrides server.api_key.

By default the server listens on TCP at 127.0.0.1:54321. Set server.port to 0
in hylo.toml to disable the server entirely.

After the server successfully binds its listen address, its process ID is written
to the pid file (see --pid-file). The file is removed when the server exits.`,
		SilenceUsage: true,
		RunE:         runServe,
	}
	serverCmd.Flags().StringVar(&servePIDFile, "pid-file", defaultServerPIDFile(),
		"path to write this process ID after the server starts listening (use empty string to skip)")

	cmd.AddCommand(serverCmd, newStartRelayCmd())

	return cmd
}

func runServe(_ *cobra.Command, _ []string) error {
	agent.WarmShellEnv() // async: captures login-shell env for agent spawns

	cfg, cfgFileUsed, err := config.Load("")
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if cfgFileUsed != "" {
		if abs, aerr := filepath.Abs(cfgFileUsed); aerr == nil {
			cfgFileUsed = abs
		}
	}

	log := logger.New(cfg.Log)

	if path, generated, err := config.EnsureAPIKey(cfg, cfgFileUsed); err != nil {
		return err
	} else if generated {
		log.Info("api key generated", "config", path, "hint", "run `hylo auth show` to view it")
	}
	if path, generated, err := config.EnsureRelayIdentity(cfg, cfgFileUsed); err != nil {
		return fmt.Errorf("prepare relay identity: %w", err)
	} else if generated {
		log.Info("relay identity created", "config", path)
	}
	if err := config.Validate(cfg); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}
	if _, err := config.ParseProxy(cfg.Server.Proxy); err != nil {
		return fmt.Errorf("server.proxy: %w", err)
	}
	if cfg.Server.IsRemote() && !cfg.Server.TLSEnabled() {
		log.Warn("listening on a non-loopback address without TLS; put a TLS reverse proxy in front (see docs/remote-deployment.md)")
	}

	// Detect whether the vault was already initialised before we open it.
	// storage.New creates .hylo on first use, so we must check beforehand.
	wasInit, err := storage.IsVaultInitialized(cfg.Vault.Path)
	if err != nil {
		log.Warn("could not check vault initialisation state", "err", err)
	}

	vault, err := storage.New(cfg.Vault.Path)
	if err != nil {
		return fmt.Errorf("open vault: %w", err)
	}
	log.Info("vault opened", "path", vault.Root())

	if err := autoInitVault(log, cfg, vault, wasInit); err != nil {
		return fmt.Errorf("auto-init vault: %w", err)
	}

	pidFile := strings.TrimSpace(servePIDFile)

	srv := server.New(cfg, cfgFileUsed, log, vault)
	err = srv.Run(pidFile)
	if errors.Is(err, server.ErrRestart) {
		relaunch.SelfOrExit()
	}
	return err
}

func autoInitVault(log *slog.Logger, cfg *config.Config, vault *storage.Vault, initialized bool) error {
	return initializeVault(context.Background(), vault, cfg, log, initialized, nil, false, io.Discard)
}
