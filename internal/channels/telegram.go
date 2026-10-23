package channels

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hardhacker/hylo/internal/mate"
	"github.com/hardhacker/hylo/internal/telegram"
)

const telegramTypingEvery = 4 * time.Second

// albumWait is how long a media group is held so its photos become one turn.
var albumWait = time.Second

type telegramLink struct {
	client *telegram.Client
	log    *slog.Logger
	on     func(chatID, userID, text string, fetch imageFetch)

	// turnMu makes the album timer and the next update submit one at a time,
	// so a caption cannot start a turn before its photos are held.
	turnMu sync.Mutex
	mu     sync.Mutex
	albums map[string]*telegramAlbum
}

type telegramAlbum struct {
	chatID string
	userID string
	text   string
	ids    []string
	err    error
	timer  *time.Timer
}

func openTelegram(ch mate.Channel, log *slog.Logger, on func(chatID, userID, text string, fetch imageFetch)) (runner, error) {
	c, err := telegram.NewClient(ch.Token, ch.ProxyURL)
	if err != nil {
		return nil, err
	}
	return &telegramLink{client: c, log: log, on: on, albums: map[string]*telegramAlbum{}}, nil
}

func (t *telegramLink) Run(ctx context.Context) error {
	if err := t.client.SetMyCommands(ctx, []telegram.BotCommand{{
		Command: "new", Description: "Start a new conversation",
	}}); err != nil && ctx.Err() == nil {
		t.log.Warn("telegram: setMyCommands failed", "err", err)
	}
	return telegram.RunMonitor(ctx, telegram.MonitorOpts{
		Client: t.client,
		Log:    func(msg string) { t.log.Info("telegram: " + msg) },
		OnMessage: func(m *telegram.Message) {
			// /start is how a chat is opened, including the pairing deep link.
			if _, ok := telegram.StartParam(m); ok {
				return
			}
			t.enqueue(m)
		},
	})
}

func (t *telegramLink) enqueue(m *telegram.Message) {
	if m == nil || m.From == nil {
		return
	}
	text := strings.TrimSpace(m.Text)
	if text == "" {
		text = strings.TrimSpace(m.Caption)
	}
	file := telegramFile(m)
	chatID := strconv.FormatInt(m.Chat.ID, 10)
	userID := strconv.FormatInt(m.From.ID, 10)
	if m.MediaGroupID == "" {
		// A following text has no media_group_id. Join it only when this
		// message has no image of its own, so a later photo keeps its caption.
		t.turnMu.Lock()
		defer t.turnMu.Unlock()
		if isNewChat(text) {
			t.dropAlbums(chatID)
			t.emit(chatID, userID, text, nil, nil)
			return
		}
		if text != "" && file.id == "" && file.err == nil {
			if a := t.takeCaptionlessAlbum(chatID, text); a != nil {
				t.emit(chatID, a.userID, a.text, a.ids, a.err)
				return
			}
		}
		var ids []string
		if file.id != "" {
			ids = []string{file.id}
		}
		t.emit(chatID, userID, text, ids, file.err)
		return
	}
	t.mu.Lock()
	a := t.albums[m.MediaGroupID]
	if a == nil {
		a = &telegramAlbum{chatID: chatID, userID: userID}
		t.albums[m.MediaGroupID] = a
	}
	if a.text == "" {
		a.text = text
	}
	if file.id != "" {
		a.ids = append(a.ids, file.id)
	}
	// A refused type explains more than a size, so it wins.
	if file.err != nil && a.err != errImageType {
		a.err = file.err
	}
	if a.timer != nil {
		a.timer.Stop()
	}
	group := m.MediaGroupID
	a.timer = time.AfterFunc(albumWait, func() { t.flush(group, a) })
	t.mu.Unlock()
}

// flush sends a unless a caption or /new took it first.
func (t *telegramLink) flush(group string, a *telegramAlbum) {
	t.turnMu.Lock()
	defer t.turnMu.Unlock()
	t.mu.Lock()
	if t.albums[group] != a {
		t.mu.Unlock()
		return
	}
	delete(t.albums, group)
	t.mu.Unlock()
	t.emit(a.chatID, a.userID, a.text, a.ids, a.err)
}

func (t *telegramLink) takeCaptionlessAlbum(chatID, text string) *telegramAlbum {
	t.mu.Lock()
	defer t.mu.Unlock()
	for id, alb := range t.albums {
		if alb.chatID != chatID || alb.text != "" {
			continue
		}
		alb.timer.Stop()
		delete(t.albums, id)
		alb.text = text
		return alb
	}
	return nil
}

func (t *telegramLink) dropAlbums(chatID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for id, alb := range t.albums {
		if alb.chatID != chatID {
			continue
		}
		alb.timer.Stop()
		delete(t.albums, id)
	}
}

type tgFile struct {
	id  string
	err error
}

func telegramFile(m *telegram.Message) tgFile {
	if m == nil {
		return tgFile{}
	}
	if len(m.Photo) > 0 {
		id, big := choosePhoto(m.Photo)
		if big {
			return tgFile{err: errImageTooBig}
		}
		return tgFile{id: id}
	}
	d := m.Document
	if d == nil || d.FileID == "" || !maybeImage(d.MIMEType, d.FileName) {
		return tgFile{}
	}
	if rejectedImage(d.MIMEType, d.FileName) {
		return tgFile{err: errImageType}
	}
	if d.FileSize > maxChannelImageBytes {
		return tgFile{err: errImageTooBig}
	}
	return tgFile{id: d.FileID}
}

func choosePhoto(sizes []telegram.PhotoSize) (string, bool) {
	var under telegram.PhotoSize
	var hasUnder, known bool
	for _, s := range sizes {
		if s.FileID == "" || s.FileSize <= 0 {
			continue
		}
		known = true
		if s.FileSize <= maxChannelImageBytes && (!hasUnder || s.FileSize >= under.FileSize) {
			under = s
			hasUnder = true
		}
	}
	if hasUnder {
		return under.FileID, false
	}
	if known {
		return "", true
	}
	for i := len(sizes) - 1; i >= 0; i-- {
		if sizes[i].FileID != "" {
			return sizes[i].FileID, false
		}
	}
	return "", false
}

// emit submits one turn. err is a picture already refused before download.
func (t *telegramLink) emit(chatID, userID, text string, ids []string, err error) {
	var fetch imageFetch
	switch {
	case err != nil:
		fetch = func(context.Context, string) ([]string, string, error) { return nil, "", err }
	case len(ids) > 0:
		fetch = func(ctx context.Context, dir string) ([]string, string, error) {
			return t.fetchImages(ctx, dir, ids)
		}
	}
	t.on(chatID, userID, text, fetch)
}

func (t *telegramLink) fetchImages(ctx context.Context, dir string, ids []string) ([]string, string, error) {
	ids, notice := capImages(ids)
	var paths []string
	for _, id := range ids {
		f, err := t.client.GetFile(ctx, id)
		if err != nil || f.FilePath == "" {
			return nil, "", errImageFetch
		}
		if f.FileSize > maxChannelImageBytes {
			return nil, "", errImageTooBig
		}
		body, err := t.client.DownloadFile(ctx, f.FilePath)
		if err != nil {
			return nil, "", errImageFetch
		}
		abs, err := saveChannelImage(dir, body, "", "")
		if err != nil {
			return nil, "", err
		}
		paths = append(paths, abs)
	}
	return paths, notice, nil
}

func (t *telegramLink) SendText(ctx context.Context, chatID, text string) error {
	id, err := strconv.ParseInt(chatID, 10, 64)
	if err != nil {
		return fmt.Errorf("telegram: bad chat id %q", chatID)
	}
	return t.client.SendMessage(ctx, id, text)
}

func (t *telegramLink) Typing(ctx context.Context, chatID string) {
	id, err := strconv.ParseInt(chatID, 10, 64)
	if err != nil {
		return
	}
	keepTyping(ctx, telegramTypingEvery, func() {
		_ = t.client.SendTyping(ctx, id)
	})
}
