package wechat

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hardhacker/hylo/internal/config"
)

func tokenPath(storageDir string) string {
	return filepath.Join(storageDir, "token.json")
}

func contextTokenPath(storageDir string) string {
	return filepath.Join(storageDir, "context_token")
}

// LoadContextToken reads the last known iLink context token, or "" if absent.
func LoadContextToken(storageDir string) string {
	data, err := os.ReadFile(contextTokenPath(storageDir))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// SaveContextToken persists the iLink context token for future sessions.
func SaveContextToken(storageDir string, token string) error {
	if err := os.MkdirAll(storageDir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(contextTokenPath(storageDir), []byte(token), 0o600)
}

// LoadToken reads a saved login token, or nil when missing/invalid.
func LoadToken(storageDir string) (*TokenData, error) {
	p := tokenPath(storageDir)
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var td TokenData
	if err := json.Unmarshal(data, &td); err != nil {
		return nil, nil
	}
	if td.Token == "" {
		return nil, nil
	}
	return &td, nil
}

// SaveToken persists login credentials.
func SaveToken(storageDir string, td TokenData) error {
	if err := os.MkdirAll(storageDir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(td, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(tokenPath(storageDir), data, 0o600)
}

// LoginOpts configures QR login.
type LoginOpts struct {
	BaseURL    string
	BotType    string
	StorageDir string
	Client     *Client
	Log        func(string)
}

// Login performs QR login and saves the token.
func Login(ctx context.Context, opts LoginOpts) (*TokenData, error) {
	client := opts.Client
	if client == nil {
		client = NewClient(config.Proxy{})
	}
	log := opts.Log
	if log == nil {
		log = func(string) {}
	}

	log("Starting WeChat QR login...")
	qr, err := client.GetBotQrcode(ctx, opts.BaseURL, opts.BotType)
	if err != nil {
		return nil, err
	}
	log("Please scan the QR code with WeChat:")
	log("QR URL: " + qr.QrcodeImgContent)

	deadline := time.Now().Add(5 * time.Minute)
	currentQrcode := qr.Qrcode
	refreshCount := 0

	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		st, err := client.GetQrcodeStatus(ctx, opts.BaseURL, currentQrcode)
		if err != nil {
			return nil, err
		}
		switch st.Status {
		case "wait":
		case "scaned":
			log("QR scanned, please confirm in WeChat...")
		case "expired":
			refreshCount++
			if refreshCount > 3 {
				return nil, fmt.Errorf("QR code expired multiple times, please retry")
			}
			log(fmt.Sprintf("QR expired, refreshing (%d/3)...", refreshCount))
			qr, err = client.GetBotQrcode(ctx, opts.BaseURL, opts.BotType)
			if err != nil {
				return nil, err
			}
			currentQrcode = qr.Qrcode
			log("New QR URL: " + qr.QrcodeImgContent)
		case "confirmed":
			log("Login successful!")
			baseURL := st.BaseURL
			if baseURL == "" {
				baseURL = opts.BaseURL
			}
			td := TokenData{
				Token:     st.BotToken,
				BaseURL:   baseURL,
				AccountID: st.ILinkBotID,
				UserID:    st.ILinkUserID,
				SavedAt:   time.Now().UTC().Format(time.RFC3339),
			}
			if err := SaveToken(opts.StorageDir, td); err != nil {
				return nil, err
			}
			log("Bot ID: " + td.AccountID)
			log("Token saved to " + tokenPath(opts.StorageDir))
			return &td, nil
		default:
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(1500 * time.Millisecond):
		}
	}
	return nil, fmt.Errorf("login timeout (5 minutes)")
}
