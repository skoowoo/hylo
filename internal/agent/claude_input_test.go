package agent

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestClaudeUserLine(t *testing.T) {
	dir := t.TempDir()
	png := filepath.Join(dir, "a.png")
	jpg := filepath.Join(dir, "b.jpg")
	if err := os.WriteFile(png, []byte("png-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(jpg, []byte("jpg-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "note.svg"), []byte("<svg/>"), 0o644); err != nil {
		t.Fatal(err)
	}

	line := claudeUserLine("look here", []string{
		png,
		filepath.Join(dir, "missing.png"),
		filepath.Join(dir, "note.svg"),
		jpg,
	})
	var frame struct {
		Type    string `json:"type"`
		Message struct {
			Role    string `json:"role"`
			Content []struct {
				Type   string `json:"type"`
				Text   string `json:"text"`
				Source struct {
					Type      string `json:"type"`
					MediaType string `json:"media_type"`
					Data      string `json:"data"`
				} `json:"source"`
			} `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(line, &frame); err != nil {
		t.Fatal(err)
	}
	if frame.Type != "user" || frame.Message.Role != "user" {
		t.Fatalf("frame = %+v", frame)
	}
	got := frame.Message.Content
	if len(got) != 3 || got[0].Type != "image" || got[1].Type != "image" || got[2].Type != "text" {
		t.Fatalf("content = %+v", got)
	}
	if got[0].Source.Type != "base64" || got[0].Source.MediaType != "image/png" {
		t.Fatalf("png source = %+v", got[0].Source)
	}
	if got[1].Source.MediaType != "image/jpeg" {
		t.Fatalf("jpeg source = %+v", got[1].Source)
	}
	raw, err := base64.StdEncoding.DecodeString(got[0].Source.Data)
	if err != nil || string(raw) != "png-bytes" {
		t.Fatalf("png data = %q err=%v", raw, err)
	}
	if got[2].Text != "look here" {
		t.Fatalf("text = %q", got[2].Text)
	}

	bare := claudeUserLine("", []string{png})
	var bareFrame struct {
		Message struct {
			Content []struct {
				Type string `json:"type"`
			} `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(bare, &bareFrame); err != nil {
		t.Fatal(err)
	}
	if len(bareFrame.Message.Content) != 1 || bareFrame.Message.Content[0].Type != "image" {
		t.Fatalf("captionless content = %+v", bareFrame.Message.Content)
	}

	var textFrame struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(claudeUserLine("hello", nil), &textFrame); err != nil {
		t.Fatal(err)
	}
	if textFrame.Message.Content != "hello" {
		t.Fatalf("text content = %q", textFrame.Message.Content)
	}
}

func TestBuildClaudeInputFormat(t *testing.T) {
	for _, ctx := range []BuildArgsContext{
		{},
		{ImagePaths: []string{"/vault/.hylo/uploads/a.png"}},
	} {
		argv := BuildInvocationArgs(GetAgentDef("claude"), ctx)
		if !containsSeq(argv, "--input-format", "stream-json") || !containsSeq(argv, "-p") {
			t.Fatalf("args = %v", argv)
		}
	}
}
