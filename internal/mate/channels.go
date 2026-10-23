package mate

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// App channels a Hylo bot can speak through. One external bot contact belongs
// to one Hylo bot; a person can own several contacts.
const (
	ChannelWechat   = "wechat"
	ChannelDiscord  = "discord"
	ChannelTelegram = "telegram"
)

// Channel is one app contact bound to a Hylo bot.
type Channel struct {
	ID          string
	MateID      string
	Kind        string
	Token       string
	OwnerUserID string
	ExternalID  string // ilink_bot_id, Discord bot user id, or Telegram bot user id
	Display     string
	BaseURL     string
	CreatedAt   time.Time
}

var (
	ErrChannelTaken      = errors.New("mate: this app bot is already connected to another bot")
	ErrChannelIncomplete = errors.New("mate: channel is incomplete")
)

// ValidChannelKind reports whether kind is a chat app Hylo can bind.
func ValidChannelKind(kind string) bool {
	switch kind {
	case ChannelWechat, ChannelDiscord, ChannelTelegram:
		return true
	default:
		return false
	}
}

const channelCols = `c.id, c.mate_id, c.kind, c.token, c.owner_user_id, c.external_id, c.display, c.base_url, c.created_at`

// ListChannels returns every binding.
func (s *Store) ListChannels() ([]Channel, error) {
	return s.queryChannels(`SELECT ` + channelCols + ` FROM bot_channels c ORDER BY c.created_at`)
}

// ListActiveChannels returns the bindings of enabled bots.
func (s *Store) ListActiveChannels() ([]Channel, error) {
	return s.queryChannels(`SELECT ` + channelCols + ` FROM bot_channels c JOIN mates m ON m.id = c.mate_id WHERE m.enabled = 1 ORDER BY c.created_at`)
}

// ListMateChannels returns the bindings on one bot.
func (s *Store) ListMateChannels(mateID string) ([]Channel, error) {
	return s.queryChannels(`SELECT `+channelCols+` FROM bot_channels c WHERE c.mate_id = ? ORDER BY c.kind`, mateID)
}

func (s *Store) queryChannels(q string, args ...any) ([]Channel, error) {
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("mate: list channels: %w", err)
	}
	defer rows.Close()
	var out []Channel
	for rows.Next() {
		var c Channel
		var created int64
		if err := rows.Scan(&c.ID, &c.MateID, &c.Kind, &c.Token, &c.OwnerUserID, &c.ExternalID, &c.Display, &c.BaseURL, &created); err != nil {
			return nil, fmt.Errorf("mate: scan channel: %w", err)
		}
		c.CreatedAt = time.UnixMilli(created)
		out = append(out, c)
	}
	return out, rows.Err()
}

// SaveChannel binds an app contact to a bot, replacing that bot's binding of the same kind.
func (s *Store) SaveChannel(c Channel) (Channel, error) {
	c.Kind = strings.TrimSpace(c.Kind)
	c.Token = strings.TrimSpace(c.Token)
	c.ExternalID = strings.TrimSpace(c.ExternalID)
	c.OwnerUserID = strings.TrimSpace(c.OwnerUserID)
	c.Display = strings.TrimSpace(c.Display)
	c.BaseURL = strings.TrimSpace(c.BaseURL)
	if !ValidChannelKind(c.Kind) || c.MateID == "" || c.Token == "" || c.ExternalID == "" || c.OwnerUserID == "" {
		return Channel{}, ErrChannelIncomplete
	}

	tx, err := s.db.Begin()
	if err != nil {
		return Channel{}, err
	}
	defer tx.Rollback() //nolint:errcheck

	var other int
	err = tx.QueryRow(
		`SELECT 1 FROM bot_channels WHERE kind = ? AND mate_id != ? AND (external_id = ? OR token = ?) LIMIT 1`,
		c.Kind, c.MateID, c.ExternalID, c.Token,
	).Scan(&other)
	if err == nil {
		return Channel{}, ErrChannelTaken
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Channel{}, err
	}

	// A replaced binding gets a new id, so a link still running on the old one
	// can never remove it.
	if _, err := tx.Exec(`DELETE FROM bot_channels WHERE mate_id = ? AND kind = ?`, c.MateID, c.Kind); err != nil {
		return Channel{}, fmt.Errorf("mate: save channel: %w", err)
	}
	now := time.Now().UnixMilli()
	c.ID = uuid.NewString()
	if _, err := tx.Exec(
		`INSERT INTO bot_channels(id, mate_id, kind, token, owner_user_id, external_id, display, base_url, created_at)
		 VALUES (?,?,?,?,?,?,?,?,?)`,
		c.ID, c.MateID, c.Kind, c.Token, c.OwnerUserID, c.ExternalID, c.Display, c.BaseURL, now,
	); err != nil {
		return Channel{}, fmt.Errorf("mate: save channel: %w", err)
	}
	// One WeChat account can keep one live bot. A new scan unbinds the previous one.
	if c.Kind == ChannelWechat {
		if _, err := tx.Exec(
			`DELETE FROM bot_channels WHERE kind = ? AND owner_user_id = ? AND id != ?`,
			c.Kind, c.OwnerUserID, c.ID,
		); err != nil {
			return Channel{}, fmt.Errorf("mate: save channel: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return Channel{}, err
	}
	c.CreatedAt = time.UnixMilli(now)
	return c, s.changed(nil)
}

// DeleteChannel removes one kind of binding from a bot.
func (s *Store) DeleteChannel(mateID, kind string) error {
	_, err := s.db.Exec(`DELETE FROM bot_channels WHERE mate_id = ? AND kind = ?`, mateID, kind)
	return s.changed(err)
}

// DeleteChannelByID removes one binding, never the one that replaced it.
func (s *Store) DeleteChannelByID(id string) error {
	_, err := s.db.Exec(`DELETE FROM bot_channels WHERE id = ?`, id)
	return s.changed(err)
}
