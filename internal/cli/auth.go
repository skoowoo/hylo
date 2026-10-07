package cli

import (
	"fmt"

	"github.com/hardhacker/hylo/internal/config"
	"github.com/spf13/cobra"
)

func newAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "auth",
		Short:        "Manage the server API key",
		SilenceUsage: true,
	}
	show := &cobra.Command{
		Use:          "show",
		Short:        "Print the API key and login URL",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE:         runAuthShow,
	}
	show.Flags().Bool("raw", false, "print only the key (for scripts and the desktop app)")
	cmd.AddCommand(show)
	cmd.AddCommand(&cobra.Command{
		Use:          "rotate",
		Short:        "Generate a new API key (restart the server to apply)",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE:         runAuthRotate,
	})
	return cmd
}

func runAuthShow(cmd *cobra.Command, _ []string) error {
	cfg, path, err := config.Load("")
	if err != nil {
		return err
	}
	if raw, _ := cmd.Flags().GetBool("raw"); raw {
		if cfg.Server.APIKey == "" {
			return fmt.Errorf("no API key yet")
		}
		fmt.Println(cfg.Server.APIKey)
		return nil
	}
	if cfg.Server.APIKey == "" {
		fmt.Println("No API key yet. It is generated on first server start, or run `hylo auth rotate` now.")
		return nil
	}
	source := path
	if cfg.Server.APIKeyFromEnv {
		source = "environment HYLO_API_KEY"
	}
	fmt.Printf("API key: %s\nSource:  %s\n", cfg.Server.APIKey, source)
	if base, err := cfg.Server.ClientBaseURL(); err == nil {
		fmt.Printf("Login:   %s/login\n", base)
	}
	return nil
}

func runAuthRotate(_ *cobra.Command, _ []string) error {
	cfg, path, err := config.Load("")
	if err != nil {
		return err
	}
	key, err := config.GenerateAPIKey()
	if err != nil {
		return err
	}
	written, err := config.PersistAPIKey(path, key)
	if err != nil {
		return err
	}
	fmt.Printf("New API key: %s\nSaved to:    %s\n", key, written)
	if cfg.Server.APIKeyFromEnv {
		fmt.Println("Warning: HYLO_API_KEY is set and still overrides the config file.")
	}
	fmt.Println("Restart the server to apply; the old key and all browser sessions stop working.")
	return nil
}
