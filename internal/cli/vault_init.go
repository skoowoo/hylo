package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/hardhacker/hylo/internal/config"
	"github.com/hardhacker/hylo/internal/plugins/assets"
	"github.com/hardhacker/hylo/internal/plugins/search"
	"github.com/hardhacker/hylo/internal/storage"
)

type vaultInitState struct {
	Notes    bool           `json:"notes"`
	Features map[string]int `json:"features"`
}

type vaultInitFeature struct {
	name    string
	version int
	build   func(context.Context, *storage.Vault, *config.Config, *slog.Logger) error
}

var vaultInitFeatures = []vaultInitFeature{
	{"image", 1, initImages},
	{"graph", 1, initGraph},
	{"preview", 1, initPreviews},
	{"index", 1, initSearch},
}

func initializeVault(ctx context.Context, vault *storage.Vault, cfg *config.Config, log *slog.Logger, initialized bool, selected map[string]bool, force bool, out io.Writer) error {
	state := vaultInitState{Features: map[string]int{}}
	stateKnown := false
	if initialized {
		data, err := os.ReadFile(initStatePath(vault))
		if err == nil {
			if err := json.Unmarshal(data, &state); err != nil {
				log.Warn("vault init: ignoring invalid state", "err", err)
			} else {
				stateKnown = true
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if state.Features == nil {
		state.Features = map[string]int{}
	}
	if initialized && !stateKnown {
		notes, err := vault.ListAllNotes(storage.ListOptions{})
		if err != nil {
			return fmt.Errorf("read registered notes: %w", err)
		}
		state.Notes = len(notes) > 0
	}
	save := func() error {
		data, err := json.MarshalIndent(state, "", "  ")
		if err != nil {
			return err
		}
		return saveInitState(vault, data)
	}
	if !state.Notes {
		// Persist an unfinished bootstrap so a retry can resume it.
		if err := save(); err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		fmt.Fprintln(out, "Registering notes...")
		if _, err := vault.ScanAndRegisterFull(); err != nil {
			return fmt.Errorf("register notes: %w", err)
		}
		state.Notes = true
		state.Features["graph"] = featureVersion("graph")
		state.Features["preview"] = featureVersion("preview")
		if err := save(); err != nil {
			return err
		}
	}
	all := true
	for _, enabled := range selected {
		if enabled {
			all = false
			break
		}
	}
	for _, feature := range vaultInitFeatures {
		if !all && !selected[feature.name] {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if !force && state.Features[feature.name] == feature.version {
			fmt.Fprintf(out, "%s: already initialized; skipping\n", feature.name)
			continue
		}
		delete(state.Features, feature.name)
		if err := save(); err != nil {
			return err
		}
		fmt.Fprintf(out, "%s: building...\n", feature.name)
		log.Info("vault init: building", "feature", feature.name)
		if err := feature.build(ctx, vault, cfg, log); err != nil {
			return fmt.Errorf("initialize %s: %w", feature.name, err)
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		state.Features[feature.name] = feature.version
		if err := save(); err != nil {
			return err
		}
		fmt.Fprintf(out, "%s: complete\n", feature.name)
		log.Info("vault init: complete", "feature", feature.name)
	}
	fmt.Fprintln(out, "Initialization complete.")
	return nil
}

func featureVersion(name string) int {
	for _, feature := range vaultInitFeatures {
		if feature.name == name {
			return feature.version
		}
	}
	return 0
}

func initImages(ctx context.Context, vault *storage.Vault, cfg *config.Config, log *slog.Logger) error {
	proxy, err := config.ParseProxy(cfg.Server.Proxy)
	if err != nil {
		return fmt.Errorf("server.proxy: %w", err)
	}
	if _, err := vault.ScanAndRegisterImages(); err != nil {
		return err
	}
	if err := vault.BackfillBodyImageLinks(); err != nil {
		return err
	}
	resources := assets.New(vault, log, proxy)
	defer resources.Stop()
	return resources.BackfillCovers(ctx)
}

func initGraph(_ context.Context, vault *storage.Vault, _ *config.Config, _ *slog.Logger) error {
	return vault.BackfillKnowledgeLinks()
}

func initPreviews(_ context.Context, vault *storage.Vault, _ *config.Config, _ *slog.Logger) error {
	_, err := vault.BackfillPreviews()
	return err
}

func initSearch(ctx context.Context, vault *storage.Vault, cfg *config.Config, log *slog.Logger) (err error) {
	if err := vault.ResetAllIndexed(); err != nil {
		return err
	}
	if err := os.RemoveAll(filepath.Join(vault.Root(), ".hylo", "data.idx")); err != nil {
		return err
	}
	indexer := search.New(cfg.Plugins.Search, vault, log)
	defer func() { err = errors.Join(err, indexer.Stop()) }()
	_, err = indexer.Backfill(ctx)
	return err
}

func initStatePath(vault *storage.Vault) string {
	return filepath.Join(vault.Root(), ".hylo", "init.json")
}

func saveInitState(vault *storage.Vault, data []byte) error {
	file, err := os.CreateTemp(filepath.Dir(initStatePath(vault)), ".init-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), initStatePath(vault))
}
