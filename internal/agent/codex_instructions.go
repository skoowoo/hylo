package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
)

// Ask Codex to resolve its own config layers, including project trust and profiles.
func codexDeveloperInstructions(ctx context.Context, bin, cwd string, env []string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "app-server")
	cmd.Dir, cmd.Env = cwd, env
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", err
	}
	defer stdin.Close()
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	stopClosing := context.AfterFunc(ctx, func() {
		_ = stdin.Close()
		_ = stdout.Close()
	})
	defer stopClosing()
	if err := cmd.Start(); err != nil {
		return "", err
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 8<<20)
	call := func(id int, method string, params any, result any) error {
		if err := json.NewEncoder(stdin).Encode(map[string]any{"id": id, "method": method, "params": params}); err != nil {
			return err
		}
		for scanner.Scan() {
			var response struct {
				ID     json.RawMessage `json:"id"`
				Result json.RawMessage `json:"result"`
				Error  *struct {
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.Unmarshal(scanner.Bytes(), &response); err != nil {
				return err
			}
			if string(response.ID) != fmt.Sprint(id) {
				continue
			}
			if response.Error != nil {
				return fmt.Errorf("codex %s: %s", method, response.Error.Message)
			}
			return json.Unmarshal(response.Result, result)
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := scanner.Err(); err != nil {
			return err
		}
		return fmt.Errorf("codex exited before %s completed", method)
	}
	var initialized json.RawMessage
	if err := call(1, "initialize", map[string]any{"clientInfo": map[string]string{"name": "hylo", "version": "1"}}, &initialized); err != nil {
		return "", err
	}
	if err := json.NewEncoder(stdin).Encode(map[string]any{"method": "initialized"}); err != nil {
		return "", err
	}
	var result struct {
		Config *struct {
			Instructions string `json:"developer_instructions"`
		} `json:"config"`
	}
	if err := call(2, "config/read", map[string]any{"includeLayers": false, "cwd": cwd}, &result); err != nil {
		return "", err
	}
	if result.Config == nil {
		return "", fmt.Errorf("codex config/read returned no config")
	}
	return result.Config.Instructions, nil
}
