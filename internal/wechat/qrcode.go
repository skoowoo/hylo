package wechat

import (
	"context"
	"fmt"
	"time"

	"github.com/hardhacker/hylo/internal/config"
)

// QrcodePollResult is one poll of QR login status.
type QrcodePollResult struct {
	Status           string
	Qrcode           string
	QrcodeImgContent string
	Token            *TokenData
}

// StartQrcodeLogin requests a new WeChat login QR code.
func StartQrcodeLogin(ctx context.Context, client *Client, baseURL, botType string) (*QrcodeResp, error) {
	if client == nil {
		client = NewClient(config.Proxy{})
	}
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return client.GetBotQrcode(ctx, baseURL, botType)
}

// PollQrcodeLogin checks login status once. When status is expired and refreshCount
// is below maxRefresh, a new QR code is fetched and returned.
func PollQrcodeLogin(ctx context.Context, client *Client, baseURL, qrcode string, refreshCount, maxRefresh int) (QrcodePollResult, int, error) {
	if client == nil {
		client = NewClient(config.Proxy{})
	}
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}

	st, err := client.GetQrcodeStatus(ctx, baseURL, qrcode)
	if err != nil {
		return QrcodePollResult{}, refreshCount, err
	}

	switch st.Status {
	case "confirmed":
		host := st.BaseURL
		if host == "" {
			host = baseURL
		}
		td := &TokenData{
			Token:     st.BotToken,
			BaseURL:   host,
			AccountID: st.ILinkBotID,
			UserID:    st.ILinkUserID,
			SavedAt:   time.Now().UTC().Format(time.RFC3339),
		}
		return QrcodePollResult{Status: st.Status, Token: td}, refreshCount, nil
	case "expired":
		if refreshCount >= maxRefresh {
			return QrcodePollResult{Status: st.Status}, refreshCount, fmt.Errorf("QR code expired multiple times, please retry")
		}
		refreshCount++
		qr, err := client.GetBotQrcode(ctx, baseURL, DefaultBotType)
		if err != nil {
			return QrcodePollResult{}, refreshCount, err
		}
		return QrcodePollResult{
			Status:           st.Status,
			Qrcode:           qr.Qrcode,
			QrcodeImgContent: qr.QrcodeImgContent,
		}, refreshCount, nil
	default:
		return QrcodePollResult{Status: st.Status}, refreshCount, nil
	}
}
