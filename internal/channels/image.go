package channels

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/hardhacker/hylo/internal/storage"
)

const (
	maxChannelImages     = 5
	maxChannelImageBytes = 8 << 20
	truncatedImageNotice = "Only the first 5 images were kept."
)

var (
	errImageFetch  = errors.New("image fetch")
	errImageType   = errors.New("image type")
	errImageTooBig = errors.New("image size")
)

// imageFetch downloads this turn's pictures into the upload directory.
// A notice is shown on the channel and the turn still runs.
type imageFetch func(ctx context.Context, dir string) (paths []string, notice string, err error)

func imageProblem(err error) string {
	switch {
	case errors.Is(err, errImageType):
		return "Please send a JPG or PNG."
	case errors.Is(err, errImageTooBig):
		return "This image is too large. Please send a smaller one."
	default:
		return "The image didn't come through. Please send it again."
	}
}

// refusedImages are pictures an agent cannot take. They still count as
// pictures, so the sender is told why instead of being ignored.
var refusedImages = map[string]string{
	".heic": "image/heic", ".heif": "image/heif", ".bmp": "image/bmp",
	".svg": "image/svg+xml", ".avif": "image/avif",
}

func mediaType(contentType string) string {
	return strings.ToLower(strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0]))
}

// maybeImage reports whether a received file is a picture, refused types included.
func maybeImage(contentType, name string) bool {
	if strings.HasPrefix(mediaType(contentType), "image/") {
		return true
	}
	ext := strings.ToLower(filepath.Ext(name))
	_, refused := refusedImages[ext]
	return refused || storage.ImageContentType(ext) != "application/octet-stream"
}

func rejectedImage(contentType, name string) bool {
	ct := mediaType(contentType)
	for ext, mime := range refusedImages {
		if ct == mime || strings.EqualFold(filepath.Ext(name), ext) {
			return true
		}
	}
	return false
}

// saveChannelImage writes one jpeg, png, gif, or webp under dir, named so the
// chat turn can show it. Other pictures are refused; a body that is not a
// picture at all is a failed download.
func saveChannelImage(dir string, data []byte, contentType, filename string) (string, error) {
	if len(data) == 0 {
		return "", errImageFetch
	}
	if len(data) > maxChannelImageBytes {
		return "", errImageTooBig
	}
	ext := storage.DetectImageExt(data, "", "")
	switch ext {
	case ".jpg", ".png", ".gif", ".webp":
	default:
		if rejectedImage(contentType, filename) {
			return "", errImageType
		}
		return "", errImageFetch
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", errImageFetch
	}
	name := uuid.NewString() + ext
	abs := filepath.Join(dir, name)
	if err := os.WriteFile(abs, data, 0o644); err != nil {
		return "", errImageFetch
	}
	return abs, nil
}

func capImages[T any](items []T) ([]T, string) {
	if len(items) <= maxChannelImages {
		return items, ""
	}
	return items[:maxChannelImages], truncatedImageNotice
}
