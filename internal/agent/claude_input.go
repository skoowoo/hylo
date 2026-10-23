package agent

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// claudeUserLine is one stream-json user message. Image blocks come first.
// An empty prompt omits the text block, so an image-only turn stays captionless.
func claudeUserLine(prompt string, paths []string) []byte {
	var blocks []any
	for _, p := range paths {
		if block, ok := claudeImageBlock(p); ok {
			blocks = append(blocks, block)
		}
	}
	var content any
	if len(blocks) == 0 {
		content = prompt
	} else {
		if strings.TrimSpace(prompt) != "" {
			blocks = append(blocks, map[string]any{"type": "text", "text": prompt})
		}
		content = blocks
	}
	frame := map[string]any{
		"type": "user",
		"message": map[string]any{
			"role":    "user",
			"content": content,
		},
	}
	b, err := json.Marshal(frame)
	if err != nil {
		b = []byte(`{"type":"user","message":{"role":"user","content":""}}`)
	}
	return append(b, '\n')
}

func claudeImageBlock(path string) (map[string]any, bool) {
	mime, ok := piImageExt[strings.ToLower(filepath.Ext(path))]
	if !ok {
		return nil, false
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return nil, false
	}
	return map[string]any{
		"type": "image",
		"source": map[string]any{
			"type":       "base64",
			"media_type": mime,
			"data":       base64.StdEncoding.EncodeToString(data),
		},
	}, true
}
