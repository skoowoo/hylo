package storage

import (
	"bytes"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const MaxImageBytes = 10 << 20

var (
	imageExtByMIME = map[string]string{
		"image/jpeg":    ".jpg",
		"image/png":     ".png",
		"image/gif":     ".gif",
		"image/webp":    ".webp",
		"image/avif":    ".avif",
		"image/svg+xml": ".svg",
	}
	imageMIMEByExt = map[string]string{
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".png":  "image/png",
		".gif":  "image/gif",
		".webp": "image/webp",
		".avif": "image/avif",
		".svg":  "image/svg+xml",
	}
)

// ErrInvalidImageRef indicates dir/name or resolved path is not allowed for gallery delete.
var ErrInvalidImageRef = errors.New("invalid image reference")

// NoteAssetKind classifies a resource extracted from / attached to a note.
type NoteAssetKind string

const (
	AssetKindCover NoteAssetKind = "cover"
	AssetKindImage NoteAssetKind = "image"
	AssetKindAudio NoteAssetKind = "audio"
	AssetKindVideo NoteAssetKind = "video"
)

// NoteAsset is one extracted resource linked to a note.
type NoteAsset struct {
	NoteDir   string
	NoteName  string
	Kind      NoteAssetKind
	Filename  string // vault-wide unique basename
	SourceURL string
	Ord       int // order within the same kind; cover uses 0
	CreatedAt time.Time
}

// Image is the metadata for a single image file inside a Vault.
//
// Which notes embed an image lives in note_assets (kind=image), not here —
// see Vault.NoteNamesForImages.
type Image struct {
	Dir       string // vault-absolute directory path, e.g. "/_assets/202501" or "/attachments"
	Name      string // filename with extension, e.g. "photo.png"
	Ext       string // lowercase extension including dot, e.g. ".png"
	Size      int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

// wikiImageLinkRe matches Obsidian wiki-style image embeds: ![[filename.ext]] or ![[filename.ext|hint]]
var wikiImageLinkRe = regexp.MustCompile(`!\[\[([^\]\|]+?)(?:\|[^\]]*?)?\]\]`)

// imageExtensions is the set of recognised image file extensions (lowercase, with dot).
var imageExtensions = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".gif":  true,
	".webp": true,
	".avif": true,
	".svg":  true,
}

// IsImagePath reports whether a filename has a recognised image extension.
func IsImagePath(name string) bool {
	return imageExtensions[strings.ToLower(filepath.Ext(name))]
}

// GetImagesByName returns all images whose filename equals name.
// Results are ordered by updated_at descending (most recently modified first).
func (g *Vault) GetImagesByName(name string) ([]Image, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return dbImageGetByName(g.db, name)
}

// RegisterImage inserts or updates the metadata row for an image.
func (g *Vault) RegisterImage(img Image) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return dbImageUpsert(g.db, img)
}

// DeleteImageMeta removes the metadata row for the image at (dir, name).
func (g *Vault) DeleteImageMeta(dir, name string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if err := dbImageDelete(g.db, dir, name); err != nil {
		return err
	}
	return dbDeleteNoteAssetsIfImageNameUnused(g.db, name)
}

// DeleteImage removes the image file at vault-relative (dir, name) and its
// metadata row. dir is vault-absolute (e.g. "/_assets/202501"); name is the
// basename only. Paths under .hylo are rejected. Missing files still clear DB.
func (g *Vault) DeleteImage(dir, name string) error {
	if name == "" || strings.Contains(name, "/") || strings.Contains(name, "\\") || strings.Contains(name, "..") {
		return fmt.Errorf("%w: invalid image name %q", ErrInvalidImageRef, name)
	}
	if !IsImagePath(name) {
		return fmt.Errorf("%w: %q is not a recognised image file", ErrInvalidImageRef, name)
	}
	dir = strings.TrimSpace(dir)
	if dir == "" || strings.Contains(dir, "..") {
		return fmt.Errorf("%w: invalid image directory %q", ErrInvalidImageRef, dir)
	}
	if !strings.HasPrefix(dir, "/") {
		dir = "/" + dir
	}
	img := Image{Dir: dir, Name: name}
	abs := g.OsImagePath(img)

	rootWithSep := g.root + string(os.PathSeparator)
	if !strings.HasPrefix(abs, rootWithSep) && abs != g.root {
		return fmt.Errorf("%w: resolved path outside vault", ErrInvalidImageRef)
	}
	internalRoot := filepath.Clean(vaultInternalPath(g.root))
	internalWithSep := internalRoot + string(os.PathSeparator)
	if strings.HasPrefix(abs, internalWithSep) || filepath.Clean(abs) == internalRoot {
		return fmt.Errorf("%w: cannot delete vault-internal path", ErrInvalidImageRef)
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	if err := os.Remove(abs); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("storage: remove image file: %w", err)
	}
	if err := dbImageDelete(g.db, dir, name); err != nil {
		return fmt.Errorf("storage: delete image metadata: %w", err)
	}
	if err := dbDeleteNoteAssetsIfImageNameUnused(g.db, name); err != nil {
		return fmt.Errorf("storage: delete image asset references: %w", err)
	}
	return nil
}

func dbDeleteNoteAssetsIfImageNameUnused(db *sql.DB, name string) error {
	imgs, err := dbImageGetByName(db, name)
	if err != nil {
		return err
	}
	if len(imgs) > 0 {
		return nil
	}
	return dbDeleteNoteAssetsByFilename(db, name)
}

// ScanAndRegisterImages walks the entire vault root and upserts a metadata row
// for every image file found. Hidden directories (names starting with ".") are
// skipped. Returns the number of images registered.
func (g *Vault) ScanAndRegisterImages() (int, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if err := dbImageClearAll(g.db); err != nil {
		return 0, err
	}

	count := 0
	err := filepath.WalkDir(g.root, func(absPath string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !IsImagePath(d.Name()) {
			return nil
		}

		rel, err := filepath.Rel(g.root, absPath)
		if err != nil {
			return nil
		}
		slashRel := "/" + filepath.ToSlash(rel)
		dir, name := PathParts(slashRel)
		ext := strings.ToLower(filepath.Ext(name))

		info, err := d.Info()
		if err != nil {
			return nil
		}

		if dbErr := dbImageUpsert(g.db, Image{
			Dir:       dir,
			Name:      name,
			Ext:       ext,
			Size:      info.Size(),
			CreatedAt: info.ModTime(),
			UpdatedAt: info.ModTime(),
		}); dbErr != nil {
			return nil // best-effort: skip on error
		}
		count++
		return nil
	})
	return count, err
}

// ListImages returns images ordered by updated_at DESC.
// beforeNs is a pagination cursor (Unix nanoseconds); 0 means start from the most recent.
func (g *Vault) ListImages(beforeNs int64, limit int) ([]Image, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return dbImageListPaged(g.db, beforeNs, limit)
}

// CountImages returns the total number of images in the vault.
func (g *Vault) CountImages() (int, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return dbImageCount(g.db)
}

// NoteNamesForImages batch-resolves, for each image filename in names, the
// basenames (without ".md") of every note that currently embeds it via
// ![[name]] — backed by note_assets' kind=image rows (see ParseImageEmbeds
// and the assets plugin, which keeps them current on every save).
func (g *Vault) NoteNamesForImages(names []string) (map[string][]string, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return dbNoteNamesByAssetFilenames(g.db, names, AssetKindImage)
}

// ParseImageEmbeds returns the ordered, deduplicated list of image filenames
// referenced via Obsidian-style ![[image.ext]] (or ![[image.ext|hint]]) wiki
// embeds in note content. Used both by BackfillBodyImageLinks (vault-wide,
// at startup) and by the assets plugin's per-save extractor
// (internal/plugins/assets), which is what keeps note_assets' kind=image
// rows current day to day.
func ParseImageEmbeds(data []byte) []string {
	if !bytes.Contains(data, []byte("![[")) {
		return nil
	}
	var out []string
	seen := map[string]bool{}
	for _, m := range wikiImageLinkRe.FindAllSubmatch(data, -1) {
		if len(m) < 2 {
			continue
		}
		target := filepath.Base(strings.TrimSpace(string(m[1])))
		if !IsImagePath(target) || seen[target] {
			continue
		}
		seen[target] = true
		out = append(out, target)
	}
	return out
}

// BackfillBodyImageLinks walks every markdown note in the vault and
// (re)writes its note_assets rows of kind=image to match the ![[image]]
// embeds currently in its body — a global reconciliation pass, run once at
// startup/init (see internal/cli/start.go, init.go) to pick up notes that
// predate this feature or haven't been saved since. Day-to-day updates are
// kept current incrementally by the assets plugin's extractBodyImages,
// which runs on every save and (via ReplaceNoteAssets) correctly handles
// embeds a note has since removed too — this backfill exists only to catch
// notes the plugin hasn't seen yet.
//
// The walk itself holds no lock (read-only); each note's ReplaceNoteAssets
// call takes the lock individually, mirroring how notes are registered in
// ScanAndRegisterFull.
func (g *Vault) BackfillBodyImageLinks() error {
	type noteImages struct {
		path   Path
		images []string
	}
	var notes []noteImages

	err := filepath.WalkDir(g.root, func(absPath string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.ToLower(filepath.Ext(d.Name())) != ".md" {
			return nil
		}

		data, err := os.ReadFile(absPath)
		if err != nil {
			return nil
		}

		rel, err := filepath.Rel(g.root, absPath)
		if err != nil {
			return nil
		}
		dir, name := PathParts("/" + filepath.ToSlash(rel))

		notes = append(notes, noteImages{path: Path(JoinPath(dir, name)), images: ParseImageEmbeds(data)})
		return nil
	})
	if err != nil {
		return fmt.Errorf("storage: walk notes for image links: %w", err)
	}

	for _, n := range notes {
		assets := make([]NoteAsset, len(n.images))
		for i, name := range n.images {
			assets[i] = NoteAsset{Kind: AssetKindImage, Filename: name, Ord: i}
		}
		if err := g.ReplaceNoteAssets(n.path, AssetKindImage, assets); err != nil {
			return fmt.Errorf("storage: backfill image links for %q: %w", n.path, err)
		}
	}
	return nil
}

// OsImagePath resolves the vault-relative Image back to an absolute OS path.
func (g *Vault) OsImagePath(img Image) string {
	rel := img.Dir
	if rel == "/" {
		rel = ""
	}
	return filepath.Join(g.root, filepath.FromSlash(rel), img.Name)
}

// ExtFromMIME returns the canonical image extension for a Content-Type, or "".
func ExtFromMIME(ct string) string {
	ct = strings.ToLower(strings.SplitN(ct, ";", 2)[0])
	return imageExtByMIME[ct]
}

// ImageContentType returns the MIME type for an image file extension.
func ImageContentType(ext string) string {
	if ct, ok := imageMIMEByExt[strings.ToLower(ext)]; ok {
		return ct
	}
	return "application/octet-stream"
}

// CheckImageMagic performs a minimal magic-bytes check for common formats.
func CheckImageMagic(data []byte, ext string) bool {
	switch strings.ToLower(ext) {
	case ".png":
		return len(data) >= 4 && data[0] == 0x89 && data[1] == 'P' && data[2] == 'N' && data[3] == 'G'
	case ".jpg", ".jpeg":
		return len(data) >= 2 && data[0] == 0xFF && data[1] == 0xD8
	case ".gif":
		return len(data) >= 3 && data[0] == 'G' && data[1] == 'I' && data[2] == 'F'
	case ".webp":
		return len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP"
	case ".avif":
		return len(data) >= 12 && string(data[4:8]) == "ftyp"
	default:
		return true
	}
}

// DetectImageExt prefers the real format from magic bytes, then Content-Type,
// then filename. Formats without a reliable magic check (svg, avif) rely on
// the latter two.
func DetectImageExt(data []byte, contentType, filename string) string {
	switch {
	case CheckImageMagic(data, ".png"):
		return ".png"
	case CheckImageMagic(data, ".jpg"):
		return ".jpg"
	case CheckImageMagic(data, ".gif"):
		return ".gif"
	case CheckImageMagic(data, ".webp"):
		return ".webp"
	}
	if ext := ExtFromMIME(contentType); ext != "" {
		return ext
	}
	if ext := strings.ToLower(filepath.Ext(filename)); imageMIMEByExt[ext] != "" {
		return ext
	}
	return ""
}

// SaveImageBytes writes an image into _assets/YYYYMM/{unixms}-{rand8}{ext}
// and registers it in the images table. The file is removed again if the
// metadata row cannot be written, so a saved image is always servable.
func (g *Vault) SaveImageBytes(data []byte, ext string) (Image, error) {
	return g.saveImage(data, "", ext)
}

// SaveImageBytesNamed is SaveImageBytes with a caller-chosen basename stem, for
// images whose filename must be reproducible (e.g. derived from a source URL).
func (g *Vault) SaveImageBytesNamed(data []byte, stem, ext string) (Image, error) {
	if stem == "" || strings.ContainsAny(stem, `/\.`) {
		return Image{}, fmt.Errorf("%w: invalid image name %q", ErrInvalidImageRef, stem)
	}
	return g.saveImage(data, stem, ext)
}

func (g *Vault) saveImage(data []byte, stem, ext string) (Image, error) {
	ext = strings.ToLower(ext)
	if ext != "" && !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	if !imageExtensions[ext] {
		return Image{}, fmt.Errorf("%w: unsupported image type %q", ErrInvalidImageRef, ext)
	}
	if len(data) == 0 {
		return Image{}, fmt.Errorf("%w: empty image", ErrInvalidImageRef)
	}
	if len(data) > MaxImageBytes {
		return Image{}, fmt.Errorf("%w: image exceeds %d bytes", ErrInvalidImageRef, MaxImageBytes)
	}
	if !CheckImageMagic(data, ext) {
		return Image{}, fmt.Errorf("%w: file bytes do not match declared image type", ErrInvalidImageRef)
	}

	month := time.Now().UTC().Format("200601")
	if stem == "" {
		var randBuf [4]byte
		_, _ = rand.Read(randBuf[:])
		stem = fmt.Sprintf("%d-%s", time.Now().UnixMilli(), hex.EncodeToString(randBuf[:]))
	}
	name := stem + ext

	absDir := filepath.Join(g.root, "_assets", month)
	if err := os.MkdirAll(absDir, 0o750); err != nil {
		return Image{}, fmt.Errorf("storage: mkdir assets: %w", err)
	}
	absPath := filepath.Join(absDir, name)
	if err := os.WriteFile(absPath, data, 0o644); err != nil {
		return Image{}, fmt.Errorf("storage: write image: %w", err)
	}

	img := Image{
		Dir:  "/_assets/" + month,
		Name: name,
		Ext:  ext,
		Size: int64(len(data)),
	}
	if err := g.RegisterImage(img); err != nil {
		_ = os.Remove(absPath)
		return Image{}, fmt.Errorf("storage: register image: %w", err)
	}
	return img, nil
}
