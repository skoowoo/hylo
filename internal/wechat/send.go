package wechat

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

const TextChunkLimit = 4000

// SendOpts carries credentials for outbound messages.
type SendOpts struct {
	BaseURL      string
	Token        string
	ContextToken string
	Client       *Client
}

// SendTextMessage sends a plain-text reply to a WeChat user.
func SendTextMessage(ctx context.Context, to, text string, opts SendOpts) (string, error) {
	if opts.ContextToken == "" {
		return "", fmt.Errorf("contextToken is required to send a message")
	}
	client := opts.Client
	if client == nil {
		client = NewClient()
	}
	clientID := "hylo-wechat-" + uuid.NewString()
	req := SendMessageReq{
		Msg: &Message{
			FromUserID:   "",
			ToUserID:     to,
			ClientID:     clientID,
			MessageType:  MessageTypeBot,
			MessageState: MessageStateFinish,
			ContextToken: opts.ContextToken,
			ItemList: []MessageItem{{
				Type:     MessageItemTypeText,
				TextItem: &TextItem{Text: text},
			}},
		},
	}
	if err := client.SendMessage(ctx, opts.BaseURL, opts.Token, req); err != nil {
		return "", err
	}
	return clientID, nil
}

// SplitText splits text into segments of at most maxLen runes, preferring line breaks.
func SplitText(text string, maxLen int) []string {
	if maxLen <= 0 || len(text) <= maxLen {
		return []string{text}
	}
	var segments []string
	remaining := text
	for len(remaining) > 0 {
		if len(remaining) <= maxLen {
			segments = append(segments, remaining)
			break
		}
		breakAt := lastIndexByte(remaining, '\n', maxLen)
		if breakAt <= 0 {
			breakAt = maxLen
		}
		segments = append(segments, remaining[:breakAt])
		remaining = remaining[breakAt:]
		if len(remaining) > 0 && remaining[0] == '\n' {
			remaining = remaining[1:]
		}
	}
	return segments
}

func lastIndexByte(s string, b byte, limit int) int {
	if limit > len(s) {
		limit = len(s)
	}
	for i := limit - 1; i >= 0; i-- {
		if s[i] == b {
			return i
		}
	}
	return -1
}
