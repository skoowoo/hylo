package cli

import (
	"fmt"
	"path/filepath"

	"github.com/hardhacker/hylo/internal/config"
	"github.com/hardhacker/hylo/internal/logger"
	"github.com/hardhacker/hylo/internal/storage"
	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	var image, graph, index, preview, force bool
	cmd := &cobra.Command{
		Use:   "init [path]",
		Short: "Initialize or rebuild vault images, graph, search and previews",
		Long: `Initialize the current directory, or an optional path, as a Hylo vault.

Without feature flags, initialize all features. Combine --image, --graph,
--index and --preview to select features. Completed features are skipped;
--force rebuilds only the selected features (all features if none are selected).

New vaults register existing Markdown notes first. Rebuilding features preserves
note metadata and source files. The path is independent of vault.path in config.`,
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
	}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		root, err := resolveInitVaultRoot(args)
		if err != nil {
			return fmt.Errorf("resolve vault path: %w", err)
		}
		initialized, err := storage.IsVaultInitialized(root)
		if err != nil {
			return err
		}
		cfg := config.MustLoadFor(root)
		vault, err := storage.New(root)
		if err != nil {
			return fmt.Errorf("open vault: %w", err)
		}
		defer vault.Close()
		selected := map[string]bool{
			"image":   image,
			"graph":   graph,
			"index":   index,
			"preview": preview,
		}
		return initializeVault(cmd.Context(), vault, cfg, logger.New(cfg.Log), initialized,
			selected, force, cmd.OutOrStdout())
	}
	flags := cmd.Flags()
	flags.BoolVar(&image, "image", false, "initialize images, body-image associations and note covers")
	flags.BoolVar(&graph, "graph", false, "initialize the knowledge graph link index")
	flags.BoolVar(&index, "index", false, "initialize the full-text search index")
	flags.BoolVar(&preview, "preview", false, "initialize note previews and checklist/image/code summaries")
	flags.BoolVar(&force, "force", false, "rebuild selected features even if already initialized")
	return cmd
}

func resolveInitVaultRoot(args []string) (string, error) {
	base := "."
	if len(args) == 1 {
		base = args[0]
	}
	return filepath.Abs(base)
}
