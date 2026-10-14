package agent

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// ProbeAgent runs resolution, version, help capability probe, and model fetch.
func ProbeAgent(ctx context.Context, def *AgentDef, configuredEnv map[string]string) AgentInfo {
	base := AgentInfo{
		ID:                 def.ID,
		Name:               def.Name,
		Bin:                def.Bin,
		StreamFormat:       def.StreamFormat,
		EventParser:        def.EventParser,
		ReasoningOptions:   def.ReasoningOptions,
		PromptViaStdin:     def.PromptViaStdin,
		MaxPromptArgBytes:  def.MaxPromptArgBytes,
		SupportsImagePaths:    def.SupportsImagePaths,
		SupportsNativeSession: def.SupportsNativeSession,
		Models:             append([]ModelOption(nil), def.FallbackModels...),
		CliExample:         CLIExample(def),
	}

	resolved := ResolveAgentExecutable(def, configuredEnv)
	if resolved == "" {
		base.Available = false
		if len(base.Models) == 0 {
			base.Models = []ModelOption{DefaultModelOption}
		}
		return base
	}
	base.Path = resolved
	base.Available = true

	envSlice := SpawnEnvForAgent(def.ID, ShellEnv(), configuredEnv)
	if len(def.StaticEnv) > 0 {
		m := envToMap(envSlice)
		for k, v := range def.StaticEnv {
			m[k] = v
		}
		envSlice = mapToEnv(m)
	}

	if len(def.VersionArgs) > 0 {
		cctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		cmd := exec.CommandContext(cctx, resolved, def.VersionArgs...)
		cmd.Env = envSlice
		out, err := cmd.Output()
		cancel()
		if err == nil {
			lines := strings.Split(strings.TrimSpace(string(out)), "\n")
			if len(lines) > 0 {
				base.Version = strings.TrimSpace(lines[0])
			}
		}
	}

	if len(def.HelpArgs) > 0 && len(def.CapabilityFlags) > 0 {
		cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		cmd := exec.CommandContext(cctx, resolved, def.HelpArgs...)
		cmd.Env = envSlice
		out, err := cmd.Output()
		cancel()
		caps := map[string]bool{}
		if err == nil {
			s := string(out)
			for flag, key := range def.CapabilityFlags {
				caps[key] = strings.Contains(s, flag)
			}
		}
		SetCapabilities(def.ID, caps)
	}

	models, err := fetchModelsForDef(ctx, def, resolved, envSlice)
	if err != nil || len(models) == 0 {
		models = append([]ModelOption(nil), def.FallbackModels...)
	}
	if len(models) == 0 {
		models = []ModelOption{DefaultModelOption}
	}
	base.Models = models

	var ids []string
	for _, m := range models {
		ids = append(ids, m.ID)
	}
	RememberLiveModels(def.ID, ids)

	return base
}

func fetchModelsForDef(ctx context.Context, def *AgentDef, resolved string, env []string) ([]ModelOption, error) {
	if def.fetchModels != nil {
		return def.fetchModels(resolved, env)
	}
	if len(def.listModelsArgs) == 0 {
		return nil, nil
	}
	to := def.listModelsTimeout
	if to <= 0 {
		to = 5000
	}
	cctx, cancel := context.WithTimeout(ctx, time.Duration(to)*time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(cctx, resolved, def.listModelsArgs...)
	cmd.Env = env
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	if def.listModelsParse == nil {
		return nil, fmt.Errorf("no listModelsParse")
	}
	parsed := def.listModelsParse(string(out))
	if len(parsed) == 0 {
		return nil, fmt.Errorf("empty parse")
	}
	return parsed, nil
}

// DetectAgents probes every built-in definition (GET /api/agents).
func DetectAgents(ctx context.Context, configuredByID map[string]map[string]string) []AgentInfo {
	if configuredByID == nil {
		configuredByID = map[string]map[string]string{}
	}
	var out []AgentInfo
	for _, d := range BuiltInAgents() {
		cfg := configuredByID[d.ID]
		if cfg == nil {
			cfg = map[string]string{}
		}
		out = append(out, ProbeAgent(ctx, d, cfg))
	}
	return out
}
