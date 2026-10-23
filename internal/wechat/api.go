package wechat

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client talks to the WeChat iLink Bot HTTP API.
type Client struct {
	HTTP *http.Client
	// CDNBase overrides the image host. Empty means the public WeChat CDN.
	CDNBase string
}

// NewClient returns a client with sensible defaults.
func NewClient() *Client {
	return &Client{HTTP: &http.Client{Timeout: 0}} // per-request timeouts
}

func randomWechatUin() string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	u := binary.BigEndian.Uint32(b[:])
	return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%d", u)))
}

func (c *Client) buildHeaders(token string) http.Header {
	h := http.Header{}
	h.Set("Content-Type", "application/json")
	h.Set("AuthorizationType", "ilink_bot_token")
	h.Set("X-WECHAT-UIN", randomWechatUin())
	if token != "" {
		h.Set("Authorization", "Bearer "+token)
	}
	return h
}

func baseInfo() BaseInfo {
	return BaseInfo{ChannelVersion: ChannelVersion}
}

func (c *Client) apiGet(ctx context.Context, baseURL, path, token string, out any) error {
	u := strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(path, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header = c.buildHeaders(token)
	resp, err := c.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return decodeJSON(resp, out)
}

func (c *Client) apiPost(ctx context.Context, baseURL, endpoint string, body any, token string, timeout time.Duration) ([]byte, error) {
	u := strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(endpoint, "/")
	payload, err := mergeBaseInfo(body)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header = c.buildHeaders(token)
	resp, err := c.do(req)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			// Long-poll timeout: treat as empty update batch.
			return []byte(`{"ret":0,"msgs":[]}`), nil
		}
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(data))
	}
	return data, nil
}

func (c *Client) do(req *http.Request) (*http.Response, error) {
	client := c.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	return client.Do(req)
}

func mergeBaseInfo(body any) ([]byte, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	m["base_info"] = baseInfo()
	return json.Marshal(m)
}

func decodeJSON(resp *http.Response, out any) error {
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(data))
	}
	return json.Unmarshal(data, out)
}

// GetUpdates long-polls for inbound messages.
func (c *Client) GetUpdates(ctx context.Context, baseURL, token, syncBuf string, timeout time.Duration) (*GetUpdatesResp, error) {
	if timeout <= 0 {
		timeout = 38 * time.Second
	}
	data, err := c.apiPost(ctx, baseURL, "ilink/bot/getupdates", map[string]string{
		"get_updates_buf": syncBuf,
	}, token, timeout)
	if err != nil {
		return nil, err
	}
	var resp GetUpdatesResp
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// SendMessage posts an outbound message.
func (c *Client) SendMessage(ctx context.Context, baseURL, token string, req SendMessageReq) error {
	_, err := c.apiPost(ctx, baseURL, "ilink/bot/sendmessage", req, token, 15*time.Second)
	return err
}

// GetConfig fetches per-user config (e.g. typing ticket).
func (c *Client) GetConfig(ctx context.Context, baseURL, token, ilinkUserID, contextToken string) (*GetConfigResp, error) {
	body := map[string]string{"ilink_user_id": ilinkUserID}
	if contextToken != "" {
		body["context_token"] = contextToken
	}
	data, err := c.apiPost(ctx, baseURL, "ilink/bot/getconfig", body, token, 10*time.Second)
	if err != nil {
		return nil, err
	}
	var resp GetConfigResp
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// SendTyping updates the typing indicator.
func (c *Client) SendTyping(ctx context.Context, baseURL, token string, req SendTypingReq) error {
	_, err := c.apiPost(ctx, baseURL, "ilink/bot/sendtyping", req, token, 10*time.Second)
	return err
}

// GetBotQrcode starts QR login.
func (c *Client) GetBotQrcode(ctx context.Context, baseURL, botType string) (*QrcodeResp, error) {
	if botType == "" {
		botType = "3"
	}
	path := fmt.Sprintf("ilink/bot/get_bot_qrcode?bot_type=%s", url.QueryEscape(botType))
	var resp QrcodeResp
	if err := c.apiGet(ctx, baseURL, path, "", &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetQrcodeStatus polls QR login status.
func (c *Client) GetQrcodeStatus(ctx context.Context, baseURL, qrcode string) (*QrcodeStatusResp, error) {
	path := "ilink/bot/get_qrcode_status?qrcode=" + url.QueryEscape(qrcode)
	var resp QrcodeStatusResp
	if err := c.apiGet(ctx, baseURL, path, "", &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
