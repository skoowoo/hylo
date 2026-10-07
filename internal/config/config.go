package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

// Config holds the full application configuration.
type Config struct {
	Server  ServerConfig  `mapstructure:"server" json:"server" toml:"server"`
	Log     LogConfig     `mapstructure:"log" json:"log" toml:"log"`
	Vault   VaultConfig   `mapstructure:"vault" json:"vault" toml:"vault"`
	Agent   AgentConfig   `mapstructure:"agent" json:"agent" toml:"agent"`
	Plugins PluginsConfig `mapstructure:"plugins" json:"plugins" toml:"plugins"`
}

// PluginsConfig groups all optional plugin configurations.
type PluginsConfig struct {
	Search  SearchConfig  `mapstructure:"search" json:"search" toml:"search"`
	GitSync GitSyncConfig `mapstructure:"git_sync" json:"git_sync" toml:"git_sync"`
	Compile CompileConfig `mapstructure:"compile" json:"compile" toml:"compile"`
	Wechat  WechatConfig  `mapstructure:"wechat" json:"wechat" toml:"wechat"`
	Discord DiscordConfig `mapstructure:"discord" json:"discord" toml:"discord"`
}

// SearchConfig holds settings for the full-text search plugin.
// The search plugin is always active; this section only controls its behaviour.
type SearchConfig struct {
	// UseJieba enables jieba-based Chinese (CJK) tokenisation for the
	// content_zh index field. Only enable if you need Chinese language
	// support; loading the jieba dictionaries costs ~50 MB and ~1 s at startup.
	// Defaults to false.
	UseJieba bool `mapstructure:"use_jieba" json:"use_jieba" toml:"use_jieba"`

	// ExcludePrefixes is a list of vault-absolute path prefixes that are
	// excluded from indexing. Notes whose path starts with any of these
	// prefixes are silently skipped. Useful for auto-generated directories
	// (e.g. "/_summaries") or archive folders you don't want in search results.
	// Prefixes without a leading "/" are accepted and normalised automatically.
	// An empty list (default) indexes everything.
	ExcludePrefixes []string `mapstructure:"exclude_prefixes" json:"exclude_prefixes" toml:"exclude_prefixes"`
}

// CompileConfig holds settings for the LLM-based note compilation plugin.
type CompileConfig struct {
	// Enabled must be true for the plugin to start (opt-in).
	Enabled bool `mapstructure:"enabled" json:"enabled" toml:"enabled"`
}

// DiscordConfig configures the Discord Bot bridge plugin.
type DiscordConfig struct {
	// Enabled must be true for the plugin to start (opt-in).
	Enabled bool `mapstructure:"enabled" json:"enabled" toml:"enabled"`

	// BotToken is the Discord Bot token from the Developer Portal (sensitive).
	BotToken string `mapstructure:"bot_token" json:"bot_token" toml:"bot_token"`

	// UserID is the owner's Discord user ID used for proactive DM push.
	// Obtain by enabling Developer Mode in Discord → right-click your avatar → Copy User ID.
	UserID string `mapstructure:"user_id" json:"user_id" toml:"user_id"`

	// ProxyURL is an optional HTTP/SOCKS5 proxy for routing Discord API and
	// WebSocket traffic. Useful in regions where discord.com is not directly
	// reachable. Example: "socks5://127.0.0.1:1080" or "http://127.0.0.1:7890".
	ProxyURL string `mapstructure:"proxy_url" json:"proxy_url" toml:"proxy_url"`
}

// DiscordConnected reports whether a bot token is configured.
func (c *Config) DiscordConnected() bool {
	return c != nil && c.Plugins.Discord.BotToken != ""
}

// WechatConfig configures the WeChat iLink bridge plugin.
type WechatConfig struct {
	// Enabled must be true for the plugin to start (opt-in).
	Enabled bool `mapstructure:"enabled" json:"enabled" toml:"enabled"`

	// Token is the iLink bot token obtained via QR login (sensitive).
	Token string `mapstructure:"token" json:"token" toml:"token"`

	// AccountID and UserID are set automatically after login (read-only in UI).
	AccountID string `mapstructure:"account_id" json:"account_id" toml:"account_id"`
	UserID    string `mapstructure:"user_id" json:"user_id" toml:"user_id"`
	SavedAt   string `mapstructure:"saved_at" json:"saved_at" toml:"saved_at"`

	// BaseURL is assigned on login when the API returns a host override.
	BaseURL string `mapstructure:"base_url" json:"base_url" toml:"base_url"`
}

// GitSyncConfig holds settings for the git-based vault sync plugin.
type GitSyncConfig struct {
	// Enabled must be true for the plugin to start (opt-in).
	Enabled bool `mapstructure:"enabled" json:"enabled" toml:"enabled"`

	// Remote is the git remote URL (HTTPS or SSH). Leave empty for local-only
	// commits without any remote sync.
	Remote string `mapstructure:"remote" json:"remote" toml:"remote"`

	// Branch is the branch to commit to and push/pull from.
	// Defaults to "main".
	Branch string `mapstructure:"branch" json:"branch" toml:"branch"`

	// AuthToken is a personal access token used for HTTPS remotes
	// (GitHub, GitLab, Gitea, etc.). Passed as the HTTP password with
	// username "git".
	AuthToken string `mapstructure:"auth_token" json:"auth_token" toml:"auth_token"`

	// AutoCommit enables automatic commits after vault mutations.
	// Changes are batched using the Debounce window to avoid one commit per
	// keystroke.
	AutoCommit bool `mapstructure:"auto_commit" json:"auto_commit" toml:"auto_commit"`

	// Debounce is how long to wait after the last mutation before committing.
	// Accepts Go duration strings: "10s", "1m", etc.
	Debounce string `mapstructure:"debounce" json:"debounce" toml:"debounce"`

	// SyncInterval is how often to pull from and push to the remote.
	// Accepts Go duration strings. Set to "" or "0" to disable remote sync.
	SyncInterval string `mapstructure:"sync_interval" json:"sync_interval" toml:"sync_interval"`

	// CommitMessage is the commit message. The literal {{.Time}} is replaced
	// with the current UTC timestamp.
	CommitMessage string `mapstructure:"commit_message" json:"commit_message" toml:"commit_message"`

	// InitIfMissing initialises a git repository in the vault root when none
	// exists. If false and there is no .git directory, the plugin disables
	// itself rather than modifying the vault directory.
	InitIfMissing bool `mapstructure:"init_if_missing" json:"init_if_missing" toml:"init_if_missing"`
}

type ServerConfig struct {
	// Host and Port enable the TCP listener.
	// Port = 0 means TCP is disabled.
	Host string `mapstructure:"host" json:"host" toml:"host"`
	Port int    `mapstructure:"port" json:"port" toml:"port"`

	// CertFile and KeyFile enable TLS (HTTPS) when both are provided.
	CertFile string `mapstructure:"cert_file" json:"cert_file" toml:"cert_file"`
	KeyFile  string `mapstructure:"key_file" json:"key_file" toml:"key_file"`

	// APIKey enables token-based authentication via the X-Hylo-API-Key header
	// (CLI/extensions) or a login cookie (browser/desktop).
	APIKey string `mapstructure:"api_key" json:"api_key" toml:"api_key"`

	// APIKeyFromEnv records that APIKey came from HYLO_API_KEY (never persisted).
	APIKeyFromEnv bool `mapstructure:"-" json:"-" toml:"-"`

	ReadTimeout  int `mapstructure:"read_timeout" json:"read_timeout" toml:"read_timeout"`    // seconds
	WriteTimeout int `mapstructure:"write_timeout" json:"write_timeout" toml:"write_timeout"` // seconds
}

type LogConfig struct {
	Level  string `mapstructure:"level" json:"level" toml:"level"`
	Format string `mapstructure:"format" json:"format" toml:"format"` // "json" | "text"
}

// VaultConfig holds settings for the Vault storage root.
type VaultConfig struct {
	// Path is the root directory of the Vault.
	// Supports ~ for the home directory.
	Path string `mapstructure:"path" json:"path" toml:"path"`

	// ShortsDir is the vault-relative directory for short notes.
	// Default: "_shorts".
	ShortsDir string `mapstructure:"shorts_dir" json:"shorts_dir" toml:"shorts_dir"`

	// KnowledgeDir is the vault-relative directory where generated knowledge notes
	// are stored. Must start with "_" to signal that it is a system directory.
	// Default: "_knowledge".
	KnowledgeDir string `mapstructure:"knowledge_dir" json:"knowledge_dir" toml:"knowledge_dir"`
}

// AgentConfig configures Open Design–compatible local agent CLI integration.
type AgentConfig struct {
	// UploadDir is a vault-relative directory; image paths in chat requests must
	// resolve under this folder (mirrors UPLOAD_DIR validation in open-design).
	UploadDir string `mapstructure:"upload_dir" json:"upload_dir" toml:"upload_dir"`

	// SystemPrompt is a global system prompt prepended to every agent run.
	// When a mate also has a system prompt the two are joined with a markdown
	// horizontal rule: <agent.system_prompt> + "\n\n---\n\n" + <mate.system_prompt>.
	// If only one side is non-empty, it is used as-is.
	SystemPrompt string `mapstructure:"system_prompt" json:"system_prompt" toml:"system_prompt"`

	// MCPEnabled injects Hylo's MCP endpoint into every agent CLI run.
	MCPEnabled bool `mapstructure:"mcp_enabled" json:"mcp_enabled" toml:"mcp_enabled"`
}

// EffectiveSystemPrompt returns the agent global system prompt.
// When the user has not set a custom prompt (empty string), it falls back to
// DefaultAgentSystemPrompt so that code improvements to the built-in default
// are always picked up automatically.
func (a AgentConfig) EffectiveSystemPrompt() string {
	if strings.TrimSpace(a.SystemPrompt) == "" {
		return DefaultAgentSystemPrompt
	}
	return a.SystemPrompt
}

// TCPAddr returns the host:port string for the TCP listener.
func (s ServerConfig) TCPAddr() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}

// TCPEnabled reports whether the TCP listener should be started.
func (s ServerConfig) TCPEnabled() bool { return s.Port > 0 }

// TLSEnabled reports whether HTTPS should be used.
func (s ServerConfig) TLSEnabled() bool {
	return s.CertFile != "" && s.KeyFile != ""
}

// MCPURL returns the MCP endpoint agents should connect to; ok is false when TCP is disabled.
func (s ServerConfig) MCPURL() (url string, ok bool) {
	if !s.TCPEnabled() {
		return "", false
	}
	host := s.Host
	switch host {
	case "", "0.0.0.0", "::", "[::]":
		host = "127.0.0.1"
	}
	scheme := "http"
	if s.TLSEnabled() {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s:%d/mcp", scheme, host, s.Port), true
}

// BrowserBaseURL returns the http:// or https:// origin for opening links in a browser.
// Requires the TCP listener (server.port > 0).
func (c *Config) BrowserBaseURL() (string, error) {
	return c.Server.ClientBaseURL()
}

// Load reads configuration from a file (optional) merged with built-in defaults.
// Config file path can be overridden via cfgFile; leave empty to use defaults.
// The returned configFileUsed path is absolute when viper read a concrete file on disk,
// otherwise an empty string (defaults-only bootstrap).
func Load(cfgFile string) (*Config, string, error) {
	v := viper.New()

	setDefaults(v)

	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("toml")
		v.AddConfigPath(".")
		v.AddConfigPath("$HOME/.hylo")
		v.AddConfigPath("/etc/hylo")
	}

	configFileUsed := ""
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, "", fmt.Errorf("reading config: %w", err)
		}
		// Config file is optional; proceed with built-in defaults only.
	} else {
		configFileUsed = v.ConfigFileUsed()
	}

	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, configFileUsed, fmt.Errorf("unmarshaling config: %w", err)
	}
	applyEnv(cfg)

	return cfg, configFileUsed, nil
}

// MustLoad is like Load but exits on error.
func MustLoad(cfgFile string) *Config {
	cfg, _, err := Load(cfgFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fatal: failed to load config: %v\n", err)
		os.Exit(1)
	}
	return cfg
}

// OptionalConfigMissing reports whether ReadInConfig failed only because no file exists.
func OptionalConfigMissing(err error) bool {
	if err == nil {
		return false
	}
	if _, ok := err.(viper.ConfigFileNotFoundError); ok {
		return true
	}
	return os.IsNotExist(err)
}

// MergedFromOptionalFile merges built-in defaults with absPath when the file exists (otherwise defaults only).
func MergedFromOptionalFile(absPath string) (*Config, error) {
	v := viper.New()
	setDefaults(v)
	v.SetConfigFile(absPath)
	if err := v.ReadInConfig(); err != nil && !OptionalConfigMissing(err) {
		return nil, fmt.Errorf("reading config %s: %w", absPath, err)
	}
	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}
	return cfg, nil
}

// DefaultAgentSystemPrompt is the built-in global system prompt used when no
// custom prompt has been set in the config file. Exported so the settings UI
// can display it as a placeholder without writing it to disk.
const DefaultAgentSystemPrompt = `You are operating inside Hylo, an AI-native personal note-taking system that stores notes as plain markdown files in a vault directory.

Knowledge base: compiled knowledge live under /_knowledge/. The index of all knowledge units is at /_knowledge/_index/.

Internal links: whenever you mention, cite, or reference a specific note — whether linking to it, naming it in a list, or surfacing it as a result — always use wiki-link syntax: [[stem]] or [[stem|display text]], where stem is the note filename without the .md extension. Never use relative file paths.

Personal memory: the user may have structured memory files under _memory/. At the start of each conversation, proactively read the files relevant to the task directly:
- Any question or task about the user themselves → read all 6 files
- Writing, creative work, style, or taste     → _memory/_preferences.md + _memory/_beliefs.md
- Planning, projects, or goals                → _memory/_goals.md + _memory/_state.md
- A specific person or relationship           → _memory/_people.md + _memory/_identity.md
- Casual conversation or daily check-in       → _memory/_state.md + _memory/_identity.md
Skip files that do not exist. Treat items under ## Active as current facts; treat ## Fading items with lower confidence.`

func setDefaults(v *viper.Viper) {
	v.SetDefault("server.host", "127.0.0.1")
	v.SetDefault("server.port", 54321)
	v.SetDefault("server.read_timeout", 1800)
	v.SetDefault("server.write_timeout", 1800)

	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "text")

	v.SetDefault("vault.path", "~/.hylo/root")
	v.SetDefault("vault.shorts_dir", "_shorts")
	v.SetDefault("vault.knowledge_dir", "_knowledge")

	v.SetDefault("agent.upload_dir", "_agent_uploads")
	v.SetDefault("agent.system_prompt", "")
	v.SetDefault("agent.mcp_enabled", true)

	// git_sync plugin defaults (disabled by default; user must opt in)
	v.SetDefault("plugins.git_sync.enabled", false)
	v.SetDefault("plugins.git_sync.branch", "main")
	v.SetDefault("plugins.git_sync.auto_commit", true)
	v.SetDefault("plugins.git_sync.debounce", "5m")
	v.SetDefault("plugins.git_sync.sync_interval", "24h")
	v.SetDefault("plugins.git_sync.commit_message", "hylo: sync {{.Time}}")
	v.SetDefault("plugins.git_sync.init_if_missing", true)

	// search plugin defaults (always active; configure behaviour here)
	v.SetDefault("plugins.search.use_jieba", true)
	v.SetDefault("plugins.search.exclude_prefixes", []string{"/_assets", "/_knowledge/_indexes"})

	// compile plugin defaults (enabled by default)
	v.SetDefault("plugins.compile.enabled", true)

	v.SetDefault("plugins.wechat.enabled", false)

	v.SetDefault("plugins.discord.enabled", false)
}
