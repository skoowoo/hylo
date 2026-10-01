package wechat

import (
	"os"
	"path/filepath"
)

const (
	DefaultBaseURL = "https://ilinkai.weixin.qq.com"
	DefaultBotType = "3"
)

// DefaultStorageDir is where the long-poll sync cursor is persisted.
func DefaultStorageDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".hylo/wechat"
	}
	return filepath.Join(home, ".hylo", "wechat")
}

// TokenFromFields builds runtime credentials from config fields.
func TokenFromFields(token, baseURL, accountID, userID, savedAt string) *TokenData {
	if token == "" {
		return nil
	}
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &TokenData{
		Token:     token,
		BaseURL:   baseURL,
		AccountID: accountID,
		UserID:    userID,
		SavedAt:   savedAt,
	}
}
