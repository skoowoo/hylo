package urlembed

import (
	"bufio"
	"context"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html/charset"
	"golang.org/x/net/idna"

	"github.com/hardhacker/hylo/internal/storage"
)

const (
	kindPage  = "page"
	kindAudio = "audio"
	kindVideo = "video"
	kindImage = "image"
	kindFile  = "file"

	pageTTL  = 7 * 24 * time.Hour
	mediaTTL = 30 * 24 * time.Hour
	errorTTL = time.Hour

	// An ok row unused this long past expiry is dropped, playback position with it.
	staleKeep  = 90 * 24 * time.Hour
	pruneEvery = time.Hour

	maxBody  = 1 << 20
	sniffLen = 4096

	// MaxURLs is the most URLs one /api/embeds request may carry.
	MaxURLs = 20
)

// Result is one URL as the card asked for it. URL is the request string so
// the client can match the response; the cache key is the canonical form.
type Result struct {
	URL         string `json:"url"`
	FinalURL    string `json:"final_url,omitempty"`
	Kind        string `json:"kind,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	SiteName    string `json:"site_name,omitempty"`
	ImageURL    string `json:"image_url,omitempty"`
	Status      string `json:"status"`
	Error       string `json:"error,omitempty"`
	PositionMS  int64  `json:"position_ms,omitempty"`
}

// Resolver fetches and caches URL classifications.
type Resolver struct {
	vault        *storage.Vault
	client       *http.Client
	now          func() time.Time
	allowPrivate bool

	mu        sync.Mutex
	inflight  map[string]*flight
	lastPrune time.Time
}

type flight struct {
	done   chan struct{}
	result Result
}

// NewResolver resolves URLs for vault, refusing private and loopback targets.
// env supplies the proxy settings; nil fetches directly.
func NewResolver(v *storage.Vault, env func() []string) *Resolver {
	return &Resolver{
		vault:    v,
		client:   safeClient(env),
		now:      time.Now,
		inflight: map[string]*flight{},
	}
}

// ResolveEach emits each URL's result as soon as it is known: cached rows at
// once, the rest as their fetches finish, so a slow site holds back only its
// own card. emit is never called concurrently. With retry a cached failure is
// fetched again.
func (r *Resolver) ResolveEach(ctx context.Context, urls []string, retry bool, emit func(Result)) {
	canons := make([]string, len(urls))
	keys := make([]string, 0, len(urls))
	for i, raw := range urls {
		if c, err := canonical(raw); err == nil {
			canons[i] = c
			keys = append(keys, c)
		}
	}
	rows, _ := r.vault.FreshURLEmbeds(keys, r.now().UnixMilli())

	var mu sync.Mutex
	send := func(res Result) {
		mu.Lock()
		defer mu.Unlock()
		emit(res)
	}
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for i, raw := range urls {
		if canons[i] == "" {
			send(Result{URL: raw, Status: "error", Error: "invalid_url"})
			continue
		}
		if row, ok := rows[canons[i]]; ok && usable(row, retry) {
			send(resultFrom(raw, row))
			continue
		}
		wg.Add(1)
		go func(raw string) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			send(r.resolve(ctx, raw, retry))
		}(raw)
	}
	wg.Wait()
}

// Resolve returns the cached row or fetches it. A cancelled ctx stops the wait;
// a fetch already started still finishes and is cached.
func (r *Resolver) Resolve(ctx context.Context, raw string) Result {
	return r.resolve(ctx, raw, false)
}

func (r *Resolver) resolve(ctx context.Context, raw string, retry bool) Result {
	canon, err := canonical(raw)
	if err != nil {
		return Result{URL: raw, Status: "error", Error: "invalid_url"}
	}
	if row, ok := r.fresh(canon); ok && usable(row, retry) {
		return resultFrom(raw, row)
	}
	f, leader := r.join(canon)
	if leader {
		if row, ok := r.fresh(canon); ok && usable(row, retry) {
			r.finishEarly(canon, f, resultFrom(canon, row))
		} else {
			go r.fetchInto(canon, f)
		}
	}
	select {
	case <-ctx.Done():
		return Result{URL: raw, Status: "error", Error: "fetch_failed"}
	case <-f.done:
		res := f.result
		res.URL = raw
		return res
	}
}

func (r *Resolver) fresh(canon string) (storage.URLEmbed, bool) {
	rows, err := r.vault.FreshURLEmbeds([]string{canon}, r.now().UnixMilli())
	if err != nil {
		return storage.URLEmbed{}, false
	}
	row, ok := rows[canon]
	return row, ok
}

func usable(row storage.URLEmbed, retry bool) bool {
	return !retry || row.Status == "ok"
}

func (r *Resolver) join(canon string) (*flight, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if f, ok := r.inflight[canon]; ok {
		return f, false
	}
	f := &flight{done: make(chan struct{})}
	r.inflight[canon] = f
	return f, true
}

func (r *Resolver) finishEarly(canon string, f *flight, res Result) {
	r.mu.Lock()
	f.result = res
	delete(r.inflight, canon)
	r.mu.Unlock()
	close(f.done)
}

func (r *Resolver) fetchInto(canon string, f *flight) {
	res := r.fetch(canon)
	r.mu.Lock()
	f.result = res
	delete(r.inflight, canon)
	r.mu.Unlock()
	close(f.done)
}

func (r *Resolver) fetch(canon string) Result {
	u, err := url.Parse(canon)
	if err != nil {
		return Result{URL: canon, Status: "error", Error: "invalid_url"}
	}
	if !r.allowPrivate {
		if err := validateURL(u); err != nil {
			return r.store(errRow(canon, "blocked"))
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, canon, nil)
	if err != nil {
		return r.store(errRow(canon, "invalid_url"))
	}
	req.Header.Set("User-Agent", "Hylo/1.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml;q=0.9,*/*;q=0.8")
	resp, err := r.client.Do(req)
	if err != nil {
		if !r.allowPrivate && errors.Is(err, errBlocked) {
			return r.store(errRow(canon, "blocked"))
		}
		return r.fail(canon)
	}
	defer resp.Body.Close()
	final := resp.Request.URL
	if final == nil {
		final = u
	}
	switch {
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone:
		return r.store(errRow(canon, "fetch_failed"))
	case resp.StatusCode < 200 || resp.StatusCode >= 300:
		return r.fail(canon)
	}
	header := resp.Header.Get("Content-Type")
	ct := mediaType(header)
	kind := kindFromType(ct)
	if kind == "" {
		kind = kindFromExt(final.Path)
	}
	row := storage.URLEmbed{
		URL:         canon,
		FinalURL:    final.String(),
		ContentType: ct,
		Status:      "ok",
		FetchedAt:   r.now().UnixMilli(),
	}
	if kind == "" || kind == kindPage {
		body := bufio.NewReaderSize(io.LimitReader(resp.Body, maxBody), sniffLen)
		if kind == "" {
			head, err := body.Peek(sniffLen)
			if err != nil && !errors.Is(err, io.EOF) {
				return r.fail(canon)
			}
			kind = kindFile
			if looksLikeHTML(head) {
				kind = kindPage
			}
		}
		if kind == kindPage {
			text, err := charset.NewReader(body, header)
			if err != nil {
				return r.fail(canon)
			}
			row.Title, row.Description, row.SiteName, row.ImageURL = parseOG(text, final)
		}
	}
	// Only the headers or the <head> were needed. Cancel so Close drops the
	// connection instead of draining the rest of the body.
	cancel()
	row.Kind = kind
	row.ExpiresAt = r.now().Add(ttl(row)).UnixMilli()
	return r.store(row)
}

// SetPosition stores playback for an audio or video URL. Metadata refreshes keep it.
func (r *Resolver) SetPosition(raw string, ms int64) error {
	canon, err := canonical(raw)
	if err != nil {
		return err
	}
	return r.vault.SetURLEmbedPosition(canon, ms)
}

func (r *Resolver) store(row storage.URLEmbed) Result {
	if row.FetchedAt == 0 {
		now := r.now()
		row.FetchedAt = now.UnixMilli()
		row.ExpiresAt = now.Add(ttl(row)).UnixMilli()
	}
	_ = r.vault.PutURLEmbed(row)
	r.prune()
	return resultFrom(row.URL, row)
}

func (r *Resolver) prune() {
	now := r.now()
	r.mu.Lock()
	due := now.Sub(r.lastPrune) >= pruneEvery
	if due {
		r.lastPrune = now
	}
	r.mu.Unlock()
	if due {
		_ = r.vault.PruneURLEmbeds(now.UnixMilli(), now.Add(-staleKeep).UnixMilli())
	}
}

// fail is a failure that may pass. A row that was good is kept, so its card
// survives a short outage; either way it is retried after errorTTL.
func (r *Resolver) fail(canon string) Result {
	row, ok, err := r.vault.ExtendURLEmbed(canon, r.now().Add(errorTTL).UnixMilli())
	if err == nil && ok {
		return resultFrom(canon, row)
	}
	return r.store(errRow(canon, "fetch_failed"))
}

func errRow(canon, code string) storage.URLEmbed {
	return storage.URLEmbed{URL: canon, FinalURL: canon, Status: "error", Error: code}
}

func ttl(row storage.URLEmbed) time.Duration {
	if row.Status != "ok" {
		return errorTTL
	}
	if row.Kind == kindPage {
		return pageTTL
	}
	return mediaTTL
}

func resultFrom(request string, row storage.URLEmbed) Result {
	return Result{
		URL:         request,
		FinalURL:    row.FinalURL,
		Kind:        row.Kind,
		ContentType: row.ContentType,
		Title:       row.Title,
		Description: row.Description,
		SiteName:    row.SiteName,
		ImageURL:    row.ImageURL,
		Status:      row.Status,
		Error:       row.Error,
		PositionMS:  row.PositionMS,
	}
}

func canonical(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Hostname() == "" || u.User != nil {
		return "", errBlocked
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", errBlocked
	}
	host := strings.TrimSuffix(u.Hostname(), ".")
	if ip := net.ParseIP(host); ip != nil {
		host = ip.String()
	} else {
		var idnaErr error
		host, idnaErr = idna.Lookup.ToASCII(host)
		if idnaErr != nil || host == "" {
			return "", errBlocked
		}
		host = strings.ToLower(host)
	}
	port := u.Port()
	defaultPort := (u.Scheme == "http" && port == "80") || (u.Scheme == "https" && port == "443")
	switch {
	case port != "" && !defaultPort:
		u.Host = net.JoinHostPort(host, port)
	case strings.Contains(host, ":"):
		u.Host = "[" + host + "]"
	default:
		u.Host = host
	}
	u.Fragment = ""
	if u.Path == "" {
		u.Path = "/"
	}
	return u.String(), nil
}

func mediaType(v string) string {
	mt, _, err := mime.ParseMediaType(v)
	if err != nil {
		return ""
	}
	return strings.ToLower(mt)
}

func kindFromType(mt string) string {
	switch {
	case mt == "text/html" || mt == "application/xhtml+xml":
		return kindPage
	case strings.HasPrefix(mt, "audio/"):
		return kindAudio
	case strings.HasPrefix(mt, "video/"):
		return kindVideo
	case strings.HasPrefix(mt, "image/"):
		return kindImage
	case mt == "" || mt == "application/octet-stream" || mt == "binary/octet-stream" || mt == "application/binary":
		return ""
	default:
		return kindFile
	}
}

func kindFromExt(p string) string {
	switch strings.ToLower(path.Ext(p)) {
	case ".mp3", ".m4a", ".aac", ".wav", ".ogg", ".flac", ".opus":
		return kindAudio
	case ".mp4", ".webm", ".mov", ".m4v":
		return kindVideo
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".avif":
		return kindImage
	case ".html", ".htm":
		return kindPage
	default:
		return ""
	}
}
