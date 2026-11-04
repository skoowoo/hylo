package cli

import (
	"fmt"
	"os"

	"github.com/hardhacker/hylo/internal/notes"
	"github.com/spf13/cobra"
)

func newImageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "image",
		Short:        "Save images into the vault",
		SilenceUsage: true,
	}
	cmd.AddCommand(newImageUploadCmd())
	return cmd
}

func newImageUploadCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "upload <path>",
		Short: "Save local image files into the vault and print their embeds",
		Long: `Save one or more local image files into the vault's _assets folder and print
one ![[name]] line per image, ready to put in a note.

To attach images to a short note, use "short create --images" instead.`,
		Example: `  hylo image upload board.jpg
  hylo image upload a.png b.png >> embeds.txt`,
		Args:         cobra.MinimumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := openClient()
			if err != nil {
				return err
			}
			imgs, err := notes.UploadImages(c, args)
			if err != nil {
				return err
			}
			for _, img := range imgs {
				fmt.Fprintln(os.Stdout, notes.ImageEmbed(img.Name))
			}
			return nil
		},
	}
}
