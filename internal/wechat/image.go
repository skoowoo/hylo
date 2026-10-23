package wechat

import (
	"context"
	"crypto/aes"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const cdnDownload = "https://novac2c.cdn.weixin.qq.com/c2c/download"

// DecodeImage turns a CDN body into image bytes. A missing key means the body
// is already plaintext. The query string is not a key and is not logged.
func DecodeImage(item ImageItem, body []byte) ([]byte, error) {
	if len(body) == 0 {
		return nil, fmt.Errorf("wechat: empty image")
	}
	key := imageKey(item)
	if key == nil {
		return body, nil
	}
	return decryptAES128ECB(body, key)
}

func imageKey(item ImageItem) []byte {
	if k := hexKey(item.AESKey); k != nil {
		return k
	}
	if item.Media == nil {
		return nil
	}
	raw := strings.TrimSpace(item.Media.AESKey)
	if raw == "" {
		return nil
	}
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		decoded, err = base64.RawStdEncoding.DecodeString(raw)
		if err != nil {
			return nil
		}
	}
	switch len(decoded) {
	case 16:
		return decoded
	case 32:
		return hexKey(string(decoded))
	default:
		return nil
	}
}

func hexKey(s string) []byte {
	s = strings.TrimSpace(s)
	if len(s) != 32 {
		return nil
	}
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != 16 {
		return nil
	}
	return b
}

func decryptAES128ECB(ciphertext, key []byte) ([]byte, error) {
	if len(key) != 16 || len(ciphertext) == 0 || len(ciphertext)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("wechat: bad image ciphertext")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	out := make([]byte, len(ciphertext))
	for i := 0; i < len(ciphertext); i += aes.BlockSize {
		block.Decrypt(out[i:i+aes.BlockSize], ciphertext[i:i+aes.BlockSize])
	}
	if n := int(out[len(out)-1]); n > 0 && n <= aes.BlockSize && n <= len(out) {
		pad := out[len(out)-n:]
		ok := true
		for _, b := range pad {
			if int(b) != n {
				ok = false
				break
			}
		}
		if ok {
			out = out[:len(out)-n]
		}
	}
	return out, nil
}

// DownloadCDN fetches one encrypted image. The URL carries the query param, so
// errors name the status and not the address.
func (c *Client) DownloadCDN(ctx context.Context, query string) ([]byte, error) {
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("wechat: image has no download param")
	}
	base := cdnDownload
	if c.CDNBase != "" {
		base = c.CDNBase
	}
	u, err := url.Parse(base)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("encrypted_query_param", query)
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.do(req)
	if err != nil {
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		return nil, fmt.Errorf("wechat: image download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("wechat: image download HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 8<<20+1))
}
