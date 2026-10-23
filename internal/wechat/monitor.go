package wechat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/hardhacker/hylo/internal/config"
)

// ErrSessionExpired means WeChat unbound this bot. The token will not work again.
var ErrSessionExpired = errors.New("wechat: session expired")

const (
	defaultLongPollTimeout = 35 * time.Second
	maxConsecutiveFailures = 3
	backoffDelay           = 30 * time.Second
	retryDelay             = 2 * time.Second
	sessionExpiredErrCode  = -14
)

func syncBufPath(storageDir string) string {
	return filepath.Join(storageDir, "sync-buf.json")
}

func loadSyncBuf(storageDir string) string {
	data, err := os.ReadFile(syncBufPath(storageDir))
	if err != nil {
		return ""
	}
	var v struct {
		GetUpdatesBuf string `json:"get_updates_buf"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return ""
	}
	return v.GetUpdatesBuf
}

func saveSyncBuf(storageDir, buf string) error {
	if err := os.MkdirAll(storageDir, 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(map[string]string{"get_updates_buf": buf})
	if err != nil {
		return err
	}
	return os.WriteFile(syncBufPath(storageDir), data, 0o600)
}

// MonitorOpts configures the long-poll loop.
type MonitorOpts struct {
	BaseURL         string
	Token           string
	StorageDir      string
	LongPollTimeout time.Duration
	Client          *Client
	Log             func(string)
	OnMessage       func(Message)
}

// RunMonitor long-polls getUpdates until ctx is cancelled.
func RunMonitor(ctx context.Context, opts MonitorOpts) error {
	client := opts.Client
	if client == nil {
		client = NewClient(config.Proxy{})
	}
	log := opts.Log
	if log == nil {
		log = func(string) {}
	}
	if opts.OnMessage == nil {
		return fmt.Errorf("wechat: OnMessage is required")
	}

	syncBuf := loadSyncBuf(opts.StorageDir)
	if syncBuf != "" {
		log(fmt.Sprintf("Resuming from previous sync buf (%d bytes)", len(syncBuf)))
	} else {
		log("No previous sync buf, starting fresh")
	}

	nextTimeout := opts.LongPollTimeout
	if nextTimeout <= 0 {
		nextTimeout = defaultLongPollTimeout
	}
	consecutiveFailures := 0

	for ctx.Err() == nil {
		resp, err := client.GetUpdates(ctx, opts.BaseURL, opts.Token, syncBuf, nextTimeout+3*time.Second)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			consecutiveFailures++
			log(fmt.Sprintf("getUpdates error (%d/%d): %v", consecutiveFailures, maxConsecutiveFailures, err))
			if err := sleep(ctx, backoffOrRetry(consecutiveFailures)); err != nil {
				return nil
			}
			continue
		}

		if resp.LongPollingTimeoutMs > 0 {
			nextTimeout = time.Duration(resp.LongPollingTimeoutMs) * time.Millisecond
		}

		isAPIError := (resp.Ret != 0) || (resp.ErrCode != 0)
		if isAPIError {
			if resp.ErrCode == sessionExpiredErrCode || resp.Ret == sessionExpiredErrCode {
				log(fmt.Sprintf("Session expired (errcode %d)", sessionExpiredErrCode))
				return ErrSessionExpired
			}
			consecutiveFailures++
			log(fmt.Sprintf("getUpdates failed: ret=%d errcode=%d errmsg=%s (%d/%d)",
				resp.Ret, resp.ErrCode, resp.ErrMsg, consecutiveFailures, maxConsecutiveFailures))
			if err := sleep(ctx, backoffOrRetry(consecutiveFailures)); err != nil {
				return nil
			}
			continue
		}

		consecutiveFailures = 0
		if resp.GetUpdatesBuf != "" {
			if err := saveSyncBuf(opts.StorageDir, resp.GetUpdatesBuf); err != nil {
				log("save sync buf: " + err.Error())
			}
			syncBuf = resp.GetUpdatesBuf
		}
		for _, msg := range resp.Msgs {
			opts.OnMessage(msg)
		}
	}
	return ctx.Err()
}

func backoffOrRetry(failures int) time.Duration {
	if failures >= maxConsecutiveFailures {
		return backoffDelay
	}
	return retryDelay
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
