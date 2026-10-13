package cli

import (
	"fmt"

	"github.com/hardhacker/hylo/internal/notes"
	"github.com/spf13/cobra"
)

func newSkillsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "skills",
		Short:        "Manage Claude skills",
		SilenceUsage: true,
	}
	cmd.AddCommand(newSkillsListCmd())
	cmd.AddCommand(newSkillsAddCmd())
	cmd.AddCommand(newSkillsInstallCmd())
	cmd.AddCommand(newSkillsRemoveCmd())
	return cmd
}

func newSkillsListCmd() *cobra.Command {
	var category string
	cmd := &cobra.Command{
		Use:          "list",
		Short:        "List installed skills and ones in Hylo's catalog",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := openClient()
			if err != nil {
				return err
			}
			skills, err := notes.SkillsList(c, category)
			if err != nil {
				return err
			}
			if len(skills) == 0 {
				fmt.Println("No skills installed.")
				return nil
			}
			cols := []Column{
				{Header: "NAME", MaxWidth: 40},
				{Header: "STATUS"},
				{Header: "TYPE"},
				{Header: "REPO", MaxWidth: 60},
			}
			rows := make([][]string, len(skills))
			for i, s := range skills {
				status := "disabled"
				if s.Enabled {
					status = "enabled"
				}
				kind := "external"
				if s.Default {
					kind = "built-in"
				}
				rows[i] = []string{s.Name, status, kind, s.RepoURL}
			}
			PrintTable(cols, rows)
			return nil
		},
	}
	cmd.Flags().StringVar(&category, "category", "", "only skills in this catalog category, e.g. news")
	return cmd
}

func newSkillsInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "install <name>",
		Short:        "Install a skill from Hylo's catalog, or turn on an installed one",
		Long:         `Installs a skill listed by "hylo skills list" and turns it on. To install from any GitHub repository, use "hylo skills add".`,
		Example:      `  hylo skills install hackernews`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := openClient()
			if err != nil {
				return err
			}
			msg, err := notes.SkillsInstall(c, args[0])
			if err != nil {
				return err
			}
			fmt.Printf("Skill %q %s.\n", args[0], msg)
			return nil
		},
	}
}

func newSkillsRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "remove <name>",
		Short:        "Remove an installed skill and its symlinks",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			c, err := openClient()
			if err != nil {
				return err
			}
			if err := c.SkillsRemove(name); err != nil {
				return err
			}
			fmt.Printf("Skill %q removed.\n", name)
			return nil
		},
	}
}

func newSkillsAddCmd() *cobra.Command {
	var skillName string

	cmd := &cobra.Command{
		Use:   "add <github-url>",
		Short: "Install a skill from a GitHub repository",
		Long: `Install a skill from a GitHub repository into ~/.hylo/skills/.

The repository URL can be a full HTTPS URL or the shorthand owner/repo form.
--skill is required and sets the local directory name for the installed skill.
The skill directory inside the repository is discovered automatically.

Examples:
  hylo skills add https://github.com/hardhackerlabs/podwise-cli --skill podwise
  hylo skills add hardhackerlabs/podwise-cli --skill podwise`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if skillName == "" {
				return fmt.Errorf("--skill is required")
			}
			repoURL := args[0]

			c, err := openClient()
			if err != nil {
				return err
			}

			fmt.Printf("Installing skill %q from %s...\n", skillName, repoURL)
			if err := c.SkillsInstall(repoURL, "", skillName); err != nil {
				return err
			}
			fmt.Printf("Skill %q installed to ~/.hylo/skills/%s\n", skillName, skillName)
			return nil
		},
	}

	cmd.Flags().StringVar(&skillName, "skill", "", "local directory name for the installed skill (required)")
	return cmd
}
