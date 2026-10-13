package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// CompileResult is the outcome of asking for a note to be compiled.
type CompileResult struct {
	Status string `json:"status"` // "accepted" or "already_compiled"
	Path   string `json:"path,omitempty"`
}

// Compile calls POST /api/compile/trigger: the bot compiles the note in the
// background. A note already compiled isn't an error.
func (c *Client) Compile(pathOrName string) (*CompileResult, error) {
	resp, err := c.postJSON(c.baseURL+"/api/compile/trigger", noteReadBody(pathOrName))
	if err != nil {
		return nil, wrapConnErr(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusConflict {
		return &CompileResult{Status: "already_compiled"}, nil
	}
	if resp.StatusCode != http.StatusAccepted {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("compile: %s", statusMsg(resp.StatusCode, raw))
	}
	var out CompileResult
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("compile: decode response: %w", err)
	}
	return &out, nil
}
