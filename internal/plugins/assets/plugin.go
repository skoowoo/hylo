package assets

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/hardhacker/hylo/internal/plugin"
	"github.com/hardhacker/hylo/internal/storage"
	"github.com/hardhacker/hylo/internal/util"
)

// Plugin extracts note-attached resources on create/write.
// Currently resolves cover images; later kinds (audio, video, body images)
// can be added as extra extractors without changing the note_assets table.
type Plugin struct {
	vault   *storage.Vault
	logger  *slog.Logger
	eventCh chan plugin.Event
	client  *http.Client
}

func New(vault *storage.Vault, logger *slog.Logger) *Plugin {
	return &Plugin{
		vault:   vault,
		logger:  logger,
		eventCh: make(chan plugin.Event, 256),
		client:  newFetchClient(),
	}
}

func (p *Plugin) Name() string { return "assets" }

func (p *Plugin) Notify(e plugin.Event) {
	switch e.Type {
	case plugin.EventFSCreate, plugin.EventFSWrite,
		plugin.EventVaultCreate, plugin.EventVaultWrite:
		select {
		case p.eventCh <- e:
		default:
			p.logger.Warn("assets: event channel full, dropping event",
				"type", e.Type, "path", e.Path)
		}
	}
}

func (p *Plugin) Start(ctx context.Context) error {
	p.logger.Info("assets: plugin started")
	for {
		select {
		case <-ctx.Done():
			return nil
		case e := <-p.eventCh:
			p.handle(e)
		}
	}
}

func (p *Plugin) Stop() error { return nil }

func (p *Plugin) handle(e plugin.Event) {
	if !strings.HasSuffix(strings.ToLower(e.Path), ".md") {
		return
	}
	pathStr := e.Path
	if !strings.HasPrefix(pathStr, "/") {
		pathStr = "/" + pathStr
	}
	if strings.HasPrefix(pathStr, "/_assets/") {
		return
	}

	sp, ok := storage.ParsePath(pathStr)
	if !ok {
		return
	}

	note, err := p.vault.StatNote(sp)
	if err != nil || note.Kind == storage.KindShort {
		return
	}

	raw, err := p.vault.ReadNote(sp)
	if err != nil {
		p.logger.Debug("assets: read failed", "path", pathStr, "err", err)
		return
	}
	if util.IsShortNote(raw) {
		return
	}

	p.extractCover(sp, pathStr, raw)
}

func (p *Plugin) extractCover(sp storage.Path, pathStr string, raw []byte) {
	var cover []storage.NoteAsset
	for _, src := range parseCoverSources(raw) {
		filename, sourceURL, ok := p.resolveCover(pathStr, src)
		if !ok {
			continue
		}
		cover = []storage.NoteAsset{{
			Kind:      storage.AssetKindCover,
			Filename:  filename,
			SourceURL: sourceURL,
		}}
		break
	}

	if err := p.vault.ReplaceNoteAssets(sp, storage.AssetKindCover, cover); err != nil {
		p.logger.Warn("assets: set cover failed", "path", pathStr, "err", err)
		return
	}
	if len(cover) > 0 {
		p.logger.Info("assets: cover saved", "path", pathStr, "cover", cover[0].Filename)
	}
}

// resolveCover turns a candidate into a servable image filename. Local refs
// must already be registered, otherwise the card would show a broken image.
func (p *Plugin) resolveCover(pathStr string, src coverSource) (filename, sourceURL string, ok bool) {
	if src.local != "" {
		imgs, err := p.vault.GetImagesByName(src.local)
		if err != nil || len(imgs) == 0 {
			p.logger.Debug("assets: cover image not in vault", "path", pathStr, "name", src.local)
			return "", "", false
		}
		return src.local, "", true
	}

	img, err := p.fetchRemote(src.remote)
	if err != nil {
		p.logger.Warn("assets: cover fetch failed", "path", pathStr, "url", src.remote, "err", err)
		return "", "", false
	}
	return img.Name, src.remote, true
}

func (p *Plugin) fetchRemote(rawURL string) (storage.Image, error) {
	stem := remoteStem(rawURL)
	if img, ok := p.findRemote(stem); ok {
		return img, nil
	}

	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return storage.Image{}, err
	}
	req.Header.Set("User-Agent", "Hylo/1.0")
	req.Header.Set("Accept", "image/*,*/*;q=0.8")

	res, err := p.client.Do(req)
	if err != nil {
		return storage.Image{}, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return storage.Image{}, fmt.Errorf("http %d", res.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(res.Body, storage.MaxImageBytes+1))
	if err != nil {
		return storage.Image{}, err
	}
	if len(data) > storage.MaxImageBytes {
		return storage.Image{}, fmt.Errorf("image exceeds %d bytes", storage.MaxImageBytes)
	}

	ext := storage.DetectImageExt(data, res.Header.Get("Content-Type"), rawURL)
	switch ext {
	case "":
		return storage.Image{}, fmt.Errorf("unsupported image type")
	case ".svg":
		// Untrusted markup; never pull it into the vault automatically.
		return storage.Image{}, fmt.Errorf("remote svg not allowed")
	case ".jpeg":
		ext = ".jpg"
	}
	return p.vault.SaveImageBytesNamed(data, stem, ext)
}

// findRemote returns an earlier download of the same URL, if still registered.
func (p *Plugin) findRemote(stem string) (storage.Image, bool) {
	for _, ext := range remoteExts {
		if imgs, err := p.vault.GetImagesByName(stem + ext); err == nil && len(imgs) > 0 {
			return imgs[0], true
		}
	}
	return storage.Image{}, false
}
