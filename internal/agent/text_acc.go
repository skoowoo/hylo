package agent

import "strings"

// TextAccumulator builds an assistant reply from a run's agent events. Text
// that resumes after a tool call or thinking starts a new paragraph, as the
// live chat shows it, so a block opening with a fence isn't glued to the
// sentence before it.
type TextAccumulator struct {
	b     strings.Builder
	split bool
}

// Feed applies one agent event payload.
func (t *TextAccumulator) Feed(m map[string]any) {
	switch m["type"] {
	case "text_delta":
		d, _ := m["delta"].(string)
		if d == "" {
			return
		}
		if t.split && t.b.Len() > 0 {
			s := t.b.String()
			switch {
			case strings.HasSuffix(s, "\n\n"):
			case strings.HasSuffix(s, "\n"):
				t.b.WriteString("\n")
			default:
				t.b.WriteString("\n\n")
			}
		}
		t.split = false
		t.b.WriteString(d)
	case "text_replace", "text_snapshot":
		if s, ok := m["text"].(string); ok {
			t.b.Reset()
			t.b.WriteString(s)
			t.split = false
		}
	case "tool_use", "tool_result", "thinking_start", "thinking_delta":
		t.split = true
	}
}

func (t *TextAccumulator) String() string { return t.b.String() }
