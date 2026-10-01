package handler

import (
	"log/slog"
	"net/http"
	"sync"

	"github.com/hardhacker/hylo/internal/config"
	"github.com/hardhacker/hylo/internal/wechat"
)

const wechatQrcodeRefreshMax = 3

// WechatHTTP serves WeChat login and connection status APIs.
type WechatHTTP struct {
	mu               sync.Mutex
	logger           *slog.Logger
	cfg              *config.Config
	configLoadedPath string
	client           *wechat.Client

	loginMu      sync.Mutex
	loginRefresh map[string]int // qrcode -> refresh count
}

func NewWechatHTTP(logger *slog.Logger, cfg *config.Config, configLoadedPath string) *WechatHTTP {
	return &WechatHTTP{
		logger:           logger,
		cfg:              cfg,
		configLoadedPath: configLoadedPath,
		client:           wechat.NewClient(),
		loginRefresh:     make(map[string]int),
	}
}

// Status handles GET /api/wechat/status .
func (w *WechatHTTP) Status(wr http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(wr, []string{http.MethodGet})
		return
	}
	wx := w.cfg.Plugins.Wechat
	respondJSON(wr, http.StatusOK, map[string]any{
		"connected":  w.cfg.WechatConnected(),
		"enabled":    wx.Enabled,
		"account_id": wx.AccountID,
		"saved_at":   wx.SavedAt,
	})
}

// LoginStart handles POST /api/wechat/login/start .
func (w *WechatHTTP) LoginStart(wr http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(wr, []string{http.MethodPost})
		return
	}

	qr, err := wechat.StartQrcodeLogin(r.Context(), w.client, wechat.DefaultBaseURL, wechat.DefaultBotType)
	if err != nil {
		w.logger.Warn("wechat login start failed", "err", err)
		respondJSON(wr, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}

	img, err := wechat.QrcodeDisplayFromResp(qr)
	if err != nil {
		w.logger.Warn("wechat login QR render failed", "err", err)
		respondJSON(wr, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}

	w.loginMu.Lock()
	w.loginRefresh[qr.Qrcode] = 0
	w.loginMu.Unlock()

	w.logger.Info("wechat login QR started", "qrcode", qr.Qrcode)

	respondJSON(wr, http.StatusOK, map[string]any{
		"qrcode":             qr.Qrcode,
		"qrcode_img_content": qr.QrcodeImgContent,
		"qrcode_image":       img,
	})
}

// LoginStatus handles GET /api/wechat/login/status?qrcode=... .
func (w *WechatHTTP) LoginStatus(wr http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(wr, []string{http.MethodGet})
		return
	}
	qrcode := r.URL.Query().Get("qrcode")
	if qrcode == "" {
		respondJSON(wr, http.StatusBadRequest, map[string]any{"error": "missing qrcode query parameter"})
		return
	}

	w.loginMu.Lock()
	refreshCount := w.loginRefresh[qrcode]
	w.loginMu.Unlock()

	result, newRefresh, err := wechat.PollQrcodeLogin(r.Context(), w.client, wechat.DefaultBaseURL, qrcode, refreshCount, wechatQrcodeRefreshMax)
	if err != nil {
		w.loginMu.Lock()
		delete(w.loginRefresh, qrcode)
		w.loginMu.Unlock()
		respondJSON(wr, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}

	if result.Qrcode != "" && result.Qrcode != qrcode {
		w.loginMu.Lock()
		delete(w.loginRefresh, qrcode)
		w.loginRefresh[result.Qrcode] = newRefresh
		w.loginMu.Unlock()
	} else {
		w.loginMu.Lock()
		w.loginRefresh[qrcode] = newRefresh
		w.loginMu.Unlock()
	}

	out := map[string]any{"status": result.Status}
	if result.Qrcode != "" {
		out["qrcode"] = result.Qrcode
	}
	if result.QrcodeImgContent != "" {
		out["qrcode_img_content"] = result.QrcodeImgContent
	}
	if payload := wechat.QrcodeScanPayload(&wechat.QrcodeResp{
		Qrcode:           result.Qrcode,
		QrcodeImgContent: result.QrcodeImgContent,
	}); payload != "" {
		if img, err := wechat.QrcodePNGDataURL(payload, 0); err == nil {
			out["qrcode_image"] = img
		} else {
			w.logger.Warn("wechat login QR render failed", "err", err)
		}
	}

	if result.Status == "confirmed" && result.Token != nil {
		writePath, err := config.ResolveConfigWritePath(w.configLoadedPath)
		if err != nil {
			respondJSON(wr, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		w.mu.Lock()
		err = config.PersistWechatAuth(writePath, w.cfg, *result.Token)
		w.mu.Unlock()
		if err != nil {
			respondJSON(wr, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}

		w.loginMu.Lock()
		delete(w.loginRefresh, qrcode)
		if result.Qrcode != "" {
			delete(w.loginRefresh, result.Qrcode)
		}
		w.loginMu.Unlock()

		out["connected"] = true
		out["account_id"] = result.Token.AccountID
		out["saved_at"] = result.Token.SavedAt
		out["restart_required"] = true
	}

	respondJSON(wr, http.StatusOK, out)
}

// Logout handles POST /api/wechat/logout .
func (w *WechatHTTP) Logout(wr http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(wr, []string{http.MethodPost})
		return
	}

	writePath, err := config.ResolveConfigWritePath(w.configLoadedPath)
	if err != nil {
		respondJSON(wr, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	w.mu.Lock()
	err = config.PersistClearWechatAuth(writePath, w.cfg)
	w.mu.Unlock()
	if err != nil {
		respondJSON(wr, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	respondJSON(wr, http.StatusOK, map[string]any{
		"connected":        false,
		"restart_required": true,
	})
}
