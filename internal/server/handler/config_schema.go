package handler

import "github.com/hardhacker/hylo/internal/config"

// SchemaField describes one editable configuration key for GET /api/config/schema.
type SchemaField struct {
	Key         string         `json:"key"`
	Type        string         `json:"type"` // string | int | bool | duration | string_list
	Section     string         `json:"section"`
	Label       string         `json:"label"`
	Description string         `json:"description"`
	Default     any            `json:"default,omitempty"`
	Sensitive   bool           `json:"sensitive,omitempty"`
	Multiline   bool           `json:"multiline,omitempty"`
	Enum        []string       `json:"enum,omitempty"`
	Constraints map[string]int `json:"constraints,omitempty"` // min / max where applicable (e.g. port)
}

func buildConfigSchema() []SchemaField {
	return []SchemaField{
		{Key: "server.host", Type: "string", Section: "server", Label: "Bind host",
			Description: "TCP listener address (IPv4 literal or hostname)."},
		{Key: "server.port", Type: "int", Section: "server", Label: "Port",
			Description: "TCP port; use 0 to disable the HTTP listener entirely.",
			Default:     54321, Constraints: map[string]int{"min": 0, "max": 65535}},
		{Key: "server.cert_file", Type: "string", Section: "server", Label: "TLS certificate",
			Description: "Path to PEM certificate file; enable HTTPS together with key_file.",
			Default:     ""},
		{Key: "server.key_file", Type: "string", Section: "server", Label: "TLS key",
			Description: "Path to PEM private key; enable HTTPS together with cert_file.",
			Default:     ""},
		{Key: "server.api_key", Type: "string", Section: "server", Label: "API key",
			Description: "If set, every request needs this key (X-Hylo-API-Key header, or browser login). Generated automatically on first start; see `hylo auth show`.",
			Sensitive:   true, Default: ""},
		{Key: "server.read_timeout", Type: "int", Section: "server", Label: "Read timeout (s)",
			Description: "HTTP server read deadline in seconds.",
			Default:     60, Constraints: map[string]int{"min": 0}},
		{Key: "server.write_timeout", Type: "int", Section: "server", Label: "Write timeout (s)",
			Description: "HTTP server write deadline in seconds.",
			Default:     60, Constraints: map[string]int{"min": 0}},

		{Key: "vault.path", Type: "string", Section: "vault", Label: "Vault root",
			Description: "Filesystem root directory for markdown notes (~ allowed).",
			Default:     "~/.hylo/root"},

		{Key: "log.level", Type: "string", Section: "log", Label: "Log level",
			Description: "Minimum log verbosity.",
			Default:     "info",
			Enum:        []string{"debug", "info", "warn", "error"}},
		{Key: "log.format", Type: "string", Section: "log", Label: "Log format",
			Description: "Structured JSON or human-readable text.",
			Default:     "text",
			Enum:        []string{"text", "json"}},

		{Key: "agent.system_prompt", Type: "string", Section: "agent", Label: "Agent system prompt",
			Description: "Global system prompt prepended to every agent run. Leave empty to always use the latest built-in default. Joined with the mate's own system prompt (if any) via a markdown divider.",
			Multiline: true, Default: config.DefaultAgentSystemPrompt},

		{Key: "agent.mcp_enabled", Type: "bool", Section: "agent", Label: "Inject Hylo MCP",
			Description: "Give every agent CLI run access to Hylo's MCP tools. Requires the TCP listener (server.port > 0). Not supported for Hermes.",
			Default:     true},

		{Key: "plugins.search.use_jieba", Type: "bool", Section: "plugins.search", Label: "Use jieba",
			Description: "Enable Chinese tokenisation for search (~50 MB + ~1s startup).",
			Default:     false},
		{Key: "plugins.search.exclude_prefixes", Type: "string_list", Section: "plugins.search", Label: "Exclude prefixes",
			Description: "Vault-absolute path prefixes skipped by the search index.", Default: []string{}},

		{Key: "plugins.git_sync.enabled", Type: "bool", Section: "plugins.git_sync", Label: "Git sync enabled",
			Description: "When true, vault changes can commit and optionally push to remote.", Default: false},
		{Key: "plugins.git_sync.remote", Type: "string", Section: "plugins.git_sync", Label: "Remote URL",
			Description: "HTTPS remote URL; empty keeps local commits only.", Default: ""},
		{Key: "plugins.git_sync.branch", Type: "string", Section: "plugins.git_sync", Label: "Branch",
			Description: "Branch to sync.", Default: "main"},
		{Key: "plugins.git_sync.auth_token", Type: "string", Section: "plugins.git_sync", Label: "HTTPS token",
			Description: "Personal access token (PAT) used as the HTTPS password.",
			Sensitive:   true, Default: ""},
		{Key: "plugins.git_sync.auto_commit", Type: "bool", Section: "plugins.git_sync", Label: "Auto commit",
			Description: "Batch commits after edits inside the debounce window.", Default: true},
		{Key: "plugins.git_sync.debounce", Type: "duration", Section: "plugins.git_sync", Label: "Debounce",
			Description: "Quiet period before flushing a batch commit (Go duration format).",
			Default:     "5m"},
		{Key: "plugins.git_sync.sync_interval", Type: "duration", Section: "plugins.git_sync", Label: "Sync interval",
			Description: "Pull/push interval; empty or 0 disables periodic remote sync.", Default: "24h"},
{Key: "plugins.compile.enabled", Type: "bool", Section: "plugins.compile", Label: "Compile enabled",
			Description: "When true, automated LLM knowledge compilation can run.", Default: false},

		{Key: "plugins.wechat.enabled", Type: "bool", Section: "plugins.wechat", Label: "WeChat bridge enabled",
			Description: "When true, poll WeChat iLink for direct messages and emit wechat_message mate events.", Default: false},

		{Key: "plugins.discord.enabled", Type: "bool", Section: "plugins.discord", Label: "Discord bridge enabled",
			Description: "When true, connect to Discord Gateway and emit discord_message mate events for incoming DMs.", Default: false},
		{Key: "plugins.discord.bot_token", Type: "string", Section: "plugins.discord", Label: "Bot token",
			Description: "Discord Bot token from the Developer Portal. Obtain under Bot → Reset Token.",
			Sensitive: true, Default: ""},
		{Key: "plugins.discord.user_id", Type: "string", Section: "plugins.discord", Label: "Owner user ID",
			Description: "Your Discord user ID for proactive DM push. Enable Developer Mode → right-click your avatar → Copy User ID.",
			Default: ""},
		{Key: "plugins.discord.proxy_url", Type: "string", Section: "plugins.discord", Label: "Proxy URL",
			Description: "Optional HTTP/SOCKS5 proxy for Discord API and WebSocket traffic (e.g. http://127.0.0.1:7890).",
			Default: ""},
	}
}

var staticConfigSchemaFields = buildConfigSchema()
