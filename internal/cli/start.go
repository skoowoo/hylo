package cli

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/hardhacker/hylo/internal/agent"
	"github.com/hardhacker/hylo/internal/config"
	"github.com/hardhacker/hylo/internal/logger"
	"github.com/hardhacker/hylo/internal/plugins/search"
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

The listening address (TCP host/port, timeouts) is read only from
the hylo.toml config file — not from flags or environment variables.

By default the server listens on TCP at 127.0.0.1:54321. Set server.port to 0
in hylo.toml to disable the server entirely.

After the server successfully binds its listen address, its process ID is written
to the pid file (see --pid-file). The file is removed when the server exits.`,
		SilenceUsage: true,
		RunE:         runServe,
	}
	serverCmd.Flags().StringVar(&servePIDFile, "pid-file", defaultServerPIDFile(),
		"path to write this process ID after the server starts listening (use empty string to skip)")

	cmd.AddCommand(serverCmd)

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

	// First-time open: if there are pre-existing markdown files, register them
	// so the server starts with a fully populated metadata DB and search index.
	if !wasInit {
		if err := autoInitVault(log, cfg, vault); err != nil {
			log.Warn("auto-init failed; notes may be missing from DB until next restart", "err", err)
		}
	}

	// Reconcile note→image links on every start, not just the first-ever
	// init. The assets plugin (internal/plugins/assets) keeps note_assets'
	// kind=image rows current incrementally on every save going forward;
	// this backfill only exists to catch notes that predate the feature or
	// haven't been saved since the last restart.
	if err := vault.BackfillBodyImageLinks(); err != nil {
		log.Warn("image-note links: backfill failed", "err", err)
	}

	pidFile := strings.TrimSpace(servePIDFile)

	srv := server.New(cfg, cfgFileUsed, log, vault)
	return srv.Run(pidFile)
}

// autoInitVault checks whether the (freshly created) vault root already contains
// markdown files. If it does, it runs the same registration steps as "hylo init"
// so that pre-existing notes are immediately available to the server.
// The search backfill is intentionally omitted here: the search plugin runs its
// own backfill on startup and will pick up all notes registered by this call.
func autoInitVault(log *slog.Logger, cfg *config.Config, vault *storage.Vault) error {
	if !vaultHasMarkdownFiles(vault.Root()) {
		return nil
	}

	log.Info("detected pre-existing notes in new vault; running auto-init", "root", vault.Root())

	registered, err := vault.ScanAndRegisterFull(cfg.Vault.KnowledgeDir)
	if err != nil {
		return fmt.Errorf("scan and register notes: %w", err)
	}
	log.Info("auto-init: registered notes", "count", registered)

	imgRegistered, err := vault.ScanAndRegisterImages()
	if err != nil {
		log.Warn("auto-init: image scan failed", "err", err)
	} else {
		log.Info("auto-init: registered images", "count", imgRegistered)
	}

	// image-note links are rebuilt unconditionally right after this call
	// returns (see runServe) — no need to also do it here.

	// Rebuild search index for the freshly registered notes.
	sp := search.New(cfg.Plugins.Search, vault, log)
	if _, err := sp.Backfill(context.Background()); err != nil {
		log.Warn("auto-init: search backfill failed", "err", err)
	}
	if err := sp.Stop(); err != nil {
		log.Warn("auto-init: close search indexer", "err", err)
	}

	log.Info("auto-init complete")
	return nil
}

// vaultHasMarkdownFiles reports whether root contains at least one .md file,
// skipping hidden directories (including .hylo itself).
func vaultHasMarkdownFiles(root string) bool {
	found := false
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			found = true
			return fs.SkipAll
		}
		return nil
	})
	return found
}
