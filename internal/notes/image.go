package notes

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hardhacker/hylo/internal/client"
)

// ImageEmbed is how a note embeds a stored image.
func ImageEmbed(name string) string { return "![[" + name + "]]" }

// WithImages puts the embeds below the text, one per line, the way the Shorts
// composer saves them.
func WithImages(content string, images []client.UploadedImage) string {
	content = strings.TrimSpace(content)
	if len(images) == 0 {
		return content
	}
	embeds := make([]string, len(images))
	for i, img := range images {
		embeds[i] = ImageEmbed(img.Name)
	}
	links := strings.Join(embeds, "\n")
	if content == "" {
		return links
	}
	return content + "\n\n" + links
}

// UploadImages stores local image files in order. Every path is checked first,
// so a missing one uploads nothing.
func UploadImages(c *client.Client, paths []string) ([]client.UploadedImage, error) {
	for _, p := range paths {
		if st, err := os.Stat(p); err != nil {
			return nil, fmt.Errorf("image %q: %w", p, err)
		} else if st.IsDir() {
			return nil, fmt.Errorf("image %q is a directory", p)
		}
	}
	return uploadEach(paths, c.UploadImage)
}

// UploadChatImages stores images the user attached in chat, given as the paths
// the agent was handed; only the file name is used, and the server resolves it
// inside its upload folder.
func UploadChatImages(c *client.Client, paths []string) ([]client.UploadedImage, error) {
	return uploadEach(paths, func(p string) (client.UploadedImage, error) {
		return c.UploadChatImage(filepath.Base(p))
	})
}

func uploadEach(paths []string, up func(string) (client.UploadedImage, error)) ([]client.UploadedImage, error) {
	out := make([]client.UploadedImage, 0, len(paths))
	for _, p := range paths {
		img, err := up(p)
		if err != nil {
			return nil, err
		}
		out = append(out, img)
	}
	return out, nil
}
