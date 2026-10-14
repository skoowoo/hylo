// Package agent implements Open Design–compatible local agent CLI adapters:
// discovery, argv/env construction, spawn, and stdout protocol demuxing.
package agent

// StreamFormat mirrors apps/daemon/src/agents.ts streamFormat.
type StreamFormat string

const (
	StreamClaudeJSON    StreamFormat = "claude-stream-json"
	StreamJSONEvent     StreamFormat = "json-event-stream"
	StreamPiRPC         StreamFormat = "pi-rpc"
	StreamCopilotJSON   StreamFormat = "copilot-stream-json"
	StreamPlain         StreamFormat = "plain"
)

// ModelOption is one selectable model in GET /api/agents.
type ModelOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// ReasoningOption is an optional reasoning-effort preset (Codex / Pi).
type ReasoningOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// BuildArgsContext is passed to argv builders (see agents.ts buildArgs).
type BuildArgsContext struct {
	Prompt           string
	ImagePaths       []string
	ExtraAllowedDirs []string
	Model            string
	Reasoning        string
	Cwd              string
	// SessionID is the agent-native session to resume or pre-assign.
	SessionID string
	// FirstSession is true when SessionID is host-assigned on the opening turn
	// (e.g. claude --session-id) rather than resumed (--resume).
	FirstSession bool
	// Caps holds probed CLI capability keys (e.g. "partialMessages", "addDir").
	Caps map[string]bool
}

// AgentDef describes one installed CLI adapter.
type AgentDef struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Bin              string            `json:"bin"`
	FallbackBins     []string          `json:"-"`
	VersionArgs      []string          `json:"-"`
	HelpArgs         []string          `json:"-"`
	CapabilityFlags  map[string]string `json:"-"` // flag substring -> capability key
	FallbackModels   []ModelOption     `json:"-"`
	ReasoningOptions []ReasoningOption `json:"-"`
	StreamFormat     StreamFormat      `json:"streamFormat"`
	EventParser      string            `json:"eventParser,omitempty"`
	// PromptViaStdin: write composed prompt bytes to child stdin after spawn.
	PromptViaStdin bool `json:"promptViaStdin,omitempty"`
	// MaxPromptArgBytes: when prompt must be argv, refuse oversized payloads.
	MaxPromptArgBytes int `json:"maxPromptArgBytes,omitempty"`
	// StaticEnv merged into spawn env for agents that need extra environment variables.
	StaticEnv map[string]string `json:"-"`
	// SupportsImagePaths: Pi RPC reads images from disk (see stream/pi).
	SupportsImagePaths bool `json:"supportsImagePaths,omitempty"`
	// SupportsNativeSession: CLI/protocol can continue a conversation via a
	// persisted session id (or Pi RPC session) without replaying full
	// message history in the host-composed prompt.
	SupportsNativeSession bool `json:"supportsNativeSession,omitempty"`
	build              func(*AgentDef, BuildArgsContext) []string
	listModelsArgs     []string
	listModelsTimeout  int // ms; 0 = default 5000
	listModelsParse    func(stdout string) []ModelOption
	fetchModels        func(resolvedBin string, env []string) ([]ModelOption, error)
}

// AgentInfo is returned by GET /api/agents (JSON‑safe; no function fields).
type AgentInfo struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Bin              string            `json:"bin"`
	StreamFormat     StreamFormat      `json:"streamFormat"`
	EventParser      string            `json:"eventParser,omitempty"`
	Models           []ModelOption     `json:"models"`
	Available        bool              `json:"available"`
	Path             string            `json:"path,omitempty"`
	Version          string            `json:"version,omitempty"`
	PromptViaStdin   bool              `json:"promptViaStdin,omitempty"`
	ReasoningOptions []ReasoningOption `json:"reasoningOptions,omitempty"`
	MaxPromptArgBytes int              `json:"maxPromptArgBytes,omitempty"`
	SupportsImagePaths    bool   `json:"supportsImagePaths,omitempty"`
	SupportsNativeSession bool   `json:"supportsNativeSession,omitempty"`
	CliExample            string `json:"cliExample,omitempty"`
}
