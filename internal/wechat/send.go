package wechat

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/hardhacker/hylo/internal/config"
)

// TextChunkLimit is the longest message iLink accepts.
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
		client = NewClient(config.Proxy{})
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
