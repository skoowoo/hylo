package agent

import (
	"encoding/json"
	"time"
	"unicode/utf8"
)

const (
	turnLogResultMax   = 2000
	turnLogThinkingMax = 8000
	turnLogInputMax    = 300
)

// TurnLog records a run's agent events so the chat can rebuild its trace
// after a reload, and picks out the reply: the last run of text, since text
// followed by more steps was narration of the work.
type TurnLog struct {
	events []map[string]any
	answer string
	split  bool
}

// Feed stamps m with its arrival time and records a compact copy.
func (t *TurnLog) Feed(m map[string]any) {
	if _, ok := m["at"]; !ok {
		m["at"] = time.Now().UnixMilli()
	}
	typ, _ := m["type"].(string)
	switch typ {
	case "text_delta":
		d, _ := m["delta"].(string)
		if d == "" {
			return
		}
		if t.split {
			t.answer, t.split = "", false
		}
		t.answer += d
		t.appendDelta(m, d, 0)
	case "thinking_delta":
		t.split = true
		d, _ := m["delta"].(string)
		t.appendDelta(m, d, turnLogThinkingMax)
	case "tool_use":
		t.split = true
		c := copyEvent(m)
		c["input"] = compactInput(m["input"])
		t.events = append(t.events, c)
	case "tool_result":
		t.split = true
		c := copyEvent(m)
		if s, ok := c["content"].(string); ok {
			c["content"] = clip(s, turnLogResultMax)
		}
		t.events = append(t.events, c)
	case "raw":
		t.split = true
		c := copyEvent(m)
		if s, ok := c["line"].(string); ok {
			c["line"] = clip(s, turnLogResultMax)
		}
		t.events = append(t.events, c)
	case "thinking_start", "error":
		t.split = true
		t.events = append(t.events, m)
	case "note_access":
		t.events = append(t.events, m)
	}
}

// Answer is the reply the run ends with.
func (t *TurnLog) Answer() string { return t.answer }

// JSON is the recorded event list, or "" when nothing was recorded.
func (t *TurnLog) JSON() string {
	if len(t.events) == 0 {
		return ""
	}
	b, err := json.Marshal(t.events)
	if err != nil {
		return ""
	}
	return string(b)
}

// appendDelta folds consecutive deltas of one block into one event.
func (t *TurnLog) appendDelta(m map[string]any, d string, max int) {
	if n := len(t.events); n > 0 && t.events[n-1]["type"] == m["type"] {
		last := t.events[n-1]
		prev, _ := last["delta"].(string)
		if max == 0 || utf8.RuneCountInString(prev) < max {
			last["delta"] = clip(prev+d, max)
		}
		return
	}
	c := copyEvent(m)
	c["delta"] = clip(d, max)
	t.events = append(t.events, c)
}

func copyEvent(m map[string]any) map[string]any {
	c := make(map[string]any, len(m))
	for k, v := range m {
		c[k] = v
	}
	return c
}

// compactInput keeps what a step's one-line label is drawn from, not the
// bodies a write or edit carries.
func compactInput(v any) any {
	in, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	out := map[string]any{}
	for k, x := range in {
		switch s := x.(type) {
		case string:
			if utf8.RuneCountInString(s) <= turnLogInputMax {
				out[k] = s
			}
		default:
			if k == "__notes" {
				out[k] = x
			}
		}
	}
	return out
}

func clip(s string, max int) string {
	if max == 0 || utf8.RuneCountInString(s) <= max {
		return s
	}
	return string([]rune(s)[:max]) + "…"
}
