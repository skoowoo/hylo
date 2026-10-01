package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hardhacker/hylo/internal/config"
	"github.com/hardhacker/hylo/internal/logger"
	"github.com/hardhacker/hylo/internal/plugins/search"
	"github.com/hardhacker/hylo/internal/storage"
	"github.com/spf13/cobra"
)

var (
	vaultInitLinkImagesOnly bool
	vaultInitReindex        bool
	vaultInitRebuildGraph   bool
	vaultInitPreviewOnly    bool
)

func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init [path]",
		Short: "Initialize a directory as a Hylo vault (like git init)",
		Long: `Initialize the current working directory, or an optional path, as a Hylo vault.

The vault root is chosen from your filesystem — not from vault.path in config.toml.
Run this inside the directory that contains your Markdown notes (or pass the path).

If .hylo/ already exists, the command exits without changing anything.

Config is optional: without a config file, built-in defaults apply for compile output
directory and search indexing behaviour.`,
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		RunE:         runVaultInit,
	}
	cmd.Flags().BoolVar(&vaultInitLinkImagesOnly, "link-images-only", false,
		"only rebuild image–note associations (requires an existing .hylo/ in the vault)")
	cmd.Flags().BoolVar(&vaultInitReindex, "reindex", false,
		"delete and rebuild the full-text search index from scratch (requires an existing .hylo/ in the vault)")
	cmd.Flags().BoolVar(&vaultInitRebuildGraph, "rebuild-graph", false,
		"rebuild the knowledge graph link index from scratch (requires an existing .hylo/ in the vault)")
	cmd.Flags().BoolVar(&vaultInitPreviewOnly, "preview-only", false,
		"recompute every note's preview (content excerpt + checklist/image/code flags) (requires an existing .hylo/ in the vault)")
	return cmd
}

func resolveInitVaultRoot(args []string) (string, error) {
	base := "."
	if len(args) == 1 {
		base = args[0]
	}
	abs, err := filepath.Abs(base)
	if err != nil {
		return "", err
	}
	return abs, nil
}

func runVaultInit(_ *cobra.Command, args []string) error {
	root, err := resolveInitVaultRoot(args)
	if err != nil {
		return fmt.Errorf("resolve vault path: %w", err)
	}

	initd, err := storage.IsVaultInitialized(root)
	if err != nil {
		return err
	}

	if vaultInitLinkImagesOnly {
		if !initd {
			fmt.Fprintf(os.Stderr, "Not a Hylo vault (missing %s); run hylo init first.\n", filepath.Join(root, ".hylo"))
			return fmt.Errorf("vault not initialized")
		}
		vault, err := storage.New(root)
		if err != nil {
			return fmt.Errorf("open vault: %w", err)
		}
		defer vault.Close()
		return runLinkImagesWork(vault)
	}

	if vaultInitReindex {
		if !initd {
			fmt.Fprintf(os.Stderr, "Not a Hylo vault (missing %s); run hylo init first.\n", filepath.Join(root, ".hylo"))
			return fmt.Errorf("vault not initialized")
		}
		vault, err := storage.New(root)
		if err != nil {
			return fmt.Errorf("open vault: %w", err)
		}
		defer vault.Close()
		return runReindex(root, vault)
	}

	if vaultInitRebuildGraph {
		if !initd {
			fmt.Fprintf(os.Stderr, "Not a Hylo vault (missing %s); run hylo init first.\n", filepath.Join(root, ".hylo"))
			return fmt.Errorf("vault not initialized")
		}
		vault, err := storage.New(root)
		if err != nil {
			return fmt.Errorf("open vault: %w", err)
		}
		defer vault.Close()
		return runRebuildGraph(vault)
	}

	if vaultInitPreviewOnly {
		if !initd {
			fmt.Fprintf(os.Stderr, "Not a Hylo vault (missing %s); run hylo init first.\n", filepath.Join(root, ".hylo"))
			return fmt.Errorf("vault not initialized")
		}
		vault, err := storage.New(root)
		if err != nil {
			return fmt.Errorf("open vault: %w", err)
		}
		defer vault.Close()
		return runPreviewOnly(vault)
	}

	if initd {
		fmt.Printf("Already initialized (%s exists); skipping.\n", filepath.Join(root, ".hylo"))
		return nil
	}

	cfg := config.MustLoad("")

	vault, err := storage.New(root)
	if err != nil {
		return fmt.Errorf("open vault: %w", err)
	}
	defer vault.Close()

	log := logger.New(cfg.Log)

	// Phase 1: walk the vault filesystem and register every markdown file in the
	// metadata database. Raw notes and knowledge notes are distinguished and all
	// fields (origin, compiled) are initialised from disk state.
	fmt.Printf("Scanning vault: %s\n", vault.Root())
	registered, err := vault.ScanAndRegisterFull(cfg.Vault.KnowledgeDir)
	if err != nil {
		return fmt.Errorf("scan vault: %w", err)
	}
	fmt.Printf("Registered %d notes in metadata database\n", registered)

	// Phase 1b: walk the vault for image files and register them.
	imgRegistered, err := vault.ScanAndRegisterImages()
	if err != nil {
		return fmt.Errorf("scan images: %w", err)
	}
	fmt.Printf("Registered %d images in metadata database\n", imgRegistered)

	// Phase 1c: build note→image link associations (single walk over all .md files).
	fmt.Println("Building image–note associations...")
	if err := vault.BuildImageNoteLinks(); err != nil {
		fmt.Printf("Warning: image–note links: %v\n", err)
	}

	// Phase 2: index all notes that are not yet in the full-text search index.
	fmt.Println("Building full-text search index...")
	sp := search.New(cfg.Plugins.Search, vault, log)
	indexed, err := sp.Backfill(context.Background())
	if err != nil {
		return fmt.Errorf("build search index: %w", err)
	}
	if err := sp.Stop(); err != nil {
		log.Warn("search: close index", "err", err)
	}
	fmt.Printf("Indexed %d notes in full-text search\n", indexed)
	fmt.Println("Initialization complete.")
	return nil
}

func runLinkImagesWork(vault *storage.Vault) error {
	fmt.Printf("Scanning vault: %s\n", vault.Root())
	fmt.Println("Building image–note associations...")
	if err := vault.BuildImageNoteLinks(); err != nil {
		return fmt.Errorf("link images: %w", err)
	}
	fmt.Println("Done.")
	return nil
}

func runPreviewOnly(vault *storage.Vault) error {
	fmt.Println("Recomputing note previews...")
	count, err := vault.BackfillPreviews()
	if err != nil {
		return fmt.Errorf("backfill previews: %w", err)
	}
	fmt.Printf("Recomputed preview for %d notes\n", count)
	return nil
}

func runRebuildGraph(vault *storage.Vault) error {
	cfg := config.MustLoad("")
	fmt.Println("Rebuilding knowledge graph link index...")
	if err := vault.BackfillKnowledgeLinks(cfg.Vault.KnowledgeDir); err != nil {
		return fmt.Errorf("rebuild graph: %w", err)
	}
	fmt.Println("Rebuild complete.")
	return nil
}

func runReindex(root string, vault *storage.Vault) error {
	cfg := config.MustLoad("")
	log := logger.New(cfg.Log)

	// Delete the existing bleve index so it is rebuilt with the current schema.
	idxPath := filepath.Join(root, ".hylo", "data.idx")
	fmt.Printf("Removing existing search index: %s\n", idxPath)
	if err := os.RemoveAll(idxPath); err != nil {
		return fmt.Errorf("remove index: %w", err)
	}

	// Reset indexed flags so backfill processes every note.
	fmt.Println("Resetting indexed flags in metadata database...")
	if err := vault.ResetAllIndexed(); err != nil {
		return fmt.Errorf("reset indexed flags: %w", err)
	}

	fmt.Println("Building full-text search index...")
	sp := search.New(cfg.Plugins.Search, vault, log)
	indexed, err := sp.Backfill(context.Background())
	if err != nil {
		return fmt.Errorf("build search index: %w", err)
	}
	if err := sp.Stop(); err != nil {
		log.Warn("search: close index", "err", err)
	}
	fmt.Printf("Indexed %d notes in full-text search\n", indexed)
	fmt.Println("Reindex complete.")
	return nil
}
