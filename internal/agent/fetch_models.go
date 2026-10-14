package agent

import (
	"context"
	"os/exec"
	"time"
)

func fetchPiModels(resolved string, env []string) ([]ModelOption, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, resolved, "--list-models")
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}
	m := ParsePiModels(string(out))
	if len(m) == 0 {
		return nil, nil
	}
	return m, nil
}
