package config

import (
	"time"

	"github.com/hardhacker/hylo/internal/wechat"
)

// ApplyWechatAuth writes WeChat login credentials into a config root map and live cfg.
func ApplyWechatAuth(root map[string]any, live *Config, td wechat.TokenData) {
	plugs, _ := root["plugins"].(map[string]any)
	if plugs == nil {
		plugs = map[string]any{}
		root["plugins"] = plugs
	}
	wx, _ := plugs["wechat"].(map[string]any)
	if wx == nil {
		wx = map[string]any{}
		plugs["wechat"] = wx
	}
	wx["token"] = td.Token
	wx["account_id"] = td.AccountID
	wx["user_id"] = td.UserID
	wx["saved_at"] = td.SavedAt
	wx["base_url"] = td.BaseURL

	if live != nil {
		live.Plugins.Wechat.Token = td.Token
		live.Plugins.Wechat.AccountID = td.AccountID
		live.Plugins.Wechat.UserID = td.UserID
		live.Plugins.Wechat.SavedAt = td.SavedAt
		live.Plugins.Wechat.BaseURL = td.BaseURL
	}
}

// ClearWechatAuth removes WeChat credentials from a config root map and live cfg.
func ClearWechatAuth(root map[string]any, live *Config) {
	plugs, _ := root["plugins"].(map[string]any)
	if plugs == nil {
		return
	}
	wx, _ := plugs["wechat"].(map[string]any)
	if wx == nil {
		return
	}
	delete(wx, "token")
	delete(wx, "account_id")
	delete(wx, "user_id")
	delete(wx, "saved_at")
	delete(wx, "base_url")

	if live != nil {
		live.Plugins.Wechat.Token = ""
		live.Plugins.Wechat.AccountID = ""
		live.Plugins.Wechat.UserID = ""
		live.Plugins.Wechat.SavedAt = ""
		live.Plugins.Wechat.BaseURL = ""
	}
}

// WechatConnected reports whether auth credentials are present.
func (c *Config) WechatConnected() bool {
	return c != nil && c.Plugins.Wechat.Token != ""
}

// WechatTokenData returns runtime token data from config, or nil.
func (c *Config) WechatTokenData() *wechat.TokenData {
	if c == nil {
		return nil
	}
	w := c.Plugins.Wechat
	return wechat.TokenFromFields(w.Token, w.BaseURL, w.AccountID, w.UserID, w.SavedAt)
}

// PersistWechatAuth merges credentials into the config file and live cfg.
func PersistWechatAuth(writePath string, live *Config, td wechat.TokenData) error {
	baseCfg, err := MergedFromOptionalFile(writePath)
	if err != nil {
		return err
	}
	baseMap, err := ConfigToMap(baseCfg)
	if err != nil {
		return err
	}
	ApplyWechatAuth(baseMap, live, td)
	return WriteConfigToml(writePath, baseMap)
}

// PersistClearWechatAuth removes credentials from the config file and live cfg.
func PersistClearWechatAuth(writePath string, live *Config) error {
	baseCfg, err := MergedFromOptionalFile(writePath)
	if err != nil {
		return err
	}
	baseMap, err := ConfigToMap(baseCfg)
	if err != nil {
		return err
	}
	ClearWechatAuth(baseMap, live)
	return WriteConfigToml(writePath, baseMap)
}

// NewWechatTokenData builds TokenData after a successful QR login.
func NewWechatTokenData(botToken, baseURL, accountID, userID string) wechat.TokenData {
	if baseURL == "" {
		baseURL = wechat.DefaultBaseURL
	}
	return wechat.TokenData{
		Token:     botToken,
		BaseURL:   baseURL,
		AccountID: accountID,
		UserID:    userID,
		SavedAt:   time.Now().UTC().Format(time.RFC3339),
	}
}
