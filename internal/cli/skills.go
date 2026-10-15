package cli

import (
	"fmt"
	"strings"

	"github.com/hardhacker/hylo/internal/client"
	"github.com/hardhacker/hylo/internal/notes"
	"github.com/spf13/cobra"
)

func newSkillsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skills",
		Short: "Add, turn on or off, and remove skills",
		Long: `Skills are added to ~/.hylo/skills/ from GitHub, then turned on in the vault,
where agents pick them up. Built-in skills are always on.`,
		SilenceUsage: true,
	}
	cmd.AddCommand(newSkillsListCmd(), newSkillsCatalogCmd(), newSkillsAddCmd(),
		newSkillsEnableCmd(), newSkillsDisableCmd(), newSkillsRemoveCmd())
	return cmd
}

func newSkillsListCmd() *cobra.Command {
	var table bool
	cmd := &cobra.Command{
		Use:          "list",
		Short:        "List the skills added to ~/.hylo/skills/ and the vault directories each is on in",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := openClient()
			if err != nil {
				return err
			}
			skills, err := c.SkillsList()
			if err != nil {
				return err
			}
			if !table {
				return printJSON(skills)
			}
			rows := make([][]string, len(skills))
			for i, s := range skills {
				on := strings.Join(s.Dirs, ", ")
				if on == "" {
					on = "off"
				}
				kind := ""
				if s.Default {
					kind = "built-in"
				}
				rows[i] = []string{s.Name, on, kind, s.Description}
			}
			PrintTable([]Column{{Header: "NAME", MaxWidth: 40}, {Header: "ON IN", MaxWidth: 40}, {Header: "TYPE"}, {Header: "DESCRIPTION", MaxWidth: 60}}, rows)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&table, "table", "t", false, "output in table format")
	return cmd
}

func newSkillsCatalogCmd() *cobra.Command {
	var (
		category string
		table    bool
	)
	cmd := &cobra.Command{
		Use:   "catalog",
		Short: "List the skills Hylo recommends",
		Long: `Lists the skills Hylo recommends, each with the repository and name to add it with:
  hylo skills add <repo> --skill <name>`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := openClient()
			if err != nil {
				return err
			}
			entries, err := notes.SkillsCatalog(c, category)
			if err != nil {
				return err
			}
			if !table {
				return printJSON(entries)
			}
			rows := make([][]string, len(entries))
			for i, e := range entries {
				rows[i] = []string{e.Name, e.Repo, e.Category, fmt.Sprintf("%t", e.Added)}
			}
			PrintTable([]Column{{Header: "NAME", MaxWidth: 40}, {Header: "REPO", MaxWidth: 50}, {Header: "CATEGORY"}, {Header: "ADDED"}}, rows)
			return nil
		},
	}
	cmd.Flags().StringVar(&category, "category", "", "only skills in this category, e.g. news")
	cmd.Flags().BoolVarP(&table, "table", "t", false, "output in table format")
	return cmd
}

func newSkillsAddCmd() *cobra.Command {
	var (
		name  string
		force bool
	)
	cmd := &cobra.Command{
		Use:   "add <repo>",
		Short: "Add a skill from a GitHub repository, turned off",
		Long: `Downloads a skill from a GitHub repository into ~/.hylo/skills/. It starts off;
turn it on with "hylo skills enable".

<repo> is a URL or the owner/repo shorthand. --skill is the skill's name, or else
the repository's; the skill is looked up by it in the usual places (the repository
root, <name>/, skills/<name>/, skills/*/<name>/, skills/, skill/) and added under it.
An added skill of the same name is replaced only with --force.`,
		Example: `  hylo skills add okou-ai/okou-skills --skill hackernews
  hylo skills add https://github.com/hardhackerlabs/podwise-cli --skill podwise
  hylo skills add blader/humanizer`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := openClient()
			if err != nil {
				return err
			}
			added, err := c.SkillsAdd(args[0], name, force)
			if err != nil {
				return err
			}
			fmt.Printf("Skill %q added; turn it on with: hylo skills enable %s\n", added, added)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "skill", "", "the skill's name, to find and add it by (default: the repository name)")
	cmd.Flags().BoolVar(&force, "force", false, "replace an added skill of the same name")
	return cmd
}

func newSkillsEnableCmd() *cobra.Command {
	return skillDirCmd("enable", "Turn an added skill on in the vault root or a vault directory",
		`Links the skill into the vault directory --dir (the vault root by default), so agents
started there can use it. The directory is created if it doesn't exist.`,
		"  hylo skills enable podwise\n  hylo skills enable hackernews --dir /_news",
		(*client.Client).SkillsEnable, "on")
}

func newSkillsDisableCmd() *cobra.Command {
	return skillDirCmd("disable", "Turn a skill off in the vault root or a vault directory",
		`Unlinks the skill from the vault directory --dir (the vault root by default); its
files stay in ~/.hylo/skills/.`,
		"  hylo skills disable podwise\n  hylo skills disable hackernews --dir /_news",
		(*client.Client).SkillsDisable, "off")
}

func skillDirCmd(use, short, long, example string, op func(c *client.Client, name, dir string) error, state string) *cobra.Command {
	var dir string
	cmd := &cobra.Command{
		Use:          use + " <name>",
		Short:        short,
		Long:         long,
		Example:      example,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := openClient()
			if err != nil {
				return err
			}
			if err := op(c, args[0], dir); err != nil {
				return err
			}
			where := "the vault root"
			if dir != "" {
				where = dir
			}
			fmt.Printf("Skill %q is %s in %s.\n", args[0], state, where)
			return nil
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "", "vault directory, e.g. /_news (default: the vault root)")
	return cmd
}

func newSkillsRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "remove <name>",
		Short:        "Delete a skill from ~/.hylo/skills/ and turn it off",
		Long:         `Deletes the skill's files from ~/.hylo/skills/ and turns it off in the vault; there is no undo.`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := openClient()
			if err != nil {
				return err
			}
			if err := c.SkillsRemove(args[0]); err != nil {
				return err
			}
			fmt.Printf("Skill %q removed.\n", args[0])
			return nil
		},
	}
}
