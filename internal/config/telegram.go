package config

// TelegramConfig configures the Telegram Bot bridge plugin.
type TelegramConfig struct {
	// Enabled must be true for the plugin to start (opt-in).
	Enabled bool `mapstructure:"enabled" json:"enabled" toml:"enabled"`

	// BotToken comes from @BotFather (sensitive). It, UserID and Username are
	// set by pairing in Settings, never typed into the config form.
	BotToken string `mapstructure:"bot_token" json:"bot_token" toml:"bot_token"`

	// UserID is the paired owner's Telegram user ID; messages from anyone else
	// are ignored. In a private chat it is also the chat ID.
	UserID string `mapstructure:"user_id" json:"user_id" toml:"user_id"`

	// Username is the bot's @username, for display.
	Username string `mapstructure:"username" json:"username" toml:"username"`

	// ProxyURL is an optional HTTP/SOCKS5 proxy for api.telegram.org.
	ProxyURL string `mapstructure:"proxy_url" json:"proxy_url" toml:"proxy_url"`
}

// TelegramConnected reports whether a bot is paired with its owner.
func (c *Config) TelegramConnected() bool {
	return c != nil && c.Plugins.Telegram.BotToken != "" && c.Plugins.Telegram.UserID != ""
}

// TelegramPairing is what pairing in Settings saves.
type TelegramPairing struct {
	BotToken, UserID, Username, ProxyURL string
}

// PersistTelegramPairing writes a pairing (or clears it, when p is zero) to
// the config file and live cfg. Pairing enables the bridge and clearing
// disables it, so a restart is all that is left to do.
func PersistTelegramPairing(writePath string, live *Config, p TelegramPairing) error {
	baseCfg, err := MergedFromOptionalFile(writePath)
	if err != nil {
		return err
	}
	baseMap, err := ConfigToMap(baseCfg)
	if err != nil {
		return err
	}
	plugs, _ := baseMap["plugins"].(map[string]any)
	if plugs == nil {
		plugs = map[string]any{}
		baseMap["plugins"] = plugs
	}
	tg, _ := plugs["telegram"].(map[string]any)
	if tg == nil {
		tg = map[string]any{}
		plugs["telegram"] = tg
	}
	paired := p.BotToken != ""
	tg["enabled"] = paired
	tg["bot_token"] = p.BotToken
	tg["user_id"] = p.UserID
	tg["username"] = p.Username
	if paired {
		tg["proxy_url"] = p.ProxyURL
	}
	if err := WriteConfigToml(writePath, baseMap); err != nil {
		return err
	}
	if live != nil {
		t := &live.Plugins.Telegram
		t.Enabled, t.BotToken, t.UserID, t.Username = paired, p.BotToken, p.UserID, p.Username
		if paired {
			t.ProxyURL = p.ProxyURL
		}
	}
	return nil
}
