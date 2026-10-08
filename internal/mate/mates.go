package mate

import (
	"database/sql"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ── Mates ─────────────────────────────────────────────────────────────────────

func (s *Store) ListMates() ([]Mate, error) {
	rows, err := s.db.Query(
		`SELECT m.id, m.name, m.description, m.agent_id, m.model, m.color, m.cwd, m.system_prompt, m.enabled, m.created_at, m.updated_at,
		        (SELECT COUNT(*) FROM mate_triggers t WHERE t.mate_id = m.id) AS trigger_count
         FROM mates m ORDER BY m.sort_order ASC, m.created_at ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMatesWithTriggerCount(rows)
}

func (s *Store) GetMate(id string) (*Mate, error) {
	var m Mate
	var createdMs, updatedMs int64
	var enabled int
	err := s.db.QueryRow(
		`SELECT id, name, description, agent_id, model, color, cwd, system_prompt, enabled, created_at, updated_at
         FROM mates WHERE id = ?`, id,
	).Scan(&m.ID, &m.Name, &m.Description, &m.AgentID, &m.Model, &m.Color, &m.Cwd,
		&m.SystemPrompt, &enabled, &createdMs, &updatedMs)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	m.Enabled = enabled != 0
	m.CreatedAt = time.UnixMilli(createdMs)
	m.UpdatedAt = time.UnixMilli(updatedMs)
	return &m, nil
}

func (s *Store) CreateMate(m *Mate) error {
	now := time.Now().UnixMilli()
	if m.ID == "" {
		m.ID = uuid.NewString()
	}
	m.CreatedAt = time.UnixMilli(now)
	m.UpdatedAt = time.UnixMilli(now)
	var maxOrder int64
	s.db.QueryRow(`SELECT COALESCE(MAX(sort_order), -1) FROM mates`).Scan(&maxOrder) //nolint:errcheck
	_, err := s.db.Exec(
		`INSERT INTO mates(id, name, description, agent_id, model, color, cwd, system_prompt, enabled, sort_order, created_at, updated_at)
         VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		m.ID, m.Name, m.Description, m.AgentID, m.Model, m.Color, m.Cwd,
		m.SystemPrompt, boolToInt(m.Enabled), maxOrder+1, now, now,
	)
	return err
}

// ReorderMates assigns each mate the sort_order matching its position in ids.
func (s *Store) ReorderMates(ids []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	for i, id := range ids {
		if _, err := tx.Exec(`UPDATE mates SET sort_order = ? WHERE id = ?`, i, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) UpdateMate(m *Mate) error {
	now := time.Now().UnixMilli()
	m.UpdatedAt = time.UnixMilli(now)
	_, err := s.db.Exec(
		`UPDATE mates SET name=?, description=?, agent_id=?, model=?, color=?, cwd=?, system_prompt=?, enabled=?, updated_at=?
         WHERE id=?`,
		m.Name, m.Description, m.AgentID, m.Model, m.Color, m.Cwd,
		m.SystemPrompt, boolToInt(m.Enabled), now, m.ID,
	)
	return err
}

func (s *Store) DeleteMate(id string) error {
	_, err := s.db.Exec(`DELETE FROM mates WHERE id = ?`, id)
	return err
}

// ── Triggers ──────────────────────────────────────────────────────────────────

func (s *Store) ListTriggers(mateID string) ([]MateTrigger, error) {
	rows, err := s.db.Query(
		`SELECT id, mate_id, event_types, prompt, enabled, created_at, schedule, last_fired_at, path_prefixes
         FROM mate_triggers WHERE mate_id = ? ORDER BY created_at ASC`, mateID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTriggers(rows)
}

// ListAllEnabledTriggers returns all enabled triggers for all enabled mates.
func (s *Store) ListAllEnabledTriggers() ([]MateTrigger, error) {
	rows, err := s.db.Query(
		`SELECT t.id, t.mate_id, t.event_types, t.prompt, t.enabled, t.created_at, t.schedule, t.last_fired_at, t.path_prefixes
         FROM mate_triggers t
         JOIN mates m ON m.id = t.mate_id
         WHERE t.enabled = 1 AND m.enabled = 1
         ORDER BY t.created_at ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTriggers(rows)
}

// ListScheduledTriggers returns enabled triggers with a non-empty schedule.
func (s *Store) ListScheduledTriggers() ([]MateTrigger, error) {
	rows, err := s.db.Query(
		`SELECT t.id, t.mate_id, t.event_types, t.prompt, t.enabled, t.created_at, t.schedule, t.last_fired_at, t.path_prefixes
         FROM mate_triggers t
         JOIN mates m ON m.id = t.mate_id
         WHERE t.enabled = 1 AND m.enabled = 1 AND t.schedule != ''
         ORDER BY t.created_at ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTriggers(rows)
}

// UpdateTriggerLastFiredAt records when a scheduled trigger last fired.
func (s *Store) UpdateTriggerLastFiredAt(id string, at time.Time) error {
	_, err := s.db.Exec(`UPDATE mate_triggers SET last_fired_at = ? WHERE id = ?`, at.UnixMilli(), id)
	return err
}

// ReplaceTriggersForMate atomically replaces all triggers for a mate.
func (s *Store) ReplaceTriggersForMate(mateID string, triggers []MateTrigger) error {
	existing, _ := s.ListTriggers(mateID)
	prev := make(map[string]MateTrigger, len(existing))
	for _, t := range existing {
		prev[t.ID] = t
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	if _, err := tx.Exec(`DELETE FROM mate_triggers WHERE mate_id = ?`, mateID); err != nil {
		return err
	}
	now := time.Now().UnixMilli()
	for i := range triggers {
		t := &triggers[i]
		if t.ID == "" {
			t.ID = uuid.NewString()
		}
		t.MateID = mateID
		if t.CreatedAt.IsZero() {
			t.CreatedAt = time.UnixMilli(now)
		}
		if old, ok := prev[t.ID]; ok && strings.TrimSpace(t.Schedule) == strings.TrimSpace(old.Schedule) {
			t.LastFiredAt = old.LastFiredAt
		} else {
			t.LastFiredAt = time.Time{}
		}
		lastFiredMs := int64(0)
		if !t.LastFiredAt.IsZero() {
			lastFiredMs = t.LastFiredAt.UnixMilli()
		}
		if _, err := tx.Exec(
			`INSERT INTO mate_triggers(id, mate_id, event_types, prompt, enabled, created_at, schedule, last_fired_at, path_prefixes) VALUES (?,?,?,?,?,?,?,?,?)`,
			t.ID, t.MateID, strings.Join(t.EventTypes, ","), t.Prompt, boolToInt(t.Enabled), t.CreatedAt.UnixMilli(), strings.TrimSpace(t.Schedule), lastFiredMs, strings.Join(t.PathPrefixes, ","),
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ── scan helpers ──────────────────────────────────────────────────────────────

func scanMates(rows *sql.Rows) ([]Mate, error) {
	var out []Mate
	for rows.Next() {
		m, err := scanMateRow(rows, false)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func scanMatesWithTriggerCount(rows *sql.Rows) ([]Mate, error) {
	var out []Mate
	for rows.Next() {
		m, err := scanMateRow(rows, true)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func scanMateRow(rows *sql.Rows, withTriggerCount bool) (Mate, error) {
	var m Mate
	var createdMs, updatedMs int64
	var enabled int
	var triggerCount int
	var cols = []any{
		&m.ID, &m.Name, &m.Description, &m.AgentID, &m.Model, &m.Color, &m.Cwd,
		&m.SystemPrompt, &enabled, &createdMs, &updatedMs,
	}
	if withTriggerCount {
		cols = append(cols, &triggerCount)
	}
	if err := rows.Scan(cols...); err != nil {
		return Mate{}, err
	}
	m.Enabled = enabled != 0
	m.CreatedAt = time.UnixMilli(createdMs)
	m.UpdatedAt = time.UnixMilli(updatedMs)
	if withTriggerCount {
		m.TriggerCount = triggerCount
	}
	return m, nil
}

func scanTriggers(rows *sql.Rows) ([]MateTrigger, error) {
	var out []MateTrigger
	for rows.Next() {
		var t MateTrigger
		var createdMs, lastFiredMs int64
		var enabled int
		var evTypes, schedule, pathPrefixes string
		if err := rows.Scan(&t.ID, &t.MateID, &evTypes, &t.Prompt, &enabled, &createdMs, &schedule, &lastFiredMs, &pathPrefixes); err != nil {
			return nil, err
		}
		t.Enabled = enabled != 0
		t.CreatedAt = time.UnixMilli(createdMs)
		t.Schedule = schedule
		if lastFiredMs > 0 {
			t.LastFiredAt = time.UnixMilli(lastFiredMs)
		}
		if evTypes != "" {
			t.EventTypes = strings.Split(evTypes, ",")
		}
		if pathPrefixes != "" {
			t.PathPrefixes = strings.Split(pathPrefixes, ",")
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
