package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

func SupportsInstructionAppend(agentID string) bool {
	switch agentID {
	case "pi", "claude", "codex", "opencode":
		return true
	default:
		return false
	}
}

// InstructionInjection lives until the child exits; files are private to this run.
type InstructionInjection struct {
	Args []string
	Env  map[string]string
	dir  string
}

func (i *InstructionInjection) Close() {
	if i.dir != "" {
		_ = os.RemoveAll(i.dir)
	}
}

func RedactInstructionArgs(args []string) []string {
	out := append([]string(nil), args...)
	for i, arg := range out {
		if strings.HasPrefix(arg, "developer_instructions") && strings.Contains(arg, "=") {
			out[i] = "developer_instructions=<redacted>"
		}
	}
	return out
}

func PrepareInstructions(ctx context.Context, agentID, bin, cwd, instructions string, env []string) (inj InstructionInjection, err error) {
	if err := ctx.Err(); err != nil {
		return inj, err
	}
	if !SupportsInstructionAppend(agentID) {
		return inj, fmt.Errorf("%s does not support instruction append", agentID)
	}
	defer func() {
		if err != nil {
			inj.Close()
		}
	}()
	if agentID == "codex" {
		existing, e := codexDeveloperInstructions(ctx, bin, cwd, env)
		if e != nil {
			return inj, e
		}
		if strings.TrimSpace(existing) != "" {
			instructions = existing + "\n\n" + instructions
		}
		// Use TOML encoding: Go string escapes are not all valid TOML escapes.
		value, e := toml.Marshal(map[string]string{"developer_instructions": instructions})
		if e != nil {
			return inj, e
		}
		inj.Args = []string{"-c", strings.TrimSpace(string(value))}
		return inj, nil
	}
	inj.dir, err = os.MkdirTemp("", "hylo-instructions-*")
	if err != nil {
		return inj, err
	}
	path := filepath.Join(inj.dir, "instructions.md")
	if err = os.WriteFile(path, []byte(instructions), 0600); err != nil {
		return inj, err
	}
	switch agentID {
	case "pi":
		inj.Args = []string{"--append-system-prompt", path}
	case "claude":
		inj.Args = []string{"--append-system-prompt-file", path, "--system-prompt-snapshot", "off"}
	case "opencode":
		var cfg map[string]any
		cfg, err = readOpenCodeConfig(env)
		if err != nil {
			return inj, err
		}
		var files []any
		if v, ok := cfg["instructions"]; ok {
			var valid bool
			files, valid = v.([]any)
			if !valid {
				return inj, fmt.Errorf("OPENCODE_CONFIG_CONTENT.instructions must be an array")
			}
			for _, file := range files {
				if _, ok := file.(string); !ok {
					return inj, fmt.Errorf("OPENCODE_CONFIG_CONTENT.instructions must contain file paths")
				}
			}
		}
		cfg["instructions"] = append(files, path)
		var b []byte
		b, err = json.Marshal(cfg)
		if err != nil {
			return inj, err
		}
		inj.Env = map[string]string{"OPENCODE_CONFIG_CONTENT": string(b)}
	}
	return inj, nil
}
