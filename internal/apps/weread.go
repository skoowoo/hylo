package apps

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// weReadGateway is WeChat Reading's one endpoint for its skill.
var weReadGateway = "https://i.weread.qq.com/api/agent/gateway"

// Gateway errors that mean the key is no good.
const (
	weReadBadKey = -2013
	weReadNoUser = -2010
)

// verifyWeRead reads the shelf with key: the cheapest call that needs the
// user, and its size names the account (the gateway has no profile).
func verifyWeRead(ctx context.Context, hc *http.Client, key, skillVersion string) (string, string, error) {
	body, _ := json.Marshal(map[string]string{"api_name": "/shelf/sync", "skill_version": skillVersion})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, weReadGateway, bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := hc.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("reach WeChat Reading: %w", err)
	}
	defer resp.Body.Close()
	var out struct {
		Errcode     int               `json:"errcode"`
		Errmsg      string            `json:"errmsg"`
		Books       []json.RawMessage `json:"books"`
		Albums      []json.RawMessage `json:"albums"`
		MP          json.RawMessage   `json:"mp"`
		UpgradeInfo json.RawMessage   `json:"upgrade_info"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", "", fmt.Errorf("WeChat Reading answered %s", resp.Status)
	}
	var upgrade string
	if u := string(out.UpgradeInfo); u != "" && u != "null" {
		upgrade = u
	}
	switch out.Errcode {
	case 0:
	case weReadBadKey, weReadNoUser:
		return "", upgrade, ErrRefused
	default:
		return "", upgrade, fmt.Errorf("WeChat Reading: %s (%d)", out.Errmsg, out.Errcode)
	}
	// The skill's own rule for "how many on the shelf": articles count as one.
	n := len(out.Books) + len(out.Albums)
	if m := string(out.MP); m != "" && m != "null" && m != "{}" {
		n++
	}
	if n == 1 {
		return "1 book on the shelf", upgrade, nil
	}
	return fmt.Sprintf("%d books on the shelf", n), upgrade, nil
}
