package handler

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hardhacker/hylo/internal/config"
	"github.com/hardhacker/hylo/internal/qrpng"
	"github.com/hardhacker/hylo/internal/telegram"
)

// A pairing code is the only thing standing between a stranger and the bot
// while it has no owner, so it is long, short-lived and single-use.
const telegramPairingTTL = 10 * time.Minute

// TelegramHTTP pairs a Telegram bot with its owner: the owner opens a
// t.me/<bot>?start=<code> link, and whoever sends that code becomes the only
// account the bridge answers.
type TelegramHTTP struct {
	mu               sync.Mutex // guards pairing and serializes config writes
	logger           *slog.Logger
	cfg              *config.Config
	configLoadedPath string
	pairing          *telegramPairing
}

type telegramPairing struct {
	client  *telegram.Client
	pair    config.TelegramPairing // UserID is filled in once the code arrives
	code    string
	expires time.Time
	offset  int64
	polling bool
}

func NewTelegramHTTP(logger *slog.Logger, cfg *config.Config, configLoadedPath string) *TelegramHTTP {
	return &TelegramHTTP{logger: logger, cfg: cfg, configLoadedPath: configLoadedPath}
}

// Status handles GET /api/telegram/status .
func (t *TelegramHTTP) Status(w http.ResponseWriter, r *http.Request) {
	tg := t.cfg.Plugins.Telegram
	respondJSON(w, http.StatusOK, map[string]any{
		"connected": t.cfg.TelegramConnected(),
		"enabled":   tg.Enabled,
		"username":  tg.Username,
	})
}

// PairStart handles POST /api/telegram/pair/start {bot_token, proxy_url}.
func (t *TelegramHTTP) PairStart(w http.ResponseWriter, r *http.Request) {
	var body struct {
		BotToken string `json:"bot_token"`
		ProxyURL string `json:"proxy_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.BotToken) == "" {
		respondJSON(w, http.StatusBadRequest, map[string]any{"error": "Paste the bot token from @BotFather"})
		return
	}
	if t.cfg.TelegramConnected() {
		respondJSON(w, http.StatusConflict, map[string]any{"error": "Telegram is already connected; disconnect it first"})
		return
	}
	token, proxy := strings.TrimSpace(body.BotToken), strings.TrimSpace(body.ProxyURL)
	client, err := telegram.NewClient(token, proxy)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	me, err := client.GetMe(ctx)
	if err != nil {
		if telegram.IsUnauthorized(err) {
			respondJSON(w, http.StatusBadRequest, map[string]any{"error": "Telegram rejected this bot token"})
			return
		}
		respondJSON(w, http.StatusBadGateway, map[string]any{"error": "Can't reach Telegram: " + err.Error() + ". Set a proxy if api.telegram.org is blocked."})
		return
	}
	if err := client.DeleteWebhook(ctx); err != nil {
		t.logger.Warn("telegram pairing: deleteWebhook failed", "err", err)
	}

	code, err := pairingCode()
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	link := "https://t.me/" + me.Username + "?start=" + code
	img, err := qrpng.DataURL(link)
	if err != nil {
		t.logger.Warn("telegram pairing: QR render failed", "err", err)
	}

	t.mu.Lock()
	t.pairing = &telegramPairing{
		client:  client,
		pair:    config.TelegramPairing{BotToken: token, Username: me.Username, ProxyURL: proxy},
		code:    code,
		expires: time.Now().Add(telegramPairingTTL),
	}
	t.mu.Unlock()

	respondJSON(w, http.StatusOK, map[string]any{
		"code":         code,
		"username":     me.Username,
		"link":         link,
		"qrcode_image": img,
	})
}

// PairStatus handles GET /api/telegram/pair/status?code=... : it polls the
// bot once for the owner's /start <code>. The lock is held only around the
// pairing state, never across a call to Telegram.
func (t *TelegramHTTP) PairStatus(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	t.mu.Lock()
	p := t.pairing
	if p == nil || subtle.ConstantTimeCompare([]byte(p.code), []byte(code)) != 1 {
		t.mu.Unlock()
		respondJSON(w, http.StatusOK, map[string]any{"status": "expired"})
		return
	}
	if time.Now().After(p.expires) {
		t.pairing = nil
		t.mu.Unlock()
		respondJSON(w, http.StatusOK, map[string]any{"status": "expired"})
		return
	}
	// One getUpdates at a time: a second poller would race on the offset.
	if p.polling {
		t.mu.Unlock()
		respondJSON(w, http.StatusOK, map[string]any{"status": "wait"})
		return
	}
	p.polling = true
	offset := p.offset
	t.mu.Unlock()

	ups, err := p.client.GetUpdates(r.Context(), offset, 0)

	t.mu.Lock()
	p.polling = false
	if t.pairing != p {
		t.mu.Unlock()
		respondJSON(w, http.StatusOK, map[string]any{"status": "expired"})
		return
	}
	if err != nil {
		t.mu.Unlock()
		if telegram.IsConflict(err) {
			respondJSON(w, http.StatusConflict, map[string]any{"error": "Another program is reading this bot's messages. If Telegram was connected before, restart the server and try again."})
			return
		}
		respondJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	var owner *telegram.Message
	for _, u := range ups {
		p.offset = u.UpdateID + 1
		if arg, ok := telegram.StartParam(u.Message); ok && owner == nil &&
			subtle.ConstantTimeCompare([]byte(arg), []byte(p.code)) == 1 {
			owner = u.Message
		}
	}
	if owner == nil {
		t.mu.Unlock()
		respondJSON(w, http.StatusOK, map[string]any{"status": "wait"})
		return
	}
	p.pair.UserID = strconv.FormatInt(owner.From.ID, 10)
	writePath, err := config.ResolveConfigWritePath(t.configLoadedPath)
	if err == nil {
		err = config.PersistTelegramPairing(writePath, t.cfg, p.pair)
	}
	if err == nil {
		t.pairing = nil
	}
	t.mu.Unlock()
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	// Confirm the updates read so far, so the bridge doesn't see /start again.
	if _, err := p.client.GetUpdates(r.Context(), p.offset, 0); err != nil {
		t.logger.Warn("telegram pairing: confirm updates failed", "err", err)
	}
	if err := p.client.SendMessage(r.Context(), owner.Chat.ID, "Connected to Hylo. Once the server restarts, your messages here reach your bots."); err != nil {
		t.logger.Warn("telegram pairing: confirmation message failed", "err", err)
	}
	t.logger.Info("telegram paired", "bot", p.pair.Username)
	respondJSON(w, http.StatusOK, map[string]any{
		"status":           "confirmed",
		"connected":        true,
		"username":         p.pair.Username,
		"restart_required": true,
	})
}

// Unpair handles POST /api/telegram/unpair .
func (t *TelegramHTTP) Unpair(w http.ResponseWriter, r *http.Request) {
	t.mu.Lock()
	defer t.mu.Unlock()
	writePath, err := config.ResolveConfigWritePath(t.configLoadedPath)
	if err == nil {
		err = config.PersistTelegramPairing(writePath, t.cfg, config.TelegramPairing{})
	}
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	t.pairing = nil
	respondJSON(w, http.StatusOK, map[string]any{"connected": false, "restart_required": true})
}

// pairingCode is 128 random bits in the alphabet Telegram allows for /start payloads.
func pairingCode() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
