package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const acpProtocolVersion = 1

// ACPDetectOpts mirrors detectAcpModels in apps/daemon/src/acp.ts.
type ACPDetectOpts struct {
	Bin           string
	Args          []string
	Cwd           string
	Env           []string
	Timeout       time.Duration
	ClientName    string
	ClientVersion string
	DefaultOption ModelOption
}

type acpMcpServer struct {
	Type    string   `json:"type"`
	Name    string   `json:"name"`
	Command string   `json:"command"`
	Args    []string `json:"args"`
	Env     []string `json:"env"`
}

type acpModelBundle struct {
	AvailableModels []struct {
		ModelID string `json:"modelId"`
		Name    string `json:"name"`
	} `json:"availableModels"`
	CurrentModelID string `json:"currentModelId"`
}

func normalizeACPModels(models acpModelBundle, def ModelOption) []ModelOption {
	seen := map[string]struct{}{def.ID: {}}
	out := []ModelOption{def}
	for _, m := range models.AvailableModels {
		id := strings.TrimSpace(m.ModelID)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		name := strings.TrimSpace(m.Name)
		label := id
		if name != "" && name != id {
			label = fmt.Sprintf("%s (%s)", name, id)
		}
		if id == strings.TrimSpace(models.CurrentModelID) {
			label += " • current"
		}
		out = append(out, ModelOption{ID: id, Label: label})
	}
	return out
}

// DetectACPModels runs initialize + session/new RPC to list models.
func DetectACPModels(ctx context.Context, o ACPDetectOpts) ([]ModelOption, error) {
	if o.Timeout <= 0 {
		o.Timeout = 15 * time.Second
	}
	if o.DefaultOption.ID == "" {
		o.DefaultOption = DefaultModelOption
	}
	if o.ClientName == "" {
		o.ClientName = "hylo-detect"
	}
	if o.ClientVersion == "" {
		o.ClientVersion = "agent"
	}
	cctx, cancel := context.WithTimeout(ctx, o.Timeout)
	defer cancel()

	cmd := exec.CommandContext(cctx, o.Bin, o.Args...)
	cmd.Env = o.Env
	workDir := o.Cwd
	if workDir == "" {
		workDir = "."
	}
	absCwd, err := filepath.Abs(workDir)
	if err != nil {
		return nil, err
	}
	cmd.Dir = absCwd

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	go func() { _, _ = io.Copy(io.Discard, io.LimitReader(stderr, 16000)) }()

	sendRPC := func(id float64, method string, params map[string]any) error {
		msg := map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}
		b, err := json.Marshal(msg)
		if err != nil {
			return err
		}
		b = append(b, '\n')
		_, err = stdin.Write(b)
		return err
	}

	resultCh := make(chan []ModelOption, 1)
	errCh := make(chan error, 1)

	go func() {
		scanner := bufio.NewScanner(stdout)
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, 8*1024*1024)

		expectedID := 1

		for scanner.Scan() {
			line := scanner.Bytes()
			var envelope struct {
				ID     float64         `json:"id"`
				Error  json.RawMessage `json:"error"`
				Result json.RawMessage `json:"result"`
			}
			if err := json.Unmarshal(line, &envelope); err != nil {
				continue
			}
			if envelope.Error != nil {
				errCh <- fmt.Errorf("acp rpc error: %s", string(envelope.Error))
				return
			}
			if envelope.Result == nil {
				continue
			}
			if int(envelope.ID) != expectedID {
				continue
			}
			if expectedID == 1 {
				p := map[string]any{
					"cwd":        absCwd,
					"mcpServers": []acpMcpServer{},
				}
				if err := sendRPC(2, "session/new", p); err != nil {
					errCh <- err
					return
				}
				expectedID = 2
				continue
			}
			if expectedID == 2 {
				var res struct {
					Models acpModelBundle `json:"models"`
				}
				if err := json.Unmarshal(envelope.Result, &res); err != nil {
					errCh <- err
					return
				}
				resultCh <- normalizeACPModels(res.Models, o.DefaultOption)
				return
			}
		}
		if err := scanner.Err(); err != nil {
			errCh <- err
			return
		}
		errCh <- fmt.Errorf("acp: connection closed before models")
	}()

	_ = sendRPC(1, "initialize", map[string]any{
		"protocolVersion": acpProtocolVersion,
		"clientCapabilities": map[string]any{
			"terminal": false,
		},
		"clientInfo": map[string]string{"name": o.ClientName, "version": o.ClientVersion},
	})

	select {
	case models := <-resultCh:
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		if len(models) <= 1 {
			return nil, fmt.Errorf("acp: no models from CLI")
		}
		return models, nil
	case err := <-errCh:
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return nil, err
	case <-cctx.Done():
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return nil, cctx.Err()
	}
}
