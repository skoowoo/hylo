package agent

import (
	"regexp"
	"strings"
	"sync"
)

var liveModels sync.Map // agentID -> map[string]struct{}

// RememberLiveModels caches model ids returned from GET /api/agents.
func RememberLiveModels(agentID string, ids []string) {
	m := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" {
			m[id] = struct{}{}
		}
	}
	liveModels.Store(agentID, m)
}

// IsKnownModel reports whether modelId is in the live cache or static fallback.
func IsKnownModel(def *AgentDef, modelID string) bool {
	if def == nil || modelID == "" {
		return false
	}
	if v, ok := liveModels.Load(def.ID); ok {
		if set, ok := v.(map[string]struct{}); ok {
			if _, ok := set[modelID]; ok {
				return true
			}
		}
	}
	for _, m := range def.FallbackModels {
		if m.ID == modelID {
			return true
		}
	}
	return false
}

var customModelRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/:@#-]*$`)

// SanitizeCustomModel mirrors agents.ts sanitizeCustomModel.
func SanitizeCustomModel(id string) string {
	id = strings.TrimSpace(id)
	if id == "" || len(id) > 200 {
		return ""
	}
	if !customModelRe.MatchString(id) {
		return ""
	}
	return id
}

func envToMap(env []string) map[string]string {
	m := make(map[string]string, len(env))
	for _, kv := range env {
		i := strings.IndexByte(kv, '=')
		if i <= 0 {
			continue
		}
		m[kv[:i]] = kv[i+1:]
	}
	return m
}

func mapToEnv(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k, v := range m {
		out = append(out, k+"="+v)
	}
	return out
}

func deleteKeyCI(m map[string]string, want string) {
	uw := strings.ToUpper(want)
	for k := range m {
		if strings.ToUpper(k) == uw {
			delete(m, k)
			return
		}
	}
}

func hasNonEmpty(m map[string]string, key string) bool {
	uw := strings.ToUpper(key)
	for k, v := range m {
		if strings.ToUpper(k) == uw && strings.TrimSpace(v) != "" {
			return true
		}
	}
	return false
}

// SpawnEnvForAgent merges process env with configured overrides and applies
// Claude-specific ANTHROPIC_API_KEY stripping (agents.ts spawnEnvForAgent).
func SpawnEnvForAgent(agentID string, base []string, configured map[string]string) []string {
	m := envToMap(base)
	for k, v := range expandConfiguredEnv(configured) {
		m[k] = v
	}
	if agentID == "claude" && !hasNonEmpty(m, "ANTHROPIC_BASE_URL") {
		deleteKeyCI(m, "ANTHROPIC_API_KEY")
	}
	return mapToEnv(m)
}

func expandConfiguredEnv(configured map[string]string) map[string]string {
	out := make(map[string]string)
	for k, v := range configured {
		if ev, err := expandHomePath(v); err == nil {
			out[k] = ev
		}
	}
	return out
}
