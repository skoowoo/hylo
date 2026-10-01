// Package client provides an HTTP client for the Hylo server API.
// It uses TCP transport to communicate with the server.
package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/hardhacker/hylo/internal/agent"
	"github.com/hardhacker/hylo/internal/config"
	"github.com/hardhacker/hylo/internal/storage"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// Client is an HTTP client that communicates with the Hylo server.
type Client struct {
	http    *http.Client
	baseURL string
	apiKey  string
}

// DirListing is the JSON response body returned by POST /api/vault/list.
type DirListing struct {
	Path  string         `json:"path"`
	Notes []storage.Note `json:"notes"`
	All   bool           `json:"all,omitempty"`
}

// New creates a Client connected to the running Hylo server using TCP.
// If the TCP transport is not enabled an actionable error is returned.
func New(cfg *config.Config) (*Client, error) {
	if cfg.Server.TCPEnabled() {
		protocol := "http://"
		if cfg.Server.TLSEnabled() {
			protocol = "https://"
		}
		return &Client{
			http:    &http.Client{Timeout: 30 * time.Second},
			baseURL: protocol + cfg.Server.TCPAddr(),
			apiKey:  cfg.Server.APIKey,
		}, nil
	}

	return nil, fmt.Errorf(
		"cannot connect to hylo server\n" +
			"  TCP: not configured\n\n" +
			"Start the server first: hylo serve",
	)
}

// ── Vault operations ──────────────────────────────────────────────────────────

// noteReadBody builds the JSON body for /api/vault/read.
// Arguments that contain "/" are treated as a vault-relative path (exact lookup);
// the path is normalised to start with "/" if needed.
// Arguments with no "/" are treated as a bare filename (vault-wide auto-resolve).
func noteReadBody(pathOrName string) map[string]any {
	if strings.Contains(pathOrName, "/") {
		p := pathOrName
		if !strings.HasPrefix(p, "/") {
			p = "/" + p
		}
		return map[string]any{"path": p}
	}
	return map[string]any{"name": pathOrName}
}

// Exists reports whether a node exists at pathOrName.
// Returns (false, nil) for a clean "not found"; other errors indicate real failures.
func (c *Client) Exists(pathOrName string) (bool, error) {
	resp, err := c.postJSON(c.baseURL+"/api/vault/read", noteReadBody(pathOrName))
	if err != nil {
		return false, wrapConnErr(err)
	}
	io.Copy(io.Discard, resp.Body) //nolint:errcheck
	resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, fmt.Errorf("stat %q: server returned %s", pathOrName, resp.Status)
	}
}

// ReadFile returns an io.ReadCloser for the note identified by pathOrName.
// Pass an absolute path (starting with "/") for exact access, or a bare
// filename for vault-wide auto-resolve. The caller is responsible for closing it.
func (c *Client) ReadFile(pathOrName string) (io.ReadCloser, error) {
	resp, err := c.postJSON(c.baseURL+"/api/vault/read", noteReadBody(pathOrName))
	if err != nil {
		return nil, wrapConnErr(err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("read %q: %s", pathOrName, statusMsg(resp.StatusCode, body))
	}
	return resp.Body, nil
}

// ListDir returns notes directly inside dir according to opts.
func (c *Client) ListDir(dir string, opts storage.ListOptions) ([]storage.Note, error) {
	return c.listNotes(map[string]any{"path": dir}, fmt.Sprintf("list %q", dir), opts)
}

// ListAllNotes returns every note in the vault.
func (c *Client) ListAllNotes(opts storage.ListOptions) ([]storage.Note, error) {
	return c.listNotes(map[string]any{"all": true}, "list all", opts)
}

// listNotes sends a POST /api/vault/list request with the given body and opts,
// then decodes and returns the note slice.
func (c *Client) listNotes(body map[string]any, label string, opts storage.ListOptions) ([]storage.Note, error) {
	if opts.SortByTime {
		body["sort"] = "time"
	}
	if opts.Limit > 0 {
		body["limit"] = opts.Limit
	}
	if !opts.After.IsZero() {
		body["start"] = opts.After.Format(time.DateOnly)
	}
	if !opts.Before.IsZero() {
		body["end"] = opts.Before.Format(time.DateOnly)
	}
	if len(opts.OnlyKinds) > 0 {
		strs := make([]string, len(opts.OnlyKinds))
		for i, k := range opts.OnlyKinds {
			strs[i] = string(k)
		}
		body["kinds"] = strs
	}
	if len(opts.ExcludeKinds) > 0 {
		strs := make([]string, len(opts.ExcludeKinds))
		for i, k := range opts.ExcludeKinds {
			strs[i] = string(k)
		}
		body["exclude_kinds"] = strs
	}
	resp, err := c.postJSON(c.baseURL+"/api/vault/list", body)
	if err != nil {
		return nil, wrapConnErr(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("%s: %s", label, statusMsg(resp.StatusCode, raw))
	}
	var listing DirListing
	if err := json.NewDecoder(resp.Body).Decode(&listing); err != nil {
		return nil, fmt.Errorf("%s: decode response: %w", label, err)
	}
	if listing.Notes == nil {
		listing.Notes = []storage.Note{}
	}
	return listing.Notes, nil
}

// StatNote returns DB-backed metadata for a note at relPath (vault-absolute).
// Path normalization matches read: a filename without an extension defaults to ".md".
func (c *Client) StatNote(relPath string) (storage.Note, error) {
	p := relPath
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	resp, err := c.postJSON(c.baseURL+"/api/vault/stat", map[string]any{"path": p})
	if err != nil {
		return storage.Note{}, wrapConnErr(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return storage.Note{}, fmt.Errorf("stat %q: %s", relPath, statusMsg(resp.StatusCode, raw))
	}
	var note storage.Note
	if err := json.NewDecoder(resp.Body).Decode(&note); err != nil {
		return storage.Note{}, fmt.Errorf("stat %q: decode response: %w", relPath, err)
	}
	return note, nil
}

// NoteResolveResponse is the JSON body from POST /api/notes/resolve.
type NoteResolveResponse struct {
	Name    string         `json:"name"`
	Matches []storage.Note `json:"matches"`
	Count   int            `json:"count"`
}

// ResolveNoteName resolves a bare filename (e.g. "today.md") to all vault locations
// where a note with that name exists. Use storage.Note.PathString() to build the read path.
func (c *Client) ResolveNoteName(name string) (NoteResolveResponse, error) {
	resp, err := c.postJSON(c.baseURL+"/api/notes/resolve", map[string]any{"name": name})
	if err != nil {
		return NoteResolveResponse{}, wrapConnErr(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return NoteResolveResponse{}, fmt.Errorf("resolve %q: %s", name, statusMsg(resp.StatusCode, body))
	}
	var out NoteResolveResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return NoteResolveResponse{}, fmt.Errorf("resolve %q: decode: %w", name, err)
	}
	if out.Matches == nil {
		out.Matches = []storage.Note{}
	}
	return out, nil
}

// WriteFile creates or overwrites the file at relPath with data.
func (c *Client) WriteFile(relPath string, data []byte) error {
	return c.write(relPath, data, false)
}

// AppendFile appends data to the file at relPath (creating it if absent).
// When heading is non-empty the content is inserted after the last matching
// section; an empty heading appends to the end of the file.
func (c *Client) AppendFile(relPath string, data []byte, heading string) error {
	return c.writeRaw(relPath, data, "append", heading)
}

// PrependFile inserts data into the file at relPath after the first H1.
// When heading is non-empty the content is inserted at the start of the first
// matching section; an empty heading uses the default MdPrepend behaviour.
func (c *Client) PrependFile(relPath string, data []byte, heading string) error {
	return c.writeRaw(relPath, data, "prepend", heading)
}

func (c *Client) write(relPath string, data []byte, appendMode bool) error {
	return c.writeRaw(relPath, data, "", "")
}

// writeRaw sends a write request. mode is "append", "prepend", or "" (replace).
func (c *Client) writeRaw(relPath string, data []byte, mode, section string) error {
	body := map[string]any{
		"path":    relPath,
		"content": string(data),
		"append":  mode == "append",
		"prepend": mode == "prepend",
	}
	if section != "" {
		body["section"] = section
	}
	resp, err := c.postJSON(c.baseURL+"/api/vault/write", body)
	if err != nil {
		return wrapConnErr(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("write %q: %s", relPath, statusMsg(resp.StatusCode, raw))
	}
	io.Copy(io.Discard, resp.Body) //nolint:errcheck
	return nil
}


// DeleteNote permanently deletes the note at relPath.
func (c *Client) DeleteNote(relPath string) error {
	resp, err := c.postJSON(c.baseURL+"/api/vault/delete", map[string]any{"path": relPath})
	if err != nil {
		return wrapConnErr(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete %q: %s", relPath, statusMsg(resp.StatusCode, body))
	}
	return nil
}

// MoveNote moves the note at relPath to the same-named file under newDir,
// returning its new vault-absolute path.
func (c *Client) MoveNote(relPath, newDir string) (string, error) {
	resp, err := c.postJSON(c.baseURL+"/api/vault/move", map[string]any{"path": relPath, "newDir": newDir})
	if err != nil {
		return "", wrapConnErr(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("move %q: read response: %w", relPath, err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("move %q: %s", relPath, statusMsg(resp.StatusCode, body))
	}
	var out struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", fmt.Errorf("move %q: decode response: %w", relPath, err)
	}
	return out.Path, nil
}

// RenameNote renames the note at relPath to newName, keeping it in the same
// directory (use MoveNote for a directory change). Returns the note's new
// vault-absolute path and the id of the asynchronous rename job that fixes up
// vault-wide wikilink/source_notes references to the old name (0 if it could
// not be enqueued) — see RenameStatus.
func (c *Client) RenameNote(relPath, newName string) (string, int64, error) {
	resp, err := c.postJSON(c.baseURL+"/api/vault/rename", map[string]any{"path": relPath, "newName": newName})
	if err != nil {
		return "", 0, wrapConnErr(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, fmt.Errorf("rename %q: read response: %w", relPath, err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("rename %q: %s", relPath, statusMsg(resp.StatusCode, body))
	}
	var out struct {
		Path        string `json:"path"`
		RenameJobID int64  `json:"renameJobId"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", 0, fmt.Errorf("rename %q: decode response: %w", relPath, err)
	}
	return out.Path, out.RenameJobID, nil
}

// RenameJobStatus mirrors the JSON body of GET /api/vault/rename-status.
type RenameJobStatus struct {
	Status       string `json:"status"`
	Total        int    `json:"total"`
	Done         int    `json:"done"`
	UpdatedCount int    `json:"updatedCount"`
	Error        string `json:"error"`
}

// RenameStatus fetches the progress of the asynchronous sweep enqueued by RenameNote.
func (c *Client) RenameStatus(jobID int64) (RenameJobStatus, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/api/vault/rename-status?id=%d", c.baseURL, jobID), nil)
	if err != nil {
		return RenameJobStatus{}, err
	}
	resp, err := c.do(req)
	if err != nil {
		return RenameJobStatus{}, wrapConnErr(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return RenameJobStatus{}, fmt.Errorf("rename status %d: read response: %w", jobID, err)
	}
	if resp.StatusCode != http.StatusOK {
		return RenameJobStatus{}, fmt.Errorf("rename status %d: %s", jobID, statusMsg(resp.StatusCode, body))
	}
	var out RenameJobStatus
	if err := json.Unmarshal(body, &out); err != nil {
		return RenameJobStatus{}, fmt.Errorf("rename status %d: decode response: %w", jobID, err)
	}
	return out, nil
}

// ShortEntry mirrors storage.ShortEntry for JSON decoding.
type ShortEntry struct {
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	DailyPath string    `json:"daily_path"`
}

// ShortListOptions controls filtering for Client.ListShorts.
type ShortListOptions struct {
	Dir    string
	After  time.Time
	Before time.Time
	Limit  int
}

// ListShorts returns individual short note entries matching opts, newest first.
func (c *Client) ListShorts(opts ShortListOptions) ([]ShortEntry, error) {
	body := map[string]any{}
	if opts.Dir != "" {
		body["dir"] = opts.Dir
	}
	if !opts.After.IsZero() {
		body["start"] = opts.After.Format(time.DateOnly)
	}
	if !opts.Before.IsZero() {
		body["end"] = opts.Before.Format(time.DateOnly)
	}
	if opts.Limit > 0 {
		body["limit"] = opts.Limit
	}
	resp, err := c.postJSON(c.baseURL+"/api/vault/shorts/list", body)
	if err != nil {
		return nil, wrapConnErr(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("short list: %s", statusMsg(resp.StatusCode, raw))
	}
	var out struct {
		Entries []ShortEntry `json:"entries"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("short list: decode response: %w", err)
	}
	return out.Entries, nil
}

// CreateShortResponse is the JSON body returned by POST /api/vault/shorts.
type CreateShortResponse = storage.Note

// CreateShort appends a short note to today's daily file in the shorts directory.
// dir overrides the default "_shorts" directory when non-empty.
func (c *Client) CreateShort(content, dir string) (storage.Note, error) {
	body := map[string]any{"content": content}
	if dir != "" {
		body["dir"] = dir
	}
	resp, err := c.postJSON(c.baseURL+"/api/vault/shorts", body)
	if err != nil {
		return storage.Note{}, wrapConnErr(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return storage.Note{}, fmt.Errorf("short: %s", statusMsg(resp.StatusCode, raw))
	}
	var note storage.Note
	if err := json.NewDecoder(resp.Body).Decode(&note); err != nil {
		return storage.Note{}, fmt.Errorf("short: decode response: %w", err)
	}
	return note, nil
}

// ── Search ────────────────────────────────────────────────────────────────────

// SearchResult mirrors search.SearchResult for JSON decoding.
type SearchResult struct {
	Dir       string    `json:"dir"`
	Name      string    `json:"name"`
	Kind      string    `json:"kind"`
	UpdatedAt time.Time `json:"updated_at"`
	Score     float64   `json:"score"`
	Lines     []int     `json:"lines,omitempty"`
}

// SearchResponse is the JSON envelope returned by POST /api/search.
type SearchResponse struct {
	Query   string         `json:"query"`
	Total   int            `json:"total"`
	Results []SearchResult `json:"results"`
}

// Search queries the server's search index.
// searchType is "name", "content", "tag", or "" (name, content, and tags).
func (c *Client) Search(query, searchType string, limit int) (*SearchResponse, error) {
	body := map[string]any{"q": query}
	if searchType != "" {
		body["type"] = searchType
	}
	if limit > 0 {
		body["limit"] = limit
	}
	resp, err := c.postJSON(c.baseURL+"/api/search", body)
	if err != nil {
		return nil, wrapConnErr(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("search %q: %s", query, statusMsg(resp.StatusCode, raw))
	}
	var sr SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, fmt.Errorf("search %q: decode response: %w", query, err)
	}
	return &sr, nil
}

// TagCountResponse is the JSON body from POST /api/tag/count.
type TagCountResponse struct {
	Tag   string `json:"tag"`
	Total uint64 `json:"total"`
}

// TagListResponse is the JSON body from POST /api/tag/list.
type TagListResponse struct {
	Tags []TagStat `json:"tags"`
}

// TagStat is one tag bucket (lowercased term) with document count.
type TagStat struct {
	Tag   string `json:"tag"`
	Count uint64 `json:"count"`
}

// TagCount returns how many indexed notes contain the given front matter tag.
func (c *Client) TagCount(tag string) (*TagCountResponse, error) {
	resp, err := c.postJSON(c.baseURL+"/api/tag/count", map[string]any{"tag": tag})
	if err != nil {
		return nil, wrapConnErr(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("tag count: %s", statusMsg(resp.StatusCode, raw))
	}
	var out TagCountResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("tag count: decode response: %w", err)
	}
	return &out, nil
}

// TagList returns tag → document counts from the search index facet.
// limit is the maximum number of distinct tags (0 = server default).
func (c *Client) TagList(limit int) (*TagListResponse, error) {
	body := map[string]any{}
	if limit > 0 {
		body["limit"] = limit
	}
	resp, err := c.postJSON(c.baseURL+"/api/tag/list", body)
	if err != nil {
		return nil, wrapConnErr(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("tag list: %s", statusMsg(resp.StatusCode, raw))
	}
	var out TagListResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("tag list: decode response: %w", err)
	}
	if out.Tags == nil {
		out.Tags = []TagStat{}
	}
	return &out, nil
}

// TagDeleteResponse is the JSON body from POST /api/tag/delete.
type TagDeleteResponse struct {
	Tag     string   `json:"tag"`
	Deleted int      `json:"deleted"`
	Paths   []string `json:"paths"`
}

// TagDelete removes all indexed notes that carry the given tag from the search index.
func (c *Client) TagDelete(tag string) (*TagDeleteResponse, error) {
	resp, err := c.postJSON(c.baseURL+"/api/tag/delete", map[string]any{"tag": tag})
	if err != nil {
		return nil, wrapConnErr(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("tag delete: %s", statusMsg(resp.StatusCode, raw))
	}
	var out TagDeleteResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("tag delete: decode response: %w", err)
	}
	if out.Paths == nil {
		out.Paths = []string{}
	}
	return &out, nil
}

// ── Skills ────────────────────────────────────────────────────────────────────

// SkillInfo mirrors skills.SkillInfo for JSON decoding.
type SkillInfo struct {
	Name      string `json:"name"`
	Default   bool   `json:"default"`
	Installed bool   `json:"installed"`
	Enabled   bool   `json:"enabled"`
	RepoURL   string `json:"repoUrl,omitempty"`
	SubPath   string `json:"subPath,omitempty"`
}

// SkillsList calls GET /api/skills and returns all installed skills.
func (c *Client) SkillsList() ([]SkillInfo, error) {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/api/skills", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.do(req)
	if err != nil {
		return nil, wrapConnErr(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("skills list: %s", statusMsg(resp.StatusCode, raw))
	}
	var out struct {
		Skills []SkillInfo `json:"skills"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("skills list: decode response: %w", err)
	}
	return out.Skills, nil
}

// SkillsRemove calls DELETE /api/skills/{name} to remove a skill and its symlinks.
func (c *Client) SkillsRemove(name string) error {
	req, err := http.NewRequest(http.MethodDelete, c.baseURL+"/api/skills/"+name, nil)
	if err != nil {
		return err
	}
	resp, err := c.do(req)
	if err != nil {
		return wrapConnErr(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("skills remove: %s", statusMsg(resp.StatusCode, raw))
	}
	return nil
}

// SkillsInstall calls POST /api/skills/install to clone a GitHub repository
// and extract the named skill into ~/.hylo/skills/.
// subPath is the directory inside the repo that contains SKILL.md; pass an
// empty string to use automatic heuristic search.
// Uses a 5-minute timeout because git clone can be slow on large repositories.
func (c *Client) SkillsInstall(repoURL, subPath, skillName string) error {
	body, err := json.Marshal(map[string]any{
		"repoUrl": repoURL,
		"subPath": subPath,
		"skill":   skillName,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/api/skills/install", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("X-Hylo-API-Key", c.apiKey)
	}
	installClient := &http.Client{Timeout: 5 * time.Minute}
	resp, err := installClient.Do(req)
	if err != nil {
		return wrapConnErr(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("skills install: %s", statusMsg(resp.StatusCode, raw))
	}
	return nil
}

// ── Status ────────────────────────────────────────────────────────────────────

// StatusResponse mirrors handler.StatusResponse for JSON decoding.
type StatusResponse struct {
	Notes          int    `json:"notes"`
	Indexed        uint64 `json:"indexed"`
	KnowledgeNotes int    `json:"knowledge_notes"`
	ShortDays      int    `json:"short_days"`
}

// Status fetches live vault and index counts from POST /api/status.
func (c *Client) Status() (*StatusResponse, error) {
	resp, err := c.postJSON(c.baseURL+"/api/status", nil)
	if err != nil {
		return nil, wrapConnErr(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("status: %s", statusMsg(resp.StatusCode, body))
	}
	var out StatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("status: decode response: %w", err)
	}
	return &out, nil
}

// ── Agent CLI adapters (GET /api/agents) ──────────────────────────────────────

// agentsListBody is the JSON envelope from GET /api/agents.
type agentsListBody struct {
	Agents []agent.AgentInfo `json:"agents"`
}

// AgentsList returns adapters probed by the server (Open Design–compatible).
// Uses a longer timeout than the default client because each adapter may run
// subprocess probes (version, --help, optional model listing).
func (c *Client) AgentsList(ctx context.Context) ([]agent.AgentInfo, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/agents", nil)
	if err != nil {
		return nil, err
	}
	if c.apiKey != "" {
		req.Header.Set("X-Hylo-API-Key", c.apiKey)
	}
	// Use a longer client timeout than the default 30s: server probes many CLIs.
	longClient := &http.Client{Timeout: 120 * time.Second}
	resp, err := longClient.Do(req)
	if err != nil {
		return nil, wrapConnErr(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("agents: %s", statusMsg(resp.StatusCode, body))
	}
	var out agentsListBody
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("agents: decode response: %w", err)
	}
	return out.Agents, nil
}

// AgentChatRequest is the JSON body for POST /api/chat (same shape as server chatBody).
type AgentChatRequest struct {
	AgentID          string            `json:"agentId"`
	Message          string            `json:"message"`
	SystemPrompt     string            `json:"systemPrompt,omitempty"`
	Model            string            `json:"model,omitempty"`
	Reasoning        string            `json:"reasoning,omitempty"`
	Cwd              string            `json:"cwd,omitempty"`
	ImagePaths       []string          `json:"imagePaths,omitempty"`
	ExtraAllowedDirs []string          `json:"extraAllowedDirs,omitempty"`
	MCPServers       []agent.MCPServer `json:"mcpServers,omitempty"`
	AgentCliEnv      map[string]string `json:"agentCliEnv,omitempty"`
	ProjectID        string            `json:"projectId,omitempty"`
	ConversationID   string            `json:"conversationId,omitempty"`
}

// AgentChatSSE posts to /api/chat and invokes onEvent for each SSE frame until the stream ends.
// The HTTP client uses no deadline (Timeout 0) so long agent runs do not abort early.
func (c *Client) AgentChatSSE(ctx context.Context, req AgentChatRequest, onEvent func(event string, data json.RawMessage) error) error {
	if ctx == nil {
		ctx = context.Background()
	}
	payload, err := json.Marshal(req)
	if err != nil {
		return err
	}
	hreq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/chat", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	hreq.Header.Set("Content-Type", "application/json")
	hreq.Header.Set("Accept", "text/event-stream")
	if c.apiKey != "" {
		hreq.Header.Set("X-Hylo-API-Key", c.apiKey)
	}
	base := c.http
	if base == nil {
		base = http.DefaultClient
	}
	streamClient := &http.Client{
		Transport:     base.Transport,
		CheckRedirect: base.CheckRedirect,
		Jar:           base.Jar,
		Timeout:       0,
	}
	resp, err := streamClient.Do(hreq)
	if err != nil {
		return wrapConnErr(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("chat: %s", statusMsg(resp.StatusCode, raw))
	}
	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	if !strings.HasPrefix(contentType, "text/event-stream") && contentType != "" {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("chat: expected SSE, got Content-Type %q: %s", contentType, strings.TrimSpace(string(raw)))
	}

	br := bufio.NewReader(resp.Body)
	var evt string
	var dataLines []string
	var sawEnd bool
	flush := func() error {
		if evt == "" && len(dataLines) == 0 {
			return nil
		}
		var joined strings.Builder
		for i, s := range dataLines {
			if i > 0 {
				joined.WriteByte('\n')
			}
			joined.WriteString(s)
		}
		dataLines = nil
		name := evt
		evt = ""
		raw := joined.String()
		if raw == "" {
			raw = "{}"
		}
		if err := onEvent(name, json.RawMessage(raw)); err != nil {
			return err
		}
		if name == "end" {
			sawEnd = true
		}
		return nil
	}
	// io.ErrUnexpectedEOF often means the HTTP body was cut short (chunk/framing or RST).
	// Treat it like EOF for line assembly, but only accept it as success if we already
	// received the terminal "end" SSE from the server; otherwise the run may be truncated.
	finalize := func(readErr error) error {
		if errors.Is(readErr, io.ErrUnexpectedEOF) && !sawEnd {
			return fmt.Errorf("chat: SSE ended before terminal \"end\" event (stream likely truncated): %w", readErr)
		}
		return nil
	}
	for {
		line, err := br.ReadString('\n')
		streamDone := errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF)
		if err != nil && !streamDone {
			return err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			if e := flush(); e != nil {
				return e
			}
			if streamDone {
				return finalize(err)
			}
			continue
		}
		if strings.HasPrefix(line, "event:") {
			evt = strings.TrimPrefix(line[6:], " ")
			if streamDone {
				if e := flush(); e != nil {
					return e
				}
				return finalize(err)
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			ds := ""
			if len(line) > 5 {
				ds = strings.TrimPrefix(line[5:], " ")
			}
			dataLines = append(dataLines, ds)
			if streamDone {
				if e := flush(); e != nil {
					return e
				}
				return finalize(err)
			}
			continue
		}
		if strings.HasPrefix(line, "id:") {
			if streamDone {
				if e := flush(); e != nil {
					return e
				}
				return finalize(err)
			}
			continue
		}
		if streamDone {
			if e := flush(); e != nil {
				return e
			}
			return finalize(err)
		}
	}
}

// ── Extract operations ────────────────────────────────────────────────────────

// Link represents an extracted link from a markdown document.
type Link struct {
	Kind  string // "link", "image", "autolink", "wikilink"
	Text  string // display text (link) or alt text (image); empty for autolinks
	URL   string // destination URL or wiki link target
	Title string // optional title attribute
}

// ExtractLinks extracts all links (markdown links, images, autolinks, and wiki links)
// from the note at pathOrName. Returns an empty slice if no links are found.
func (c *Client) ExtractLinks(pathOrName string) ([]Link, error) {
	rc, err := c.ReadFile(pathOrName)
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	source, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("read content: %w", err)
	}

	return ParseLinks(source), nil
}

// ── internal helpers ──────────────────────────────────────────────────────────

func (c *Client) postJSON(url string, body any) (*http.Response, error) {
	var r io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		r = bytes.NewReader(data)
	}
	req, err := http.NewRequest(http.MethodPost, url, r)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.do(req)
}

func (c *Client) do(req *http.Request) (*http.Response, error) {
	if c.apiKey != "" {
		req.Header.Set("X-Hylo-API-Key", c.apiKey)
	}
	return c.http.Do(req)
}

func statusMsg(code int, body []byte) string {
	msg := strings.TrimSpace(string(body))
	if msg == "" {
		msg = http.StatusText(code)
	}
	return fmt.Sprintf("HTTP %d: %s", code, msg)
}

func wrapConnErr(err error) error {
	return fmt.Errorf("cannot reach hylo server (%w)\nIs the server running? Try: hylo start server", err)
}

// ── Formatting helpers ────────────────────────────────────────────────────────

// Format returns a human-readable multi-line representation of a Link,
// matching the output produced by `hylo extract link`.
func (l Link) Format() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n  %s", l.Kind, l.URL)
	switch l.Kind {
	case "image":
		if l.Text != "" {
			fmt.Fprintf(&b, "\n  alt: %s", l.Text)
		}
	case "wikilink":
		if l.Text != "" {
			fmt.Fprintf(&b, "\n  display: %s", l.Text)
		}
	default:
		if l.Text != "" {
			fmt.Fprintf(&b, "\n  text: %s", l.Text)
		}
	}
	if l.Title != "" {
		fmt.Fprintf(&b, "\n  title: %s", l.Title)
	}
	return b.String()
}


// ── Link parsing ──────────────────────────────────────────────────────────────

// ParseWikiLinkTargets returns the normalised filenames from all wiki links in
// source. Targets that lack a ".md" extension have one appended. Section anchors
// (e.g. "Note#section") are stripped. Duplicate targets are deduplicated.
func ParseWikiLinkTargets(source []byte) []string {
	links := ParseLinks(source)
	seen := make(map[string]struct{}, len(links))
	var out []string
	for _, l := range links {
		if l.Kind != "wikilink" {
			continue
		}
		name := strings.TrimSpace(l.URL)
		if name == "" {
			continue
		}
		if i := strings.IndexByte(name, '#'); i >= 0 {
			name = strings.TrimSpace(name[:i])
		}
		if name == "" {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(name), ".md") {
			name += ".md"
		}
		if _, dup := seen[name]; !dup {
			seen[name] = struct{}{}
			out = append(out, name)
		}
	}
	return out
}

// ParseLinks extracts all links (markdown links, images, autolinks, wiki links)
// from raw markdown source.
func ParseLinks(source []byte) []Link {
	doc := mdParser.Parse(text.NewReader(source))
	var out []Link
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch v := n.(type) {
		case *ast.Link:
			out = append(out, Link{
				Kind:  "link",
				Text:  MDInlineText(v, source),
				URL:   string(v.Destination),
				Title: string(v.Title),
			})
			return ast.WalkSkipChildren, nil
		case *ast.Image:
			out = append(out, Link{
				Kind:  "image",
				Text:  MDInlineText(v, source),
				URL:   string(v.Destination),
				Title: string(v.Title),
			})
			return ast.WalkSkipChildren, nil
		case *ast.AutoLink:
			out = append(out, Link{
				Kind: "autolink",
				URL:  string(v.URL(source)),
			})
		case *WikiLink:
			link := Link{
				Kind: "wikilink",
				URL:  string(v.Target),
			}
			if len(v.Alias) > 0 {
				link.Text = string(v.Alias)
			}
			out = append(out, link)
		}
		return ast.WalkContinue, nil
	})
	return out
}

// ParseRemoteImageURLs returns unique http(s) URLs from markdown that denote
// remote images suitable for download. It uses ParseLinks and keeps:
//   - every image link (![](url)) whose destination is http or https;
//   - autolinks, inline links, and wiki links whose URL looks like a direct
//     image file (path ends with a recognised image extension).
func ParseRemoteImageURLs(source []byte) []string {
	links := ParseLinks(source)
	seen := make(map[string]struct{}, len(links))
	var out []string
	for _, l := range links {
		u := strings.TrimSpace(l.URL)
		if u == "" || !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
			continue
		}
		switch l.Kind {
		case "image":
			appendUniqueURL(&out, seen, u)
		case "autolink", "link", "wikilink":
			if urlPathLooksLikeImage(u) {
				appendUniqueURL(&out, seen, u)
			}
		}
	}
	return out
}

func appendUniqueURL(dst *[]string, seen map[string]struct{}, u string) {
	if _, ok := seen[u]; ok {
		return
	}
	seen[u] = struct{}{}
	*dst = append(*dst, u)
}

func urlPathLooksLikeImage(raw string) bool {
	pu, err := url.Parse(raw)
	if err != nil || pu.Path == "" {
		return false
	}
	base := path.Base(pu.Path)
	return storage.IsImagePath(base)
}

// WikiLink represents an Obsidian-style wiki link: [[Target]] or [[Target|Alias]].
type WikiLink struct {
	ast.BaseInline
	Target []byte
	Alias  []byte
}

var kindWikiLink = ast.NewNodeKind("WikiLink")

func (w *WikiLink) Kind() ast.NodeKind { return kindWikiLink }
func (w *WikiLink) Dump(source []byte, level int) {
	ast.DumpHelper(w, source, level, map[string]string{
		"Target": string(w.Target),
		"Alias":  string(w.Alias),
	}, nil)
}

// wikiLinkParser is a goldmark inline parser for [[...]] syntax.
type wikiLinkParser struct{}

func (p *wikiLinkParser) Trigger() []byte { return []byte{'['} }

func (p *wikiLinkParser) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	line, _ := block.PeekLine()
	if len(line) < 4 || line[1] != '[' {
		return nil
	}
	rest := line[2:]
	end := bytes.Index(rest, []byte("]]"))
	if end < 0 {
		return nil
	}
	inner := bytes.TrimSpace(rest[:end])
	block.Advance(end + 4)

	var target, alias []byte
	if i := bytes.IndexByte(inner, '|'); i >= 0 {
		target = bytes.TrimSpace(inner[:i])
		alias = bytes.TrimSpace(inner[i+1:])
	} else {
		target = inner
	}
	return &WikiLink{Target: target, Alias: alias}
}

// mdParser is a shared goldmark parser with wiki link support.
var mdParser = parser.NewParser(
	parser.WithBlockParsers(parser.DefaultBlockParsers()...),
	parser.WithInlineParsers(
		append(parser.DefaultInlineParsers(), util.Prioritized(&wikiLinkParser{}, 150))...,
	),
	parser.WithParagraphTransformers(parser.DefaultParagraphTransformers()...),
)

// MDParse parses markdown source with the shared goldmark parser (wiki link aware).
func MDParse(source []byte) ast.Node {
	return mdParser.Parse(text.NewReader(source))
}

// MDInlineText extracts plain text from an inline AST container (e.g. a heading).
func MDInlineText(n ast.Node, source []byte) string {
	var sb strings.Builder
	_ = ast.Walk(n, func(child ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if t, ok := child.(*ast.Text); ok {
			sb.Write(t.Segment.Value(source))
			if t.SoftLineBreak() {
				sb.WriteByte(' ')
			}
		}
		return ast.WalkContinue, nil
	})
	return strings.TrimSpace(sb.String())
}
