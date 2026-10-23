package storage

import (
	"database/sql"
	"errors"
	"strings"
)

// URLEmbed is one cached resolution of a canonical URL.
type URLEmbed struct {
	URL         string
	FinalURL    string
	Kind        string
	ContentType string
	Title       string
	Description string
	SiteName    string
	ImageURL    string
	Status      string
	Error       string
	FetchedAt   int64
	ExpiresAt   int64
	PositionMS  int64
}

// FreshURLEmbeds returns unexpired rows for the given canonical URLs.
func (g *Vault) FreshURLEmbeds(urls []string, nowMs int64) (map[string]URLEmbed, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return dbFreshURLEmbeds(g.db, urls, nowMs)
}

// PutURLEmbed inserts or replaces a row. Playback position is kept on conflict.
func (g *Vault) PutURLEmbed(e URLEmbed) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return dbPutURLEmbed(g.db, e)
}

// SetURLEmbedPosition records playback for an audio or video row, in milliseconds.
func (g *Vault) SetURLEmbedPosition(url string, ms int64) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	_, err := g.db.Exec(
		`UPDATE url_embeds SET position_ms = ? WHERE url = ? AND status = 'ok' AND kind IN ('audio', 'video')`,
		ms, url,
	)
	return err
}

// ExtendURLEmbed pushes back the expiry of an ok row and returns it; false when there is none.
func (g *Vault) ExtendURLEmbed(url string, expiresAt int64) (URLEmbed, bool, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	e, err := scanURLEmbed(g.db.QueryRow(
		`UPDATE url_embeds SET expires_at = ? WHERE url = ? AND status = 'ok' RETURNING `+urlEmbedCols,
		expiresAt, url,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return URLEmbed{}, false, nil
	}
	return e, err == nil, err
}

// PruneURLEmbeds deletes expired error rows, and ok rows expired before keepBefore.
// Ok rows outlive expiry so a failed refresh can fall back on them and playback is kept.
func (g *Vault) PruneURLEmbeds(nowMs, keepBefore int64) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	_, err := g.db.Exec(
		`DELETE FROM url_embeds WHERE expires_at < ? AND (status != 'ok' OR expires_at < ?)`,
		nowMs, keepBefore,
	)
	return err
}

const urlEmbedCols = `url, final_url, kind, content_type, title, description, site_name, image_url, status, error, fetched_at, expires_at, position_ms`

func scanURLEmbed(row interface{ Scan(...any) error }) (URLEmbed, error) {
	var e URLEmbed
	err := row.Scan(&e.URL, &e.FinalURL, &e.Kind, &e.ContentType, &e.Title, &e.Description, &e.SiteName, &e.ImageURL, &e.Status, &e.Error, &e.FetchedAt, &e.ExpiresAt, &e.PositionMS)
	return e, err
}

func dbFreshURLEmbeds(db *sql.DB, urls []string, nowMs int64) (map[string]URLEmbed, error) {
	out := make(map[string]URLEmbed)
	if len(urls) == 0 {
		return out, nil
	}
	q := `SELECT ` + urlEmbedCols + ` FROM url_embeds WHERE expires_at > ? AND url IN (` + strings.Repeat("?,", len(urls)-1) + `?)`
	args := make([]any, 0, len(urls)+1)
	args = append(args, nowMs)
	for _, u := range urls {
		args = append(args, u)
	}
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		e, err := scanURLEmbed(rows)
		if err != nil {
			return nil, err
		}
		out[e.URL] = e
	}
	return out, rows.Err()
}

func dbPutURLEmbed(db *sql.DB, e URLEmbed) error {
	_, err := db.Exec(`
		INSERT INTO url_embeds (
			url, final_url, kind, content_type, title, description, site_name, image_url, status, error, fetched_at, expires_at
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(url) DO UPDATE SET
			final_url=excluded.final_url,
			kind=excluded.kind,
			content_type=excluded.content_type,
			title=excluded.title,
			description=excluded.description,
			site_name=excluded.site_name,
			image_url=excluded.image_url,
			status=excluded.status,
			error=excluded.error,
			fetched_at=excluded.fetched_at,
			expires_at=excluded.expires_at`,
		e.URL, e.FinalURL, e.Kind, e.ContentType, e.Title, e.Description, e.SiteName, e.ImageURL, e.Status, e.Error, e.FetchedAt, e.ExpiresAt,
	)
	return err
}
