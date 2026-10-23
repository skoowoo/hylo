package handler

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hardhacker/hylo/internal/config"
	"github.com/hardhacker/hylo/internal/discord"
	"github.com/hardhacker/hylo/internal/mate"
	"github.com/hardhacker/hylo/internal/qrpng"
	"github.com/hardhacker/hylo/internal/telegram"
	"github.com/hardhacker/hylo/internal/wechat"
)

const telegramPairingTTL = 10 * time.Minute

// ChannelHTTP binds WeChat, Discord and Telegram contacts to one Hylo bot.
type ChannelHTTP struct {
	logger *slog.Logger
	store  *mate.Store
	proxy  config.Proxy

	mu      sync.Mutex
	pairing map[string]*telegramPairing // mate id → in-progress Telegram pairing

	loginMu      sync.Mutex
	loginRefresh map[string]int // wechat qrcode → refresh count
}

type telegramPairing struct {
	client   *telegram.Client
	mateID   string
	token    string
	username string
	botID    string
	code     string
	expires  time.Time
	offset   int64
	polling  bool
}

func NewChannelHTTP(logger *slog.Logger, store *mate.Store, proxy config.Proxy) *ChannelHTTP {
	return &ChannelHTTP{
		logger:       logger,
		store:        store,
		proxy:        proxy,
		pairing:      map[string]*telegramPairing{},
		loginRefresh: map[string]int{},
	}
}

func (h *ChannelHTTP) bot(w http.ResponseWriter, r *http.Request) (*mate.Mate, bool) {
	m, err := h.store.GetMate(r.PathValue("id"))
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return nil, false
	}
	if m == nil {
		respondJSON(w, http.StatusNotFound, map[string]any{"error": "bot not found"})
		return nil, false
	}
	return m, true
}

// List handles GET /api/mates/{id}/channels.
func (h *ChannelHTTP) List(w http.ResponseWriter, r *http.Request) {
	m, ok := h.bot(w, r)
	if !ok {
		return
	}
	id := m.ID
	list, err := h.store.ListMateChannels(id)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	out := make([]map[string]any, 0, len(list))
	for _, c := range list {
		out = append(out, channelJSON(c))
	}
	respondJSON(w, http.StatusOK, map[string]any{"channels": out})
}

func channelJSON(c mate.Channel) map[string]any {
	return map[string]any{
		"kind":        c.Kind,
		"display":     c.Display,
		"ownerUserId": c.OwnerUserID,
	}
}

// Delete handles DELETE /api/mates/{id}/channels/{kind}.
func (h *ChannelHTTP) Delete(w http.ResponseWriter, r *http.Request) {
	m, ok := h.bot(w, r)
	if !ok {
		return
	}
	id := m.ID
	kind := r.PathValue("kind")
	if !mate.ValidChannelKind(kind) {
		respondJSON(w, http.StatusBadRequest, map[string]any{"error": "unknown channel"})
		return
	}
	if err := h.store.DeleteChannel(id, kind); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if kind == mate.ChannelTelegram {
		h.mu.Lock()
		delete(h.pairing, id)
		h.mu.Unlock()
	}
	respondJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// DiscordConnect handles POST /api/mates/{id}/channels/discord.
func (h *ChannelHTTP) DiscordConnect(w http.ResponseWriter, r *http.Request) {
	m, ok := h.bot(w, r)
	if !ok {
		return
	}
	id := m.ID
	var body struct {
		BotToken string `json:"bot_token"`
		UserID   string `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}
	token := strings.TrimSpace(body.BotToken)
	owner := strings.TrimSpace(body.UserID)
	if token == "" || owner == "" {
		respondJSON(w, http.StatusBadRequest, map[string]any{"error": "Bot token and your Discord user ID are required"})
		return
	}
	botID, username, err := discord.Identify(token, h.proxy)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]any{"error": "Discord rejected this bot token"})
		return
	}
	if _, err := h.store.SaveChannel(mate.Channel{
		MateID: id, Kind: mate.ChannelDiscord, Token: token, OwnerUserID: owner,
		ExternalID: botID, Display: username,
	}); err != nil {
		h.saveError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"ok": true, "display": username})
}

// TelegramStart handles POST /api/mates/{id}/channels/telegram/pair/start.
func (h *ChannelHTTP) TelegramStart(w http.ResponseWriter, r *http.Request) {
	m, ok := h.bot(w, r)
	if !ok {
		return
	}
	id := m.ID
	var body struct {
		BotToken string `json:"bot_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.BotToken) == "" {
		respondJSON(w, http.StatusBadRequest, map[string]any{"error": "Paste the bot token from @BotFather"})
		return
	}
	token := strings.TrimSpace(body.BotToken)
	client := telegram.NewClient(token, h.proxy)
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	me, err := client.GetMe(ctx)
	if err != nil {
		if telegram.IsUnauthorized(err) {
			respondJSON(w, http.StatusBadRequest, map[string]any{"error": "Telegram rejected this bot token"})
			return
		}
		respondJSON(w, http.StatusBadGateway, map[string]any{"error": "Can't reach Telegram: " + err.Error() + ". If api.telegram.org is blocked, set the proxy under Settings → Server → Config → Proxy and restart."})
		return
	}
	if h.taken(id, mate.ChannelTelegram, token, strconv.FormatInt(me.ID, 10)) {
		h.conflict(w)
		return
	}
	if err := client.DeleteWebhook(ctx); err != nil {
		h.logger.Warn("telegram pairing: deleteWebhook failed", "err", err)
	}
	code, err := pairingCode()
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	link := "https://t.me/" + me.Username + "?start=" + code
	img, err := qrpng.DataURL(link)
	if err != nil {
		h.logger.Warn("telegram pairing: QR render failed", "err", err)
	}
	h.mu.Lock()
	h.pairing[id] = &telegramPairing{
		client: client, mateID: id, token: token, username: me.Username,
		botID: strconv.FormatInt(me.ID, 10), code: code,
		expires: time.Now().Add(telegramPairingTTL),
	}
	h.mu.Unlock()
	respondJSON(w, http.StatusOK, map[string]any{
		"code": code, "username": me.Username, "link": link, "qrcode_image": img,
	})
}

// TelegramStatus handles GET /api/mates/{id}/channels/telegram/pair/status?code=.
func (h *ChannelHTTP) TelegramStatus(w http.ResponseWriter, r *http.Request) {
	m, ok := h.bot(w, r)
	if !ok {
		return
	}
	id := m.ID
	code := r.URL.Query().Get("code")
	h.mu.Lock()
	p := h.pairing[id]
	if p == nil || subtle.ConstantTimeCompare([]byte(p.code), []byte(code)) != 1 {
		h.mu.Unlock()
		respondJSON(w, http.StatusOK, map[string]any{"status": "expired"})
		return
	}
	if time.Now().After(p.expires) {
		delete(h.pairing, id)
		h.mu.Unlock()
		respondJSON(w, http.StatusOK, map[string]any{"status": "expired"})
		return
	}
	if p.polling {
		h.mu.Unlock()
		respondJSON(w, http.StatusOK, map[string]any{"status": "wait"})
		return
	}
	p.polling = true
	offset := p.offset
	h.mu.Unlock()

	ups, err := p.client.GetUpdates(r.Context(), offset, 0)

	h.mu.Lock()
	p.polling = false
	if h.pairing[id] != p {
		h.mu.Unlock()
		respondJSON(w, http.StatusOK, map[string]any{"status": "expired"})
		return
	}
	if err != nil {
		h.mu.Unlock()
		if telegram.IsConflict(err) {
			respondJSON(w, http.StatusConflict, map[string]any{"error": "Another program is reading this bot's messages. Disconnect it there and try again."})
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
		h.mu.Unlock()
		respondJSON(w, http.StatusOK, map[string]any{"status": "wait"})
		return
	}
	ownerID := strconv.FormatInt(owner.From.ID, 10)
	chatID := owner.Chat.ID
	delete(h.pairing, id)
	h.mu.Unlock()

	// Confirm the updates already read, before the live poller starts.
	if _, err := p.client.GetUpdates(r.Context(), p.offset, 0); err != nil {
		h.logger.Warn("telegram pairing: confirm updates failed", "err", err)
	}
	name := m.Name
	if name == "" {
		name = "this bot"
	}
	if _, err := h.store.SaveChannel(mate.Channel{
		MateID: id, Kind: mate.ChannelTelegram, Token: p.token, OwnerUserID: ownerID,
		ExternalID: p.botID, Display: "@" + p.username,
	}); err != nil {
		h.saveError(w, err)
		return
	}
	if err := p.client.SendMessage(r.Context(), chatID, "Connected to "+name+". Messages here continue that bot's chat. Send /new to start a new one."); err != nil {
		h.logger.Warn("telegram pairing: confirmation failed", "err", err)
	}
	respondJSON(w, http.StatusOK, map[string]any{"status": "confirmed", "username": p.username})
}

// WechatStart handles POST /api/mates/{id}/channels/wechat/login/start.
func (h *ChannelHTTP) WechatStart(w http.ResponseWriter, r *http.Request) {
	_, ok := h.bot(w, r)
	if !ok {
		return
	}
	qr, err := wechat.StartQrcodeLogin(r.Context(), wechat.NewClient(h.proxy), wechat.DefaultBaseURL, wechat.DefaultBotType)
	if err != nil {
		h.logger.Warn("wechat login start failed", "err", err)
		respondJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	img, err := wechat.QrcodeDisplayFromResp(qr)
	if err != nil {
		respondJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	h.loginMu.Lock()
	h.loginRefresh[qr.Qrcode] = 0
	h.loginMu.Unlock()
	respondJSON(w, http.StatusOK, map[string]any{
		"qrcode": qr.Qrcode, "qrcode_img_content": qr.QrcodeImgContent, "qrcode_image": img,
	})
}

// WechatStatus handles GET /api/mates/{id}/channels/wechat/login/status?qrcode=.
func (h *ChannelHTTP) WechatStatus(w http.ResponseWriter, r *http.Request) {
	m, ok := h.bot(w, r)
	if !ok {
		return
	}
	id := m.ID
	qrcode := r.URL.Query().Get("qrcode")
	if qrcode == "" {
		respondJSON(w, http.StatusBadRequest, map[string]any{"error": "missing qrcode"})
		return
	}
	h.loginMu.Lock()
	refreshCount := h.loginRefresh[qrcode]
	h.loginMu.Unlock()

	result, newRefresh, err := wechat.PollQrcodeLogin(r.Context(), wechat.NewClient(h.proxy), wechat.DefaultBaseURL, qrcode, refreshCount, 3)
	if err != nil {
		h.loginMu.Lock()
		delete(h.loginRefresh, qrcode)
		h.loginMu.Unlock()
		respondJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	h.loginMu.Lock()
	if result.Qrcode != "" && result.Qrcode != qrcode {
		delete(h.loginRefresh, qrcode)
		h.loginRefresh[result.Qrcode] = newRefresh
	} else {
		h.loginRefresh[qrcode] = newRefresh
	}
	h.loginMu.Unlock()

	out := map[string]any{"status": result.Status}
	if result.Qrcode != "" {
		out["qrcode"] = result.Qrcode
	}
	if result.QrcodeImgContent != "" {
		out["qrcode_img_content"] = result.QrcodeImgContent
	}
	if payload := wechat.QrcodeScanPayload(&wechat.QrcodeResp{Qrcode: result.Qrcode, QrcodeImgContent: result.QrcodeImgContent}); payload != "" {
		if img, err := qrpng.DataURL(payload); err == nil {
			out["qrcode_image"] = img
		}
	}
	if result.Status == "confirmed" && result.Token != nil {
		td := result.Token
		if td.AccountID == "" || td.UserID == "" || td.Token == "" {
			respondJSON(w, http.StatusBadGateway, map[string]any{"error": "WeChat login did not return a bot"})
			return
		}
		if _, err := h.store.SaveChannel(mate.Channel{
			MateID: id, Kind: mate.ChannelWechat, Token: td.Token, OwnerUserID: td.UserID,
			ExternalID: td.AccountID, Display: td.AccountID, BaseURL: td.BaseURL,
		}); err != nil {
			h.saveError(w, err)
			return
		}
		h.loginMu.Lock()
		delete(h.loginRefresh, qrcode)
		if result.Qrcode != "" {
			delete(h.loginRefresh, result.Qrcode)
		}
		h.loginMu.Unlock()
		out["connected"] = true
		out["display"] = td.AccountID
	}
	respondJSON(w, http.StatusOK, out)
}

// taken spares a long pairing that SaveChannel would refuse at the end.
func (h *ChannelHTTP) taken(mateID, kind, token, externalID string) bool {
	list, err := h.store.ListChannels()
	if err != nil {
		return false
	}
	for _, c := range list {
		if c.Kind != kind || c.MateID == mateID {
			continue
		}
		if token != "" && c.Token == token {
			return true
		}
		if externalID != "" && c.ExternalID == externalID {
			return true
		}
	}
	return false
}

func (h *ChannelHTTP) conflict(w http.ResponseWriter) {
	respondJSON(w, http.StatusConflict, map[string]any{"error": "This app bot is already connected to another Hylo bot"})
}

func (h *ChannelHTTP) saveError(w http.ResponseWriter, err error) {
	if errors.Is(err, mate.ErrChannelTaken) {
		h.conflict(w)
		return
	}
	respondJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
}

func pairingCode() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
